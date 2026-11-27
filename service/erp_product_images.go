package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

const erpImageRenewLead = 30 * 24 * time.Hour

var errERPImageReadbackPending = errors.New("ERP 回读未确认两栏商品图")

type ERPProductImages struct {
	repo    repo.ERPProductImageRepo
	tasks   repo.TaskRepo
	signer  *ERPImageProxySigner
	bridge  ERPBridgeService
	client  *http.Client
	oss     *OSSDirectService
	ossHost string
	hosts   map[string]bool
}

func NewERPProductImages(r repo.ERPProductImageRepo, tasks repo.TaskRepo, signer *ERPImageProxySigner, bridge ERPBridgeService, oss *OSSDirectService) *ERPProductImages {
	s := &ERPProductImages{repo: r, tasks: tasks, signer: signer, bridge: bridge,
		hosts: map[string]bool{"images-erp.sursung.com": true}}
	s.addHost(signer.publicBaseURL)
	if oss != nil && oss.Enabled() {
		s.oss = oss
		s.addHost(oss.publicBucketURL())
		if bucket, err := url.Parse(oss.publicBucketURL()); err == nil {
			s.ossHost = bucket.Hostname()
		}
	}
	s.client = &http.Client{Timeout: 8 * time.Second, CheckRedirect: func(req *http.Request, previous []*http.Request) error {
		if len(previous) >= 4 || !s.allowedURL(req.URL) {
			return fmt.Errorf("ERP image redirect outside the configured image hosts")
		}
		return nil
	}}
	return s
}

func (s *ERPProductImages) addHost(raw string) {
	if parsed, err := url.Parse(raw); err == nil && parsed.Hostname() != "" {
		s.hosts[strings.ToLower(parsed.Hostname())] = true
	}
}

func (s *ERPProductImages) allowedURL(u *url.URL) bool {
	return u.Scheme == "https" && u.User == nil && (u.Port() == "" || u.Port() == "443") &&
		s.hosts[strings.ToLower(u.Hostname())]
}

func (s *ERPProductImages) proxyReference(raw string) (repo.ERPProductImageReference, bool) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return repo.ERPProductImageReference{}, false
	}
	base, err := url.Parse(s.signer.publicBaseURL)
	if err != nil || !strings.EqualFold(u.Host, base.Host) || u.Scheme != base.Scheme {
		return repo.ERPProductImageReference{}, false
	}
	path := strings.TrimRight(base.Path, "/") + ERPImageProxyPathPrefix + "/"
	id, err := strconv.ParseInt(strings.TrimPrefix(u.Path, path), 10, 64)
	if !strings.HasPrefix(u.Path, path) || err != nil || id <= 0 {
		return repo.ERPProductImageReference{}, false
	}
	result := repo.ERPProductImageReference{VersionID: id}
	if seconds, err := strconv.ParseInt(u.Query().Get("exp"), 10, 64); err == nil && seconds > 0 {
		expiry := time.Unix(seconds, 0).UTC()
		result.ExpiresAt = &expiry
	}
	return result, true
}

func (s *ERPProductImages) probe(ctx context.Context, raw string) error {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !s.allowedURL(u) {
		return fmt.Errorf("URL is not on a configured ERP image host")
	}
	// Some ERP CDN URLs contain an unescaped display filename in the query.
	// Preserve all existing percent escapes and signed parameter order.
	u.RawQuery = escapeERPImageQuery(u.RawQuery)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("invalid ERP image request")
	}
	req.Header.Set("Range", "bytes=0-31")
	response, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("ERP image request failed")
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("ERP image returned HTTP %d", response.StatusCode)
	}
	header, err := io.ReadAll(io.LimitReader(response.Body, 32))
	if err != nil || !erpImageHeader(header) {
		return fmt.Errorf("ERP image did not return supported image bytes")
	}
	return nil
}

func escapeERPImageQuery(raw string) string {
	var encoded strings.Builder
	for i := 0; i < len(raw); i++ {
		value := raw[i]
		if value <= ' ' || value >= 0x7f {
			const digits = "0123456789ABCDEF"
			encoded.WriteByte('%')
			encoded.WriteByte(digits[value>>4])
			encoded.WriteByte(digits[value&15])
		} else {
			encoded.WriteByte(value)
		}
	}
	return encoded.String()
}

func erpImageHeader(header []byte) bool {
	value := string(header)
	return strings.HasPrefix(value, "\xff\xd8\xff") || strings.HasPrefix(value, "\x89PNG\r\n\x1a\n") ||
		strings.HasPrefix(value, "GIF87a") || strings.HasPrefix(value, "GIF89a") ||
		(len(value) >= 12 && strings.HasPrefix(value, "RIFF") && value[8:12] == "WEBP")
}

