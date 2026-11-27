package service

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"image"
	"image/color"
	drawstd "image/draw"
	_ "image/gif"
	"image/jpeg"
	_ "image/png"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"
	_ "golang.org/x/image/tiff"
	_ "golang.org/x/image/webp"

	"workflow/domain"
	"workflow/repo"
)

const (
	erpSetMaxMembers    = 64
	erpSetMemberBytes   = 64 << 20
	erpSetTotalBytes    = 256 << 20
	erpSetDecodedPixels = 24 << 20
)

var erpSetDeclaredSize = regexp.MustCompile(`([0-9]{1,3}|[一二两三四五六七八九十]{1,3})\s*(?:条|幅|张|件|片|个)\s*(?:装|/套)`)

type productManagementERPSetResources interface {
	ListResourceGroups(context.Context, domain.ResourceGroupListParams) ([]domain.TaskAssetGroup, int64, error)
	LockERPProductImageSet(context.Context, repo.Tx, domain.TaskAssetGroup) error
}

type erpSetImageStore interface {
	OpenObject(context.Context, string) (io.ReadCloser, error)
	HeadObject(context.Context, string) (bool, error)
	UploadDerivedPreviewObject(context.Context, string, string, []byte) error
}

func (s *productManagementService) erpImageSetForRecord(ctx context.Context, record *domain.ProductManagementRecord) (*domain.TaskAssetGroup, error) {
	if s.erpSetResources == nil || record.TaskID <= 0 || strings.TrimSpace(record.SKUCode) == "" {
		return nil, nil
	}
	sku := strings.TrimSpace(record.SKUCode)
	groups, total, err := s.erpSetResources.ListResourceGroups(ctx, domain.ResourceGroupListParams{
		TaskID: record.TaskID, SKUCode: sku, Page: 1, PageSize: 100,
		Access: domain.ResourceGroupAccessFilter{Global: true},
	})
	if err != nil {
		return nil, err
	}
	if total != int64(len(groups)) {
		return nil, errors.New("当前 SKU 的资源组范围无法完整核对")
	}
	for i := range groups {
		group := &groups[i]
		rev := group.FinalizedRevision
		if rev == nil {
			rev = group.WorkingRevision
		}
		if rev == nil || rev.Mode != domain.TaskAssetGroupModeSet {
			continue
		}
		if len(groups) != 1 {
			return nil, errors.New("当前 SKU 对应多个资源组，无法唯一确认套装")
		}
		if err := validateERPImageSet(record, group); err != nil {
			return nil, err
		}
		return group, nil
	}
	return nil, nil
}

func validateERPImageSet(record *domain.ProductManagementRecord, group *domain.TaskAssetGroup) error {
	sku := strings.TrimSpace(record.SKUCode)
	if group.ID <= 0 || group.TaskID != record.TaskID || group.SKUCode != sku || group.MigrationIncomplete {
		return errors.New("套装与当前 SKU 或任务归属不一致")
	}
	switch group.ScopeKind {
	case domain.TaskAssetGroupScopeSKU:
		if group.TaskSKUItemID == nil || *group.TaskSKUItemID <= 0 ||
			(record.TaskSKUItemID != nil && *record.TaskSKUItemID != *group.TaskSKUItemID) {
			return errors.New("套装未绑定当前 SKU 明细")
		}
	case domain.TaskAssetGroupScopeTask:
		if group.TaskSKUItemID != nil || record.TaskSKUItemID != nil {
			return errors.New("任务级定稿的 SKU 归属不一致")
		}
	default:
		return errors.New("套装不属于可确认的商品资源范围")
	}
	rev := group.FinalizedRevision
	if rev == nil || group.FinalizedRevisionID == nil || rev.ID != *group.FinalizedRevisionID ||
		rev.GroupID != group.ID || rev.Status != domain.TaskAssetGroupRevisionFinalized ||
		rev.Mode != domain.TaskAssetGroupModeSet || rev.FinalOutputWaived {
		return errors.New("当前 SKU 尚无完整的已定稿套装")
	}
	if len(rev.Items) < 2 || len(rev.Items) > erpSetMaxMembers {
		return fmt.Errorf("当前 SKU 的套装定稿包含 %d 个成员，无法生成可用整套图", len(rev.Items))
	}
	expected, err := erpSetSizeInName(record.ProductName)
	if err != nil {
		return err
	}
	if expected != 0 && expected != len(rev.Items) {
		return fmt.Errorf("商品名称标注 %d 件，定稿有 %d 件，已暂缓 ERP 图片同步", expected, len(rev.Items))
	}
	versions := make(map[int64]bool, len(rev.Items))
	keys := make(map[string]bool, len(rev.Items))
	for i, item := range rev.Items {
		file := item.File
		if item.ID <= 0 || item.RevisionID != rev.ID || item.TaskAssetID <= 0 ||
			file == nil || file.TaskAssetID != item.TaskAssetID || file.Availability != domain.TaskResourceFileAvailable ||
			strings.TrimSpace(file.StorageKey) == "" || versions[item.TaskAssetID] || keys[file.StorageKey] {
			return fmt.Errorf("套装第 %d 件缺少可用原图、归属证据或与其他成员重复", i+1)
		}
		if i > 0 && item.SortOrder <= rev.Items[i-1].SortOrder {
			return errors.New("套装成员顺序无法唯一确认")
		}
		versions[item.TaskAssetID] = true
		keys[file.StorageKey] = true
	}
	return nil
}

