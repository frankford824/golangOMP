package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math/big"
	"sort"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	erpCostFeedDefaultLimit = 1000
	erpCostFeedMaxLimit     = 5000
	erpCostBatchMaxItems    = 2000
	erpHistoryUpstreamBatch = 50
	erpCostCursorVersion    = 1
)

type JSTHistoryCostProvider interface {
	QueryHistoryCosts(ctx context.Context, query domain.JSTHistoryCostQuery) (*domain.JSTHistoryCostResponse, error)
}

type ERPCostAPIService interface {
	Feed(ctx context.Context, updatedSince, cursor string, limit int) (*domain.ERPCostFeedResult, *domain.AppError)
	BatchQuery(ctx context.Context, skuIDs []string) (*domain.ERPBatchCostResult, *domain.AppError)
	History(ctx context.Context, skuIDs []string, asOf string, wmsCoIDs []int64) (*domain.ERPHistoryCostResult, *domain.AppError)
	Changes(ctx context.Context, since, cursor string, limit int) (*domain.ERPCostChangesResult, *domain.AppError)
}

type erpCostAPIService struct {
	repo            repo.ERPCostReadRepo
	historyProvider JSTHistoryCostProvider
	cursorSecret    []byte
}

func NewERPCostAPIService(readRepo repo.ERPCostReadRepo, historyProvider JSTHistoryCostProvider, cursorSecret string) ERPCostAPIService {
	return &erpCostAPIService{
		repo:            readRepo,
		historyProvider: historyProvider,
		cursorSecret:    []byte(strings.TrimSpace(cursorSecret)),
	}
}

type erpCostFeedCursor struct {
	Version        int    `json:"v"`
	Kind           string `json:"kind"`
	UpdatedSince   string `json:"updated_since"`
	Watermark      string `json:"watermark"`
	LastModifiedAt string `json:"last_modified_at"`
	LastSKUID      string `json:"last_sku_id"`
}

type erpCostChangeCursor struct {
	Version      int    `json:"v"`
	Kind         string `json:"kind"`
	ChangedSince string `json:"changed_since"`
	WatermarkID  int64  `json:"watermark_id"`
	LastID       int64  `json:"last_id"`
}

func (s *erpCostAPIService) Feed(ctx context.Context, updatedSinceRaw, cursorRaw string, limit int) (*domain.ERPCostFeedResult, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "ERP cost read repository is not configured", nil)
	}
	limit = normalizeERPCostPageLimit(limit)
	updatedSince, appErr := parseERPCostTimestamp(updatedSinceRaw, false)
	if appErr != nil {
		return nil, appErr
	}
	watermark := time.Time{}
	lastModifiedAt := updatedSince
	lastSKUID := ""
	if strings.TrimSpace(cursorRaw) != "" {
		var cursor erpCostFeedCursor
		if err := s.decodeCursor(cursorRaw, &cursor); err != nil || cursor.Version != erpCostCursorVersion || cursor.Kind != "feed" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost feed cursor", nil)
		}
		cursorSince, err := time.Parse(time.RFC3339Nano, cursor.UpdatedSince)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost feed cursor timestamp", nil)
		}
		if strings.TrimSpace(updatedSinceRaw) != "" && !updatedSince.Equal(cursorSince) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "updated_since does not match cursor snapshot", nil)
		}
		updatedSince = cursorSince
		watermark, err = time.Parse(time.RFC3339Nano, cursor.Watermark)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost feed cursor watermark", nil)
		}
		lastModifiedAt, err = time.Parse(time.RFC3339Nano, cursor.LastModifiedAt)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost feed cursor position", nil)
		}
		lastSKUID = cursor.LastSKUID
	} else {
		var err error
		watermark, err = s.repo.InventoryWatermark(ctx)
		if err != nil {
			return nil, infraAppError("read ERP cost feed watermark", err)
		}
	}
	items, err := s.repo.ListInventoryCosts(ctx, repo.ERPCostFeedPageQuery{
		UpdatedSince:   updatedSince,
		Watermark:      watermark,
		LastModifiedAt: lastModifiedAt,
		LastSKUID:      lastSKUID,
		Limit:          limit + 1,
	})
	if err != nil {
		return nil, infraAppError("read ERP cost feed", err)
	}
	if err := normalizeERPCostSKUs(items); err != nil {
		return nil, infraAppError("normalize ERP cost feed precision", err)
	}
	result := &domain.ERPCostFeedResult{
		Data:            items,
		Watermark:       watermark,
		SnapshotVersion: snapshotVersion("jst_inventory", updatedSince.Format(time.RFC3339Nano), watermark.Format(time.RFC3339Nano)),
	}
	if len(items) > limit {
		result.Data = items[:limit]
		last := result.Data[len(result.Data)-1]
		cursor, err := s.encodeCursor(erpCostFeedCursor{
			Version:        erpCostCursorVersion,
			Kind:           "feed",
			UpdatedSince:   updatedSince.Format(time.RFC3339Nano),
			Watermark:      watermark.Format(time.RFC3339Nano),
			LastModifiedAt: last.ModifiedAt.Format(time.RFC3339Nano),
			LastSKUID:      last.SKUID,
		})
		if err != nil {
			return nil, infraAppError("encode ERP cost feed cursor", err)
		}
		result.NextCursor = cursor
	}
	return result, nil
}