func (s *ERPProductImages) current(ctx context.Context, sku string) (*domain.ERPProduct, error) {
	product, appErr := s.bridge.GetProductByID(ctx, sku)
	if appErr != nil || product == nil || product.ImageFields == nil ||
		!strings.EqualFold(strings.TrimSpace(firstNonEmptyString(product.SKUID, product.SKUCode)), sku) {
		return nil, fmt.Errorf("ERP did not return both authoritative image slots for this SKU")
	}
	return product, nil
}

func (s *ERPProductImages) references(ctx context.Context, sku string, fields domain.ERPProductImageFields, requireActive bool) ([]repo.ERPProductImageReference, error) {
	var refs []repo.ERPProductImageReference
	for _, raw := range []string{fields.Pic, fields.PicBig} {
		if ref, ok := s.proxyReference(raw); ok {
			if _, err := s.repo.ActiveAsset(ctx, ref.VersionID); err != nil {
				if requireActive || !errors.Is(err, sql.ErrNoRows) {
					return nil, err
				}
				continue
			}
			refs = append(refs, ref)
			continue
		}
		if u, err := url.Parse(strings.TrimSpace(raw)); err == nil && u.Hostname() == s.ossHost && s.allowedURL(u) {
			versions, err := s.repo.ActiveObjectVersions(ctx, sku, strings.TrimPrefix(u.Path, "/"))
			if err != nil {
				return nil, err
			}
			var expiry *time.Time
			if seconds, err := strconv.ParseInt(u.Query().Get("Expires"), 10, 64); err == nil {
				at := time.Unix(seconds, 0).UTC()
				expiry = &at
			}
			for _, version := range versions {
				refs = append(refs, repo.ERPProductImageReference{VersionID: version, ExpiresAt: expiry})
			}
		}
	}
	byVersion := make(map[int64]int, len(refs))
	unique := make([]repo.ERPProductImageReference, 0, len(refs))
	for _, ref := range refs {
		at, seen := byVersion[ref.VersionID]
		if !seen {
			byVersion[ref.VersionID] = len(unique)
			unique = append(unique, ref)
			continue
		}
		if ref.ExpiresAt != nil && (unique[at].ExpiresAt == nil || ref.ExpiresAt.Before(*unique[at].ExpiresAt)) {
			unique[at].ExpiresAt = ref.ExpiresAt
		}
	}
	return unique, nil
}

func (s *ERPProductImages) withLease(ctx context.Context, sku string, action func(context.Context, string) error) (err error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(ctx, 90*time.Second)
	defer cancel()
	token := hex.EncodeToString(id[:])
	now := time.Now().UTC()
	acquired, err := s.repo.Acquire(ctx, sku, token, now, now.Add(2*time.Minute))
	if err != nil {
		return err
	}
	if !acquired {
		return fmt.Errorf("%w：已有图片同步在执行", errERPImageReadbackPending)
	}
	defer func() {
		issue := ""
		next := time.Now().UTC().Add(min(7*24*time.Hour, s.signer.tokenTTL/4))
		if err != nil {
			issue = truncateProductManagementSyncError(err.Error())
			next = time.Now().UTC().Add(48 * time.Hour)
		}
		finishCtx, stop := context.WithTimeout(context.WithoutCancel(ctx), 4*time.Second)
		defer stop()
		err = errors.Join(err, s.repo.Finish(finishCtx, sku, token, next, issue))
	}()
	return action(ctx, token)
}

// Publish is the write boundary. A failed or ambiguous write keeps both the
// observed and proposed versions pinned until an exact pair is read back.
func (s *ERPProductImages) Publish(ctx context.Context, sku, imageURL string) error {
	return s.withLease(ctx, sku, func(ctx context.Context, token string) error {
		current, err := s.current(ctx, sku)
		if err != nil {
			return err
		}
		return s.publish(ctx, sku, token, current, domain.ERPProductImageFields{Pic: imageURL, PicBig: imageURL})
	})
}

