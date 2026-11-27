package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const (
	erpImageGapPageSize      = 50
	erpImageGapPriorityBurst = 3
)

var (
	errERPImageGapUnclear = errors.New("ERP 单边缺图暂缓：小图归属无法确认")
	erpImageFilenameSKU   = regexp.MustCompile("^[A-Z]{2,8}[0-9]{4,}$")
)

type ERPImageBatchReader interface {
	BatchProductsBySKU(context.Context, []string) ([]*domain.ERPProduct, error)
}

type erpImageGapSweep struct {
	after   string
	pending []string
	burst   int
}

func (q *erpImageGapSweep) takePriority() (string, bool) {
	if len(q.pending) == 0 || q.burst == erpImageGapPriorityBurst {
		q.burst = 0
		return "", false
	}
	sku := q.pending[0]
	q.pending = q.pending[1:]
	q.burst++
	return sku, true
}

func hasSingleImageGap(fields *domain.ERPProductImageFields) bool {
	return fields != nil && ((strings.TrimSpace(fields.Pic) == "") != (strings.TrimSpace(fields.PicBig) == ""))
}

func (s *ERPProductImages) discoverImageGaps(ctx context.Context, batch ERPImageBatchReader, sweep *erpImageGapSweep, logger *zap.Logger) error {
	// The scan has a separate bounded budget. Keep the ordinary maintenance
	// path alive even when the upstream batch route stalls.
	scanCtx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	skus, err := s.repo.ImageGapScanSKUs(scanCtx, sweep.after, time.Now().UTC(), erpImageGapPageSize)
	if err != nil {
		return err
	}
	if len(skus) == 0 {
		sweep.after = ""
		return nil
	}
	products, err := batch.BatchProductsBySKU(scanCtx, skus)
	if err != nil {
		return err
	}
	sweep.after = skus[len(skus)-1]
	bySKU := make(map[string]*domain.ERPProduct, len(products))
	for _, product := range products {
		if product != nil {
			key := strings.TrimSpace(firstNonEmptyString(product.SKUID, product.SKUCode))
			if _, duplicate := bySKU[key]; duplicate {
				bySKU[key] = nil // An ambiguous API response cannot schedule a write.
			} else {
				bySKU[key] = product
			}
		}
	}
	detected := 0
	for _, sku := range skus {
		product := bySKU[sku]
		if product == nil || !hasSingleImageGap(product.ImageFields) {
			continue
		}
		detected++
		pic := strings.TrimSpace(product.ImageFields.Pic)
		// Anonymous native CDN files are not suitable for automatic copying.
		// Proxy links still get an exact SKU ownership check under the lease.
		if pic == "" || s.mayConfirmSmallImage(sku, pic) {
			sweep.pending = append(sweep.pending, sku)
		}
	}
	if detected > 0 {
		logger.Info("erp_product_image_gap_scan", zap.Int("scanned", len(skus)),
			zap.Int("single_side_gaps", detected), zap.Int("queued", len(sweep.pending)),
			zap.Int("held", detected-len(sweep.pending)))
	}
	return nil
}

func (s *ERPProductImages) mayConfirmSmallImage(sku, pic string) bool {
	u, err := url.Parse(pic)
	if err != nil || !s.allowedURL(u) || len(pic) > 300 {
		return false
	}
	if erpNativeImageNamesExactSKU(u, sku) {
		return true
	}
	if _, ok := s.proxyReference(pic); ok {
		return true
	}
	_, ok := s.explicitLegacyImageID(u)
	return ok
}

func erpNativeImageNamesExactSKU(u *url.URL, sku string) bool {
	if u.Hostname() != "images-erp.sursung.com" || !strings.HasPrefix(u.Path, "/prod/erp/ItemSku/") {
		return false
	}
	query, err := url.ParseQuery(escapeERPImageQuery(u.RawQuery))
	if err != nil {
		return false
	}
	names := []string{path.Base(u.Path)}
	names = append(names, query["name"]...)
	names = append(names, query["filename"]...)
	seen := false
	sku = strings.ToUpper(sku)
	for _, filename := range names {
		for _, token := range strings.FieldsFunc(strings.ToUpper(filename), func(r rune) bool {
			return !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9')
		}) {
			if token == sku && sku != "" {
				seen = true
			} else if erpImageFilenameSKU.MatchString(token) {
				return false
			}
		}
	}
	return seen
}

func (s *ERPProductImages) explicitLegacyImageID(u *url.URL) (int64, bool) {
	base, err := url.Parse(s.signer.publicBaseURL)
	if err != nil || u.Scheme != base.Scheme || !strings.EqualFold(u.Host, base.Host) {
		return 0, false
	}
	prefix := strings.TrimRight(base.Path, "/") + ERPImageProxyPathPrefix + "/oss/"
	raw, ok := strings.CutPrefix(u.Path, prefix)
	if !ok {
		return 0, false
	}
	id, err := strconv.ParseInt(raw, 10, 64)
	return id, err == nil && id > 0
}

func (s *ERPProductImages) confirmSmallImage(ctx context.Context, sku, pic string, target repo.ERPProductImageTarget, own *domain.TaskAsset) (bool, error) {
	u, err := url.Parse(pic)
	if err != nil || !s.allowedURL(u) || len(pic) > 300 {
		return false, nil
	}
	if erpNativeImageNamesExactSKU(u, sku) {
		return true, nil
	}
	if ref, ok := s.proxyReference(pic); ok {
		asset, err := s.repo.ActiveAsset(ctx, ref.VersionID)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		if err != nil || asset == nil || target.TaskID <= 0 || asset.TaskID != target.TaskID {
			return false, err
		}
		if asset.ScopeSKUCode != nil && strings.TrimSpace(*asset.ScopeSKUCode) != "" {
			return strings.TrimSpace(*asset.ScopeSKUCode) == sku, nil
		}
		return asset.ID == target.VersionID || (own != nil && own.ID == asset.ID), nil
	}
	if id, ok := s.explicitLegacyImageID(u); ok {
		key, err := s.repo.LegacyObjectKeyForSKU(ctx, id, sku)
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return key != "", err
	}
	return false, nil
}

func (s *ERPProductImages) fillBlankLargeImage(ctx context.Context, sku, token string, current *domain.ERPProduct, target repo.ERPProductImageTarget, own *domain.TaskAsset) error {
	if current.ImageFields == nil || strings.TrimSpace(current.ImageFields.Pic) == "" ||
		strings.TrimSpace(current.ImageFields.PicBig) != "" {
		return nil
	}
	pic := strings.TrimSpace(current.ImageFields.Pic)
	confirmed, err := s.confirmSmallImage(ctx, sku, pic, target, own)
	if err != nil {
		return fmt.Errorf("ERP 单边缺图暂缓：无法核对小图归属：%w", err)
	}
	if !confirmed {
		return errERPImageGapUnclear
	}
	// Publish probes the same URL and sends only the missing slot. It compares
	// both image fields and the existing master data after the ERP write.
	if err := s.publish(ctx, sku, token, current, domain.ERPProductImageFields{Pic: pic, PicBig: pic}); err != nil {
		return fmt.Errorf("ERP 单边缺图暂缓：%w", err)
	}
	return nil
}