func (s *erpCostAPIService) BatchQuery(ctx context.Context, rawSKUIDs []string) (*domain.ERPBatchCostResult, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "ERP cost read repository is not configured", nil)
	}
	skuIDs, appErr := normalizeERPCostSKUIDs(rawSKUIDs, erpCostBatchMaxItems)
	if appErr != nil {
		return nil, appErr
	}
	items, watermark, err := s.repo.BatchInventoryCosts(ctx, skuIDs)
	if err != nil {
		return nil, infraAppError("batch query ERP costs", err)
	}
	if err := normalizeERPCostSKUs(items); err != nil {
		return nil, infraAppError("normalize ERP batch cost precision", err)
	}
	bySKU := make(map[string]domain.ERPCostSKU, len(items))
	for _, item := range items {
		bySKU[erpCostSKUKey(item.SKUID)] = item
	}
	ordered := make([]domain.ERPCostSKU, 0, len(items))
	missing := make([]string, 0)
	for _, skuID := range skuIDs {
		if item, ok := bySKU[erpCostSKUKey(skuID)]; ok {
			ordered = append(ordered, item)
		} else {
			missing = append(missing, skuID)
		}
	}
	return &domain.ERPBatchCostResult{
		Data:          ordered,
		MissingSKUIDs: missing,
		Watermark:     watermark,
		SnapshotVersion: snapshotVersionForValue("jst_inventory_batch", struct {
			Watermark time.Time           `json:"watermark"`
			Data      []domain.ERPCostSKU `json:"data"`
		}{Watermark: watermark, Data: ordered}),
	}, nil
}

func (s *erpCostAPIService) History(ctx context.Context, rawSKUIDs []string, asOfRaw string, wmsCoIDs []int64) (*domain.ERPHistoryCostResult, *domain.AppError) {
	if s == nil || s.historyProvider == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "JST history cost provider is not configured on this Bridge", nil)
	}
	skuIDs, appErr := normalizeERPCostSKUIDs(rawSKUIDs, erpCostBatchMaxItems)
	if appErr != nil {
		return nil, appErr
	}
	asOf, err := time.Parse("2006-01-02", strings.TrimSpace(asOfRaw))
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "as_of must use YYYY-MM-DD", nil)
	}
	periods := make([]domain.JSTHistoryCostPeriod, 0)
	for start := 0; start < len(skuIDs); start += erpHistoryUpstreamBatch {
		end := start + erpHistoryUpstreamBatch
		if end > len(skuIDs) {
			end = len(skuIDs)
		}
		response, err := s.historyProvider.QueryHistoryCosts(ctx, domain.JSTHistoryCostQuery{
			SKUIDs:                skuIDs[start:end],
			WMSCoIDs:              wmsCoIDs,
			GetWay:                "all",
			IsUseItemSKUCostPrice: true,
		})
		if err != nil {
			return nil, domain.NewAppError("erp_upstream_failure", "JST history cost upstream failed", map[string]interface{}{
				"failure_class": classifyERPRemoteErr(err),
				"retryable":     isERPCostUpstreamRetryable(err),
			})
		}
		if response != nil {
			periods = append(periods, response.Periods...)
		}
	}
	selected := selectHistoryCostPeriods(periods, asOf)
	resolved := make(map[string]struct{}, len(selected))
	items := make([]domain.ERPHistoryCostItem, 0, len(selected))
	for _, period := range selected {
		cost, err := decimalFourPointer(period.CostPrice)
		if err != nil {
			return nil, infraAppError("normalize JST history cost precision", err)
		}
		resolved[erpCostSKUKey(period.SKUID)] = struct{}{}
		items = append(items, domain.ERPHistoryCostItem{
			SKUID: period.SKUID, WMSCoID: period.WMSCoID, CostPrice: cost,
			AsOf: asOf.Format("2006-01-02"), BeginDate: period.BeginDate,
			EndDate: period.EndDate, Remark: period.Remark,
		})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].SKUID == items[j].SKUID {
			return items[i].WMSCoID < items[j].WMSCoID
		}
		return items[i].SKUID < items[j].SKUID
	})
	missing := make([]string, 0)
	for _, skuID := range skuIDs {
		if _, ok := resolved[erpCostSKUKey(skuID)]; !ok {
			missing = append(missing, skuID)
		}
	}
	return &domain.ERPHistoryCostResult{
		Data:            items,
		MissingSKUIDs:   missing,
		SnapshotVersion: snapshotVersionForValue("jst_history_cost", items),
	}, nil
}