func (s *ERPProductImages) publish(ctx context.Context, sku, token string, current *domain.ERPProduct, desired domain.ERPProductImageFields) error {
	old, err := s.references(ctx, sku, *current.ImageFields, false)
	if err != nil {
		return err
	}
	proposed, err := s.references(ctx, sku, desired, true)
	if err != nil {
		return err
	}
	for _, value := range []string{desired.Pic, desired.PicBig} {
		if err := s.probe(ctx, value); err != nil {
			return err
		}
		if desired.Pic == desired.PicBig {
			break
		}
	}
	if err := s.repo.RecordReferences(ctx, sku, token, append(old, proposed...), false); err != nil {
		return err
	}
	payload := domain.ERPProductUpsertPayload{
		ProductID: sku, SKUID: sku, SKUCode: sku, IID: current.IID,
		Name: current.Name, ProductName: current.ProductName,
		ShortName: current.ShortName, ProductShortName: current.ProductShortName,
		Pic: desired.Pic, PicBig: desired.PicBig, Operation: "product_management_image_sync", Source: "product_management",
	}
	var writeErr *domain.AppError
	if !erpImagePairMatches(desired, current.ImageFields) {
		// A transport error can follow an accepted ERP write. Confirm the actual
		// fields before deciding whether the worker should retry.
		_, writeErr = s.bridge.UpsertProduct(ctx, payload)
	}
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(attempt) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return ctx.Err()
			case <-timer.C:
			}
		}
		readback, err := s.current(ctx, sku)
		if err != nil || !erpImagePairMatches(desired, readback.ImageFields) {
			continue
		}
		payload.CostPrice = current.CostPrice
		if mismatches := productManagementERPBaseReadbackMismatches(readback, payload); len(mismatches) > 0 {
			return fmt.Errorf("ERP 商品资料保护校验失败：%s", strings.Join(mismatches, "；"))
		}
		if firstNonEmptyString(readback.ProductShortName, readback.ShortName) != firstNonEmptyString(current.ProductShortName, current.ShortName) {
			return fmt.Errorf("ERP 商品简称保护校验失败")
		}
		return s.repo.RecordReferences(ctx, sku, token, proposed, true)
	}
	if writeErr != nil {
		return fmt.Errorf("%w：ERP 写入状态 %s", errERPImageReadbackPending, writeErr.Code)
	}
	return errERPImageReadbackPending
}

func erpImagePairMatches(expected domain.ERPProductImageFields, actual *domain.ERPProductImageFields) bool {
	return actual != nil && strings.TrimSpace(expected.Pic) != "" && strings.TrimSpace(expected.PicBig) != "" &&
		erpImageSlotMatches(expected.Pic, actual.Pic) &&
		erpImageSlotMatches(expected.PicBig, actual.PicBig)
}

func erpImageSlotMatches(expected, actual string) bool {
	if u, err := url.Parse(expected); err == nil && strings.HasPrefix(u.Path, ERPImageProxyPathPrefix+"/") {
		return productManagementERPImageURLsMatch(expected, actual)
	}
	return strings.TrimSpace(expected) == strings.TrimSpace(actual)
}

func (s *ERPProductImages) ownReference(ctx context.Context, sku string, target repo.ERPProductImageTarget) (*domain.TaskAsset, error) {
	if target.TaskID <= 0 {
		return nil, nil
	}
	item, err := s.tasks.GetSKUItemBySKUCode(ctx, sku)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if item == nil || item.TaskID != target.TaskID {
		return nil, nil
	}
	var match *domain.TaskAsset
	for _, ref := range domain.NormalizeReferenceFileRefs(item.ReferenceFileRefs) {
		a, err := s.repo.FindReference(ctx, target.TaskID, sku, ref)
		if err != nil {
			return nil, err
		}
		if a == nil || !isProductManagementERPImageAsset(a) {
			continue
		}
		if match != nil && a.ID != match.ID {
			// Ambiguous task references cannot replace a product image. The
			// worker can still renew an explicitly selected version.
			return nil, nil
		}
		match = a
	}
	return match, nil
}