func erpSetSizeInName(name string) (int, error) {
	expected := 0
	for _, match := range erpSetDeclaredSize.FindAllStringSubmatch(name, -1) {
		n, err := strconv.Atoi(match[1])
		if err != nil {
			n = erpChineseSetCount(match[1])
		}
		if n <= 0 || (expected > 0 && expected != n) {
			return 0, errors.New("商品名称中的套装件数无法唯一确认")
		}
		expected = n
	}
	return expected, nil
}

func erpChineseSetCount(text string) int {
	digits := map[string]int{"一": 1, "二": 2, "两": 2, "三": 3, "四": 4, "五": 5, "六": 6, "七": 7, "八": 8, "九": 9}
	before, after, ten := strings.Cut(text, "十")
	if !ten {
		return digits[text]
	}
	high := 1
	if before != "" {
		high = digits[before]
	}
	low := digits[after]
	if high == 0 || (after != "" && low == 0) {
		return 0
	}
	return high*10 + low
}

func readERPSetObject(ctx context.Context, store erpSetImageStore, key string, limit int64) ([]byte, error) {
	if limit <= 0 {
		return nil, errors.New("套装原图超过合图容量")
	}
	object, err := store.OpenObject(ctx, key)
	if err != nil {
		return nil, err
	}
	defer object.Close()
	data, err := io.ReadAll(io.LimitReader(object, limit+1))
	if err != nil {
		return nil, err
	}
	if len(data) == 0 || int64(len(data)) > limit {
		return nil, errors.New("套装原图为空或超过合图容量")
	}
	return data, nil
}

func decodeERPSetMember(ctx context.Context, file *domain.TaskResourceFile, source []byte) (image.Image, error) {
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(source)); err == nil &&
		cfg.Width > 0 && cfg.Height > 0 && int64(cfg.Width)*int64(cfg.Height) <= erpSetDecodedPixels {
		pixels, _, err := image.Decode(bytes.NewReader(source))
		if err == nil {
			return pixels, nil
		}
	}
	// Use the existing bounded preview renderer for large raster or layered
	// originals. No different SKU or asset version is substituted.
	extension := normalizePreviewFileExtension(file.FileName)
	switch extension {
	case ".jpg", ".png", ".webp", ".gif", ".tiff", ".psd", ".psb":
	default:
		return nil, fmt.Errorf("不支持的套装原图格式 %q", extension)
	}
	temp, err := os.MkdirTemp("", "erp-set-member-*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(temp)
	path := filepath.Join(temp, "source"+extension)
	if err := os.WriteFile(path, source, 0600); err != nil {
		return nil, err
	}
	body, err := NewExternalAssetPreviewRenderer().Render(ctx, path,
		AssetPreviewSourceMeta{Filename: file.FileName, MimeType: file.MimeType},
		AssetPreviewRenderSpec{MaxWidth: 1600, MaxHeight: 1600, Quality: 86})
	if err != nil {
		return nil, err
	}
	result, _, err := image.Decode(bytes.NewReader(body))
	return result, err
}