func (s *erpCostAPIService) Changes(ctx context.Context, sinceRaw, cursorRaw string, limit int) (*domain.ERPCostChangesResult, *domain.AppError) {
	if s == nil || s.repo == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "ERP cost read repository is not configured", nil)
	}
	limit = normalizeERPCostPageLimit(limit)
	since, appErr := parseERPCostTimestamp(sinceRaw, false)
	if appErr != nil {
		return nil, appErr
	}
	var watermarkID, lastID int64
	if strings.TrimSpace(cursorRaw) != "" {
		var cursor erpCostChangeCursor
		if err := s.decodeCursor(cursorRaw, &cursor); err != nil || cursor.Version != erpCostCursorVersion || cursor.Kind != "changes" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost changes cursor", nil)
		}
		cursorSince, err := time.Parse(time.RFC3339Nano, cursor.ChangedSince)
		if err != nil {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cost changes cursor timestamp", nil)
		}
		if strings.TrimSpace(sinceRaw) != "" && !since.Equal(cursorSince) {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "since does not match cursor snapshot", nil)
		}
		since = cursorSince
		watermarkID = cursor.WatermarkID
		lastID = cursor.LastID
	} else {
		var err error
		watermarkID, err = s.repo.CostChangeWatermark(ctx)
		if err != nil {
			return nil, infraAppError("read ERP cost change watermark", err)
		}
	}
	items, err := s.repo.ListCostChanges(ctx, repo.ERPCostChangePageQuery{
		ChangedSince: since, WatermarkID: watermarkID, LastID: lastID, Limit: limit + 1,
	})
	if err != nil {
		return nil, infraAppError("read ERP cost changes", err)
	}
	if err := normalizeERPCostChanges(items); err != nil {
		return nil, infraAppError("normalize ERP cost change precision", err)
	}
	result := &domain.ERPCostChangesResult{
		Data:            items,
		Watermark:       watermarkID,
		SnapshotVersion: snapshotVersion("jst_cost_changes", since.Format(time.RFC3339Nano), strconv.FormatInt(watermarkID, 10)),
	}
	if len(items) > limit {
		result.Data = items[:limit]
		cursor, err := s.encodeCursor(erpCostChangeCursor{
			Version: erpCostCursorVersion, Kind: "changes",
			ChangedSince: since.Format(time.RFC3339Nano), WatermarkID: watermarkID,
			LastID: result.Data[len(result.Data)-1].ID,
		})
		if err != nil {
			return nil, infraAppError("encode ERP cost changes cursor", err)
		}
		result.NextCursor = cursor
	}
	return result, nil
}

func normalizeERPCostPageLimit(limit int) int {
	if limit <= 0 {
		return erpCostFeedDefaultLimit
	}
	if limit > erpCostFeedMaxLimit {
		return erpCostFeedMaxLimit
	}
	return limit
}

func normalizeERPCostSKUIDs(values []string, limit int) ([]string, *domain.AppError) {
	if len(values) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "sku_ids is required", nil)
	}
	seen := make(map[string]struct{}, len(values))
	out := make([]string, 0, len(values))
	for _, raw := range values {
		value := strings.TrimSpace(raw)
		if value == "" {
			continue
		}
		if len(value) > 100 {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "sku_id must not exceed 100 characters", nil)
		}
		key := erpCostSKUKey(value)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, value)
		if len(out) > limit {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, fmt.Sprintf("sku_ids must contain at most %d unique values", limit), nil)
		}
	}
	if len(out) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "sku_ids is required", nil)
	}
	return out, nil
}

func parseERPCostTimestamp(raw string, required bool) (time.Time, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		if required {
			return time.Time{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "timestamp is required", nil)
		}
		return time.Unix(0, 0).UTC(), nil
	}
	if unix, err := strconv.ParseInt(raw, 10, 64); err == nil {
		if len(raw) >= 13 {
			return time.UnixMilli(unix).UTC(), nil
		}
		return time.Unix(unix, 0).UTC(), nil
	}
	for _, layout := range []string{time.RFC3339Nano, "2006-01-02 15:04:05.999999", "2006-01-02 15:04:05"} {
		if value, err := time.Parse(layout, raw); err == nil {
			return value, nil
		}
	}
	return time.Time{}, domain.NewAppError(domain.ErrCodeInvalidRequest, "timestamp must be RFC3339, Unix seconds, or Unix milliseconds", nil)
}