func (s *ERPProductImages) renewLegacyURL(ctx context.Context, sku, raw string, target repo.ERPProductImageTarget) (string, error) {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || !s.allowedURL(u) || s.oss == nil {
		return "", nil
	}
	base, _ := url.Parse(s.signer.publicBaseURL)
	legacyPrefix := strings.TrimRight(base.Path, "/") + ERPImageProxyPathPrefix + "/oss/"
	if strings.EqualFold(u.Host, base.Host) && strings.HasPrefix(u.Path, legacyPrefix) {
		id, err := strconv.ParseInt(strings.TrimPrefix(u.Path, legacyPrefix), 10, 64)
		if err != nil || id <= 0 {
			return "", fmt.Errorf("invalid legacy ERP image identifier")
		}
		key, err := s.repo.LegacyObjectKeyForSKU(ctx, id, sku)
		if err != nil {
			return "", err
		}
		expiry, _ := strconv.ParseInt(u.Query().Get("exp"), 10, 64)
		if time.Until(time.Unix(expiry, 0)) > min(erpImageRenewLead, s.signer.tokenTTL/2) &&
			s.signer.matchesCurrent(id, "legacy-oss:"+key, u.Query().Get("exp"), u.Query().Get("sig")) {
			return raw, s.probe(ctx, raw)
		}
		if renewed := s.signer.BuildLegacyImageURL(id, key); renewed != nil {
			return *renewed, nil
		}
		return "", fmt.Errorf("legacy ERP image cannot be renewed")
	}
	key := strings.TrimPrefix(u.Path, "/")
	q := u.Query()
	if u.Hostname() != s.ossHost || q.Get("Expires") == "" || q.Get("Signature") == "" ||
		!erpImageLegacyKeyBelongs(key, target) {
		return "", nil
	}
	registered, err := s.repo.HasRegisteredObject(ctx, target.TaskID, key)
	if err != nil || registered {
		return "", err
	}
	exists, err := s.oss.HeadObject(ctx, key)
	if err != nil || !exists {
		return "", fmt.Errorf("historical ERP image object is unavailable")
	}
	id, err := s.repo.FindLegacyObject(ctx, sku, key)
	if err != nil {
		return "", err
	}
	if renewed := s.signer.BuildLegacyImageURL(id, key); renewed != nil {
		return *renewed, nil
	}
	return "", fmt.Errorf("historical ERP image cannot be signed")
}

func erpImageLegacyKeyBelongs(key string, target repo.ERPProductImageTarget) bool {
	if !target.SingleSKU || target.TaskNo == "" || !strings.HasPrefix(key, "tasks/"+target.TaskNo+"/assets/") {
		return false
	}
	for _, part := range strings.Split(key, "/") {
		if part == "" || part == "." || part == ".." || strings.ContainsAny(part, "\\\x00") {
			return false
		}
	}
	key = strings.ToLower(key)
	for _, suffix := range []string{".jpg", ".jpeg", ".png", ".webp", ".gif"} {
		if strings.HasSuffix(key, suffix) {
			return true
		}
	}
	return false
}

func (s *ERPProductImages) replacement(ctx context.Context, sku, raw string, target repo.ERPProductImageTarget, own *domain.TaskAsset) (string, error) {
	if ref, ok := s.proxyReference(raw); ok {
		asset, err := s.repo.ActiveAsset(ctx, ref.VersionID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		if asset != nil && asset.TaskID == target.TaskID &&
			(asset.ID == target.VersionID || (own != nil && asset.ID == own.ID) ||
				(asset.ScopeSKUCode != nil && *asset.ScopeSKUCode == sku)) {
			u, _ := url.Parse(raw)
			if ref.ExpiresAt != nil && time.Until(*ref.ExpiresAt) > min(erpImageRenewLead, s.signer.tokenTTL/2) &&
				s.signer.verifiesCurrentKey(asset, u.Query().Get("exp"), u.Query().Get("sig")) && s.probe(ctx, raw) == nil {
				return raw, nil
			}
			if renewed := s.signer.BuildImageURL(asset); renewed != nil {
				return *renewed, nil
			}
		}
	}
	if own != nil && (strings.TrimSpace(raw) == "" || s.referencesSameFile(raw, own)) {
		if renewed := s.signer.BuildImageURL(own); renewed != nil {
			return *renewed, nil
		}
	}
	if target.VersionID > 0 {
		asset, err := s.repo.ActiveAsset(ctx, target.VersionID)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return "", err
		}
		if asset != nil && asset.TaskID == target.TaskID && s.referencesSameFile(raw, asset) {
			if signed := s.signer.BuildImageURL(asset); signed != nil {
				return *signed, nil
			}
		}
	}
	if renewed, err := s.renewLegacyURL(ctx, sku, raw, target); err != nil || renewed != "" {
		return renewed, err
	}
	if err := s.probe(ctx, raw); err != nil {
		return "", fmt.Errorf("当前图片不可读且没有可确认的同 SKU 原图：%w", err)
	}
	if u, err := url.Parse(raw); err == nil && u.Query().Has("Expires") {
		expires, _ := strconv.ParseInt(u.Query().Get("Expires"), 10, 64)
		if time.Until(time.Unix(expires, 0)) <= erpImageRenewLead {
			return "", fmt.Errorf("历史 OSS 直链即将过期，缺少可确认的同 SKU 可代理资产")
		}
	}
	return raw, nil
}