func composeERPProductSet(ctx context.Context, store erpSetImageStore, items []domain.TaskAssetGroupRevisionItem) ([]byte, error) {
	n := len(items)
	if n < 2 || n > erpSetMaxMembers {
		return nil, errors.New("套装成员数量无效")
	}
	// Tall posters share one row for the common 2–6 member sets. Each
	// tile fits the original aspect ratio without cutting off any design.
	columns := min(n, 6)
	rows := (n + columns - 1) / columns
	tileWidth := min(310, 1800/columns)
	tileHeight := min(850, 1500/rows)
	const gutter = 8
	canvas := image.NewRGBA(image.Rect(0, 0, tileWidth*columns, tileHeight*rows))
	drawstd.Draw(canvas, canvas.Bounds(), image.NewUniform(color.White), image.Point{}, drawstd.Src)
	remaining := int64(erpSetTotalBytes)
	packedWidth := 0
	for i, item := range items {
		file := item.File
		if file == nil {
			return nil, fmt.Errorf("套装第 %d 件无原图", i+1)
		}
		source, err := readERPSetObject(ctx, store, file.StorageKey, min(int64(erpSetMemberBytes), remaining))
		if err != nil {
			return nil, fmt.Errorf("套装第 %d 件原图不可读：%w", i+1, err)
		}
		if file.FileSize != nil && *file.FileSize > 0 && int64(len(source)) != *file.FileSize {
			return nil, fmt.Errorf("套装第 %d 件原图大小和定稿记录不一致", i+1)
		}
		remaining -= int64(len(source))
		img, err := decodeERPSetMember(ctx, file, source)
		if err != nil || img.Bounds().Empty() {
			return nil, fmt.Errorf("套装第 %d 件不能生成可读预览", i+1)
		}
		bounds := img.Bounds()
		w, h := tileWidth-2*gutter, tileHeight-2*gutter
		if int64(w)*int64(bounds.Dy()) <= int64(h)*int64(bounds.Dx()) {
			h = max(1, w*bounds.Dy()/bounds.Dx())
		} else {
			w = max(1, h*bounds.Dx()/bounds.Dy())
		}
		x := (i%columns)*tileWidth + (tileWidth-w)/2
		if rows == 1 {
			// A tighter row keeps long, narrow banners legible in order thumbnails.
			x = packedWidth + gutter
			packedWidth += w + 2*gutter
		}
		y := (i/columns)*tileHeight + (tileHeight-h)/2
		draw.CatmullRom.Scale(canvas, image.Rect(x, y, x+w, y+h), img, bounds, drawstd.Over, nil)
	}
	var cover image.Image = canvas
	if rows == 1 {
		cover = canvas.SubImage(image.Rect(0, 0, packedWidth, tileHeight))
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, cover, &jpeg.Options{Quality: 87}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func (s *productManagementService) syncFinalizedERPImageSet(ctx context.Context, record *domain.ProductManagementRecord, group *domain.TaskAssetGroup) *domain.AppError {
	if s.imagePublications == nil || s.imageProxy == nil || !s.imageProxy.Enabled() ||
		s.ossDirect == nil || !s.ossDirect.Enabled() || s.txRunner == nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "套装图片发布服务未就绪，已暂缓同步", nil)
	}
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.erpSetResources.LockERPProductImageSet(ctx, tx, *group)
	}); err != nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "套装 SKU 归属或定稿版本校验未通过："+err.Error(), nil)
	}
	select {
	case s.erpSetRenderSlot <- struct{}{}:
	case <-ctx.Done():
		return infraAppError("wait for ERP set image render", ctx.Err())
	}
	body, err := composeERPProductSet(ctx, s.ossDirect, group.FinalizedRevision.Items)
	<-s.erpSetRenderSlot
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "套装图片未同步："+err.Error(), nil)
	}
	key := fmt.Sprintf("erp-product-images/sets/%d/%d/%d/%x.jpg", group.TaskID, group.ID,
		group.FinalizedRevision.ID, sha256.Sum256(body))
	exists, err := s.ossDirect.HeadObject(ctx, key)
	if err == nil && !exists {
		err = s.ossDirect.UploadDerivedPreviewObject(ctx, key, "image/jpeg", body)
	}
	if err != nil {
		return infraAppError("store ERP set image", err)
	}
	stored, err := readERPSetObject(ctx, s.ossDirect, key, int64(len(body)))
	if err != nil || !bytes.Equal(body, stored) {
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, "合图存储校验未通过，已暂缓同步", nil)
	}
	sku := strings.TrimSpace(record.SKUCode)
	err = s.imagePublications.withLease(ctx, sku, func(ctx context.Context, token string) error {
		current, err := s.imagePublications.current(ctx, sku)
		if err != nil {
			return err
		}
		if strings.TrimSpace(firstNonEmptyString(current.SKUID, current.SKUCode)) != sku {
			return errors.New("ERP 回读 SKU 与套装归属不一致")
		}
		count, err := erpSetSizeInName(firstNonEmptyString(current.ProductName, current.Name))
		if err != nil {
			return err
		}
		if count != 0 && count != len(group.FinalizedRevision.Items) {
			return errors.New("ERP 商品名称标注的件数与已定稿套装不一致")
		}
		// Check the current set before registering its SKU alias. The guarded
		// write repeats the check under the resource locks immediately before ERP.
		if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
			return s.erpSetResources.LockERPProductImageSet(ctx, tx, *group)
		}); err != nil {
			return err
		}
		id, err := s.imagePublications.repo.FindLegacyObject(ctx, sku, key)
		if err != nil {
			return err
		}
		url := s.imageProxy.BuildLegacyImageURL(id, key)
		if url == nil {
			return errors.New("无法签发套装图地址")
		}
		guard := func(ctx context.Context, write func(context.Context)) error {
			writeCtx, stop := context.WithTimeout(ctx, 15*time.Second)
			defer stop()
			return s.txRunner.RunInTx(writeCtx, func(tx repo.Tx) error {
				if err := s.erpSetResources.LockERPProductImageSet(writeCtx, tx, *group); err != nil {
					return err
				}
				write(writeCtx)
				return nil
			})
		}
		return s.imagePublications.publishGuarded(ctx, sku, token, current, domain.ERPProductImageFields{Pic: *url, PicBig: *url}, guard)
	})
	if err != nil {
		message := "套装图片未同步：" + err.Error()
		if errors.Is(err, errERPImageReadbackPending) {
			message = "ERP 图片回读校验未通过：" + err.Error()
		}
		return domain.NewAppError(domain.ErrCodeInvalidStateTransition, message, nil)
	}
	return nil
}