func normalizeERPCostSKUs(items []domain.ERPCostSKU) error {
	for idx := range items {
		cost, err := decimalFourPointer(items[idx].CostPrice)
		if err != nil {
			return fmt.Errorf("sku %s cost_price: %w", items[idx].SKUID, err)
		}
		items[idx].CostPrice = cost
	}
	return nil
}

func normalizeERPCostChanges(items []domain.ERPCostChange) error {
	for idx := range items {
		oldCost, err := decimalFourPointer(items[idx].OldCostPrice)
		if err != nil {
			return fmt.Errorf("change %d old_cost_price: %w", items[idx].ID, err)
		}
		newCost, err := decimalFourPointer(items[idx].NewCostPrice)
		if err != nil {
			return fmt.Errorf("change %d new_cost_price: %w", items[idx].ID, err)
		}
		items[idx].OldCostPrice = oldCost
		items[idx].NewCostPrice = newCost
	}
	return nil
}

func decimalFourPointer(raw *string) (*string, error) {
	if raw == nil {
		return nil, nil
	}
	value := strings.TrimSpace(*raw)
	parsed, _, err := big.ParseFloat(value, 10, 256, big.ToNearestEven)
	if err != nil {
		return nil, fmt.Errorf("invalid decimal %q", value)
	}
	formatted := parsed.Text('f', 4)
	return &formatted, nil
}

func selectHistoryCostPeriods(periods []domain.JSTHistoryCostPeriod, asOf time.Time) []domain.JSTHistoryCostPeriod {
	selected := make(map[string]domain.JSTHistoryCostPeriod)
	selectedBegin := make(map[string]time.Time)
	for _, period := range periods {
		if strings.TrimSpace(period.SKUID) == "" || !historyPeriodContains(period, asOf) {
			continue
		}
		begin, _ := parseHistoryDate(period.BeginDate)
		key := period.WMSCoID + "\x00" + period.SKUID
		if previous, ok := selectedBegin[key]; ok && !begin.After(previous) {
			continue
		}
		selected[key] = period
		selectedBegin[key] = begin
	}
	out := make([]domain.JSTHistoryCostPeriod, 0, len(selected))
	for _, period := range selected {
		out = append(out, period)
	}
	return out
}

func historyPeriodContains(period domain.JSTHistoryCostPeriod, asOf time.Time) bool {
	begin, beginOK := parseHistoryDate(period.BeginDate)
	end, endOK := parseHistoryDate(period.EndDate)
	if strings.TrimSpace(period.BeginDate) != "" && !beginOK {
		return false
	}
	if strings.TrimSpace(period.EndDate) != "" && !endOK {
		return false
	}
	if beginOK && asOf.Before(begin) {
		return false
	}
	if endOK && asOf.After(end) {
		return false
	}
	return true
}

func erpCostSKUKey(value string) string {
	return strings.ToUpper(strings.TrimSpace(value))
}

func parseHistoryDate(raw string) (time.Time, bool) {
	raw = strings.TrimSpace(raw)
	if len(raw) >= 10 {
		raw = raw[:10]
	}
	value, err := time.Parse("2006-01-02", raw)
	return value, err == nil
}

func snapshotVersion(parts ...string) string {
	hash := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(hash[:])
}

func snapshotVersionForValue(prefix string, value interface{}) string {
	raw, _ := json.Marshal(value)
	return snapshotVersion(prefix, string(raw))
}

func isERPCostUpstreamRetryable(err error) bool {
	var requestErr *erpBridgeRequestError
	if errors.As(err, &requestErr) {
		return requestErr.Timeout || requestErr.Retryable
	}
	var httpErr *erpBridgeHTTPError
	if errors.As(err, &httpErr) {
		return httpErr.Retryable
	}
	var openWebErr *erpBridgeOpenWebError
	if errors.As(err, &openWebErr) {
		return openWebErr.Retryable
	}
	return false
}

func (s *erpCostAPIService) encodeCursor(value interface{}) (string, error) {
	if len(s.cursorSecret) == 0 {
		return "", fmt.Errorf("ERP Bridge cost API token is not configured")
	}
	payload, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, s.cursorSecret)
	_, _ = mac.Write(payload)
	return base64.RawURLEncoding.EncodeToString(payload) + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil)), nil
}

func (s *erpCostAPIService) decodeCursor(raw string, target interface{}) error {
	if len(s.cursorSecret) == 0 {
		return fmt.Errorf("ERP Bridge cost API token is not configured")
	}
	parts := strings.Split(strings.TrimSpace(raw), ".")
	if len(parts) != 2 {
		return fmt.Errorf("invalid cursor envelope")
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return err
	}
	signature, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, s.cursorSecret)
	_, _ = mac.Write(payload)
	if !hmac.Equal(signature, mac.Sum(nil)) {
		return fmt.Errorf("cursor signature mismatch")
	}
	return json.Unmarshal(payload, target)
}