func (s *ERPProductImages) referencesSameFile(raw string, asset *domain.TaskAsset) bool {
	if ref, ok := s.proxyReference(raw); ok {
		return ref.VersionID == asset.ID
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return false
	}
	key := erpImageProxyAssetStorageKey(asset)
	if !u.IsAbs() && strings.HasPrefix(u.Path, "/v1/assets/files/") {
		return key == strings.TrimPrefix(u.Path, "/v1/assets/files/")
	}
	return s.allowedURL(u) && u.Hostname() != "images-erp.sursung.com" && strings.TrimPrefix(u.Path, "/") == key
}

func (s *ERPProductImages) Schedule(ctx context.Context, sku string) error {
	if strings.TrimSpace(sku) == "" {
		return fmt.Errorf("ERP image check needs an exact SKU")
	}
	return s.repo.Schedule(ctx, strings.TrimSpace(sku), time.Now().UTC())
}

func (s *ERPProductImages) RequestReference(ctx context.Context, sku string, taskID int64, refID string) error {
	return s.repo.RequestReference(ctx, sku, repo.ERPProductImageRequest{TaskID: taskID, RefID: strings.TrimSpace(refID)}, time.Now().UTC())
}

func (s *ERPProductImages) publishRequestedReference(ctx context.Context, sku, token string, current *domain.ERPProduct, request repo.ERPProductImageRequest) error {
	item, err := s.tasks.GetSKUItemBySKUCode(ctx, sku)
	if err != nil {
		return err
	}
	if item == nil || item.TaskID != request.TaskID {
		return fmt.Errorf("requested image does not belong to the exact SKU task")
	}
	asset, err := s.repo.FindReference(ctx, request.TaskID, sku, domain.ReferenceFileRef{RefID: request.RefID})
	if err != nil {
		return err
	}
	if !isProductManagementERPImageAsset(asset) {
		return fmt.Errorf("SKU image reference has no active picture version")
	}
	belongs := asset.ScopeSKUCode != nil && strings.TrimSpace(*asset.ScopeSKUCode) == sku
	for _, ref := range item.ReferenceFileRefs {
		belongs = belongs || ref.RefID == request.RefID || ref.CanonicalID() == request.RefID
	}
	if !belongs {
		return fmt.Errorf("requested reference is not bound to this SKU")
	}
	signed := s.signer.BuildImageURL(asset)
	if signed == nil {
		return fmt.Errorf("SKU image reference cannot be signed")
	}
	if err := s.publish(ctx, sku, token, current, domain.ERPProductImageFields{Pic: *signed, PicBig: *signed}); err != nil {
		return err
	}
	return s.repo.ClearReference(ctx, sku, request)
}

func (s *ERPProductImages) CheckNext(ctx context.Context) (string, error) {
	sku, err := s.repo.NextSKU(ctx, time.Now().UTC())
	if err != nil || sku == "" {
		return sku, err
	}
	err = s.withLease(ctx, sku, func(ctx context.Context, token string) error {
		current, err := s.current(ctx, sku)
		if err != nil {
			return err
		}
		refs, err := s.references(ctx, sku, *current.ImageFields, false)
		if err != nil {
			return err
		}
		if err := s.repo.RecordReferences(ctx, sku, token, refs, false); err != nil {
			return err
		}
		request, err := s.repo.PendingReference(ctx, sku)
		if err != nil {
			return err
		}
		if request.RefID != "" {
			return s.publishRequestedReference(ctx, sku, token, current, request)
		}
		target, err := s.repo.Target(ctx, sku)
		if err != nil {
			return err
		}
		own, err := s.ownReference(ctx, sku, target)
		if err != nil {
			return err
		}
		pic, picErr := s.replacement(ctx, sku, current.ImageFields.Pic, target, own)
		big, bigErr := s.replacement(ctx, sku, current.ImageFields.PicBig, target, own)
		if picErr != nil || bigErr != nil {
			return errors.Join(picErr, bigErr)
		}
		desired := domain.ERPProductImageFields{Pic: pic, PicBig: big}
		if erpImagePairMatches(desired, current.ImageFields) {
			return nil
		}
		return s.publish(ctx, sku, token, current, desired)
	})
	return sku, err
}

func (s *ERPProductImages) Run(ctx context.Context, logger *zap.Logger) {
	ticker := time.NewTicker(15 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			sku, err := s.CheckNext(ctx)
			if err != nil && ctx.Err() == nil {
				logger.Warn("erp_product_image_check_failed", zap.String("sku", sku), zap.Error(err))
			}
		}
	}
}