// Routine renewals and manual single-image calls must not replace a set image
// with an individual member. A confirmed set alias may renew its signature.
func (s *ERPProductImages) preventSingleERPSetImage(ctx context.Context, sku string, current, desired domain.ERPProductImageFields) error {
	scope, ok := s.repo.(interface {
		RequiresERPImageSet(context.Context, string) (bool, error)
	})
	if !ok {
		return nil
	}
	required, err := scope.RequiresERPImageSet(ctx, sku)
	if err != nil || !required {
		return err
	}
	alias := func(raw string) (int64, bool) {
		u, err := url.Parse(strings.TrimSpace(raw))
		if err != nil {
			return 0, false
		}
		return s.explicitLegacyImageID(u)
	}
	small, smallOK := alias(desired.Pic)
	large, largeOK := alias(desired.PicBig)
	var alreadyPublished bool
	if smallOK && largeOK && small == large {
		alreadyPublished = true
		for _, slot := range []string{current.Pic, current.PicBig} {
			if strings.TrimSpace(slot) == "" {
				continue
			}
			if id, ok := alias(slot); !ok || id != small {
				alreadyPublished = false
				break
			}
		}
	}
	if alreadyPublished && (strings.TrimSpace(current.Pic) != "" || strings.TrimSpace(current.PicBig) != "") {
		key, err := s.repo.LegacyObjectKeyForSKU(ctx, small, sku)
		if err != nil {
			return err
		}
		if strings.HasPrefix(key, "erp-product-images/sets/") {
			return nil
		}
	}
	return errors.New("套装必须同步整套商品图，已保留 ERP 原图")
}
