package assetworkbench

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/xuri/excelize/v2"

	"workflow/domain"
	"workflow/repo"
	baseservice "workflow/service"
	assetcenter "workflow/service/asset_center"
)

func testWorkbenchOSSDirect() *baseservice.OSSDirectService {
	return baseservice.NewOSSDirectService(baseservice.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        "https://oss-cn-hangzhou.aliyuncs.com",
		PublicEndpoint:  "https://assets.example.com",
		Bucket:          "workflow-assets",
		AccessKeyID:     "test-id",
		AccessKeySecret: "test-secret",
		PresignExpiry:   15 * time.Minute,
	})
}

func assetCapabilityActor(id int64, permissions ...domain.PermissionCode) domain.RequestActor {
	assignment := domain.AccessAssignment{ID: 1, UserID: id, RoleID: 9001, RoleCode: "migrated_asset_manager", ScopeMode: domain.AccessScopeGlobal, SourceType: "direct"}
	sources := make([]domain.EffectiveAccessNote, 0, len(permissions))
	for _, permission := range permissions {
		sources = append(sources, domain.EffectiveAccessNote{Permission: permission, RoleID: assignment.RoleID, RoleCode: assignment.RoleCode, SourceType: assignment.SourceType, ScopeMode: assignment.ScopeMode})
	}
	return domain.RequestActor{
		ID:          id,
		Permissions: append([]domain.PermissionCode(nil), permissions...),
		EffectiveAccess: &domain.EffectiveAccess{
			UserID: id, Permissions: append([]domain.PermissionCode(nil), permissions...),
			Assignments: []domain.AccessAssignment{assignment}, Sources: sources,
		},
	}
}

func containsString(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func containsRole(values []domain.Role, target domain.Role) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func testDifficultyClass(code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	code = strings.TrimSpace(code)
	if code == "" || code == domain.AssetWorkbenchWorkerTypeAll {
		return nil, sql.ErrNoRows
	}
	return &domain.AssetWorkbenchDifficultyClass{
		ID:        1,
		Code:      code,
		Name:      code,
		Enabled:   true,
		SortOrder: 10,
	}, nil
}

type priceOnlyRepo struct {
	repo.AssetWorkbenchRepo
	price   *domain.AssetWorkbenchPriceMatrix
	coupons []*domain.AssetWorkbenchPromoCoupon
}

func (r *priceOnlyRepo) FindActivePrice(context.Context, string, string, string, time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	return r.price, nil
}

func (r *priceOnlyRepo) ListActivePromoCoupons(context.Context, string, string, string, time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error) {
	return r.coupons, nil
}

type deductionOnlyRepo struct {
	repo.AssetWorkbenchRepo
	rule         *domain.AssetWorkbenchDeductionRule
	welfareRules []*domain.AssetWorkbenchWelfareRule
}

func (r *deductionOnlyRepo) FindActiveDeductionRule(context.Context, string, string, string, time.Time) (*domain.AssetWorkbenchDeductionRule, error) {
	return r.rule, nil
}

func (r *deductionOnlyRepo) FindActiveWelfareRules(context.Context, string, string, time.Time) ([]*domain.AssetWorkbenchWelfareRule, error) {
	return r.welfareRules, nil
}

func (r *deductionOnlyRepo) GetProfileByUserID(context.Context, int64) (*domain.AssetWorkbenchProfile, error) {
	return nil, sql.ErrNoRows
}

type settlementReportRepo struct {
	repo.AssetWorkbenchRepo
	rule           *domain.AssetWorkbenchDeductionRule
	deductionCalls int
	profiles       map[int64]*domain.AssetWorkbenchProfile
	difficulties   []*domain.AssetWorkbenchDifficultyClass
}

func (r *settlementReportRepo) FindActiveDeductionRule(context.Context, string, string, string, time.Time) (*domain.AssetWorkbenchDeductionRule, error) {
	r.deductionCalls++
	return r.rule, nil
}

func (r *settlementReportRepo) FindActiveWelfareRules(context.Context, string, string, time.Time) ([]*domain.AssetWorkbenchWelfareRule, error) {
	return nil, nil
}

func (r *settlementReportRepo) GetProfileByUserID(_ context.Context, userID int64) (*domain.AssetWorkbenchProfile, error) {
	profile := r.profiles[userID]
	if profile == nil {
		return nil, sql.ErrNoRows
	}
	return profile, nil
}

func (r *settlementReportRepo) ListDifficultyClasses(context.Context, repo.AssetWorkbenchDifficultyClassFilter) ([]*domain.AssetWorkbenchDifficultyClass, error) {
	if r.difficulties != nil {
		return r.difficulties, nil
	}
	return []*domain.AssetWorkbenchDifficultyClass{
		{Code: "A", Name: "A", Enabled: true, SortOrder: 10},
		{Code: "B", Name: "B", Enabled: true, SortOrder: 20},
		{Code: "C", Name: "C", Enabled: true, SortOrder: 30},
	}, nil
}

type errorImportRepo struct {
	repo.AssetWorkbenchRepo
	items        []*domain.AssetWorkbenchSubmissionItem
	profiles     []*domain.AssetWorkbenchProfile
	difficulties []*domain.AssetWorkbenchDifficultyClass
	batch        *domain.AssetWorkbenchErrorImportBatch
	records      []*domain.AssetWorkbenchErrorRecord
	events       []*domain.AssetWorkbenchEvent
}

func (r *errorImportRepo) ListSubmissionItemsByMonth(context.Context, string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	return r.items, nil
}

func (r *errorImportRepo) GetProfileByUserID(_ context.Context, userID int64) (*domain.AssetWorkbenchProfile, error) {
	for _, item := range r.profiles {
		if item != nil && item.UserID == userID {
			return item, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *errorImportRepo) ListProfiles(_ context.Context, filter repo.AssetWorkbenchProfileFilter) ([]*domain.AssetWorkbenchProfile, int64, error) {
	items := make([]*domain.AssetWorkbenchProfile, 0, len(r.profiles))
	keyword := strings.TrimSpace(filter.Keyword)
	for _, item := range r.profiles {
		if item == nil {
			continue
		}
		if keyword != "" && !strings.Contains(item.RealName, keyword) {
			continue
		}
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *errorImportRepo) ListDifficultyClasses(context.Context, repo.AssetWorkbenchDifficultyClassFilter) ([]*domain.AssetWorkbenchDifficultyClass, error) {
	if r.difficulties != nil {
		return r.difficulties, nil
	}
	return []*domain.AssetWorkbenchDifficultyClass{
		{Code: "A", Name: "A类", Enabled: true, SortOrder: 10},
		{Code: "B", Name: "B类", Enabled: true, SortOrder: 20},
		{Code: "C", Name: "C类", Enabled: true, SortOrder: 30},
	}, nil
}

func (r *errorImportRepo) CreateErrorImportBatch(_ context.Context, _ repo.Tx, batch *domain.AssetWorkbenchErrorImportBatch) (*domain.AssetWorkbenchErrorImportBatch, error) {
	copyBatch := *batch
	copyBatch.ID = 9001
	r.batch = &copyBatch
	return &copyBatch, nil
}

func (r *errorImportRepo) CreateErrorRecord(_ context.Context, _ repo.Tx, record *domain.AssetWorkbenchErrorRecord) (*domain.AssetWorkbenchErrorRecord, error) {
	copyRecord := *record
	copyRecord.ID = int64(len(r.records) + 1)
	r.records = append(r.records, &copyRecord)
	return &copyRecord, nil
}

func (r *errorImportRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type supplementRepo struct {
	repo.AssetWorkbenchRepo
	items           []*domain.AssetWorkbenchSubmissionItem
	supplements     []*domain.AssetWorkbenchSettlementSupplement
	permissions     []*domain.AssetWorkbenchSupplementPermission
	confirmedMonths map[int64][]string
	created         *domain.AssetWorkbenchSettlementSupplement
	lastListFilter  repo.AssetWorkbenchSettlementSupplementFilter
	events          []*domain.AssetWorkbenchEvent
	filesByItem     map[int64][]*domain.AssetWorkbenchSubmissionFile
	deletedFileIDs  []int64
	voidedItemIDs   []int64
	refreshedIDs    []int64
}

func (r *supplementRepo) GetDifficultyClass(_ context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	return testDifficultyClass(code)
}

func (r *supplementRepo) ListSubmissionItemsByMonth(context.Context, string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	return r.items, nil
}

func (r *supplementRepo) ListSettlementSupplements(_ context.Context, filter repo.AssetWorkbenchSettlementSupplementFilter) ([]*domain.AssetWorkbenchSettlementSupplement, int64, error) {
	r.lastListFilter = filter
	items := make([]*domain.AssetWorkbenchSettlementSupplement, 0, len(r.supplements))
	for _, item := range r.supplements {
		if item == nil {
			continue
		}
		if filter.PayeeUserID != nil && item.PayeeUserID != *filter.PayeeUserID {
			continue
		}
		if filter.BusinessMonth != "" && item.BusinessMonth != filter.BusinessMonth {
			continue
		}
		if filter.OrderNo != "" && item.OrderNo != filter.OrderNo {
			continue
		}
		if filter.Status != "" && item.Status != filter.Status {
			continue
		}
		if filter.SupplementDate != "" && item.SupplementDate != filter.SupplementDate {
			continue
		}
		if filter.SupplementDateFrom != "" && item.SupplementDate < filter.SupplementDateFrom {
			continue
		}
		if filter.SupplementDateTo != "" && item.SupplementDate > filter.SupplementDateTo {
			continue
		}
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *supplementRepo) CreateSettlementSupplement(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSettlementSupplement) (*domain.AssetWorkbenchSettlementSupplement, error) {
	copyItem := *item
	copyItem.ID = 7001
	r.created = &copyItem
	return &copyItem, nil
}

func (r *supplementRepo) GetSettlementSupplementForUpdate(_ context.Context, _ repo.Tx, id int64) (*domain.AssetWorkbenchSettlementSupplement, error) {
	for _, item := range r.supplements {
		if item != nil && item.ID == id {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *supplementRepo) VoidSettlementSupplement(_ context.Context, _ repo.Tx, id int64) (*domain.AssetWorkbenchSettlementSupplement, error) {
	for _, item := range r.supplements {
		if item != nil && item.ID == id {
			item.Status = domain.AssetWorkbenchSupplementStatusVoided
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *supplementRepo) ListSubmissionFilesForUpdate(_ context.Context, _ repo.Tx, itemID int64) ([]*domain.AssetWorkbenchSubmissionFile, error) {
	return append([]*domain.AssetWorkbenchSubmissionFile(nil), r.filesByItem[itemID]...), nil
}

func (r *supplementRepo) DeleteSubmissionFile(_ context.Context, _ repo.Tx, fileID int64, _ int64, _ string, _ time.Time) error {
	r.deletedFileIDs = append(r.deletedFileIDs, fileID)
	return nil
}

func (r *supplementRepo) VoidSubmissionItem(_ context.Context, _ repo.Tx, itemID int64, _ int64, _ string, _ time.Time) (*domain.AssetWorkbenchSubmissionItem, error) {
	r.voidedItemIDs = append(r.voidedItemIDs, itemID)
	return &domain.AssetWorkbenchSubmissionItem{ID: itemID, SubmissionID: 9000 + itemID, QCStatus: domain.AssetWorkbenchSubmissionStatusVoided}, nil
}

func (r *supplementRepo) RefreshSubmissionTotals(_ context.Context, _ repo.Tx, submissionID int64) error {
	r.refreshedIDs = append(r.refreshedIDs, submissionID)
	return nil
}

func (r *supplementRepo) GetSupplementPermission(_ context.Context, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error) {
	for _, item := range r.permissions {
		if item != nil && item.PayeeUserID == payeeUserID && item.BusinessMonth == businessMonth {
			return item, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *supplementRepo) GetSupplementPermissionForUpdate(ctx context.Context, _ repo.Tx, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error) {
	return r.GetSupplementPermission(ctx, payeeUserID, businessMonth)
}

func (r *supplementRepo) ListSupplementPermissions(_ context.Context, filter repo.AssetWorkbenchSupplementPermissionFilter) ([]*domain.AssetWorkbenchSupplementPermission, int64, error) {
	items := make([]*domain.AssetWorkbenchSupplementPermission, 0, len(r.permissions))
	for _, item := range r.permissions {
		if item == nil {
			continue
		}
		if filter.PayeeUserID != nil && item.PayeeUserID != *filter.PayeeUserID {
			continue
		}
		if filter.BusinessMonth != "" && item.BusinessMonth != filter.BusinessMonth {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		items = append(items, item)
	}
	return items, int64(len(items)), nil
}

func (r *supplementRepo) UpsertSupplementPermission(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSupplementPermission) (*domain.AssetWorkbenchSupplementPermission, error) {
	copyItem := *item
	if copyItem.ID == 0 {
		copyItem.ID = int64(len(r.permissions) + 1)
	}
	for i, existing := range r.permissions {
		if existing != nil && existing.PayeeUserID == item.PayeeUserID && existing.BusinessMonth == item.BusinessMonth {
			r.permissions[i] = &copyItem
			return &copyItem, nil
		}
	}
	r.permissions = append(r.permissions, &copyItem)
	return &copyItem, nil
}

func (r *supplementRepo) HasConfirmedSettlementForPayeeMonth(_ context.Context, payeeUserID int64, businessMonth string) (bool, error) {
	for _, month := range r.confirmedMonths[payeeUserID] {
		if month == businessMonth {
			return true, nil
		}
	}
	return false, nil
}

func (r *supplementRepo) ListConfirmedSettlementMonthsByPayee(_ context.Context, payeeUserID int64) ([]string, error) {
	return append([]string(nil), r.confirmedMonths[payeeUserID]...), nil
}

func (r *supplementRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type adjustmentRepo struct {
	repo.AssetWorkbenchRepo
	batch          *domain.AssetWorkbenchSettlementBatch
	adjustment     *domain.AssetWorkbenchSettlementAdjustment
	settlementItem *domain.AssetWorkbenchSettlementItem
	appliedAmount  float64
	events         []*domain.AssetWorkbenchEvent
}

func (r *adjustmentRepo) LockSettlementBatch(_ context.Context, _ repo.Tx, batchID int64) (*domain.AssetWorkbenchSettlementBatch, error) {
	if r.batch == nil || r.batch.ID != batchID {
		return nil, sql.ErrNoRows
	}
	copyBatch := *r.batch
	return &copyBatch, nil
}

func (r *adjustmentRepo) CreateSettlementAdjustment(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSettlementAdjustment) (*domain.AssetWorkbenchSettlementAdjustment, error) {
	copyItem := *item
	copyItem.ID = 8001
	r.adjustment = &copyItem
	return &copyItem, nil
}

func (r *adjustmentRepo) CreateSettlementItem(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSettlementItem) (*domain.AssetWorkbenchSettlementItem, error) {
	copyItem := *item
	copyItem.ID = 8101
	r.settlementItem = &copyItem
	return &copyItem, nil
}

func (r *adjustmentRepo) ApplySettlementBatchAdjustment(_ context.Context, _ repo.Tx, _ int64, signedAmount float64) error {
	r.appliedAmount += signedAmount
	if r.batch != nil {
		r.batch.AdjustmentAmount += signedAmount
		r.batch.NetAmount += signedAmount
	}
	return nil
}

func (r *adjustmentRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type settlementBatchRepo struct {
	repo.AssetWorkbenchRepo
	items                 []*domain.AssetWorkbenchSubmissionItem
	supplements           []*domain.AssetWorkbenchSettlementSupplement
	createdBatch          *domain.AssetWorkbenchSettlementBatch
	settlementItems       []*domain.AssetWorkbenchSettlementItem
	attachedItemIDs       []int64
	attachedSupplementIDs []int64
	cancelledBatchID      int64
	cancelReason          string
	confirmedBatchID      int64
	frozenBatchID         int64
	events                []*domain.AssetWorkbenchEvent
}

func (r *settlementBatchRepo) ListErrorRecordsByMonth(context.Context, string) ([]*domain.AssetWorkbenchErrorRecord, error) {
	return nil, nil
}

func (r *settlementBatchRepo) LockSettleableItems(context.Context, repo.Tx, string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	return r.items, nil
}

func (r *settlementBatchRepo) LockSettleableSupplements(context.Context, repo.Tx, string) ([]*domain.AssetWorkbenchSettlementSupplement, error) {
	return r.supplements, nil
}

func (r *settlementBatchRepo) FindActiveWelfareRules(context.Context, string, string, time.Time) ([]*domain.AssetWorkbenchWelfareRule, error) {
	return nil, nil
}

func (r *settlementBatchRepo) CreateSettlementBatch(_ context.Context, _ repo.Tx, batch *domain.AssetWorkbenchSettlementBatch) (*domain.AssetWorkbenchSettlementBatch, error) {
	copyBatch := *batch
	copyBatch.ID = 8801
	r.createdBatch = &copyBatch
	return &copyBatch, nil
}

func (r *settlementBatchRepo) CreateSettlementItem(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSettlementItem) (*domain.AssetWorkbenchSettlementItem, error) {
	copyItem := *item
	copyItem.ID = int64(len(r.settlementItems) + 1)
	r.settlementItems = append(r.settlementItems, &copyItem)
	return &copyItem, nil
}

func (r *settlementBatchRepo) AttachItemsToSettlementBatch(_ context.Context, _ repo.Tx, batchID int64, itemIDs []int64) error {
	r.attachedItemIDs = append([]int64(nil), itemIDs...)
	return nil
}

func (r *settlementBatchRepo) AttachSupplementsToSettlementBatch(_ context.Context, _ repo.Tx, batchID int64, supplementIDs []int64) error {
	r.attachedSupplementIDs = append([]int64(nil), supplementIDs...)
	return nil
}

func (r *settlementBatchRepo) ConfirmSettlementBatch(_ context.Context, _ repo.Tx, batchID int64, _ int64, _ time.Time) error {
	r.confirmedBatchID = batchID
	return nil
}

func (r *settlementBatchRepo) LockSettlementBatch(_ context.Context, _ repo.Tx, batchID int64) (*domain.AssetWorkbenchSettlementBatch, error) {
	return &domain.AssetWorkbenchSettlementBatch{ID: batchID, Status: domain.AssetWorkbenchBatchStatusGenerated}, nil
}

func (r *settlementBatchRepo) ListSettlementItemsByBatch(_ context.Context, batchID int64) ([]*domain.AssetWorkbenchSettlementItem, error) {
	if len(r.settlementItems) > 0 {
		return r.settlementItems, nil
	}
	return []*domain.AssetWorkbenchSettlementItem{{
		ID:            1,
		BatchID:       batchID,
		ItemType:      domain.AssetWorkbenchItemTypeGrossPiecework,
		PayeeUserID:   77,
		BusinessMonth: "2026-06",
		Amount:        32,
		Quantity:      1,
		Direction:     "credit",
	}}, nil
}

func (r *settlementBatchRepo) FreezeSettlementPayouts(_ context.Context, _ repo.Tx, batchID int64, _ time.Time, snapshots map[int64]json.RawMessage) error {
	if len(snapshots) == 0 {
		return domain.NewAppError(domain.ErrCodeConflict, "missing snapshots", nil)
	}
	r.frozenBatchID = batchID
	return nil
}

func (r *settlementBatchRepo) GetProfileByUserID(_ context.Context, userID int64) (*domain.AssetWorkbenchProfile, error) {
	idCard := "330100199001010000"
	return &domain.AssetWorkbenchProfile{
		UserID:        userID,
		RealName:      "结算人员",
		IDCard:        &idCard,
		AlipayAccount: "payee@example.com",
		Status:        domain.AssetWorkbenchProfileStatusActive,
	}, nil
}

func (r *settlementBatchRepo) CancelGeneratedSettlementBatch(_ context.Context, _ repo.Tx, batchID int64, _ int64, reason string, _ time.Time) error {
	r.cancelledBatchID = batchID
	r.cancelReason = reason
	return nil
}

func (r *settlementBatchRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type registerIdentityStub struct {
	params baseserviceRegisterParams
}

type baseserviceRegisterParams struct {
	username    string
	displayName string
	mobile      string
}

func (s *registerIdentityStub) RegisterAssetWorkbenchUser(_ context.Context, p baseservice.RegisterAssetWorkbenchUserParams) (*domain.AuthResult, *domain.AppError) {
	s.params = baseserviceRegisterParams{username: p.Username, displayName: p.DisplayName, mobile: p.Mobile}
	return &domain.AuthResult{
		User: &domain.User{
			ID:          77,
			Username:    p.Username,
			DisplayName: p.DisplayName,
			Roles:       []domain.Role{domain.RoleAssetSubmitter},
		},
		Session: &domain.AuthSession{Token: "token-77"},
	}, nil
}

type registerProfileRepo struct {
	repo.AssetWorkbenchRepo
	profile      *domain.AssetWorkbenchProfile
	membership   *domain.AppMembership
	gradePeriods []*domain.AssetWorkbenchGradePeriod
	events       []*domain.AssetWorkbenchEvent
}

func (r *registerProfileRepo) UpsertProfile(_ context.Context, _ repo.Tx, profile *domain.AssetWorkbenchProfile) (*domain.AssetWorkbenchProfile, error) {
	copyProfile := *profile
	copyProfile.ID = 100
	r.profile = &copyProfile
	return &copyProfile, nil
}

func (r *registerProfileRepo) AppendGradePeriod(_ context.Context, _ repo.Tx, period *domain.AssetWorkbenchGradePeriod) (*domain.AssetWorkbenchGradePeriod, error) {
	copyPeriod := *period
	copyPeriod.ID = int64(len(r.gradePeriods) + 1)
	r.gradePeriods = append(r.gradePeriods, &copyPeriod)
	return &copyPeriod, nil
}

func (r *registerProfileRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

func (r *registerProfileRepo) OpenMembership(_ context.Context, _ repo.Tx, params repo.AssetWorkbenchAccessOpenParams) (*domain.AppMembership, error) {
	r.membership = &domain.AppMembership{
		ID:           1,
		AppCode:      domain.AssetWorkbenchAppCode,
		UserID:       params.UserID,
		Status:       params.Status,
		IdentityType: params.IdentityType,
		Source:       params.Source,
	}
	return r.membership, nil
}

type bootstrapAccessRepo struct {
	repo.AssetWorkbenchRepo
}

func (r *bootstrapAccessRepo) GetMembership(_ context.Context, _ string, userID int64) (*domain.AppMembership, error) {
	return &domain.AppMembership{
		ID:           1,
		AppCode:      domain.AssetWorkbenchAppCode,
		UserID:       userID,
		Status:       domain.AppMembershipStatusActive,
		IdentityType: domain.AppMembershipIdentityStaff,
	}, nil
}

func (r *bootstrapAccessRepo) GetProfileByUserID(context.Context, int64) (*domain.AssetWorkbenchProfile, error) {
	return nil, sql.ErrNoRows
}

type roleBackfillAccessRepo struct {
	repo.AssetWorkbenchRepo
	membership *domain.AppMembership
	event      *domain.AppIdentityEvent
}

func (r *roleBackfillAccessRepo) GetMembership(_ context.Context, appCode string, userID int64) (*domain.AppMembership, error) {
	if appCode != domain.AssetWorkbenchAppCode || userID != 303 || r.membership == nil {
		return nil, sql.ErrNoRows
	}
	copyMembership := *r.membership
	return &copyMembership, nil
}

func (r *roleBackfillAccessRepo) UpsertMembership(_ context.Context, _ repo.Tx, membership *domain.AppMembership) (*domain.AppMembership, error) {
	copyMembership := *membership
	copyMembership.ID = 1
	r.membership = &copyMembership
	return &copyMembership, nil
}

func (r *roleBackfillAccessRepo) CreateAppIdentityEvent(_ context.Context, _ repo.Tx, event *domain.AppIdentityEvent) (*domain.AppIdentityEvent, error) {
	copyEvent := *event
	copyEvent.ID = 1
	r.event = &copyEvent
	return &copyEvent, nil
}

func (r *roleBackfillAccessRepo) GetProfileByUserID(context.Context, int64) (*domain.AssetWorkbenchProfile, error) {
	return nil, sql.ErrNoRows
}

type memberRoleUpdateRepo struct {
	repo.AssetWorkbenchRepo
	listFilters []repo.AssetWorkbenchMemberFilter
	event       *domain.AppIdentityEvent
}

func (r *memberRoleUpdateRepo) GetMembership(_ context.Context, appCode string, userID int64) (*domain.AppMembership, error) {
	if appCode != domain.AssetWorkbenchAppCode || userID != 302 {
		return nil, sql.ErrNoRows
	}
	return &domain.AppMembership{
		ID:           5,
		AppCode:      domain.AssetWorkbenchAppCode,
		UserID:       302,
		Status:       domain.AppMembershipStatusActive,
		IdentityType: domain.AppMembershipIdentityStaff,
		CreatedAt:    time.Date(2026, 6, 27, 8, 13, 19, 0, time.UTC),
		UpdatedAt:    time.Date(2026, 6, 27, 8, 13, 19, 0, time.UTC),
	}, nil
}

func (r *memberRoleUpdateRepo) ListMembers(_ context.Context, filter repo.AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, error) {
	r.listFilters = append(r.listFilters, filter)
	if filter.UserID != 302 {
		return nil, 0, nil
	}
	return []*domain.AssetWorkbenchMember{
		{
			UserID:      302,
			Username:    "定制美工测试管理员",
			DisplayName: "定制美工测试管理员",
			RealName:    "张三",
			Status:      domain.AppMembershipStatusActive,
			Identity:    domain.AppMembershipIdentityStaff,
			Roles:       []domain.Role{domain.RoleAssetSubmitter, domain.RoleAssetManager},
			CreatedAt:   time.Date(2026, 6, 27, 8, 13, 19, 0, time.UTC),
			UpdatedAt:   time.Date(2026, 6, 27, 8, 13, 19, 0, time.UTC),
		},
	}, 1, nil
}

func (r *memberRoleUpdateRepo) CreateAppIdentityEvent(_ context.Context, _ repo.Tx, event *domain.AppIdentityEvent) (*domain.AppIdentityEvent, error) {
	copyEvent := *event
	copyEvent.ID = 1
	r.event = &copyEvent
	return &copyEvent, nil
}

type memberRoleUpdateUserRepo struct {
	repo.UserRepo
	roles          []domain.Role
	replacedUserID int64
	replacedRoles  []domain.Role
}

func (r *memberRoleUpdateUserRepo) ListRoles(_ context.Context, userID int64) ([]domain.Role, error) {
	if userID != 302 {
		return nil, sql.ErrNoRows
	}
	return append([]domain.Role{}, r.roles...), nil
}

func (r *memberRoleUpdateUserRepo) ReplaceRoles(_ context.Context, _ repo.Tx, userID int64, roles []domain.Role) error {
	r.replacedUserID = userID
	r.replacedRoles = append([]domain.Role{}, roles...)
	r.roles = append([]domain.Role{}, roles...)
	return nil
}

type profileListRepo struct {
	repo.AssetWorkbenchRepo
	items        []*domain.AssetWorkbenchProfile
	saved        *domain.AssetWorkbenchProfile
	gradePeriods []*domain.AssetWorkbenchGradePeriod
	events       []*domain.AssetWorkbenchEvent
	pendingItems []*domain.AssetWorkbenchSubmissionItem
	price        *domain.AssetWorkbenchPriceMatrix
	refreshed    []int64
}

func (r *profileListRepo) ListProfiles(context.Context, repo.AssetWorkbenchProfileFilter) ([]*domain.AssetWorkbenchProfile, int64, error) {
	return r.items, int64(len(r.items)), nil
}

func (r *profileListRepo) GetProfileByUserID(_ context.Context, userID int64) (*domain.AssetWorkbenchProfile, error) {
	for _, item := range r.items {
		if item != nil && item.UserID == userID {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *profileListRepo) UpsertProfile(_ context.Context, _ repo.Tx, profile *domain.AssetWorkbenchProfile) (*domain.AssetWorkbenchProfile, error) {
	copyProfile := *profile
	if copyProfile.ID == 0 {
		copyProfile.ID = 100
	}
	r.saved = &copyProfile
	return &copyProfile, nil
}

func (r *profileListRepo) AppendGradePeriod(_ context.Context, _ repo.Tx, period *domain.AssetWorkbenchGradePeriod) (*domain.AssetWorkbenchGradePeriod, error) {
	copyPeriod := *period
	copyPeriod.ID = int64(len(r.gradePeriods) + 1)
	r.gradePeriods = append(r.gradePeriods, &copyPeriod)
	return &copyPeriod, nil
}

func (r *profileListRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

func (r *profileListRepo) ListPendingGradeSubmissionItemsForPayee(_ context.Context, _ repo.Tx, payeeUserID int64, limit int) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	items := []*domain.AssetWorkbenchSubmissionItem{}
	for _, item := range r.pendingItems {
		if item == nil || item.PayeeUserID != payeeUserID || item.PricingStatus != domain.AssetWorkbenchPricingStatusPendingGrade {
			continue
		}
		if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
		if limit > 0 && len(items) >= limit {
			break
		}
	}
	return items, nil
}

func (r *profileListRepo) FindActivePrice(_ context.Context, workerType, jobGrade, difficulty string, _ time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	if r.price == nil {
		return nil, sql.ErrNoRows
	}
	if r.price.WorkerType != workerType || r.price.JobGrade != jobGrade || r.price.DifficultyClass != difficulty {
		return nil, sql.ErrNoRows
	}
	copyPrice := *r.price
	return &copyPrice, nil
}

func (r *profileListRepo) ListActivePromoCoupons(context.Context, string, string, string, time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error) {
	return nil, nil
}

func (r *profileListRepo) UpdateSubmissionItemPricing(_ context.Context, _ repo.Tx, next *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	for _, item := range r.pendingItems {
		if item == nil || item.ID != next.ID {
			continue
		}
		item.BasePriceRuleID = next.BasePriceRuleID
		item.BaseUnitPrice = next.BaseUnitPrice
		item.WorkerTypeSnapshot = next.WorkerTypeSnapshot
		item.JobGradeSnapshot = next.JobGradeSnapshot
		item.PromoCouponID = next.PromoCouponID
		item.PromoSnapshot = next.PromoSnapshot
		item.PricingSnapshot = next.PricingSnapshot
		item.GrossAmount = next.GrossAmount
		item.PricingStatus = next.PricingStatus
		copyItem := *item
		return &copyItem, nil
	}
	return nil, sql.ErrNoRows
}

func (r *profileListRepo) RefreshSubmissionTotals(_ context.Context, _ repo.Tx, submissionID int64) error {
	r.refreshed = append(r.refreshed, submissionID)
	return nil
}

type profileNotificationCall struct {
	userID      int64
	ntype       domain.NotificationType
	payload     json.RawMessage
	dedupeScope string
	dedupeKey   string
}

type profileNotificationStub struct {
	calls []profileNotificationCall
}

func (s *profileNotificationStub) CreateDedupedNotification(_ context.Context, userID int64, ntype domain.NotificationType, payload json.RawMessage, dedupeScope, dedupeKey string) (*domain.Notification, bool, error) {
	s.calls = append(s.calls, profileNotificationCall{
		userID:      userID,
		ntype:       ntype,
		payload:     payload,
		dedupeScope: dedupeScope,
		dedupeKey:   dedupeKey,
	})
	return &domain.Notification{ID: int64(len(s.calls)), UserID: userID, NotificationType: ntype, Payload: payload}, true, nil
}

type assetWorkbenchTestTx struct{}

func (assetWorkbenchTestTx) IsTx() {}

type assetWorkbenchTestTxRunner struct{}

func (assetWorkbenchTestTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(assetWorkbenchTestTx{})
}

type priceMatrixVersionRepo struct {
	repo.AssetWorkbenchRepo
	before          *domain.AssetWorkbenchPriceMatrix
	existing        []*domain.AssetWorkbenchPriceMatrix
	closedEffective *time.Time
	created         *domain.AssetWorkbenchPriceMatrix
	events          []*domain.AssetWorkbenchEvent
}

func (r *priceMatrixVersionRepo) GetPriceMatrixForUpdate(_ context.Context, _ repo.Tx, id int64) (*domain.AssetWorkbenchPriceMatrix, error) {
	if r.before == nil || r.before.ID != id {
		return nil, sql.ErrNoRows
	}
	copyItem := *r.before
	return &copyItem, nil
}

func (r *priceMatrixVersionRepo) GetDifficultyClass(_ context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	return testDifficultyClass(code)
}

func (r *priceMatrixVersionRepo) LockPriceMatrixDimension(_ context.Context, _ repo.Tx, workerType, jobGrade, difficultyClass string) ([]*domain.AssetWorkbenchPriceMatrix, error) {
	items := []*domain.AssetWorkbenchPriceMatrix{}
	for _, item := range r.existing {
		if item.WorkerType != workerType || item.JobGrade != jobGrade || item.DifficultyClass != difficultyClass {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, nil
}

func (r *priceMatrixVersionRepo) SetPriceMatrixEffectiveTo(_ context.Context, _ repo.Tx, id int64, effectiveTo *time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	if r.before == nil || r.before.ID != id {
		return nil, sql.ErrNoRows
	}
	copyItem := *r.before
	if effectiveTo != nil {
		value := *effectiveTo
		r.closedEffective = &value
		copyItem.EffectiveTo = &value
	} else {
		r.closedEffective = nil
		copyItem.EffectiveTo = nil
	}
	return &copyItem, nil
}

func (r *priceMatrixVersionRepo) CreatePriceMatrix(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchPriceMatrix) (*domain.AssetWorkbenchPriceMatrix, error) {
	copyItem := *item
	copyItem.ID = 200
	r.created = &copyItem
	return &copyItem, nil
}

func (r *priceMatrixVersionRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

func TestSupersedePriceMatrixPublishesNewVersionByClosingPriorRule(t *testing.T) {
	current := &domain.AssetWorkbenchPriceMatrix{
		ID:              101,
		WorkerType:      domain.AssetWorkbenchWorkerTypeParttime,
		JobGrade:        "J1",
		DifficultyClass: "A",
		UnitPrice:       12,
		EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Enabled:         true,
		RevisionNo:      1,
		CreatedBy:       7,
	}
	workbenchRepo := &priceMatrixVersionRepo{
		before:   current,
		existing: []*domain.AssetWorkbenchPriceMatrix{current},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	created, appErr := svc.SupersedePriceMatrix(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetTemplateAdmin}}, current.ID, CreatePriceMatrixParams{
		WorkerType:      domain.AssetWorkbenchWorkerTypeParttime,
		JobGrade:        "J1",
		DifficultyClass: "A",
		UnitPrice:       15,
		EffectiveFrom:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	if appErr != nil {
		t.Fatalf("SupersedePriceMatrix() appErr = %v", appErr)
	}
	if created == nil || created.ID != 200 {
		t.Fatalf("created = %+v, want generated row", created)
	}
	if created.RevisionNo != 2 {
		t.Fatalf("created.RevisionNo = %d, want 2", created.RevisionNo)
	}
	if workbenchRepo.closedEffective == nil {
		t.Fatalf("closedEffective = nil, want previous day")
	}
	wantClosed := time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC)
	if !workbenchRepo.closedEffective.Equal(wantClosed) {
		t.Fatalf("closedEffective = %s, want %s", workbenchRepo.closedEffective, wantClosed)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventPriceSuperseded {
		t.Fatalf("events = %+v, want one price superseded event", workbenchRepo.events)
	}
}

func TestSupersedePriceMatrixRejectsDimensionChange(t *testing.T) {
	current := &domain.AssetWorkbenchPriceMatrix{
		ID:              101,
		WorkerType:      domain.AssetWorkbenchWorkerTypeParttime,
		JobGrade:        "J1",
		DifficultyClass: "A",
		UnitPrice:       12,
		EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		Enabled:         true,
		RevisionNo:      1,
	}
	workbenchRepo := &priceMatrixVersionRepo{
		before:   current,
		existing: []*domain.AssetWorkbenchPriceMatrix{current},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	_, appErr := svc.SupersedePriceMatrix(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetTemplateAdmin}}, current.ID, CreatePriceMatrixParams{
		WorkerType:      domain.AssetWorkbenchWorkerTypeParttime,
		JobGrade:        "J2",
		DifficultyClass: "A",
		UnitPrice:       15,
		EffectiveFrom:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("appErr = %+v, want invalid request", appErr)
	}
	if workbenchRepo.created != nil || workbenchRepo.closedEffective != nil {
		t.Fatalf("created = %+v closedEffective = %+v, want no mutation", workbenchRepo.created, workbenchRepo.closedEffective)
	}
}

type downloadFileRepo struct {
	repo.AssetWorkbenchRepo
	files  map[int64]*domain.AssetWorkbenchSubmissionFile
	events []*domain.AssetWorkbenchEvent
}

func (r *downloadFileRepo) GetSubmissionFile(_ context.Context, fileID int64) (*domain.AssetWorkbenchSubmissionFile, error) {
	file := r.files[fileID]
	if file == nil {
		return nil, sql.ErrNoRows
	}
	copyFile := *file
	return &copyFile, nil
}

func (r *downloadFileRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type systemAssetDownloaderStub struct {
	downloadCalls int
	batchCalls    int
	assetIDs      []int64
	batchAssetIDs []int64
	info          *domain.AssetDownloadInfo
}

func (s *systemAssetDownloaderStub) DownloadLatest(_ context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	s.downloadCalls++
	s.assetIDs = append(s.assetIDs, assetID)
	if s.info != nil {
		copyInfo := *s.info
		return &copyInfo, nil
	}
	url := "https://assets.example.com/system/" + strconv.FormatInt(assetID, 10)
	return &domain.AssetDownloadInfo{
		DownloadMode: domain.AssetDownloadModeDirect,
		DownloadURL:  &url,
		Filename:     "system-asset.psd",
		FileSize:     2048,
		MimeType:     "image/vnd.adobe.photoshop",
	}, nil
}

type systemAssetPreviewDownloaderStub struct {
	systemAssetDownloaderStub
	previewCalls int
	previewIDs   []int64
	previewInfo  *domain.AssetDownloadInfo
	previewErr   *domain.AppError
}

type systemAssetPreviewerStub struct {
	previewCalls int
	previewIDs   []int64
	previewInfo  *domain.AssetDownloadInfo
}

func (s *systemAssetPreviewerStub) GetAssetPreviewInfoByID(_ context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	s.previewCalls++
	s.previewIDs = append(s.previewIDs, assetID)
	copyInfo := *s.previewInfo
	return &copyInfo, nil
}

func (s *systemAssetPreviewDownloaderStub) GetAssetPreviewInfoByID(_ context.Context, assetID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	s.previewCalls++
	s.previewIDs = append(s.previewIDs, assetID)
	if s.previewErr != nil {
		return nil, s.previewErr
	}
	if s.previewInfo != nil {
		copyInfo := *s.previewInfo
		return &copyInfo, nil
	}
	url := "https://assets.example.com/system/preview/" + strconv.FormatInt(assetID, 10)
	return &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeDirect,
		DownloadURL:      &url,
		Filename:         "preview.webp",
		FileSize:         1024,
		MimeType:         "image/webp",
		PreviewAvailable: true,
	}, nil
}

type uploadDirectorySessionRepo struct {
	repo.AssetWorkbenchRepo
	directories []*domain.AssetWorkbenchUploadDirectory
	created     *domain.AssetWorkbenchUploadSession
	events      []*domain.AssetWorkbenchEvent
}

func (r *uploadDirectorySessionRepo) GetDifficultyClass(_ context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	return testDifficultyClass(code)
}

func (r *uploadDirectorySessionRepo) ListUploadDirectories(_ context.Context, filter repo.AssetWorkbenchUploadDirectoryFilter) ([]*domain.AssetWorkbenchUploadDirectory, error) {
	items := make([]*domain.AssetWorkbenchUploadDirectory, 0, len(r.directories))
	for _, item := range r.directories {
		if item == nil {
			continue
		}
		if filter.Enabled != nil && item.Enabled != *filter.Enabled {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, nil
}

func (r *uploadDirectorySessionRepo) GetUploadDirectory(_ context.Context, directoryID int64) (*domain.AssetWorkbenchUploadDirectory, error) {
	for _, item := range r.directories {
		if item != nil && item.ID == directoryID {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *uploadDirectorySessionRepo) CreateUploadSession(_ context.Context, _ repo.Tx, session *domain.AssetWorkbenchUploadSession) (*domain.AssetWorkbenchUploadSession, error) {
	copySession := *session
	copySession.ID = 9101
	r.created = &copySession
	return &copySession, nil
}

func (r *uploadDirectorySessionRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type submissionDirectoryDifficultyRepo struct {
	repo.AssetWorkbenchRepo
	profile                 *domain.AssetWorkbenchProfile
	price                   *domain.AssetWorkbenchPriceMatrix
	session                 *domain.AssetWorkbenchUploadSession
	submission              *domain.AssetWorkbenchSubmission
	item                    *domain.AssetWorkbenchSubmissionItem
	files                   []*domain.AssetWorkbenchSubmissionFile
	sessionStatus           string
	events                  []*domain.AssetWorkbenchEvent
	members                 []*domain.AssetWorkbenchMember
	permissions             []*domain.AssetWorkbenchSupplementPermission
	lockedPermissionEnabled *bool
	supplements             []*domain.AssetWorkbenchSettlementSupplement
	createdSupplement       *domain.AssetWorkbenchSettlementSupplement
}

func (r *submissionDirectoryDifficultyRepo) ListMembers(_ context.Context, filter repo.AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, error) {
	items := make([]*domain.AssetWorkbenchMember, 0, len(r.members))
	for _, member := range r.members {
		if member == nil {
			continue
		}
		copyMember := *member
		copyMember.Roles = append([]domain.Role(nil), member.Roles...)
		items = append(items, &copyMember)
	}
	return items, int64(len(items)), nil
}

func (r *submissionDirectoryDifficultyRepo) GetDifficultyClass(_ context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	return testDifficultyClass(code)
}

func (r *submissionDirectoryDifficultyRepo) GetProfileByUserID(_ context.Context, userID int64) (*domain.AssetWorkbenchProfile, error) {
	if r.profile == nil || r.profile.UserID != userID {
		return nil, sql.ErrNoRows
	}
	copyProfile := *r.profile
	return &copyProfile, nil
}

func (r *submissionDirectoryDifficultyRepo) GetUploadSession(_ context.Context, sessionID string) (*domain.AssetWorkbenchUploadSession, error) {
	if r.session == nil || r.session.SessionID != sessionID {
		return nil, sql.ErrNoRows
	}
	copySession := *r.session
	return &copySession, nil
}

func (r *submissionDirectoryDifficultyRepo) CreateSubmission(_ context.Context, _ repo.Tx, submission *domain.AssetWorkbenchSubmission) (*domain.AssetWorkbenchSubmission, error) {
	copySubmission := *submission
	copySubmission.ID = 5001
	r.submission = &copySubmission
	return &copySubmission, nil
}

func (r *submissionDirectoryDifficultyRepo) CreateSubmissionItem(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	copyItem := *item
	copyItem.ID = 6001
	r.item = &copyItem
	return &copyItem, nil
}

func (r *submissionDirectoryDifficultyRepo) CreateSubmissionFile(_ context.Context, _ repo.Tx, file *domain.AssetWorkbenchSubmissionFile) (*domain.AssetWorkbenchSubmissionFile, error) {
	copyFile := *file
	copyFile.ID = int64(7000 + len(r.files) + 1)
	r.files = append(r.files, &copyFile)
	return &copyFile, nil
}

func (r *submissionDirectoryDifficultyRepo) UpdateUploadSessionStatus(_ context.Context, _ repo.Tx, sessionID, status string, _ *time.Time, _ *time.Time, _ *int64) error {
	if r.session == nil || r.session.SessionID != sessionID {
		return sql.ErrNoRows
	}
	r.sessionStatus = status
	return nil
}

func (r *submissionDirectoryDifficultyRepo) RefreshSubmissionTotals(_ context.Context, _ repo.Tx, _ int64) error {
	return nil
}

func (r *submissionDirectoryDifficultyRepo) FindActivePrice(_ context.Context, workerType, jobGrade, difficulty string, _ time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	if r.price == nil || r.price.WorkerType != workerType || r.price.JobGrade != jobGrade || r.price.DifficultyClass != difficulty {
		return nil, sql.ErrNoRows
	}
	copyPrice := *r.price
	return &copyPrice, nil
}

func (r *submissionDirectoryDifficultyRepo) ListActivePromoCoupons(context.Context, string, string, string, time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error) {
	return nil, nil
}

func (r *submissionDirectoryDifficultyRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

func (r *submissionDirectoryDifficultyRepo) GetSupplementPermission(_ context.Context, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error) {
	for _, item := range r.permissions {
		if item != nil && item.PayeeUserID == payeeUserID && item.BusinessMonth == businessMonth {
			copyItem := *item
			return &copyItem, nil
		}
	}
	return nil, sql.ErrNoRows
}

func (r *submissionDirectoryDifficultyRepo) GetSupplementPermissionForUpdate(ctx context.Context, _ repo.Tx, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error) {
	item, err := r.GetSupplementPermission(ctx, payeeUserID, businessMonth)
	if err != nil {
		return nil, err
	}
	if r.lockedPermissionEnabled != nil {
		item.Enabled = *r.lockedPermissionEnabled
	}
	return item, nil
}

func (r *submissionDirectoryDifficultyRepo) ListSettlementSupplements(_ context.Context, filter repo.AssetWorkbenchSettlementSupplementFilter) ([]*domain.AssetWorkbenchSettlementSupplement, int64, error) {
	items := make([]*domain.AssetWorkbenchSettlementSupplement, 0, len(r.supplements))
	for _, item := range r.supplements {
		if item == nil || (filter.PayeeUserID != nil && item.PayeeUserID != *filter.PayeeUserID) || (filter.BusinessMonth != "" && item.BusinessMonth != filter.BusinessMonth) || (filter.OrderNo != "" && item.OrderNo != filter.OrderNo) {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
	}
	return items, int64(len(items)), nil
}

func (r *submissionDirectoryDifficultyRepo) ListSubmissionItemsByMonth(_ context.Context, _ string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	return nil, nil
}

func (r *submissionDirectoryDifficultyRepo) CreateSettlementSupplement(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSettlementSupplement) (*domain.AssetWorkbenchSettlementSupplement, error) {
	copyItem := *item
	copyItem.ID = 8001
	r.createdSupplement = &copyItem
	r.supplements = append(r.supplements, &copyItem)
	return &copyItem, nil
}

type clientMaterialRepo struct {
	repo.AssetWorkbenchRepo
	materials                 map[int64]*domain.AssetWorkbenchClientMaterial
	events                    []*domain.AssetWorkbenchEvent
	currentResourceRevisionID int64
	resourceCoverItemID       int64
	batchJobs                 []*domain.AssetWorkbenchBatchJob
}

func (r *clientMaterialRepo) ResolveResourceGroupPublication(_ context.Context, groupID, finalizedRevisionID, coverRevisionItemID int64) (*domain.ResourceGroupPublicationSnapshot, error) {
	revisionID := finalizedRevisionID
	if revisionID == 0 {
		revisionID = r.currentResourceRevisionID
	}
	coverID := r.resourceCoverItemID
	if coverID == 0 {
		coverID = 501
	}
	return &domain.ResourceGroupPublicationSnapshot{
		GroupID: groupID, TaskID: 20, TaskNo: "RW-20", SKUCode: "SKU-20", Mode: domain.TaskAssetGroupModeSingle,
		FinalizedRevisionID: revisionID, CurrentFinalizedRevisionID: r.currentResourceRevisionID, CoverRevisionItemID: coverID,
		Files: []domain.TaskResourceFile{{RevisionItemID: coverID, TaskAssetID: 900, FileName: "final.png", MimeType: "image/png", StorageKey: "tasks/20/final.png"}},
	}, nil
}

func (r *clientMaterialRepo) CreateClientMaterial(_ context.Context, _ repo.Tx, material *domain.AssetWorkbenchClientMaterial) (*domain.AssetWorkbenchClientMaterial, error) {
	if r.materials == nil {
		r.materials = map[int64]*domain.AssetWorkbenchClientMaterial{}
	}
	copyItem := *material
	copyItem.ID = int64(len(r.materials) + 1)
	r.materials[copyItem.ID] = &copyItem
	return &copyItem, nil
}

func (r *clientMaterialRepo) UpdateClientMaterial(_ context.Context, _ repo.Tx, material *domain.AssetWorkbenchClientMaterial) (*domain.AssetWorkbenchClientMaterial, error) {
	copyItem := *material
	r.materials[copyItem.ID] = &copyItem
	return &copyItem, nil
}

func (r *clientMaterialRepo) GetClientMaterial(_ context.Context, materialID int64) (*domain.AssetWorkbenchClientMaterial, error) {
	item := r.materials[materialID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *clientMaterialRepo) ListClientMaterials(_ context.Context, filter repo.AssetWorkbenchClientMaterialFilter) ([]*domain.AssetWorkbenchClientMaterial, error) {
	items := make([]*domain.AssetWorkbenchClientMaterial, 0, len(r.materials))
	for _, item := range r.materials {
		if item == nil || (filter.Enabled != nil && item.Enabled != *filter.Enabled) {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (r *clientMaterialRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

func (r *clientMaterialRepo) CreateBatchJob(_ context.Context, _ repo.Tx, job *domain.AssetWorkbenchBatchJob) (*domain.AssetWorkbenchBatchJob, error) {
	copyJob := *job
	copyJob.ID = int64(len(r.batchJobs) + 1)
	copyJob.RequestPayload = append(json.RawMessage(nil), job.RequestPayload...)
	r.batchJobs = append(r.batchJobs, &copyJob)
	return &copyJob, nil
}

func (r *clientMaterialRepo) UpdateBatchJobProgress(_ context.Context, _ repo.Tx, job *domain.AssetWorkbenchBatchJob) error {
	return nil
}

func (r *clientMaterialRepo) CompleteBatchJob(_ context.Context, _ repo.Tx, job *domain.AssetWorkbenchBatchJob) error {
	return nil
}

type effectiveAccessResolverStub struct {
	effective *domain.EffectiveAccess
	err       *domain.AppError
}

func (s *effectiveAccessResolverStub) EffectiveAccess(_ context.Context, _ int64) (*domain.EffectiveAccess, *domain.AppError) {
	return s.effective, s.err
}

func TestBatchUpdateClientMaterialsPersistsAnImmutableResolvedSnapshot(t *testing.T) {
	repository := &clientMaterialRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(repository, assetWorkbenchTestTxRunner{}))
	actor := assetCapabilityActor(9, domain.PermissionAssetPublish)
	items := make([]CreateClientMaterialParams, assetWorkbenchClientMaterialBatchUpdateLimit+1)
	for index := range items {
		items[index] = CreateClientMaterialParams{SourceType: "external_asset", SourceRef: fmt.Sprintf("ext-%d", index+1), ResourceID: fmt.Sprintf("ext-%d", index+1)}
	}

	result, appErr := svc.BatchUpdateClientMaterials(context.Background(), actor, BatchUpdateClientMaterialsParams{Action: "publish", Items: items})
	if appErr != nil || result == nil || !result.AsyncRequired || len(repository.batchJobs) != 1 {
		t.Fatalf("BatchUpdateClientMaterials() result=%+v error=%+v jobs=%d", result, appErr, len(repository.batchJobs))
	}
	var payload BatchUpdateClientMaterialsParams
	if err := json.Unmarshal(repository.batchJobs[0].RequestPayload, &payload); err != nil {
		t.Fatalf("decode snapshot: %v", err)
	}
	if payload.SelectionScope != "resolved_snapshot" || len(payload.Items) != len(items) || len(payload.Folders) != 0 || payload.Query != "" {
		t.Fatalf("queued payload = %+v, want exact immutable items", payload)
	}
}

func TestBatchWorkerFailsClosedWhenPublishPermissionWasRevoked(t *testing.T) {
	repository := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{}, currentResourceRevisionID: 100, resourceCoverItemID: 501}
	resolver := &effectiveAccessResolverStub{effective: &domain.EffectiveAccess{UserID: 9}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(repository, assetWorkbenchTestTxRunner{}), WithEffectiveAccessResolver(resolver))
	payload := mustJSON(BatchUpdateClientMaterialsParams{Action: "publish", SelectionScope: "resolved_snapshot", Items: []CreateClientMaterialParams{{
		SourceType: "task_resource_group", ResourceGroupID: 10, FinalizedRevisionID: 100, CoverRevisionItemID: 501,
	}}})
	job := &domain.AssetWorkbenchBatchJob{ID: 1, JobID: "job-1", JobType: domain.AssetWorkbenchAsyncJobTypeClientMaterialBatchUpdate, Action: "publish", RequestedBy: 9, RequestPayload: payload}

	appErr := svc.processBatchJob(context.Background(), job)
	if appErr == nil || len(repository.materials) != 0 || !strings.Contains(job.ErrorMessage, "revoked") {
		t.Fatalf("processBatchJob() error=%+v job=%+v materials=%d", appErr, job, len(repository.materials))
	}
}

func TestBatchWorkerNeverInventsPermissionsWithoutEffectiveAccess(t *testing.T) {
	repository := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(repository, assetWorkbenchTestTxRunner{}))
	payload := mustJSON(BatchUpdateClientMaterialsParams{Action: "publish", SelectionScope: "resolved_snapshot", Items: []CreateClientMaterialParams{{SourceType: "external_asset", SourceRef: "ext-1"}}})
	job := &domain.AssetWorkbenchBatchJob{ID: 1, JobID: "job-1", JobType: domain.AssetWorkbenchAsyncJobTypeClientMaterialBatchUpdate, Action: "publish", RequestedBy: 9, RequestPayload: payload}

	appErr := svc.processBatchJob(context.Background(), job)
	if appErr == nil || len(repository.materials) != 0 || !strings.Contains(job.ErrorMessage, "resolver") {
		t.Fatalf("processBatchJob() error=%+v job=%+v materials=%d", appErr, job, len(repository.materials))
	}
}

func TestResourceGroupPublicationPinsResolvedRevisionAndRejectsWrongCover(t *testing.T) {
	repository := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{}, currentResourceRevisionID: 100, resourceCoverItemID: 501}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(repository, assetWorkbenchTestTxRunner{}), WithOSSDirect(testWorkbenchOSSDirect()))
	actor := assetCapabilityActor(9, domain.PermissionAssetPublish)

	created, appErr := svc.CreateClientMaterial(context.Background(), actor, CreateClientMaterialParams{
		SourceType: "task_resource_group", ResourceGroupID: 10, FinalizedRevisionID: 100, CoverRevisionItemID: 501, Title: "Pinned set",
	})
	if appErr != nil {
		t.Fatalf("CreateClientMaterial() error = %+v", appErr)
	}
	if created.FinalizedRevisionID == nil || *created.FinalizedRevisionID != 100 {
		t.Fatalf("created finalized revision = %v, want 100", created.FinalizedRevisionID)
	}

	// A later finalized group revision must not move an existing publication.
	repository.currentResourceRevisionID = 200
	info, appErr := svc.clientMaterialDownloadInfo(context.Background(), created)
	if appErr != nil {
		t.Fatalf("clientMaterialDownloadInfo() error = %+v", appErr)
	}
	if info == nil || info.Filename != "final.png" || created.FinalizedRevisionID == nil || *created.FinalizedRevisionID != 100 {
		t.Fatalf("pinned download = %+v material=%+v", info, created)
	}

	_, appErr = svc.CreateClientMaterial(context.Background(), actor, CreateClientMaterialParams{
		SourceType: "task_resource_group", ResourceGroupID: 10, FinalizedRevisionID: 100, CoverRevisionItemID: 501,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("superseded revision appErr = %+v, want conflict", appErr)
	}

	_, appErr = svc.CreateClientMaterial(context.Background(), actor, CreateClientMaterialParams{
		SourceType: "task_resource_group", ResourceGroupID: 10, CoverRevisionItemID: 999,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("wrong cover appErr = %+v", appErr)
	}
}

func (s *systemAssetDownloaderStub) BuildBatchDownloadManifest(_ context.Context, assetIDs []int64, _ ...assetcenter.BatchDownloadOption) (*assetcenter.BatchDownloadManifest, *domain.AppError) {
	s.batchCalls++
	s.batchAssetIDs = append([]int64(nil), assetIDs...)
	items := make([]assetcenter.BatchDownloadItem, 0, len(assetIDs))
	for _, assetID := range assetIDs {
		items = append(items, assetcenter.BatchDownloadItem{
			AssetID:     assetID,
			TaskID:      assetID + 100,
			Filename:    "system-asset-" + strconv.FormatInt(assetID, 10) + ".psd",
			FileSize:    2048,
			DownloadURL: "https://assets.example.com/system/" + strconv.FormatInt(assetID, 10),
		})
	}
	return &assetcenter.BatchDownloadManifest{
		Items:        items,
		SuccessCount: len(items),
		TotalSize:    int64(len(items)) * 2048,
	}, nil
}

type externalMaterialProviderStub struct {
	searchCalls   int
	searchQueries []domain.AssetSearchQuery
	searchResult  *assetcenter.SearchResult
	pagedSearch   []*assetcenter.AssetDetail
	browseCalls   []assetcenter.MaterialBrowseQuery
	detailCalls   []int64
	downloadCalls []int64
	previewCalls  []int64
	browseResults map[string]*assetcenter.MaterialBrowseResult
	details       map[int64]*assetcenter.AssetDetail
	downloadInfo  *domain.AssetDownloadInfo
	previewInfo   *domain.AssetDownloadInfo
}

func (s *externalMaterialProviderStub) Search(_ context.Context, query domain.AssetSearchQuery) (*assetcenter.SearchResult, *domain.AppError) {
	s.searchCalls++
	s.searchQueries = append(s.searchQueries, query)
	if query.BusinessLane.Valid() && query.Source == domain.AssetResourceSourceExternal {
		return &assetcenter.SearchResult{Items: []*assetcenter.AssetDetail{}, Total: 0, Page: query.Page, Size: query.Size}, nil
	}
	if s.pagedSearch != nil {
		rows := append([]*assetcenter.AssetDetail(nil), s.pagedSearch...)
		if query.OperationalVisibleOnly {
			rows = filterVisibleWorkbenchMaterialAssets(rows)
		}
		start := (query.Page - 1) * query.Size
		if start > len(rows) {
			start = len(rows)
		}
		end := start + query.Size
		if end > len(rows) {
			end = len(rows)
		}
		return &assetcenter.SearchResult{
			Items: append([]*assetcenter.AssetDetail(nil), rows[start:end]...),
			Total: int64(len(rows)), Page: query.Page, Size: query.Size,
		}, nil
	}
	if s.searchResult != nil {
		result := *s.searchResult
		result.Items = append([]*assetcenter.AssetDetail(nil), s.searchResult.Items...)
		return &result, nil
	}
	return &assetcenter.SearchResult{}, nil
}

type resourceGroupMaterialProviderStub struct {
	items   []domain.TaskAssetGroup
	queries []domain.ResourceGroupListParams
}

func (s *resourceGroupMaterialProviderStub) ListResourceGroups(_ context.Context, _ domain.RequestActor, params domain.ResourceGroupListParams) (*domain.ResourceGroupListResult, *domain.AppError) {
	s.queries = append(s.queries, params)
	items := append([]domain.TaskAssetGroup(nil), s.items...)
	if params.BusinessLane.Valid() {
		filtered := items[:0]
		for _, item := range items {
			if item.BusinessLane == params.BusinessLane {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	if params.FormatCategory != "" && params.FormatCategory != domain.AssetFormatCategoryAll {
		filtered := items[:0]
		for _, item := range items {
			if testResourceGroupMatchesFormat(item, params.FormatCategory) {
				filtered = append(filtered, item)
			}
		}
		items = filtered
	}
	start := (params.Page - 1) * params.PageSize
	if start > len(items) {
		start = len(items)
	}
	end := start + params.PageSize
	if end > len(items) {
		end = len(items)
	}
	return &domain.ResourceGroupListResult{
		Items: append([]domain.TaskAssetGroup(nil), items[start:end]...),
		Page:  params.Page, PageSize: params.PageSize, Total: int64(len(items)),
	}, nil
}

func testResourceGroupMatchesFormat(item domain.TaskAssetGroup, category domain.AssetFormatCategoryFilter) bool {
	if item.FinalizedRevision == nil {
		return false
	}
	if testResourceFileMatchesFormat(item.FinalizedRevision.SourceFile, category) {
		return true
	}
	for _, revisionItem := range item.FinalizedRevision.Items {
		if testResourceFileMatchesFormat(revisionItem.File, category) {
			return true
		}
	}
	return false
}

func testResourceFileMatchesFormat(file *domain.TaskResourceFile, category domain.AssetFormatCategoryFilter) bool {
	if file == nil {
		return false
	}
	name := strings.ToLower(file.FileName)
	mime := strings.ToLower(file.MimeType)
	switch category {
	case domain.AssetFormatCategoryImage:
		return strings.HasPrefix(mime, "image/") && !strings.Contains(mime, "photoshop")
	case domain.AssetFormatCategoryDesign:
		return strings.Contains(mime, "photoshop") || strings.HasSuffix(name, ".psd") || strings.HasSuffix(name, ".ai")
	}
	return false
}

func (s *externalMaterialProviderStub) BrowseMaterials(_ context.Context, query assetcenter.MaterialBrowseQuery) (*assetcenter.MaterialBrowseResult, *domain.AppError) {
	s.browseCalls = append(s.browseCalls, query)
	if s.browseResults != nil && s.browseResults[query.Path] != nil {
		result := *s.browseResults[query.Path]
		result.Folders = append([]assetcenter.MaterialFolder(nil), result.Folders...)
		result.Files = append([]*assetcenter.AssetDetail(nil), result.Files...)
		return &result, nil
	}
	return &assetcenter.MaterialBrowseResult{
		Path:    query.Path,
		Folders: []assetcenter.MaterialFolder{},
		Files:   []*assetcenter.AssetDetail{},
		Total:   0,
		Page:    query.Page,
		Size:    query.Size,
	}, nil
}

func (s *externalMaterialProviderStub) GetExternalDetail(_ context.Context, externalID int64) (*assetcenter.AssetDetail, *domain.AppError) {
	s.detailCalls = append(s.detailCalls, externalID)
	if s.details != nil && s.details[externalID] != nil {
		detail := *s.details[externalID]
		return &detail, nil
	}
	return nil, domain.ErrNotFound
}

func (s *externalMaterialProviderStub) GetDetail(_ context.Context, assetID int64) (*assetcenter.AssetDetail, *domain.AppError) {
	s.detailCalls = append(s.detailCalls, assetID)
	if s.details != nil && s.details[assetID] != nil {
		detail := *s.details[assetID]
		return &detail, nil
	}
	return nil, domain.ErrNotFound
}

func (s *externalMaterialProviderStub) DownloadExternal(_ context.Context, externalID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	s.downloadCalls = append(s.downloadCalls, externalID)
	if s.downloadInfo != nil {
		info := *s.downloadInfo
		return &info, nil
	}
	url := "https://assets.example.com/external/" + strconv.FormatInt(externalID, 10)
	return &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeDirect,
		DownloadURL:      &url,
		Filename:         "external-material.png",
		FileSize:         4096,
		MimeType:         "image/png",
		PreviewAvailable: true,
	}, nil
}

func (s *externalMaterialProviderStub) PreviewExternal(_ context.Context, externalID int64) (*domain.AssetDownloadInfo, *domain.AppError) {
	s.previewCalls = append(s.previewCalls, externalID)
	if s.previewInfo != nil {
		info := *s.previewInfo
		return &info, nil
	}
	url := "https://assets.example.com/external/preview/" + strconv.FormatInt(externalID, 10)
	return &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeDirect,
		DownloadURL:      &url,
		Filename:         "external-material.png",
		FileSize:         4096,
		MimeType:         "image/png",
		PreviewAvailable: true,
	}, nil
}

type itemActionRepo struct {
	repo.AssetWorkbenchRepo
	items        map[int64]*domain.AssetWorkbenchSubmissionItem
	submissions  map[int64]*domain.AssetWorkbenchSubmission
	filesByItem  map[int64][]*domain.AssetWorkbenchSubmissionFile
	price        *domain.AssetWorkbenchPriceMatrix
	profile      *domain.AssetWorkbenchProfile
	events       []*domain.AssetWorkbenchEvent
	refreshCalls int
}

func (r *itemActionRepo) GetDifficultyClass(_ context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	return testDifficultyClass(code)
}

func (r *itemActionRepo) GetSubmissionItem(_ context.Context, itemID int64) (*domain.AssetWorkbenchSubmissionItem, error) {
	item := r.items[itemID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *itemActionRepo) GetSubmission(_ context.Context, submissionID int64) (*domain.AssetWorkbenchSubmission, error) {
	submission := r.submissions[submissionID]
	if submission == nil {
		return nil, sql.ErrNoRows
	}
	copySubmission := *submission
	return &copySubmission, nil
}

func (r *itemActionRepo) ListSubmissionItemsByMonth(_ context.Context, businessMonth string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	items := make([]*domain.AssetWorkbenchSubmissionItem, 0, len(r.items))
	for _, item := range r.items {
		if item == nil {
			continue
		}
		if businessMonth != "" && item.BusinessMonth != businessMonth {
			continue
		}
		copyItem := *item
		items = append(items, &copyItem)
	}
	sort.Slice(items, func(i, j int) bool { return items[i].ID < items[j].ID })
	return items, nil
}

func (r *itemActionRepo) ListSubmissionFiles(_ context.Context, submissionItemID int64) ([]*domain.AssetWorkbenchSubmissionFile, error) {
	files := make([]*domain.AssetWorkbenchSubmissionFile, 0, len(r.filesByItem[submissionItemID]))
	for _, file := range r.filesByItem[submissionItemID] {
		if file == nil {
			continue
		}
		copyFile := *file
		files = append(files, &copyFile)
	}
	return files, nil
}

func (r *itemActionRepo) GetProfileByUserID(_ context.Context, userID int64) (*domain.AssetWorkbenchProfile, error) {
	if r.profile == nil || r.profile.UserID != userID {
		return nil, sql.ErrNoRows
	}
	copyProfile := *r.profile
	return &copyProfile, nil
}

func (r *itemActionRepo) UpdateSubmissionItemQCStatus(_ context.Context, _ repo.Tx, itemID int64, qcStatus string) (*domain.AssetWorkbenchSubmissionItem, error) {
	item := r.items[itemID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "not mutable", nil)
	}
	item.QCStatus = qcStatus
	copyItem := *item
	return &copyItem, nil
}

func (r *itemActionRepo) VoidSubmissionItem(_ context.Context, _ repo.Tx, itemID int64, actorID int64, reason string, at time.Time) (*domain.AssetWorkbenchSubmissionItem, error) {
	item := r.items[itemID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "not mutable", nil)
	}
	item.QCStatus = domain.AssetWorkbenchSubmissionStatusVoided
	item.VoidedAt = &at
	item.VoidedBy = &actorID
	item.VoidReason = reason
	copyItem := *item
	return &copyItem, nil
}

func (r *itemActionRepo) UpdateSubmissionItemPricing(_ context.Context, _ repo.Tx, next *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	item := r.items[next.ID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "not mutable", nil)
	}
	item.BasePriceRuleID = next.BasePriceRuleID
	item.BaseUnitPrice = next.BaseUnitPrice
	item.WorkerTypeSnapshot = next.WorkerTypeSnapshot
	item.JobGradeSnapshot = next.JobGradeSnapshot
	item.PromoCouponID = next.PromoCouponID
	item.PromoSnapshot = next.PromoSnapshot
	item.PricingSnapshot = next.PricingSnapshot
	item.GrossAmount = next.GrossAmount
	item.PricingStatus = next.PricingStatus
	copyItem := *item
	return &copyItem, nil
}

func (r *itemActionRepo) RefreshSubmissionTotals(_ context.Context, _ repo.Tx, _ int64) error {
	r.refreshCalls++
	return nil
}

func (r *itemActionRepo) FindActivePrice(_ context.Context, workerType, jobGrade, difficulty string, _ time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	if r.price == nil {
		return nil, sql.ErrNoRows
	}
	if r.price.WorkerType != workerType || r.price.JobGrade != jobGrade || r.price.DifficultyClass != difficulty {
		return nil, sql.ErrNoRows
	}
	return r.price, nil
}

func (r *itemActionRepo) ListActivePromoCoupons(context.Context, string, string, string, time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error) {
	return nil, nil
}

func (r *itemActionRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type batchFileMutationRepo struct {
	repo.AssetWorkbenchRepo
	directories   map[int64]*domain.AssetWorkbenchUploadDirectory
	files         map[int64]*domain.AssetWorkbenchSubmissionFile
	items         map[int64]*domain.AssetWorkbenchSubmissionItem
	price         *domain.AssetWorkbenchPriceMatrix
	blockedDelete map[int64]bool
	failVoid      bool
	updatedFiles  []*domain.AssetWorkbenchSubmissionFile
	updatedItems  []*domain.AssetWorkbenchSubmissionItem
	deletedFiles  []int64
	refreshed     []int64
	events        []*domain.AssetWorkbenchEvent
}

func (r *batchFileMutationRepo) mutableItem(itemID int64) (*domain.AssetWorkbenchSubmissionItem, error) {
	if r.items == nil {
		return nil, nil
	}
	item := r.items[itemID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "not mutable", nil)
	}
	return item, nil
}

func (r *batchFileMutationRepo) mutableItemForFile(fileID int64) (*domain.AssetWorkbenchSubmissionItem, error) {
	file := r.files[fileID]
	if file == nil {
		return nil, sql.ErrNoRows
	}
	return r.mutableItem(file.SubmissionItemID)
}

func (r *batchFileMutationRepo) GetUploadDirectory(_ context.Context, directoryID int64) (*domain.AssetWorkbenchUploadDirectory, error) {
	directory := r.directories[directoryID]
	if directory == nil {
		return nil, sql.ErrNoRows
	}
	copyDirectory := *directory
	return &copyDirectory, nil
}

func (r *batchFileMutationRepo) ListSubmissionFilesByIDs(_ context.Context, fileIDs []int64) ([]*domain.AssetWorkbenchSubmissionFile, error) {
	items := make([]*domain.AssetWorkbenchSubmissionFile, 0, len(fileIDs))
	seen := map[int64]bool{}
	for _, fileID := range fileIDs {
		if seen[fileID] {
			continue
		}
		seen[fileID] = true
		file := r.files[fileID]
		if file == nil {
			continue
		}
		copyFile := *file
		items = append(items, &copyFile)
	}
	return items, nil
}

func (r *batchFileMutationRepo) GetSubmissionFile(_ context.Context, fileID int64) (*domain.AssetWorkbenchSubmissionFile, error) {
	file := r.files[fileID]
	if file == nil {
		return nil, sql.ErrNoRows
	}
	copyFile := *file
	return &copyFile, nil
}

func (r *batchFileMutationRepo) UpdateSubmissionFileLocation(_ context.Context, _ repo.Tx, file *domain.AssetWorkbenchSubmissionFile) (*domain.AssetWorkbenchSubmissionFile, error) {
	if file == nil || r.files[file.ID] == nil {
		return nil, sql.ErrNoRows
	}
	if _, err := r.mutableItemForFile(file.ID); err != nil {
		return nil, err
	}
	copyFile := *file
	r.files[file.ID] = &copyFile
	r.updatedFiles = append(r.updatedFiles, &copyFile)
	return &copyFile, nil
}

func (r *batchFileMutationRepo) UpdateSubmissionFileDisplayName(_ context.Context, _ repo.Tx, fileID int64, displayName string) (*domain.AssetWorkbenchSubmissionFile, error) {
	file := r.files[fileID]
	if file == nil {
		return nil, sql.ErrNoRows
	}
	copyFile := *file
	copyFile.DisplayName = displayName
	r.files[fileID] = &copyFile
	r.updatedFiles = append(r.updatedFiles, &copyFile)
	return &copyFile, nil
}

func (r *batchFileMutationRepo) DeleteSubmissionFile(_ context.Context, _ repo.Tx, fileID int64, _ int64, _ string, _ time.Time) error {
	if r.blockedDelete[fileID] {
		return domain.NewAppError(domain.ErrCodeConflict, "blocked file", nil)
	}
	if r.files[fileID] == nil {
		return sql.ErrNoRows
	}
	if _, err := r.mutableItemForFile(fileID); err != nil {
		return err
	}
	delete(r.files, fileID)
	r.deletedFiles = append(r.deletedFiles, fileID)
	return nil
}

func (r *batchFileMutationRepo) GetSubmissionItem(_ context.Context, itemID int64) (*domain.AssetWorkbenchSubmissionItem, error) {
	if r.items != nil {
		item := r.items[itemID]
		if item == nil {
			return nil, sql.ErrNoRows
		}
		copyItem := *item
		return &copyItem, nil
	}
	for _, file := range r.files {
		if file != nil && file.SubmissionItemID == itemID {
			return &domain.AssetWorkbenchSubmissionItem{
				ID:               itemID,
				SubmissionID:     file.SubmissionID,
				PayeeUserID:      file.OwnerUserID,
				OrderNo:          "AWF",
				DifficultyClass:  "A",
				Finalized:        true,
				PageCount:        1,
				ItemCount:        1,
				BusinessMonth:    "2026-07",
				SubmittedAt:      time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC),
				SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
				QCStatus:         domain.AssetWorkbenchSubmissionStatusSubmitted,
			}, nil
		}
	}
	return &domain.AssetWorkbenchSubmissionItem{
		ID:               itemID,
		SubmissionID:     9001,
		PayeeUserID:      99,
		OrderNo:          "AWF",
		DifficultyClass:  "A",
		Finalized:        true,
		PageCount:        1,
		ItemCount:        1,
		BusinessMonth:    "2026-07",
		SubmittedAt:      time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC),
		SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
		QCStatus:         domain.AssetWorkbenchSubmissionStatusSubmitted,
	}, nil
}

func (r *batchFileMutationRepo) ListSubmissionFiles(_ context.Context, submissionItemID int64) ([]*domain.AssetWorkbenchSubmissionFile, error) {
	items := []*domain.AssetWorkbenchSubmissionFile{}
	for _, file := range r.files {
		if file != nil && file.SubmissionItemID == submissionItemID {
			copyFile := *file
			items = append(items, &copyFile)
		}
	}
	return items, nil
}

func (r *batchFileMutationRepo) FindActivePrice(_ context.Context, workerType, jobGrade, difficulty string, _ time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	if r.price == nil || r.price.WorkerType != workerType || r.price.JobGrade != jobGrade || r.price.DifficultyClass != difficulty {
		return nil, sql.ErrNoRows
	}
	copyPrice := *r.price
	return &copyPrice, nil
}

func (r *batchFileMutationRepo) ListActivePromoCoupons(context.Context, string, string, string, time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error) {
	return nil, nil
}

func (r *batchFileMutationRepo) UpdateSubmissionItemEditableFields(_ context.Context, _ repo.Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	if item == nil {
		return nil, sql.ErrNoRows
	}
	if _, err := r.mutableItem(item.ID); err != nil {
		return nil, err
	}
	copyItem := *item
	r.items[item.ID] = &copyItem
	r.updatedItems = append(r.updatedItems, &copyItem)
	return &copyItem, nil
}

func (r *batchFileMutationRepo) VoidSubmissionItem(_ context.Context, _ repo.Tx, itemID int64, _ int64, _ string, _ time.Time) (*domain.AssetWorkbenchSubmissionItem, error) {
	if r.failVoid {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "void failed", nil)
	}
	voidedAt := time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC)
	if r.items == nil {
		return &domain.AssetWorkbenchSubmissionItem{ID: itemID, SubmissionID: 9001, VoidedAt: &voidedAt}, nil
	}
	item, err := r.mutableItem(itemID)
	if err != nil {
		return nil, err
	}
	copyItem := *item
	copyItem.QCStatus = domain.AssetWorkbenchSubmissionStatusVoided
	copyItem.VoidedAt = &voidedAt
	r.items[itemID] = &copyItem
	r.updatedItems = append(r.updatedItems, &copyItem)
	return &copyItem, nil
}

func (r *batchFileMutationRepo) RefreshSubmissionTotals(_ context.Context, _ repo.Tx, submissionID int64) error {
	r.refreshed = append(r.refreshed, submissionID)
	return nil
}

func (r *batchFileMutationRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

type rollbackBatchFileTxRunner struct {
	repo *batchFileMutationRepo
}

func (r rollbackBatchFileTxRunner) RunInTx(_ context.Context, fn func(tx repo.Tx) error) error {
	files := map[int64]*domain.AssetWorkbenchSubmissionFile{}
	for id, file := range r.repo.files {
		if file == nil {
			continue
		}
		copyFile := *file
		files[id] = &copyFile
	}
	deleted := append([]int64(nil), r.repo.deletedFiles...)
	updatedFiles := append([]*domain.AssetWorkbenchSubmissionFile(nil), r.repo.updatedFiles...)
	updatedItems := append([]*domain.AssetWorkbenchSubmissionItem(nil), r.repo.updatedItems...)
	refreshed := append([]int64(nil), r.repo.refreshed...)
	events := append([]*domain.AssetWorkbenchEvent(nil), r.repo.events...)
	items := map[int64]*domain.AssetWorkbenchSubmissionItem{}
	for id, item := range r.repo.items {
		if item == nil {
			continue
		}
		copyItem := *item
		items[id] = &copyItem
	}
	err := fn(assetWorkbenchTestTx{})
	if err != nil {
		r.repo.files = files
		r.repo.deletedFiles = deleted
		r.repo.updatedFiles = updatedFiles
		r.repo.updatedItems = updatedItems
		r.repo.refreshed = refreshed
		r.repo.events = events
		r.repo.items = items
	}
	return err
}

type settlementLockRaceTxRunner struct {
	repo    *batchFileMutationRepo
	itemID  int64
	batchID int64
}

func (r settlementLockRaceTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	item := r.repo.items[r.itemID]
	if item == nil {
		return sql.ErrNoRows
	}
	locked := *item
	batchID := r.batchID
	locked.SettlementStatus = domain.AssetWorkbenchSettlementStatusInBatch
	locked.CurrentSettlementBatchID = &batchID
	r.repo.items[r.itemID] = &locked
	return (rollbackBatchFileTxRunner{repo: r.repo}).RunInTx(ctx, fn)
}

type recordedOSSRequest struct {
	Method     string
	Path       string
	CopySource string
}

type recordingOSSTransport struct {
	mu       sync.Mutex
	requests []recordedOSSRequest
}

func (r *recordingOSSTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	r.mu.Lock()
	r.requests = append(r.requests, recordedOSSRequest{
		Method:     req.Method,
		Path:       req.URL.EscapedPath(),
		CopySource: req.Header.Get("x-oss-copy-source"),
	})
	r.mu.Unlock()
	status := http.StatusOK
	if req.Method == http.MethodDelete {
		status = http.StatusNoContent
	}
	return &http.Response{
		StatusCode: status,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader("")),
		Request:    req,
	}, nil
}

func (r *recordingOSSTransport) snapshot() []recordedOSSRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]recordedOSSRequest(nil), r.requests...)
}

func useRecordingOSSTransport(t *testing.T) *recordingOSSTransport {
	t.Helper()
	transport := &recordingOSSTransport{}
	previous := http.DefaultTransport
	http.DefaultTransport = transport
	t.Cleanup(func() {
		http.DefaultTransport = previous
	})
	return transport
}

type submissionVoidRepo struct {
	repo.AssetWorkbenchRepo
	submission *domain.AssetWorkbenchSubmission
	items      []*domain.AssetWorkbenchSubmissionItem
	events     []*domain.AssetWorkbenchEvent
}

func (r *submissionVoidRepo) GetSubmissionForUpdate(_ context.Context, _ repo.Tx, submissionID int64) (*domain.AssetWorkbenchSubmission, error) {
	if r.submission == nil || r.submission.ID != submissionID {
		return nil, sql.ErrNoRows
	}
	copySubmission := *r.submission
	return &copySubmission, nil
}

func (r *submissionVoidRepo) VoidSubmission(_ context.Context, _ repo.Tx, submissionID int64, actorID int64, reason string, at time.Time) (*domain.AssetWorkbenchSubmission, error) {
	if r.submission == nil || r.submission.ID != submissionID {
		return nil, sql.ErrNoRows
	}
	if r.submission.Status == domain.AssetWorkbenchSubmissionStatusVoided {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "already voided", nil)
	}
	for _, item := range r.items {
		if item == nil {
			continue
		}
		if item.SettlementStatus != domain.AssetWorkbenchSettlementStatusUnsettled || item.CurrentSettlementBatchID != nil {
			return nil, domain.NewAppError(domain.ErrCodeConflict, "blocked", nil)
		}
	}
	for _, item := range r.items {
		if item == nil || item.QCStatus == domain.AssetWorkbenchSubmissionStatusVoided {
			continue
		}
		item.QCStatus = domain.AssetWorkbenchSubmissionStatusVoided
		item.VoidedAt = &at
		item.VoidedBy = &actorID
		item.VoidReason = reason
	}
	r.submission.Status = domain.AssetWorkbenchSubmissionStatusVoided
	copySubmission := *r.submission
	return &copySubmission, nil
}

func (r *submissionVoidRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
}

func TestRegisterCreatesPendingProfileFromSelfRegistration(t *testing.T) {
	identity := &registerIdentityStub{}
	workbenchRepo := &registerProfileRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithIdentityRegistrar(identity),
	)

	result, appErr := svc.Register(context.Background(), RegisterParams{
		Account:       "piece_worker",
		Name:          "计件人员",
		Phone:         "13800000991",
		Password:      "Pass1234",
		WorkerType:    domain.AssetWorkbenchWorkerTypeParttime,
		Province:      "浙江",
		City:          "杭州",
		IDCard:        "330100199001010000",
		Gender:        "female",
		AlipayAccount: "piece-worker@example.com",
	})
	if appErr != nil {
		t.Fatalf("Register() error = %+v", appErr)
	}
	if result == nil || result.Auth == nil || result.Auth.Session == nil || result.Auth.Session.Token != "token-77" {
		t.Fatalf("Register() result = %+v", result)
	}
	if identity.params.username != "piece_worker" || identity.params.displayName != "计件人员" || identity.params.mobile != "13800000991" {
		t.Fatalf("identity params = %+v", identity.params)
	}
	if workbenchRepo.profile == nil {
		t.Fatal("profile was not saved")
	}
	if workbenchRepo.profile.UserID != 77 || workbenchRepo.profile.Status != domain.AssetWorkbenchProfileStatusPending {
		t.Fatalf("profile = %+v", workbenchRepo.profile)
	}
	if workbenchRepo.profile.WorkerType != domain.AssetWorkbenchWorkerTypeParttime || workbenchRepo.profile.JobGrade != "" {
		t.Fatalf("profile worker snapshot = %+v", workbenchRepo.profile)
	}
	if !workbenchRepo.profile.PIICompleted {
		t.Fatalf("profile should be marked PII completed when name, phone and id card are provided: %+v", workbenchRepo.profile)
	}
	if len(workbenchRepo.gradePeriods) != 1 || workbenchRepo.gradePeriods[0].WorkerType != domain.AssetWorkbenchWorkerTypeParttime {
		t.Fatalf("grade periods = %+v", workbenchRepo.gradePeriods)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventProfileUpserted {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestRegisterRejectsIncompleteProfileBeforeCreatingIdentity(t *testing.T) {
	identity := &registerIdentityStub{}
	workbenchRepo := &registerProfileRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithIdentityRegistrar(identity),
	)

	_, appErr := svc.Register(context.Background(), RegisterParams{
		Account:    "piece_worker",
		Name:       "计件人员",
		Phone:      "13800000991",
		Password:   "Pass1234",
		WorkerType: domain.AssetWorkbenchWorkerTypeParttime,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("Register() error = %+v, want invalid request", appErr)
	}
	if identity.params.username != "" {
		t.Fatalf("identity must not be created before profile validation: %+v", identity.params)
	}
	if workbenchRepo.profile != nil {
		t.Fatalf("incomplete profile must not be saved: %+v", workbenchRepo.profile)
	}
}

func TestListProfilesMasksPIIForListResponses(t *testing.T) {
	phone := "13800000991"
	idCard := "330100199001010000"
	workbenchRepo := &profileListRepo{items: []*domain.AssetWorkbenchProfile{
		{
			ID:            10,
			UserID:        77,
			WorkerType:    domain.AssetWorkbenchWorkerTypeParttime,
			JobGrade:      "J1",
			RealName:      "计件人员",
			Phone:         &phone,
			IDCard:        &idCard,
			AlipayAccount: "piece-worker@example.com",
			Status:        domain.AssetWorkbenchProfileStatusActive,
			PIICompleted:  true,
		},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	items, total, appErr := svc.ListProfiles(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleHRAdmin},
	}, repo.AssetWorkbenchProfileFilter{Page: 1, PageSize: 20})
	if appErr != nil {
		t.Fatalf("ListProfiles() error = %+v", appErr)
	}
	if total != 1 || len(items) != 1 {
		t.Fatalf("ListProfiles() total=%d items=%d", total, len(items))
	}
	if items[0].Phone == nil || *items[0].Phone == phone {
		t.Fatalf("phone should be masked, got %+v", items[0].Phone)
	}
	if items[0].IDCard == nil || *items[0].IDCard == idCard {
		t.Fatalf("id_card should be masked, got %+v", items[0].IDCard)
	}
	if items[0].AlipayAccount == "piece-worker@example.com" {
		t.Fatalf("alipay account should be masked, got %q", items[0].AlipayAccount)
	}
	if items[0].PIICompleted {
		t.Fatalf("legacy completion flag must be recalculated when required fields are missing: %+v", items[0])
	}
}

func TestGetProfileReturnsFullPIIAndAuditsAccess(t *testing.T) {
	phone := "13800000991"
	idCard := "330100199001010000"
	workbenchRepo := &profileListRepo{items: []*domain.AssetWorkbenchProfile{
		{
			ID:            10,
			UserID:        77,
			RealName:      "计件人员",
			Phone:         &phone,
			IDCard:        &idCard,
			AlipayAccount: "piece-worker@example.com",
		},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	profile, appErr := svc.GetProfile(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleHRAdmin},
	}, 77)
	if appErr != nil {
		t.Fatalf("GetProfile() error = %+v", appErr)
	}
	if profile.Phone == nil || *profile.Phone != phone || profile.IDCard == nil || *profile.IDCard != idCard || profile.AlipayAccount != "piece-worker@example.com" {
		t.Fatalf("GetProfile() should return complete PII, got %+v", profile)
	}
	if len(workbenchRepo.events) != 1 {
		t.Fatalf("events = %+v, want one profile access event", workbenchRepo.events)
	}
	event := workbenchRepo.events[0]
	if event.EventType != domain.AssetWorkbenchEventProfilePIIViewed || event.EntityType != domain.AssetWorkbenchEntityProfile || event.EntityID == nil || *event.EntityID != 10 {
		t.Fatalf("profile access event = %+v", event)
	}
	if len(event.Before) != 0 || len(event.After) != 0 {
		t.Fatalf("profile access audit must not copy PII snapshots: %+v", event)
	}
}

func TestGetProfileRejectsSubmitterPIIAccess(t *testing.T) {
	workbenchRepo := &profileListRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	_, appErr := svc.GetProfile(context.Background(), domain.RequestActor{
		ID:    77,
		Roles: []domain.Role{domain.RoleAssetSubmitter},
	}, 88)
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("GetProfile() error = %+v, want permission denied", appErr)
	}
}

func TestUpsertMyProfileRequiresEveryClientManagedField(t *testing.T) {
	workbenchRepo := &profileListRepo{}
	notifier := &profileNotificationStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithNotificationCreator(notifier),
	)

	_, appErr := svc.UpsertMyProfile(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}, UpsertProfileParams{
		RealName: "计件人员",
		Phone:    "13800000077",
		Province: "浙江",
		City:     "杭州",
		IDCard:   "330100199001010077",
		Gender:   "female",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("UpsertMyProfile() error = %+v, want invalid request", appErr)
	}
	if workbenchRepo.saved != nil {
		t.Fatalf("incomplete profile must not be saved: %+v", workbenchRepo.saved)
	}
	if len(notifier.calls) != 0 {
		t.Fatalf("rejected profile must not emit a completion notification: %+v", notifier.calls)
	}
}

func TestUpsertMyProfileValidatesIDCardAndGender(t *testing.T) {
	workbenchRepo := &profileListRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	valid := UpsertProfileParams{
		RealName:      "计件人员",
		Phone:         "13800000078",
		IDCard:        "330100199001010078",
		Province:      "浙江",
		City:          "杭州",
		Gender:        "male",
		AlipayAccount: "13800000078",
	}
	for name, mutate := range map[string]func(*UpsertProfileParams){
		"short id card":  func(params *UpsertProfileParams) { params.IDCard = "33010019900101007" },
		"id card with X": func(params *UpsertProfileParams) { params.IDCard = "33010019900101007X" },
		"unknown gender": func(params *UpsertProfileParams) { params.Gender = "unknown" },
	} {
		t.Run(name, func(t *testing.T) {
			params := valid
			mutate(&params)
			_, appErr := svc.UpsertMyProfile(context.Background(), domain.RequestActor{ID: 78, Roles: []domain.Role{domain.RoleAssetSubmitter}}, params)
			if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
				t.Fatalf("UpsertMyProfile() error = %+v, want invalid request", appErr)
			}
		})
	}
}

func TestUpsertMyProfileSavesCompleteClientProfile(t *testing.T) {
	workbenchRepo := &profileListRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	saved, appErr := svc.UpsertMyProfile(context.Background(), domain.RequestActor{ID: 79, Roles: []domain.Role{domain.RoleAssetSubmitter}}, UpsertProfileParams{
		RealName:      "计件人员",
		Phone:         "13800000079",
		Province:      "浙江",
		City:          "杭州",
		IDCard:        "330100199001010079",
		Gender:        "female",
		AlipayAccount: "13800000079",
	})
	if appErr != nil {
		t.Fatalf("UpsertMyProfile() error = %+v", appErr)
	}
	if saved == nil || !saved.PIICompleted {
		t.Fatalf("complete profile should be saved as complete: %+v", saved)
	}
}

func TestHRUpsertProfilePreservesExistingPIIWhenOmitted(t *testing.T) {
	phone := "13800000991"
	idCard := "330100199001010000"
	onboardedAt := time.Date(2026, 5, 1, 0, 0, 0, 0, time.UTC)
	workbenchRepo := &profileListRepo{items: []*domain.AssetWorkbenchProfile{
		{
			ID:            10,
			UserID:        77,
			WorkerType:    domain.AssetWorkbenchWorkerTypeParttime,
			JobGrade:      "J1",
			RealName:      "计件人员",
			Phone:         &phone,
			IDCard:        &idCard,
			Gender:        "female",
			AlipayAccount: "piece-worker@example.com",
			OnboardedAt:   &onboardedAt,
			Status:        domain.AssetWorkbenchProfileStatusPending,
		},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	saved, appErr := svc.HRUpsertProfile(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleHRAdmin},
	}, 77, UpsertProfileParams{
		WorkerType: domain.AssetWorkbenchWorkerTypeFulltime,
		JobGrade:   "P1",
		RealName:   "计件人员",
		Province:   "浙江",
		City:       "杭州",
		Status:     domain.AssetWorkbenchProfileStatusActive,
		Reason:     "HR 定级",
	})
	if appErr != nil {
		t.Fatalf("HRUpsertProfile() error = %+v", appErr)
	}
	if saved.Phone == nil || *saved.Phone != phone {
		t.Fatalf("phone should be preserved, got %+v", saved.Phone)
	}
	if saved.IDCard == nil || *saved.IDCard != idCard {
		t.Fatalf("id_card should be preserved, got %+v", saved.IDCard)
	}
	if saved.AlipayAccount != "piece-worker@example.com" || saved.Gender != "female" || saved.OnboardedAt == nil || !saved.OnboardedAt.Equal(onboardedAt) {
		t.Fatalf("PII should be preserved, got %+v", saved)
	}
	if !saved.PIICompleted {
		t.Fatalf("preserved PII should keep profile completed: %+v", saved)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].Reason != "HR 定级" {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestHRUpsertProfileAutoRepricesPendingGradeItems(t *testing.T) {
	submittedAt := time.Date(2026, 7, 1, 14, 56, 13, 0, time.UTC)
	workbenchRepo := &profileListRepo{
		items: []*domain.AssetWorkbenchProfile{
			{
				UserID:     77,
				WorkerType: domain.AssetWorkbenchWorkerTypeParttime,
				Status:     domain.AssetWorkbenchProfileStatusPending,
			},
		},
		pendingItems: []*domain.AssetWorkbenchSubmissionItem{
			{
				ID:               501,
				SubmissionID:     9001,
				PayeeUserID:      77,
				OrderNo:          "6954064249637049871",
				DifficultyClass:  "A",
				Finalized:        true,
				PageCount:        2,
				ItemCount:        1,
				BusinessMonth:    "2026-07",
				SubmittedAt:      submittedAt,
				PricingStatus:    domain.AssetWorkbenchPricingStatusPendingGrade,
				QCStatus:         domain.AssetWorkbenchSubmissionStatusSubmitted,
				SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
			},
		},
		price: &domain.AssetWorkbenchPriceMatrix{
			ID:              99,
			WorkerType:      domain.AssetWorkbenchWorkerTypeParttime,
			JobGrade:        "J1",
			DifficultyClass: "A",
			UnitPrice:       1.14,
			EffectiveFrom:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	_, appErr := svc.HRUpsertProfile(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleHRAdmin},
	}, 77, UpsertProfileParams{
		WorkerType: domain.AssetWorkbenchWorkerTypeParttime,
		JobGrade:   "J1",
		RealName:   "计件人员",
		Status:     domain.AssetWorkbenchProfileStatusActive,
		Reason:     "HR 定级",
	})
	if appErr != nil {
		t.Fatalf("HRUpsertProfile() error = %+v", appErr)
	}
	item := workbenchRepo.pendingItems[0]
	if item.WorkerTypeSnapshot != domain.AssetWorkbenchWorkerTypeParttime || item.JobGradeSnapshot != "J1" {
		t.Fatalf("snapshots = %q/%q, want parttime/J1", item.WorkerTypeSnapshot, item.JobGradeSnapshot)
	}
	if item.PricingStatus != domain.AssetWorkbenchPricingStatusPriced || item.GrossAmount != 2.28 {
		t.Fatalf("pricing = %s gross=%v, want priced 2.28", item.PricingStatus, item.GrossAmount)
	}
	if item.BasePriceRuleID == nil || *item.BasePriceRuleID != 99 || item.BaseUnitPrice == nil || *item.BaseUnitPrice != 1.14 {
		t.Fatalf("price snapshot rule=%v unit=%v, want 99/1.14", item.BasePriceRuleID, item.BaseUnitPrice)
	}
	if len(workbenchRepo.refreshed) != 1 || workbenchRepo.refreshed[0] != 9001 {
		t.Fatalf("refreshed = %+v, want [9001]", workbenchRepo.refreshed)
	}
	if len(workbenchRepo.events) != 2 ||
		workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventProfileUpserted ||
		workbenchRepo.events[1].EventType != domain.AssetWorkbenchEventItemRepriced {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestGetFileDownloadSignsVisibleFileAndWritesEvent(t *testing.T) {
	workbenchRepo := &downloadFileRepo{files: map[int64]*domain.AssetWorkbenchSubmissionFile{
		10: {
			ID:               10,
			SubmissionID:     20,
			SubmissionItemID: 30,
			OwnerUserID:      77,
			ObjectKey:        "asset-workbench/uploads/2026/06/s1/final.psd",
			OriginalFilename: "final.psd",
			MimeType:         "image/vnd.adobe.photoshop",
			FileSize:         1024,
		},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithOSSDirect(testWorkbenchOSSDirect()),
	)

	meta, appErr := svc.GetFileDownload(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}, 10)
	if appErr != nil {
		t.Fatalf("GetFileDownload() error = %+v", appErr)
	}
	if meta.DownloadURL == "" || meta.Filename != "final.psd" || meta.FileID != 10 {
		t.Fatalf("download meta = %+v", meta)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventFileDownloaded {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestBatchDownloadManifestSkipsInvisibleAndDeduplicatesNames(t *testing.T) {
	workbenchRepo := &downloadFileRepo{files: map[int64]*domain.AssetWorkbenchSubmissionFile{
		10: {
			ID:               10,
			SubmissionID:     20,
			SubmissionItemID: 30,
			OwnerUserID:      77,
			ObjectKey:        "asset-workbench/uploads/2026/06/s1/final.psd",
			OriginalFilename: "final.psd",
		},
		11: {
			ID:               11,
			SubmissionID:     21,
			SubmissionItemID: 31,
			OwnerUserID:      77,
			ObjectKey:        "asset-workbench/uploads/2026/06/s2/final.psd",
			OriginalFilename: "final.psd",
		},
		12: {
			ID:               12,
			SubmissionID:     22,
			SubmissionItemID: 32,
			OwnerUserID:      88,
			ObjectKey:        "asset-workbench/uploads/2026/06/s3/hidden.psd",
			OriginalFilename: "hidden.psd",
		},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithOSSDirect(testWorkbenchOSSDirect()),
	)

	manifest, appErr := svc.BuildFileBatchDownloadManifest(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}, []int64{10, 11, 12})
	if appErr != nil {
		t.Fatalf("BuildFileBatchDownloadManifest() error = %+v", appErr)
	}
	if len(manifest.Items) != 2 || len(manifest.Failures) != 1 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if manifest.Items[0].Filename != "final.psd" || manifest.Items[1].Filename != "final-2.psd" {
		t.Fatalf("filenames = %+v", manifest.Items)
	}
	if manifest.Failures[0].FileID != 12 || manifest.Failures[0].Reason != "not_visible" {
		t.Fatalf("failures = %+v", manifest.Failures)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventFileBatchDownloaded {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestImportErrorRecordsMatchesUniqueItemsAndCountsAmbiguous(t *testing.T) {
	payee100 := int64(100)
	workbenchRepo := &errorImportRepo{items: []*domain.AssetWorkbenchSubmissionItem{
		{ID: 501, PayeeUserID: 100, OrderNo: "ORD-1", QCStatus: domain.AssetWorkbenchSubmissionStatusSubmitted},
		{ID: 502, PayeeUserID: 101, OrderNo: "ORD-1", QCStatus: domain.AssetWorkbenchSubmissionStatusSubmitted},
		{ID: 503, PayeeUserID: 100, OrderNo: "ORD-2", QCStatus: domain.AssetWorkbenchSubmissionStatusChecked},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	batch, appErr := svc.ImportErrorRecords(context.Background(), actor, ImportErrorRecordsParams{
		BusinessMonth:    "2026-06",
		OriginalFilename: "errors.xlsx",
		Records: []ImportErrorRecordInput{
			{OrderNo: "ORD-1", PayeeUserID: &payee100, ErrorCount: 2},
			{OrderNo: "ORD-1", ErrorCount: 3},
			{OrderNo: "MISS", ErrorCount: 1},
		},
	})
	if appErr != nil {
		t.Fatalf("ImportErrorRecords() error = %+v", appErr)
	}
	if batch.MatchedRows != 1 || batch.AmbiguousRows != 1 || batch.UnmatchedRows != 1 {
		t.Fatalf("batch counts = matched:%d ambiguous:%d unmatched:%d", batch.MatchedRows, batch.AmbiguousRows, batch.UnmatchedRows)
	}
	if len(workbenchRepo.records) != 3 {
		t.Fatalf("records = %+v", workbenchRepo.records)
	}
	if workbenchRepo.records[0].MatchStatus != domain.AssetWorkbenchErrorMatchStatusMatched || workbenchRepo.records[0].SubmissionItemID == nil || *workbenchRepo.records[0].SubmissionItemID != 501 {
		t.Fatalf("matched record = %+v", workbenchRepo.records[0])
	}
	if workbenchRepo.records[1].MatchStatus != domain.AssetWorkbenchErrorMatchStatusAmbiguous || workbenchRepo.records[1].SubmissionItemID != nil {
		t.Fatalf("ambiguous record = %+v", workbenchRepo.records[1])
	}
	if workbenchRepo.records[2].MatchStatus != domain.AssetWorkbenchErrorMatchStatusUnmatched || workbenchRepo.records[2].SubmissionItemID != nil {
		t.Fatalf("unmatched record = %+v", workbenchRepo.records[2])
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventErrorImportCreated {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestImportErrorRecordsExcelMatchesQualityTemplateByPayeeAndDifficulty(t *testing.T) {
	workbenchRepo := &errorImportRepo{
		profiles: []*domain.AssetWorkbenchProfile{
			{UserID: 100, RealName: "张三", WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "J1"},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}
	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	_ = f.SetSheetRow(sheet, "A1", &[]interface{}{"说明：", "格式", "", "关联我们的分类，用于计价的", "绑定系统全职/兼职注册人", "文字", "选其一", "文字", "文字", "文字", "隐藏"})
	_ = f.SetSheetRow(sheet, "A2", &[]interface{}{"导入模板：", "日期", "线上订单号", "分类", "出错人", "问题描述", "抽查/售后", "处理方法", "登记人", "备注", "出错数"})
	_ = f.SetSheetRow(sheet, "A3", &[]interface{}{"", 46204, "3310254339917022991", "A类", "张三", "年龄做错了", "抽查", "重修", "李四", "首版", 8})
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	batch, appErr := svc.ImportErrorRecordsExcel(context.Background(), actor, "2026-07", "quality-errors.xlsx", bytes.NewReader(buf.Bytes()))
	if appErr != nil {
		t.Fatalf("ImportErrorRecordsExcel() error = %+v", appErr)
	}
	if batch.MatchedRows != 1 || batch.UnmatchedRows != 0 || batch.AmbiguousRows != 0 {
		t.Fatalf("batch = %+v, want one matched row", batch)
	}
	if len(workbenchRepo.records) != 1 {
		t.Fatalf("records = %+v", workbenchRepo.records)
	}
	record := workbenchRepo.records[0]
	if record.PayeeUserID == nil || *record.PayeeUserID != 100 || record.DifficultyClass != "A" || record.ErrorCount != 8 {
		t.Fatalf("record = %+v, want payee 100 A x8", record)
	}
	if record.OccurredDate == nil || record.OccurredDate.Format("2006-01-02") != "2026-07-01" {
		t.Fatalf("occurred date = %v, want 2026-07-01", record.OccurredDate)
	}
}

func TestCreateSettlementSupplementWritesDuplicateHintWithoutBlocking(t *testing.T) {
	workbenchRepo := &supplementRepo{
		items: []*domain.AssetWorkbenchSubmissionItem{
			{ID: 501, PayeeUserID: 1001, OrderNo: "ORD-1", QCStatus: domain.AssetWorkbenchSubmissionStatusChecked},
		},
		supplements: []*domain.AssetWorkbenchSettlementSupplement{
			{ID: 601, PayeeUserID: 1001, BusinessMonth: "2026-06", OrderNo: "ORD-1", Status: domain.AssetWorkbenchSupplementStatusApproved},
		},
		permissions: []*domain.AssetWorkbenchSupplementPermission{
			{ID: 1, PayeeUserID: 1001, BusinessMonth: "2026-06", Enabled: true, GrantedBy: 99},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 6, 20, 4, 0, 0, 0, time.UTC) }
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	created, appErr := svc.CreateSettlementSupplement(context.Background(), actor, CreateSettlementSupplementParams{
		PayeeUserID:     1001,
		BusinessMonth:   "2026-06",
		OrderNo:         "ORD-1",
		SupplementDate:  "2026-06-15",
		DifficultyClass: "A",
		PageCount:       1,
		GrossAmount:     12,
		Status:          domain.AssetWorkbenchSupplementStatusApproved,
	})
	if appErr != nil {
		t.Fatalf("CreateSettlementSupplement() error = %+v", appErr)
	}
	var hint map[string]interface{}
	if err := json.Unmarshal(created.DuplicateHint, &hint); err != nil {
		t.Fatalf("duplicate hint json: %v", err)
	}
	if hint["has_duplicates"] != true {
		t.Fatalf("duplicate hint = %#v", hint)
	}
	if hint["supplement_date"] != "2026-06-15" {
		t.Fatalf("supplement_date hint = %#v, want 2026-06-15", hint["supplement_date"])
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSupplementCreated {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestCreateSettlementSupplementAllowsTargetDateOutsideCurrentBusinessMonth(t *testing.T) {
	workbenchRepo := &supplementRepo{
		permissions: []*domain.AssetWorkbenchSupplementPermission{
			{ID: 1, PayeeUserID: 1001, BusinessMonth: "2026-07", Enabled: true, GrantedBy: 99},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC) }
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	created, appErr := svc.CreateSettlementSupplement(context.Background(), actor, CreateSettlementSupplementParams{
		PayeeUserID:     1001,
		BusinessMonth:   "2026-07",
		OrderNo:         "ORD-1",
		SupplementDate:  "2026-06-15",
		DifficultyClass: "A",
		PageCount:       1,
		GrossAmount:     12,
		Status:          domain.AssetWorkbenchSupplementStatusApproved,
	})
	if appErr != nil {
		t.Fatalf("CreateSettlementSupplement() error = %+v", appErr)
	}
	if created.BusinessMonth != "2026-07" || created.SupplementDate != "2026-06-15" {
		t.Fatalf("created = %+v, want July settlement with June target date", created)
	}
}

func TestCreateSettlementSupplementLetsPayeeUploadPricedFileWithoutNormalPiecework(t *testing.T) {
	permission := &domain.AssetWorkbenchSupplementPermission{ID: 1, PayeeUserID: 1001, BusinessMonth: "2026-07", Enabled: true, GrantedBy: 99}
	workbenchRepo := &submissionDirectoryDifficultyRepo{
		profile: &domain.AssetWorkbenchProfile{UserID: 1001, WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "P1"},
		price:   &domain.AssetWorkbenchPriceMatrix{ID: 88, WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "P1", DifficultyClass: "A", UnitPrice: 12},
		session: &domain.AssetWorkbenchUploadSession{
			ID: 9101, SessionID: "supplement-upload-1", OwnerUserID: 1001, Status: domain.AssetWorkbenchUploadStatusUploaded,
			ObjectKey: "asset-workbench/uploads/supplement/poster.jpg", OriginalFilename: "poster.jpg", MimeType: "image/jpeg", FileSize: 2048,
			UploadDirectoryName: "A类成品", UploadDirectoryDifficultyClass: "A",
		},
		permissions: []*domain.AssetWorkbenchSupplementPermission{permission},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC) }
	actor := domain.RequestActor{ID: 1001, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	created, appErr := svc.CreateSettlementSupplement(context.Background(), actor, CreateSettlementSupplementParams{
		PayeeUserID:      1001,
		BusinessMonth:    "2026-07",
		SupplementDate:   "2026-06-15",
		Finalized:        true,
		PageCount:        1,
		UploadSessionIDs: []string{"supplement-upload-1"},
	})
	if appErr != nil {
		t.Fatalf("CreateSettlementSupplement(upload) error = %+v", appErr)
	}
	if created.SubmissionItemID == nil || *created.SubmissionItemID != 6001 || created.OrderNo != "poster.jpg" || created.DifficultyClass != "A" || created.GrossAmount != 12 {
		t.Fatalf("created supplement = %+v, want linked A-class item worth 12", created)
	}
	if workbenchRepo.item == nil || workbenchRepo.item.EntryKind != domain.AssetWorkbenchSubmissionEntryKindSupplement || workbenchRepo.item.GrossAmount != 12 {
		t.Fatalf("submission item = %+v, want supplement entry kind and priced amount", workbenchRepo.item)
	}
	if len(created.Files) != 1 || created.Files[0].OriginalFilename != "poster.jpg" || created.Files[0].UploadDirectoryName != "A类成品" {
		t.Fatalf("created files = %+v, want linked uploaded file", created.Files)
	}
	if workbenchRepo.sessionStatus != domain.AssetWorkbenchUploadStatusSubmitted {
		t.Fatalf("upload session status = %q, want submitted", workbenchRepo.sessionStatus)
	}
	if len(workbenchRepo.events) != 2 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSubmissionCreated || workbenchRepo.events[1].EventType != domain.AssetWorkbenchEventSupplementCreated {
		t.Fatalf("events = %+v, want submission and supplement events", workbenchRepo.events)
	}
}

func TestCreateSettlementSupplementRejectsSelfUploadWhenPermissionClosesBeforeWrite(t *testing.T) {
	closed := false
	workbenchRepo := &submissionDirectoryDifficultyRepo{
		profile: &domain.AssetWorkbenchProfile{UserID: 1001, WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "P1"},
		price:   &domain.AssetWorkbenchPriceMatrix{ID: 88, WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "P1", DifficultyClass: "A", UnitPrice: 12},
		session: &domain.AssetWorkbenchUploadSession{
			ID: 9101, SessionID: "supplement-upload-closed", OwnerUserID: 1001, Status: domain.AssetWorkbenchUploadStatusUploaded,
			OriginalFilename: "poster.jpg", UploadDirectoryDifficultyClass: "A",
		},
		permissions:             []*domain.AssetWorkbenchSupplementPermission{{ID: 1, PayeeUserID: 1001, BusinessMonth: "2026-07", Enabled: true}},
		lockedPermissionEnabled: &closed,
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC) }

	_, appErr := svc.CreateSettlementSupplement(context.Background(), domain.RequestActor{ID: 1001, Roles: []domain.Role{domain.RoleAssetSubmitter}}, CreateSettlementSupplementParams{
		PayeeUserID: 1001, BusinessMonth: "2026-07", SupplementDate: "2026-06-15", PageCount: 1,
		UploadSessionIDs: []string{"supplement-upload-closed"},
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateSettlementSupplement(closed race) error = %+v, want permission denied", appErr)
	}
	if workbenchRepo.submission != nil || workbenchRepo.item != nil || workbenchRepo.createdSupplement != nil || len(workbenchRepo.files) != 0 {
		t.Fatalf("closed permission must leave no database writes: submission=%+v item=%+v supplement=%+v files=%+v", workbenchRepo.submission, workbenchRepo.item, workbenchRepo.createdSupplement, workbenchRepo.files)
	}
}

func TestCreateSettlementSupplementRejectsPayeeSpoofing(t *testing.T) {
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(&supplementRepo{}, assetWorkbenchTestTxRunner{}))
	_, appErr := svc.CreateSettlementSupplement(context.Background(), domain.RequestActor{ID: 1001, Roles: []domain.Role{domain.RoleAssetSubmitter}}, CreateSettlementSupplementParams{
		PayeeUserID: 2002, BusinessMonth: "2026-07", OrderNo: "poster.jpg", DifficultyClass: "A", UploadSessionIDs: []string{"session"},
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateSettlementSupplement(other payee) error = %+v, want permission denied", appErr)
	}
}

func TestListSettlementSupplementsNormalizesDateFilterAndSort(t *testing.T) {
	workbenchRepo := &supplementRepo{
		supplements: []*domain.AssetWorkbenchSettlementSupplement{
			{ID: 601, PayeeUserID: 1001, BusinessMonth: "2026-06", OrderNo: "海报.jpg", SupplementDate: "2026-06-15", Status: domain.AssetWorkbenchSupplementStatusApproved},
			{ID: 602, PayeeUserID: 1001, BusinessMonth: "2026-06", OrderNo: "挂布.jpg", SupplementDate: "2026-06-16", Status: domain.AssetWorkbenchSupplementStatusApproved},
			{ID: 603, PayeeUserID: 1002, BusinessMonth: "2026-06", OrderNo: "海报.jpg", SupplementDate: "2026-06-15", Status: domain.AssetWorkbenchSupplementStatusVoided},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	items, total, appErr := svc.ListSettlementSupplements(context.Background(), actor, repo.AssetWorkbenchSettlementSupplementFilter{
		BusinessMonth:      "2026-06",
		SupplementDateFrom: "2026/06/15",
		SupplementDateTo:   "2026.06.15",
		Status:             domain.AssetWorkbenchSupplementStatusApproved,
		SortBy:             "supplement_date",
		SortDir:            "asc",
		Page:               1,
		PageSize:           20,
	})
	if appErr != nil {
		t.Fatalf("ListSettlementSupplements() error = %+v", appErr)
	}
	if total != 1 || len(items) != 1 || items[0].ID != 601 {
		t.Fatalf("items=%+v total=%d, want only approved 2026-06-15 row", items, total)
	}
	if workbenchRepo.lastListFilter.SupplementDateFrom != "2026-06-15" ||
		workbenchRepo.lastListFilter.SupplementDateTo != "2026-06-15" ||
		workbenchRepo.lastListFilter.SortBy != "supplement_date" ||
		workbenchRepo.lastListFilter.SortDir != "asc" {
		t.Fatalf("repo filter = %+v, want normalized date range and sort", workbenchRepo.lastListFilter)
	}
}

func TestListSettlementSupplementsRejectsInvalidDateAndSort(t *testing.T) {
	workbenchRepo := &supplementRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	_, _, appErr := svc.ListSettlementSupplements(context.Background(), actor, repo.AssetWorkbenchSettlementSupplementFilter{
		SupplementDate: "2026-13-01",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("ListSettlementSupplements(invalid date) error = %+v, want invalid request", appErr)
	}

	_, _, appErr = svc.ListSettlementSupplements(context.Background(), actor, repo.AssetWorkbenchSettlementSupplementFilter{
		SortBy: "gross_amount; DROP TABLE asset_workbench_settlement_supplements",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("ListSettlementSupplements(invalid sort) error = %+v, want invalid request", appErr)
	}
}

func TestBatchDeleteSettlementSupplementsDeletesFilesItemsAndAmounts(t *testing.T) {
	linkedItemID := int64(501)
	workbenchRepo := &supplementRepo{
		supplements: []*domain.AssetWorkbenchSettlementSupplement{
			{ID: 601, SubmissionItemID: &linkedItemID, PayeeUserID: 1001, BusinessMonth: "2026-07", OrderNo: "wrong.jpg", Status: domain.AssetWorkbenchSupplementStatusApproved, GrossAmount: 12},
			{ID: 602, PayeeUserID: 1001, BusinessMonth: "2026-07", OrderNo: "manual", Status: domain.AssetWorkbenchSupplementStatusDraft, GrossAmount: 3},
		},
		filesByItem: map[int64][]*domain.AssetWorkbenchSubmissionFile{
			501: {{ID: 701, SubmissionItemID: 501, OwnerUserID: 1001, OriginalFilename: "wrong.jpg"}},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 1001, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	result, appErr := svc.BatchDeleteSettlementSupplements(context.Background(), actor, BatchDeleteSettlementSupplementsParams{
		SupplementIDs: []int64{601, 602, 601},
		Reason:        "上传错文件",
	})
	if appErr != nil {
		t.Fatalf("BatchDeleteSettlementSupplements() error = %+v", appErr)
	}
	if fmt.Sprint(result.DeletedIDs) != "[601 602]" || len(result.Supplements) != 2 {
		t.Fatalf("result = %+v, want two unique deleted supplements", result)
	}
	if fmt.Sprint(workbenchRepo.deletedFileIDs) != "[701]" || fmt.Sprint(workbenchRepo.voidedItemIDs) != "[501]" || len(workbenchRepo.refreshedIDs) != 1 {
		t.Fatalf("files=%v items=%v refresh=%v", workbenchRepo.deletedFileIDs, workbenchRepo.voidedItemIDs, workbenchRepo.refreshedIDs)
	}
	for _, row := range workbenchRepo.supplements {
		if row.Status != domain.AssetWorkbenchSupplementStatusVoided {
			t.Fatalf("supplement %d status = %q, want voided", row.ID, row.Status)
		}
	}
}

func TestBatchDeleteSettlementSupplementsRejectsOtherPayeeAndLockedRows(t *testing.T) {
	workbenchRepo := &supplementRepo{
		supplements: []*domain.AssetWorkbenchSettlementSupplement{
			{ID: 601, PayeeUserID: 2002, Status: domain.AssetWorkbenchSupplementStatusApproved},
			{ID: 602, PayeeUserID: 1001, Status: domain.AssetWorkbenchSupplementStatusInBatch},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 1001, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	if _, appErr := svc.BatchDeleteSettlementSupplements(context.Background(), actor, BatchDeleteSettlementSupplementsParams{SupplementIDs: []int64{601}, Reason: "错传"}); appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("other payee error = %+v, want permission denied", appErr)
	}
	admin := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}
	if _, appErr := svc.BatchDeleteSettlementSupplements(context.Background(), admin, BatchDeleteSettlementSupplementsParams{SupplementIDs: []int64{602}, Reason: "错传"}); appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("locked row error = %+v, want conflict", appErr)
	}
	if _, appErr := svc.BatchDeleteSettlementSupplements(context.Background(), admin, BatchDeleteSettlementSupplementsParams{SupplementIDs: []int64{602}, Reason: " "}); appErr == nil || appErr.Code != domain.ErrCodeReasonRequired {
		t.Fatalf("empty reason error = %+v, want reason required", appErr)
	}
}

func TestBootstrapTreatsTwoPayrollRowsAsActiveContract(t *testing.T) {
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(&bootstrapAccessRepo{}, assetWorkbenchTestTxRunner{}))
	result, appErr := svc.Bootstrap(context.Background(), domain.RequestActor{
		ID:    1001,
		Roles: []domain.Role{domain.RoleAssetSubmitter},
	})
	if appErr != nil {
		t.Fatalf("Bootstrap() error = %+v", appErr)
	}
	for _, item := range result.DeferredBusinessItems {
		if item.Key == "employee_month_two_rows" {
			t.Fatalf("employee_month_two_rows should not be deferred after payroll_rows contract was implemented: %+v", result.DeferredBusinessItems)
		}
	}
	if !containsString(result.ArchitectureGuardrails, "payroll_rows always emit normal piecework and supplement piecework rows per payee/month") {
		t.Fatalf("guardrails = %+v, want payroll_rows contract", result.ArchitectureGuardrails)
	}
}

func TestBootstrapDerivesWorkbenchCapabilitiesForHRAndSuperAdmin(t *testing.T) {
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(&bootstrapAccessRepo{}, assetWorkbenchTestTxRunner{}))
	hrResult, appErr := svc.Bootstrap(context.Background(), domain.RequestActor{
		ID:    1001,
		Roles: []domain.Role{domain.RoleHRAdmin},
	})
	if appErr != nil {
		t.Fatalf("Bootstrap(HRAdmin) error = %+v", appErr)
	}
	if !containsString(hrResult.Capabilities, "asset.workbench.profile.manage") {
		t.Fatalf("HR capabilities = %+v, want profile.manage", hrResult.Capabilities)
	}

	superResult, appErr := svc.Bootstrap(context.Background(), domain.RequestActor{
		ID:    1002,
		Roles: []domain.Role{domain.RoleSuperAdmin},
	})
	if appErr != nil {
		t.Fatalf("Bootstrap(SuperAdmin) error = %+v", appErr)
	}
	for _, capability := range []string{
		"asset.workbench.submit",
		"asset.workbench.system_search",
		"asset.workbench.cost_center.manage",
		"asset.workbench.settlement",
	} {
		if !containsString(superResult.Capabilities, capability) {
			t.Fatalf("super admin capabilities = %+v, missing %s", superResult.Capabilities, capability)
		}
	}
}

func TestEntryAutoOpensMembershipForExistingAssetRoles(t *testing.T) {
	workbenchRepo := &roleBackfillAccessRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))

	result, appErr := svc.Entry(context.Background(), domain.RequestActor{
		ID:    303,
		Roles: []domain.Role{domain.RoleAssetSubmitter, domain.RoleAssetManager},
	})
	if appErr != nil {
		t.Fatalf("Entry() error = %+v", appErr)
	}
	if result == nil || result.State != "ready" {
		t.Fatalf("entry result = %+v, want ready", result)
	}
	if workbenchRepo.membership == nil || workbenchRepo.membership.Status != domain.AppMembershipStatusActive {
		t.Fatalf("membership = %+v, want active", workbenchRepo.membership)
	}
	if workbenchRepo.membership.Source != domain.AppMembershipSourceMainOpsOpened {
		t.Fatalf("membership source = %q, want %q", workbenchRepo.membership.Source, domain.AppMembershipSourceMainOpsOpened)
	}
	if workbenchRepo.membership.IdentityType != domain.AppMembershipIdentityStaff {
		t.Fatalf("membership identity = %q, want %q", workbenchRepo.membership.IdentityType, domain.AppMembershipIdentityStaff)
	}
	if workbenchRepo.event == nil || workbenchRepo.event.Action != domain.AppIdentityActionAccessOpened {
		t.Fatalf("identity event = %+v, want access opened", workbenchRepo.event)
	}
}

func TestUpdateMemberRolesReloadsMemberByUserID(t *testing.T) {
	workbenchRepo := &memberRoleUpdateRepo{}
	userRepo := &memberRoleUpdateUserRepo{roles: []domain.Role{domain.RoleAssetSubmitter}}
	svc := NewService(
		Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithUserRepository(userRepo),
	)

	member, appErr := svc.UpdateMemberRoles(context.Background(), domain.RequestActor{
		ID:    1,
		Roles: []domain.Role{domain.RoleSuperAdmin},
	}, 302, UpdateMemberRolesParams{
		Roles:  []domain.Role{domain.RoleAssetSubmitter, domain.RoleAssetManager},
		Reason: "workbench role update",
	})
	if appErr != nil {
		t.Fatalf("UpdateMemberRoles() error = %+v", appErr)
	}
	if member == nil || member.UserID != 302 {
		t.Fatalf("member = %+v, want user_id=302", member)
	}
	if userRepo.replacedUserID != 302 || !containsRole(userRepo.replacedRoles, domain.RoleAssetManager) {
		t.Fatalf("replaced roles user=%d roles=%+v, want AssetManager for user 302", userRepo.replacedUserID, userRepo.replacedRoles)
	}
	if len(workbenchRepo.listFilters) != 1 {
		t.Fatalf("ListMembers calls = %d, want 1", len(workbenchRepo.listFilters))
	}
	if got := workbenchRepo.listFilters[0]; got.UserID != 302 || got.Keyword != "" {
		t.Fatalf("reload filter = %+v, want exact UserID without keyword", got)
	}
	if workbenchRepo.event == nil || workbenchRepo.event.Action != domain.AppIdentityActionRolesUpdated {
		t.Fatalf("identity event = %+v", workbenchRepo.event)
	}
}

func TestCreateSettlementSupplementRequiresOpenPermission(t *testing.T) {
	workbenchRepo := &supplementRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 6, 20, 4, 0, 0, 0, time.UTC) }
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	_, appErr := svc.CreateSettlementSupplement(context.Background(), actor, CreateSettlementSupplementParams{
		PayeeUserID:     1001,
		BusinessMonth:   "2026-06",
		OrderNo:         "ORD-1",
		DifficultyClass: "A",
		PageCount:       1,
		GrossAmount:     12,
		Status:          domain.AssetWorkbenchSupplementStatusApproved,
	})
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("CreateSettlementSupplement() error = %+v, want permission denied", appErr)
	}
	if workbenchRepo.created != nil {
		t.Fatalf("created supplement without permission: %+v", workbenchRepo.created)
	}
}

func TestUpsertSupplementPermissionWritesEvent(t *testing.T) {
	workbenchRepo := &supplementRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 6, 20, 4, 0, 0, 0, time.UTC) }
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	saved, appErr := svc.UpsertSupplementPermission(context.Background(), actor, UpsertSupplementPermissionParams{
		PayeeUserID:   1001,
		BusinessMonth: "2026-06",
		Enabled:       true,
		Reason:        "漏上传补录",
	})
	if appErr != nil {
		t.Fatalf("UpsertSupplementPermission() error = %+v", appErr)
	}
	if saved.PayeeUserID != 1001 || !saved.Enabled {
		t.Fatalf("saved permission = %+v", saved)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSupplementPermissionChanged {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestUpsertSupplementPermissionRejectsNonCurrentNaturalMonth(t *testing.T) {
	workbenchRepo := &supplementRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 7, 10, 2, 0, 0, 0, time.UTC) }
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	_, appErr := svc.UpsertSupplementPermission(context.Background(), actor, UpsertSupplementPermissionParams{
		PayeeUserID:   1001,
		BusinessMonth: "2026-06",
		Enabled:       true,
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("UpsertSupplementPermission() error = %+v, want invalid request", appErr)
	}
	if len(workbenchRepo.permissions) != 0 || len(workbenchRepo.events) != 0 {
		t.Fatalf("permission should not be saved for a past month: permissions=%+v events=%+v", workbenchRepo.permissions, workbenchRepo.events)
	}
}

func TestListSupplementEligibleMonthsReturnsCurrentNaturalMonth(t *testing.T) {
	workbenchRepo := &supplementRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time { return time.Date(2026, 7, 31, 16, 30, 0, 0, time.UTC) }
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	months, appErr := svc.ListSupplementEligibleMonths(context.Background(), actor, 1001)
	if appErr != nil {
		t.Fatalf("ListSupplementEligibleMonths() error = %+v", appErr)
	}
	if len(months) != 1 || months[0] != "2026-08" {
		t.Fatalf("months = %+v, want Asia/Shanghai current month 2026-08", months)
	}
}

func TestCreateSettlementAdjustmentRequiresConfirmedBatch(t *testing.T) {
	workbenchRepo := &adjustmentRepo{batch: &domain.AssetWorkbenchSettlementBatch{
		ID:            9001,
		BatchNo:       "AWB-1",
		BusinessMonth: "2026-06",
		Status:        domain.AssetWorkbenchBatchStatusGenerated,
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	_, appErr := svc.CreateSettlementAdjustment(context.Background(), actor, CreateSettlementAdjustmentParams{
		BatchID:     9001,
		PayeeUserID: 1001,
		Amount:      12,
		Reason:      "已生成批次应取消后重结算",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("CreateSettlementAdjustment(generated batch) appErr = %+v", appErr)
	}
	if workbenchRepo.adjustment != nil || workbenchRepo.settlementItem != nil {
		t.Fatalf("adjustment should not be created for generated batch: adj=%+v item=%+v", workbenchRepo.adjustment, workbenchRepo.settlementItem)
	}
}

func TestCreateSettlementAdjustmentAppendsItemAndSignedTotals(t *testing.T) {
	workbenchRepo := &adjustmentRepo{batch: &domain.AssetWorkbenchSettlementBatch{
		ID:            9001,
		BatchNo:       "AWB-1",
		BusinessMonth: "2026-06",
		Status:        domain.AssetWorkbenchBatchStatusConfirmed,
		NetAmount:     100,
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	created, appErr := svc.CreateSettlementAdjustment(context.Background(), actor, CreateSettlementAdjustmentParams{
		BatchID:        9001,
		PayeeUserID:    1001,
		AdjustmentType: domain.AssetWorkbenchAdjustmentTypeReversal,
		Amount:         12,
		Reason:         "重复结算冲正",
	})
	if appErr != nil {
		t.Fatalf("CreateSettlementAdjustment() error = %+v", appErr)
	}
	if created.Amount != -12 || created.AdjustmentType != domain.AssetWorkbenchAdjustmentTypeReversal {
		t.Fatalf("adjustment = %+v", created)
	}
	if workbenchRepo.settlementItem == nil || workbenchRepo.settlementItem.ItemType != domain.AssetWorkbenchItemTypeReversal || workbenchRepo.settlementItem.Direction != "debit" || workbenchRepo.settlementItem.Amount != 12 {
		t.Fatalf("settlement item = %+v", workbenchRepo.settlementItem)
	}
	if workbenchRepo.appliedAmount != -12 || workbenchRepo.batch.NetAmount != 88 {
		t.Fatalf("applied amount = %v batch = %+v", workbenchRepo.appliedAmount, workbenchRepo.batch)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSettlementAdjusted {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestBuildSettlementPayrollRowsFromItemsIncludesAdjustmentsInNormalRow(t *testing.T) {
	rows := buildSettlementPayrollRowsFromItems("2026-06", []*domain.AssetWorkbenchSettlementItem{
		{
			ID:            1,
			ItemType:      domain.AssetWorkbenchItemTypeGrossPiecework,
			PayeeUserID:   1001,
			BusinessMonth: "2026-06",
			Amount:        120,
			Quantity:      4,
			Direction:     "credit",
		},
		{
			ID:            2,
			ItemType:      domain.AssetWorkbenchItemTypeAdjustment,
			PayeeUserID:   1001,
			BusinessMonth: "2026-06",
			Amount:        20,
			Quantity:      1,
			Direction:     "credit",
		},
		{
			ID:            3,
			ItemType:      domain.AssetWorkbenchItemTypeReversal,
			PayeeUserID:   1001,
			BusinessMonth: "2026-06",
			Amount:        12,
			Quantity:      1,
			Direction:     "debit",
		},
		{
			ID:            4,
			ItemType:      domain.AssetWorkbenchItemTypeSupplement,
			PayeeUserID:   1001,
			BusinessMonth: "2026-06",
			Amount:        18,
			Quantity:      2,
			Direction:     "credit",
		},
	})
	if len(rows) != 2 {
		t.Fatalf("payroll rows = %+v, want fixed normal+supplement rows", rows)
	}
	normal := rows[0]
	supplement := rows[1]
	if normal.RowType != domain.AssetWorkbenchPayrollRowTypeNormalPiecework || normal.AdjustmentAmount != 8 || normal.NetAmount != 128 {
		t.Fatalf("normal payroll row = %+v, want adjustment 8 and net 128", normal)
	}
	if supplement.RowType != domain.AssetWorkbenchPayrollRowTypeSupplementPiecework || supplement.SupplementAmount != 18 || supplement.NetAmount != 18 {
		t.Fatalf("supplement payroll row = %+v, want supplement net 18", supplement)
	}
}

func TestGenerateSettlementBatchAttachesSubmissionItemsAtItemLevel(t *testing.T) {
	submittedAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	workbenchRepo := &settlementBatchRepo{items: []*domain.AssetWorkbenchSubmissionItem{
		{
			ID:                 501,
			PayeeUserID:        1001,
			OrderNo:            "ORD-501",
			DifficultyClass:    "A",
			PageCount:          2,
			GrossAmount:        20,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			BusinessMonth:      "2026-06",
			SubmittedAt:        submittedAt,
		},
		{
			ID:                 502,
			PayeeUserID:        1001,
			OrderNo:            "ORD-502",
			DifficultyClass:    "B",
			PageCount:          1,
			GrossAmount:        12,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			BusinessMonth:      "2026-06",
			SubmittedAt:        submittedAt,
		},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))

	batch, appErr := svc.GenerateSettlementBatch(context.Background(), domain.RequestActor{
		ID:    9,
		Roles: []domain.Role{domain.RoleAssetSettlement},
	}, "2026-06")
	if appErr != nil {
		t.Fatalf("GenerateSettlementBatch() error = %+v", appErr)
	}
	if batch == nil || batch.ID != 8801 || batch.NetAmount != 32 {
		t.Fatalf("batch = %+v, want net 32", batch)
	}
	if len(workbenchRepo.attachedItemIDs) != 2 || workbenchRepo.attachedItemIDs[0] != 501 || workbenchRepo.attachedItemIDs[1] != 502 {
		t.Fatalf("attached item ids = %+v", workbenchRepo.attachedItemIDs)
	}
	if len(workbenchRepo.settlementItems) != 2 {
		t.Fatalf("settlement items = %+v, want one gross line per submission item", workbenchRepo.settlementItems)
	}
	for _, item := range workbenchRepo.settlementItems {
		if item.ItemType != domain.AssetWorkbenchItemTypeGrossPiecework || item.SubmissionItemID == nil {
			t.Fatalf("settlement item should reference submission_item gross line: %+v", item)
		}
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSettlementGenerated {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestConfirmAndCancelSettlementBatchWriteEvents(t *testing.T) {
	workbenchRepo := &settlementBatchRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 9, Roles: []domain.Role{domain.RoleAssetSettlement}}

	if appErr := svc.ConfirmSettlementBatch(context.Background(), actor, 8801); appErr != nil {
		t.Fatalf("ConfirmSettlementBatch() error = %+v", appErr)
	}
	if workbenchRepo.confirmedBatchID != 8801 {
		t.Fatalf("confirmed batch id = %d", workbenchRepo.confirmedBatchID)
	}
	if appErr := svc.CancelSettlementBatch(context.Background(), actor, 8802, ""); appErr == nil || appErr.Code != domain.ErrCodeReasonRequired {
		t.Fatalf("CancelSettlementBatch(empty reason) appErr = %+v", appErr)
	}
	if appErr := svc.CancelSettlementBatch(context.Background(), actor, 8802, "  生成错误  "); appErr != nil {
		t.Fatalf("CancelSettlementBatch() error = %+v", appErr)
	}
	if workbenchRepo.cancelledBatchID != 8802 || workbenchRepo.cancelReason != "生成错误" {
		t.Fatalf("cancelled batch = %d reason = %q", workbenchRepo.cancelledBatchID, workbenchRepo.cancelReason)
	}
	if len(workbenchRepo.events) != 2 ||
		workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSettlementConfirmed ||
		workbenchRepo.events[1].EventType != domain.AssetWorkbenchEventSettlementCancelled {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestSystemAssetDownloadRequiresManagerRole(t *testing.T) {
	downloader := &systemAssetDownloaderStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetDownloader(downloader))

	_, appErr := svc.SystemAssetDownload(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}, 1001)
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("SystemAssetDownload(submitter) appErr = %+v", appErr)
	}
	if downloader.downloadCalls != 0 {
		t.Fatalf("downloadCalls = %d, want 0", downloader.downloadCalls)
	}
}

func TestCreateUploadSessionRequiresDirectoryWhenConfigured(t *testing.T) {
	workbenchRepo := &uploadDirectorySessionRepo{directories: []*domain.AssetWorkbenchUploadDirectory{
		{ID: 11, Name: "客户端 A", OSSPrefix: "client-a", Enabled: true},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithOSSDirect(testWorkbenchOSSDirect()),
	)
	actor := domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	_, appErr := svc.CreateUploadSession(context.Background(), actor, CreateUploadSessionParams{
		OriginalFilename: "final.psd",
		FileSize:         128,
		MimeType:         "image/vnd.adobe.photoshop",
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("CreateUploadSession(without directory) appErr = %+v", appErr)
	}
	if workbenchRepo.created != nil {
		t.Fatalf("created session = %+v, want nil", workbenchRepo.created)
	}
}

func TestUploadDirectorySnapshotObjectKeyUsesDirectoryPrefix(t *testing.T) {
	workbenchRepo := &uploadDirectorySessionRepo{directories: []*domain.AssetWorkbenchUploadDirectory{
		{ID: 11, Name: "客户端 A", OSSPrefix: "client-a", DifficultyClass: "C", Enabled: true},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	directory, appErr := svc.resolveUploadDirectoryForSession(context.Background(), 11)
	if appErr != nil {
		t.Fatalf("resolveUploadDirectoryForSession() error = %+v", appErr)
	}
	if directory.ID != 11 || directory.Name != "客户端 A" || directory.OSSPrefix != "client-a" {
		t.Fatalf("directory = %+v", directory)
	}
	if directory.DifficultyClass != "C" {
		t.Fatalf("directory difficulty = %q, want C", directory.DifficultyClass)
	}
	key := svc.buildObjectKey(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "session-1", "../final.psd", directory, "final.psd")
	if !strings.Contains(key, "/uploads/client-a/2026/06/session-1/final.psd") || strings.Contains(key, "..") {
		t.Fatalf("object key = %q", key)
	}
}

func TestCreateSubmissionInfersDifficultyFromUploadDirectorySnapshot(t *testing.T) {
	sessionID := "session-c"
	directoryID := int64(11)
	uploadedAt := time.Date(2026, 7, 3, 1, 58, 30, 0, time.UTC)
	workbenchRepo := &submissionDirectoryDifficultyRepo{
		members: []*domain.AssetWorkbenchMember{
			{UserID: 88, Status: domain.AppMembershipStatusActive, Roles: []domain.Role{domain.RoleAssetManager}},
			{UserID: 77, Status: domain.AppMembershipStatusActive, Roles: []domain.Role{domain.RoleAssetSubmitter}},
		},
		profile: &domain.AssetWorkbenchProfile{
			UserID:     77,
			WorkerType: domain.AssetWorkbenchWorkerTypeParttime,
			JobGrade:   "J1",
			Status:     domain.AssetWorkbenchProfileStatusActive,
		},
		price: &domain.AssetWorkbenchPriceMatrix{
			ID:              101,
			WorkerType:      domain.AssetWorkbenchWorkerTypeParttime,
			JobGrade:        "J1",
			DifficultyClass: "C",
			UnitPrice:       12.5,
		},
		session: &domain.AssetWorkbenchUploadSession{
			ID:                             9001,
			SessionID:                      sessionID,
			OwnerUserID:                    77,
			UploadDirectoryID:              &directoryID,
			UploadDirectoryName:            "C 类定稿",
			UploadDirectoryPrefix:          "client-c",
			UploadDirectoryDifficultyClass: "C",
			Status:                         domain.AssetWorkbenchUploadStatusUploaded,
			ObjectKey:                      "asset-workbench/uploads/client-c/2026/06/session-c/final.jpg",
			OriginalFilename:               "final.jpg",
			MimeType:                       "image/jpeg",
			FileSize:                       2048,
			UploadedAt:                     &uploadedAt,
		},
	}
	notifier := &profileNotificationStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}), WithNotificationCreator(notifier))
	svc.nowFn = func() time.Time {
		return time.Date(2026, 7, 3, 2, 0, 0, 0, time.UTC)
	}
	actor := domain.RequestActor{ID: 77, Username: "上传人员", Roles: []domain.Role{domain.RoleAssetSubmitter}}

	detail, appErr := svc.CreateSubmission(context.Background(), actor, CreateSubmissionParams{
		Items: []CreateSubmissionItemParams{{
			OrderNo:          "RW-20260703-C-001",
			Finalized:        true,
			PageCount:        1,
			UploadSessionIDs: []string{sessionID},
		}},
	})
	if appErr != nil {
		t.Fatalf("CreateSubmission() appErr = %+v", appErr)
	}
	if detail == nil || len(detail.Items) != 1 || len(detail.Items[0].Files) != 1 {
		t.Fatalf("detail = %+v, want one item and one file", detail)
	}
	if workbenchRepo.item == nil || workbenchRepo.item.DifficultyClass != "C" {
		t.Fatalf("created item difficulty = %+v, want C", workbenchRepo.item)
	}
	if workbenchRepo.item.TemplateID != nil {
		t.Fatalf("template id = %+v, want nil when client uploads through directory", workbenchRepo.item.TemplateID)
	}
	if workbenchRepo.files[0].UploadDirectoryDifficultyClass != "C" {
		t.Fatalf("file directory difficulty = %q, want C", workbenchRepo.files[0].UploadDirectoryDifficultyClass)
	}
	if !workbenchRepo.files[0].CreatedAt.Equal(uploadedAt) {
		t.Fatalf("file created_at = %s, want upload completion %s", workbenchRepo.files[0].CreatedAt, uploadedAt)
	}
	if workbenchRepo.sessionStatus != domain.AssetWorkbenchUploadStatusSubmitted {
		t.Fatalf("session status = %q, want submitted", workbenchRepo.sessionStatus)
	}
	if len(notifier.calls) != 1 || notifier.calls[0].userID != 88 || notifier.calls[0].ntype != domain.NotificationTypeAssetWorkbenchSubmissionCreated {
		t.Fatalf("submission notification = %+v, want one asset manager notification", notifier.calls)
	}
}

func TestSystemAssetPreviewUsesSharedPreviewerBeforeDownload(t *testing.T) {
	url := "https://assets.example.com/system/preview.webp"
	previewer := &systemAssetPreviewDownloaderStub{
		previewInfo: &domain.AssetDownloadInfo{
			DownloadMode:     domain.AssetDownloadModeDirect,
			DownloadURL:      &url,
			Filename:         "preview.webp",
			FileSize:         1024,
			MimeType:         "image/webp",
			PreviewAvailable: true,
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetDownloader(previewer))
	actor := assetCapabilityActor(1, domain.PermissionAssetView)

	meta, appErr := svc.SystemAssetPreview(context.Background(), actor, 1001)
	if appErr != nil {
		t.Fatalf("SystemAssetPreview() error = %+v", appErr)
	}
	if meta == nil || meta.PreviewURL != url || !meta.PreviewAvailable || meta.Status != domain.AssetWorkbenchPreviewStatusReady {
		t.Fatalf("preview meta = %+v, want shared preview URL", meta)
	}
	if previewer.previewCalls != 1 || previewer.previewIDs[0] != 1001 {
		t.Fatalf("preview calls = %d ids=%+v, want one preview call for 1001", previewer.previewCalls, previewer.previewIDs)
	}
	if previewer.downloadCalls != 0 {
		t.Fatalf("downloadCalls = %d, want 0", previewer.downloadCalls)
	}
}

func TestSystemAssetPreviewUsesSeparatelyWiredDerivedPreviewer(t *testing.T) {
	url := "https://assets.example.com/system/derived-preview.webp"
	downloader := &systemAssetDownloaderStub{}
	previewer := &systemAssetPreviewerStub{previewInfo: &domain.AssetDownloadInfo{
		DownloadMode:     domain.AssetDownloadModeDirect,
		DownloadURL:      &url,
		Filename:         "preview.webp",
		MimeType:         "image/webp",
		PreviewAvailable: true,
	}}
	svc := NewService(
		Config{Timezone: "Asia/Shanghai"},
		WithSystemAssetDownloader(downloader),
		WithSystemAssetPreviewer(previewer),
	)

	meta, appErr := svc.SystemAssetPreview(context.Background(), assetCapabilityActor(1, domain.PermissionAssetView), 14354)
	if appErr != nil {
		t.Fatalf("SystemAssetPreview() error = %+v", appErr)
	}
	if meta == nil || meta.PreviewURL != url || meta.MimeType != "image/webp" || !meta.PreviewAvailable {
		t.Fatalf("preview meta = %+v, want derived WebP", meta)
	}
	if previewer.previewCalls != 1 || len(previewer.previewIDs) != 1 || previewer.previewIDs[0] != 14354 {
		t.Fatalf("preview calls = %d ids=%+v, want asset 14354", previewer.previewCalls, previewer.previewIDs)
	}
	if downloader.downloadCalls != 0 {
		t.Fatalf("downloadCalls = %d, want 0", downloader.downloadCalls)
	}
}

func TestOverviewSearchReturnsEmptyItemsInsteadOfNil(t *testing.T) {
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{}}
	svc := NewService(
		Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
	)

	result, appErr := svc.OverviewSearch(
		context.Background(),
		domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}},
		OverviewSearchParams{Query: "DZC000027", Scope: "operational", Page: 1, PageSize: 60},
	)
	if appErr != nil {
		t.Fatalf("OverviewSearch() error = %+v", appErr)
	}
	if result == nil || result.Items == nil || len(result.Items) != 0 {
		t.Fatalf("OverviewSearch() items = %#v, want non-nil empty slice", result)
	}
}

func TestOverviewSearchMatchesIndexedClientMaterialSKU(t *testing.T) {
	publishedAt := time.Date(2026, 7, 13, 8, 0, 0, 0, time.UTC)
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{
		982: {
			ID:               982,
			AssetID:          14354,
			SourceType:       string(domain.AssetResourceSourceSystem),
			ResourceID:       "14354",
			Title:            "真硕定制海报",
			FilenameSnapshot: "真硕-定制海报.psd",
			Enabled:          true,
			PublishedAt:      publishedAt,
		},
	}}
	provider := &externalMaterialProviderStub{
		searchResult: &assetcenter.SearchResult{
			Items: []*assetcenter.AssetDetail{{
				ID:             14354,
				ResourceID:     "14354",
				SourceType:     string(domain.AssetResourceSourceSystem),
				ScopeSKUCode:   "DZC000027",
				SKUCode:        "DZC000027",
				PrimarySKUCode: "DZC000027",
				BusinessLane:   domain.TaskBusinessLaneCustomization,
			}},
			Total: 1,
			Page:  1,
			Size:  60,
		},
	}
	svc := NewService(
		Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetSearcher(provider),
	)

	result, appErr := svc.OverviewSearch(
		context.Background(),
		domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}},
		OverviewSearchParams{Query: "DZC000027", Scope: "operational", Page: 1, PageSize: 60},
	)
	if appErr != nil {
		t.Fatalf("OverviewSearch() error = %+v", appErr)
	}
	if result == nil || len(result.Items) != 1 {
		t.Fatalf("OverviewSearch() result = %#v, want one client material", result)
	}
	if got := result.Items[0].SecondaryCode; got != "DZC000027" {
		t.Fatalf("OverviewSearch() secondary_code = %q, want DZC000027", got)
	}
	var resultMeta map[string]interface{}
	if err := json.Unmarshal(result.Items[0].Meta, &resultMeta); err != nil {
		t.Fatalf("OverviewSearch() meta decode: %v", err)
	}
	if got := resultMeta["business_lane"]; got != string(domain.TaskBusinessLaneCustomization) {
		t.Fatalf("OverviewSearch() business_lane = %#v, want customization", got)
	}
	if provider.searchCalls != 1 {
		t.Fatalf("OverviewSearch() indexed search calls = %d, want 1", provider.searchCalls)
	}

	titleResult, titleErr := svc.OverviewSearch(
		context.Background(),
		domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}},
		OverviewSearchParams{Query: "真硕", Scope: "operational", Page: 1, PageSize: 60},
	)
	if titleErr != nil || titleResult == nil || len(titleResult.Items) != 1 {
		t.Fatalf("OverviewSearch(title) result = %#v error = %+v, want one client material", titleResult, titleErr)
	}
	if provider.searchCalls != 1 {
		t.Fatalf("OverviewSearch(title) indexed search calls = %d, want raw snapshot match without another indexed search", provider.searchCalls)
	}
}

func TestListClientMaterialsHydratesSystemBusinessLane(t *testing.T) {
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{
		982: {ID: 982, AssetID: 14354, SourceType: string(domain.AssetResourceSourceSystem), SourceRef: "14354", Enabled: true},
	}}
	provider := &externalMaterialProviderStub{details: map[int64]*assetcenter.AssetDetail{
		14354: {ID: 14354, BusinessLane: domain.TaskBusinessLaneCustomization},
	}}
	svc := NewService(
		Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetSearcher(provider),
	)

	items, appErr := svc.ListClientMaterials(
		context.Background(),
		domain.RequestActor{ID: 77, Permissions: []domain.PermissionCode{domain.PermissionAssetView}},
		false,
	)
	if appErr != nil {
		t.Fatalf("ListClientMaterials() error = %+v", appErr)
	}
	if len(items) != 1 || items[0].BusinessLane != string(domain.TaskBusinessLaneCustomization) {
		t.Fatalf("ListClientMaterials() items = %+v, want customization lane", items)
	}
}

func TestClientMaterialDownloadRequiresEnabledPublishedMaterial(t *testing.T) {
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{
		1: {ID: 1, AssetID: 1001, Title: "素材 A", Enabled: true},
		2: {ID: 2, AssetID: 1002, Title: "素材 B", Enabled: false},
	}}
	downloader := &systemAssetDownloaderStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetDownloader(downloader),
	)
	actor := domain.RequestActor{ID: 77, Permissions: []domain.PermissionCode{domain.PermissionAssetDownload}}

	info, appErr := svc.ClientMaterialDownload(context.Background(), actor, 1)
	if appErr != nil {
		t.Fatalf("ClientMaterialDownload(enabled) error = %+v", appErr)
	}
	if info == nil || downloader.downloadCalls != 1 || downloader.assetIDs[0] != 1001 {
		t.Fatalf("download info = %+v calls = %d ids = %+v", info, downloader.downloadCalls, downloader.assetIDs)
	}
	_, appErr = svc.ClientMaterialDownload(context.Background(), actor, 2)
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("ClientMaterialDownload(disabled) appErr = %+v", appErr)
	}
	if downloader.downloadCalls != 1 {
		t.Fatalf("downloadCalls = %d, want 1", downloader.downloadCalls)
	}
}

func TestClientMaterialPreviewRequiresEnabledPublishedMaterial(t *testing.T) {
	url := "https://assets.example.com/system/1001.png"
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{
		1: {ID: 1, AssetID: 1001, Title: "素材 A", Enabled: true},
		2: {ID: 2, AssetID: 1002, Title: "素材 B", Enabled: false},
	}}
	downloader := &systemAssetDownloaderStub{
		info: &domain.AssetDownloadInfo{
			DownloadMode: domain.AssetDownloadModeDirect,
			DownloadURL:  &url,
			Filename:     "system-asset.png",
			FileSize:     2048,
			MimeType:     "image/png",
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetDownloader(downloader),
	)
	actor := domain.RequestActor{ID: 77, Permissions: []domain.PermissionCode{domain.PermissionAssetView}}

	meta, appErr := svc.ClientMaterialPreview(context.Background(), actor, 1)
	if appErr != nil {
		t.Fatalf("ClientMaterialPreview(enabled) error = %+v", appErr)
	}
	if meta == nil || meta.AssetID != 1001 || meta.PreviewURL != url || !meta.PreviewAvailable {
		t.Fatalf("preview meta = %+v, want ready preview for asset 1001", meta)
	}
	if len(workbenchRepo.events) != 0 {
		t.Fatalf("preview wrote %d events, want 0", len(workbenchRepo.events))
	}
	_, appErr = svc.ClientMaterialPreview(context.Background(), actor, 2)
	if appErr == nil || appErr.Code != domain.ErrCodeNotFound {
		t.Fatalf("ClientMaterialPreview(disabled) appErr = %+v", appErr)
	}
	if downloader.downloadCalls != 1 {
		t.Fatalf("downloadCalls = %d, want 1", downloader.downloadCalls)
	}
}

func TestExternalClientMaterialDownloadAndPreviewUseExternalProvider(t *testing.T) {
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{
		1: {
			ID:               1,
			AssetID:          501,
			SourceType:       string(domain.AssetResourceSourceExternal),
			SourceRef:        domain.ExternalAssetResourceID(501),
			Title:            "外部素材",
			FilenameSnapshot: "external.png",
			MimeTypeSnapshot: "image/png",
			Enabled:          true,
		},
	}}
	externalProvider := &externalMaterialProviderStub{}
	systemDownloader := &systemAssetDownloaderStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetSearcher(externalProvider),
		WithSystemAssetDownloader(systemDownloader),
	)
	actor := domain.RequestActor{ID: 77, Permissions: []domain.PermissionCode{domain.PermissionAssetView, domain.PermissionAssetDownload}}

	info, appErr := svc.ClientMaterialDownload(context.Background(), actor, 1)
	if appErr != nil {
		t.Fatalf("ClientMaterialDownload(external) error = %+v", appErr)
	}
	if info == nil || len(externalProvider.downloadCalls) != 1 || externalProvider.downloadCalls[0] != 501 {
		t.Fatalf("download info = %+v external calls = %+v", info, externalProvider.downloadCalls)
	}
	if systemDownloader.downloadCalls != 0 {
		t.Fatalf("system downloader calls = %d, want 0", systemDownloader.downloadCalls)
	}

	meta, appErr := svc.ClientMaterialPreview(context.Background(), actor, 1)
	if appErr != nil {
		t.Fatalf("ClientMaterialPreview(external) error = %+v", appErr)
	}
	if meta == nil || meta.SourceType != "external_asset" || meta.SourceRef != domain.ExternalAssetResourceID(501) || !meta.PreviewAvailable {
		t.Fatalf("preview meta = %+v, want external ready preview", meta)
	}
	if len(externalProvider.previewCalls) != 1 || externalProvider.previewCalls[0] != 501 {
		t.Fatalf("preview calls = %+v", externalProvider.previewCalls)
	}
}

func TestExternalClientMaterialPendingDownloadDoesNotRecordCompletedEvent(t *testing.T) {
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{
		1: {
			ID:               1,
			AssetID:          501,
			SourceType:       string(domain.AssetResourceSourceExternal),
			SourceRef:        domain.ExternalAssetResourceID(501),
			FilenameSnapshot: "external.psd",
			Enabled:          true,
		},
	}}
	externalProvider := &externalMaterialProviderStub{downloadInfo: &domain.AssetDownloadInfo{
		DownloadMode: domain.AssetDownloadModeDirect,
		AccessHint:   "external_netdisk_prepare_required",
		Filename:     "external.psd",
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetSearcher(externalProvider),
	)
	actor := domain.RequestActor{ID: 77, Permissions: []domain.PermissionCode{domain.PermissionAssetDownload}}

	info, appErr := svc.ClientMaterialDownload(context.Background(), actor, 1)
	if appErr != nil {
		t.Fatalf("ClientMaterialDownload() error = %+v", appErr)
	}
	if info == nil || !strings.Contains(info.AccessHint, "prepare_required") {
		t.Fatalf("download info = %+v, want pending preparation", info)
	}
	if len(workbenchRepo.events) != 0 {
		t.Fatalf("pending download recorded %d events, want 0", len(workbenchRepo.events))
	}
}

func TestClientMaterialBatchDownloadSupportsMixedSources(t *testing.T) {
	workbenchRepo := &clientMaterialRepo{materials: map[int64]*domain.AssetWorkbenchClientMaterial{
		1: {ID: 1, AssetID: 1001, SourceType: string(domain.AssetResourceSourceSystem), SourceRef: "1001", Title: "系统素材", Enabled: true},
		2: {ID: 2, AssetID: 501, SourceType: string(domain.AssetResourceSourceExternal), SourceRef: domain.ExternalAssetResourceID(501), Title: "外部素材", Enabled: true},
	}}
	externalProvider := &externalMaterialProviderStub{}
	systemDownloader := &systemAssetDownloaderStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetSearcher(externalProvider),
		WithSystemAssetDownloader(systemDownloader),
	)

	manifest, appErr := svc.ClientMaterialBatchDownloadManifest(
		context.Background(),
		domain.RequestActor{ID: 77, Permissions: []domain.PermissionCode{domain.PermissionAssetDownload}},
		ClientMaterialBatchDownloadParams{MaterialIDs: []int64{1, 2}},
	)
	if appErr != nil {
		t.Fatalf("ClientMaterialBatchDownloadManifest() error = %+v", appErr)
	}
	if manifest.SuccessCount != 2 || len(manifest.Items) != 2 {
		t.Fatalf("manifest = %+v, want two mixed-source items", manifest)
	}
	if manifest.Items[0].SourceType != string(domain.AssetResourceSourceSystem) || manifest.Items[1].SourceType != "external_asset" {
		t.Fatalf("manifest items = %+v, want system then external", manifest.Items)
	}
	if systemDownloader.downloadCalls != 1 || len(externalProvider.downloadCalls) != 1 {
		t.Fatalf("system calls = %d external calls = %+v", systemDownloader.downloadCalls, externalProvider.downloadCalls)
	}
}

func TestSystemAssetPreviewUsesPreviewableDownloadURL(t *testing.T) {
	url := "https://assets.example.com/system/1001.png"
	downloader := &systemAssetDownloaderStub{
		info: &domain.AssetDownloadInfo{
			DownloadMode: domain.AssetDownloadModeDirect,
			DownloadURL:  &url,
			Filename:     "system-asset.png",
			FileSize:     2048,
			MimeType:     "image/png",
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetDownloader(downloader))

	meta, appErr := svc.SystemAssetPreview(context.Background(), assetCapabilityActor(99, domain.PermissionAssetView), 1001)
	if appErr != nil {
		t.Fatalf("SystemAssetPreview() error = %+v", appErr)
	}
	if meta.Status != domain.AssetWorkbenchPreviewStatusReady || !meta.PreviewAvailable || meta.PreviewURL != url || meta.DownloadURL != url {
		t.Fatalf("preview meta = %+v, want ready direct preview", meta)
	}
}

func TestSystemAssetPreviewRequiresManagerRole(t *testing.T) {
	downloader := &systemAssetDownloaderStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetDownloader(downloader))

	_, appErr := svc.SystemAssetPreview(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}, 1001)
	if appErr == nil || appErr.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("SystemAssetPreview(submitter) appErr = %+v", appErr)
	}
	if downloader.downloadCalls != 0 {
		t.Fatalf("downloadCalls = %d, want 0", downloader.downloadCalls)
	}
}

func TestSystemAssetBatchDownloadDelegatesAndWritesEvent(t *testing.T) {
	workbenchRepo := &downloadFileRepo{}
	downloader := &systemAssetDownloaderStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithSystemAssetDownloader(downloader),
	)

	manifest, appErr := svc.SystemAssetBatchDownloadManifest(
		context.Background(),
		assetCapabilityActor(99, domain.PermissionAssetDownload),
		SystemAssetBatchDownloadParams{AssetIDs: []int64{1001, 1002}, NamingMode: "business"},
	)
	if appErr != nil {
		t.Fatalf("SystemAssetBatchDownloadManifest() error = %+v", appErr)
	}
	if manifest.SuccessCount != 2 || len(manifest.Items) != 2 {
		t.Fatalf("manifest = %+v", manifest)
	}
	if downloader.batchCalls != 1 || len(downloader.batchAssetIDs) != 2 || downloader.batchAssetIDs[0] != 1001 || downloader.batchAssetIDs[1] != 1002 {
		t.Fatalf("downloader calls = %d ids = %+v", downloader.batchCalls, downloader.batchAssetIDs)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSystemAssetBatchDownloaded {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestUpdateSubmissionItemQCRequiresReasonForNeedsFixAndWritesEvent(t *testing.T) {
	item := &domain.AssetWorkbenchSubmissionItem{
		ID:               501,
		SubmissionID:     9001,
		PayeeUserID:      77,
		OrderNo:          "ORD-501",
		QCStatus:         domain.AssetWorkbenchSubmissionStatusSubmitted,
		SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
	}
	workbenchRepo := &itemActionRepo{items: map[int64]*domain.AssetWorkbenchSubmissionItem{501: item}}
	notifier := &profileNotificationStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}), WithNotificationCreator(notifier))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}

	if _, appErr := svc.UpdateSubmissionItemQC(context.Background(), actor, 501, UpdateSubmissionItemQCParams{QCStatus: domain.AssetWorkbenchSubmissionStatusNeedsFix}); appErr == nil || appErr.Code != domain.ErrCodeReasonRequired {
		t.Fatalf("UpdateSubmissionItemQC(needs_fix without reason) appErr = %+v", appErr)
	}
	updated, appErr := svc.UpdateSubmissionItemQC(context.Background(), actor, 501, UpdateSubmissionItemQCParams{QCStatus: domain.AssetWorkbenchSubmissionStatusChecked})
	if appErr != nil {
		t.Fatalf("UpdateSubmissionItemQC(checked) error = %+v", appErr)
	}
	if updated.QCStatus != domain.AssetWorkbenchSubmissionStatusChecked {
		t.Fatalf("qc_status = %q", updated.QCStatus)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventItemQCUpdated {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
	if len(notifier.calls) != 1 || notifier.calls[0].userID != 77 || notifier.calls[0].ntype != domain.NotificationTypeAssetWorkbenchQCUpdated {
		t.Fatalf("qc notification = %+v, want one payee notification", notifier.calls)
	}
}

func TestImportSubmissionItemQCExcelMatchesBySubmissionAndFilename(t *testing.T) {
	item := &domain.AssetWorkbenchSubmissionItem{
		ID:               501,
		SubmissionID:     9001,
		PayeeUserID:      77,
		OrderNo:          "AWF20260703080000ABCDEF12",
		QCStatus:         domain.AssetWorkbenchSubmissionStatusSubmitted,
		SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
		BusinessMonth:    "2026-07",
	}
	workbenchRepo := &itemActionRepo{
		items: map[int64]*domain.AssetWorkbenchSubmissionItem{501: item},
		submissions: map[int64]*domain.AssetWorkbenchSubmission{9001: &domain.AssetWorkbenchSubmission{
			ID:           9001,
			SubmissionNo: "SUB-202607-001",
		}},
		filesByItem: map[int64][]*domain.AssetWorkbenchSubmissionFile{501: []*domain.AssetWorkbenchSubmissionFile{
			{
				ID:                  7001,
				SubmissionID:        9001,
				SubmissionItemID:    501,
				OriginalFilename:    "客户海报.jpg",
				UploadDirectoryName: "挂布",
			},
		}},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}
	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	if err := f.SetSheetRow(sheet, "A1", &[]interface{}{"提交编号", "文件名", "质检状态", "原因"}); err != nil {
		t.Fatalf("set header row: %v", err)
	}
	if err := f.SetSheetRow(sheet, "A2", &[]interface{}{"SUB-202607-001", "客户海报.jpg", "通过", ""}); err != nil {
		t.Fatalf("set data row: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}

	result, appErr := svc.ImportSubmissionItemQCExcel(context.Background(), actor, "2026-07", bytes.NewReader(buf.Bytes()))
	if appErr != nil {
		t.Fatalf("ImportSubmissionItemQCExcel appErr = %+v", appErr)
	}
	if len(result.Failures) != 0 || len(result.Updated) != 1 {
		t.Fatalf("result = %+v, want one update and no failures", result)
	}
	if item.QCStatus != domain.AssetWorkbenchSubmissionStatusChecked {
		t.Fatalf("qc_status = %q, want checked", item.QCStatus)
	}
}

func TestVoidSubmissionItemRequiresUnsettledAndRefreshesTotals(t *testing.T) {
	batchID := int64(123)
	workbenchRepo := &itemActionRepo{items: map[int64]*domain.AssetWorkbenchSubmissionItem{
		501: {
			ID:                       501,
			SubmissionID:             9001,
			PayeeUserID:              77,
			OrderNo:                  "ORD-501",
			QCStatus:                 domain.AssetWorkbenchSubmissionStatusChecked,
			SettlementStatus:         domain.AssetWorkbenchSettlementStatusInBatch,
			CurrentSettlementBatchID: &batchID,
		},
		502: {
			ID:               502,
			SubmissionID:     9001,
			PayeeUserID:      77,
			OrderNo:          "ORD-502",
			QCStatus:         domain.AssetWorkbenchSubmissionStatusChecked,
			SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
		},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	if _, appErr := svc.VoidSubmissionItem(context.Background(), actor, 501, VoidSubmissionItemParams{Reason: "重复提交"}); appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("VoidSubmissionItem(in batch) appErr = %+v", appErr)
	}
	updated, appErr := svc.VoidSubmissionItem(context.Background(), actor, 502, VoidSubmissionItemParams{Reason: "重复提交"})
	if appErr != nil {
		t.Fatalf("VoidSubmissionItem(unsettled) error = %+v", appErr)
	}
	if updated.QCStatus != domain.AssetWorkbenchSubmissionStatusVoided || updated.VoidReason != "重复提交" {
		t.Fatalf("voided item = %+v", updated)
	}
	if workbenchRepo.refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", workbenchRepo.refreshCalls)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventItemVoided {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestVoidSubmissionVoidsAllUnsettledItemsAndWritesEvent(t *testing.T) {
	workbenchRepo := &submissionVoidRepo{
		submission: &domain.AssetWorkbenchSubmission{ID: 9001, SubmissionNo: "AW-9001", Status: domain.AssetWorkbenchSubmissionStatusSubmitted},
		items: []*domain.AssetWorkbenchSubmissionItem{
			{ID: 501, SubmissionID: 9001, QCStatus: domain.AssetWorkbenchSubmissionStatusChecked, SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled},
			{ID: 502, SubmissionID: 9001, QCStatus: domain.AssetWorkbenchSubmissionStatusSubmitted, SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	updated, appErr := svc.VoidSubmission(context.Background(), actor, 9001, VoidSubmissionParams{Reason: "整批重复上传"})
	if appErr != nil {
		t.Fatalf("VoidSubmission() error = %+v", appErr)
	}
	if updated.Status != domain.AssetWorkbenchSubmissionStatusVoided {
		t.Fatalf("submission status = %q", updated.Status)
	}
	for _, item := range workbenchRepo.items {
		if item.QCStatus != domain.AssetWorkbenchSubmissionStatusVoided || item.VoidReason != "整批重复上传" || item.VoidedBy == nil || *item.VoidedBy != 99 {
			t.Fatalf("voided item = %+v", item)
		}
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSubmissionVoided {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestVoidSubmissionBlocksInBatchItems(t *testing.T) {
	batchID := int64(123)
	workbenchRepo := &submissionVoidRepo{
		submission: &domain.AssetWorkbenchSubmission{ID: 9001, SubmissionNo: "AW-9001", Status: domain.AssetWorkbenchSubmissionStatusSubmitted},
		items: []*domain.AssetWorkbenchSubmissionItem{
			{ID: 501, SubmissionID: 9001, QCStatus: domain.AssetWorkbenchSubmissionStatusChecked, SettlementStatus: domain.AssetWorkbenchSettlementStatusInBatch, CurrentSettlementBatchID: &batchID},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	_, appErr := svc.VoidSubmission(context.Background(), actor, 9001, VoidSubmissionParams{Reason: "整批重复上传"})
	if appErr == nil || appErr.Code != domain.ErrCodeConflict {
		t.Fatalf("VoidSubmission(in batch) appErr = %+v, want conflict", appErr)
	}
	if workbenchRepo.submission.Status == domain.AssetWorkbenchSubmissionStatusVoided || len(workbenchRepo.events) != 0 {
		t.Fatalf("submission should not be voided: submission=%+v events=%+v", workbenchRepo.submission, workbenchRepo.events)
	}
}

func TestBatchMoveFilesReturnsPerFileFailuresWhenOSSDisabled(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		directories: map[int64]*domain.AssetWorkbenchUploadDirectory{
			11: {ID: 11, Name: "客户端 A", OSSPrefix: "client-a", Enabled: true},
		},
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {
				ID:               501,
				SubmissionID:     9001,
				SubmissionItemID: 8001,
				ObjectKey:        "asset-workbench/uploads/2026/06/session-a/source.psd",
				OriginalFilename: "source.psd",
			},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}

	result, appErr := svc.BatchMoveFiles(context.Background(), actor, BatchMoveFilesParams{
		FileIDs:           []int64{501, 999, 501, 0},
		UploadDirectoryID: 11,
		Reason:            "批量归档",
	})
	if appErr != nil {
		t.Fatalf("BatchMoveFiles() appErr = %+v", appErr)
	}
	if len(result.Files) != 0 || len(result.Failures) != 2 {
		t.Fatalf("result = %+v, want 2 per-file failures and no moved files", result)
	}
	if result.Failures[0].FileID != 501 || !strings.Contains(result.Failures[0].Reason, "OSS direct move is not enabled") {
		t.Fatalf("first failure = %+v", result.Failures[0])
	}
	if result.Failures[1].FileID != 999 || result.Failures[1].Reason != "file not found" {
		t.Fatalf("second failure = %+v", result.Failures[1])
	}
	if len(workbenchRepo.updatedFiles) != 0 || len(workbenchRepo.events) != 0 {
		t.Fatalf("move should not update files or events when OSS is disabled: updated=%+v events=%+v", workbenchRepo.updatedFiles, workbenchRepo.events)
	}
}

func TestBatchMoveFilesRejectsPartialPricedWorkSelection(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		directories: map[int64]*domain.AssetWorkbenchUploadDirectory{
			11: {ID: 11, Name: "B类", OSSPrefix: "class-b", DifficultyClass: "B", Enabled: true},
		},
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, ObjectKey: "asset-workbench/uploads/folder/a.jpg", IsFolderUpload: true},
			502: {ID: 502, SubmissionID: 9001, SubmissionItemID: 8001, ObjectKey: "asset-workbench/uploads/folder/b.jpg", IsFolderUpload: true},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}

	result, appErr := svc.BatchMoveFiles(context.Background(), actor, BatchMoveFilesParams{
		FileIDs:           []int64{501},
		UploadDirectoryID: 11,
		Reason:            "改为B类",
	})
	if appErr != nil {
		t.Fatalf("BatchMoveFiles() appErr = %+v", appErr)
	}
	if len(result.Files) != 0 || len(result.Failures) != 1 || result.Failures[0].FileID != 501 || !strings.Contains(result.Failures[0].Reason, "selected together") {
		t.Fatalf("result = %+v, want one complete-work selection failure", result)
	}
	if len(workbenchRepo.updatedFiles) != 0 || len(workbenchRepo.events) != 0 {
		t.Fatalf("partial priced work must not be moved: updated=%+v events=%+v", workbenchRepo.updatedFiles, workbenchRepo.events)
	}
}

func TestBatchMoveFilesRejectsSettlementLockedPricedWork(t *testing.T) {
	batchID := int64(7001)
	workbenchRepo := &batchFileMutationRepo{
		directories: map[int64]*domain.AssetWorkbenchUploadDirectory{
			11: {ID: 11, Name: "B类", OSSPrefix: "class-b", DifficultyClass: "B", Enabled: true},
		},
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, ObjectKey: "asset-workbench/uploads/a.jpg"},
		},
		items: map[int64]*domain.AssetWorkbenchSubmissionItem{
			8001: {
				ID: 8001, SubmissionID: 9001, PayeeUserID: 99, OrderNo: "AWF", DifficultyClass: "C", PageCount: 1, ItemCount: 1,
				BusinessMonth: "2026-07", SubmittedAt: time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC),
				SettlementStatus: domain.AssetWorkbenchSettlementStatusInBatch, CurrentSettlementBatchID: &batchID,
				QCStatus: domain.AssetWorkbenchSubmissionStatusSubmitted,
			},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}), WithOSSDirect(testWorkbenchOSSDirect()))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}

	result, appErr := svc.BatchMoveFiles(context.Background(), actor, BatchMoveFilesParams{FileIDs: []int64{501}, UploadDirectoryID: 11})
	if appErr != nil {
		t.Fatalf("BatchMoveFiles() appErr = %+v", appErr)
	}
	if len(result.Files) != 0 || len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Reason, "settlement batch attachment") {
		t.Fatalf("result = %+v, want settlement-lock failure", result)
	}
	if len(workbenchRepo.updatedFiles) != 0 || len(workbenchRepo.events) != 0 {
		t.Fatalf("locked priced work must not be moved: updated=%+v events=%+v", workbenchRepo.updatedFiles, workbenchRepo.events)
	}
}

func TestBuildSubmissionItemForDifficultyPreservesFolderPieceCount(t *testing.T) {
	submittedAt := time.Date(2026, 7, 8, 8, 0, 0, 0, time.UTC)
	templateID := int64(71)
	workbenchRepo := &itemActionRepo{
		price: &domain.AssetWorkbenchPriceMatrix{
			ID: 88, WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "P1", DifficultyClass: "B", UnitPrice: 0.63,
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	before := &domain.AssetWorkbenchSubmissionItem{
		ID: 8001, SubmissionID: 9001, PayeeUserID: 99, OrderNo: "AWF", DifficultyClass: "C", Finalized: true,
		PageCount: 1, ItemCount: 1, BusinessMonth: "2026-07", SubmittedAt: submittedAt,
		TemplateID: &templateID, TemplateNameSnapshot: "兼职海报", CategorySnapshot: "运营素材",
		WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeParttime, JobGradeSnapshot: "P1",
		SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled, QCStatus: domain.AssetWorkbenchSubmissionStatusSubmitted,
	}

	repriced, appErr := svc.buildSubmissionItemForDifficulty(context.Background(), before, "B")
	if appErr != nil {
		t.Fatalf("buildSubmissionItemForDifficulty() appErr = %+v", appErr)
	}
	if repriced.DifficultyClass != "B" || repriced.PageCount != 1 || repriced.ItemCount != 1 || repriced.GrossAmount != 0.63 {
		t.Fatalf("repriced = %+v, want B class, one piece and 0.63", repriced)
	}
	if repriced.TemplateID == nil || *repriced.TemplateID != templateID || repriced.TemplateNameSnapshot != "兼职海报" || repriced.CategorySnapshot != "运营素材" {
		t.Fatalf("repriced template identity = %+v, want original template snapshots", repriced)
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal(repriced.PricingSnapshot, &snapshot); err != nil {
		t.Fatalf("pricing snapshot json: %v", err)
	}
	if snapshot["template_id"] != float64(templateID) || snapshot["template_name"] != "兼职海报" || snapshot["category"] != "运营素材" || snapshot["difficulty_class"] != "B" {
		t.Fatalf("pricing snapshot = %#v, want template/category identity with target difficulty", snapshot)
	}
}

func TestUpdateSubmissionFileDisplayNameAllowsSettlementRole(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 7, OriginalFilename: "source.psd", DisplayName: "source.psd"},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	updated, appErr := svc.UpdateSubmissionFileDisplayName(context.Background(), actor, 501, " 夏季主视觉 ")
	if appErr != nil {
		t.Fatalf("UpdateSubmissionFileDisplayName() appErr = %+v", appErr)
	}
	if updated.DisplayName != "夏季主视觉" || workbenchRepo.files[501].DisplayName != "夏季主视觉" {
		t.Fatalf("display name not updated: updated=%+v stored=%+v", updated.DisplayName, workbenchRepo.files[501].DisplayName)
	}
	if updated.OriginalFilename != "source.psd" {
		t.Fatalf("original filename changed: %q", updated.OriginalFilename)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventFileRenamed {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestBatchDeleteFilesSplitsSuccessAndFailureAndWritesEvents(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, ObjectKey: "asset-workbench/uploads/a.psd", PreviewKey: "asset-workbench/previews/a.jpg"},
			502: {ID: 502, SubmissionID: 9002, SubmissionItemID: 8002, ObjectKey: "asset-workbench/uploads/b.psd"},
		},
		blockedDelete: map[int64]bool{502: true},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}

	if _, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{FileIDs: []int64{501}, Reason: " "}); appErr == nil || appErr.Code != domain.ErrCodeReasonRequired {
		t.Fatalf("BatchDeleteFiles(empty reason) appErr = %+v", appErr)
	}
	result, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{
		FileIDs: []int64{501, 502, 999, 501},
		Reason:  "重复文件",
	})
	if appErr != nil {
		t.Fatalf("BatchDeleteFiles() appErr = %+v", appErr)
	}
	if len(result.Deleted) != 1 || result.Deleted[0] != 501 {
		t.Fatalf("deleted = %+v, want [501]", result.Deleted)
	}
	if len(result.Failures) != 2 || result.Failures[0].FileID != 502 || result.Failures[1].FileID != 999 {
		t.Fatalf("failures = %+v", result.Failures)
	}
	if _, ok := workbenchRepo.files[501]; ok {
		t.Fatalf("file 501 should be deleted")
	}
	if len(workbenchRepo.refreshed) != 1 || workbenchRepo.refreshed[0] != 9001 {
		t.Fatalf("refreshed = %+v, want [9001]", workbenchRepo.refreshed)
	}
	if len(workbenchRepo.events) != 2 ||
		workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventFileDeleted ||
		workbenchRepo.events[1].EventType != domain.AssetWorkbenchEventItemVoided {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestBatchDeleteFilesRejectsSupplementFilesToProtectSupplementAmount(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99, ObjectKey: "asset-workbench/uploads/supplement.jpg"},
		},
		items: map[int64]*domain.AssetWorkbenchSubmissionItem{
			8001: {
				ID: 8001, SubmissionID: 9001, PayeeUserID: 99, OrderNo: "supplement.jpg", DifficultyClass: "A", PageCount: 1, ItemCount: 1,
				BusinessMonth: "2026-07", SubmittedAt: time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC), EntryKind: domain.AssetWorkbenchSubmissionEntryKindSupplement,
				SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled, QCStatus: domain.AssetWorkbenchSubmissionStatusSubmitted,
			},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	result, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{FileIDs: []int64{501}, Reason: "错传"})
	if appErr != nil {
		t.Fatalf("BatchDeleteFiles() appErr = %+v", appErr)
	}
	if len(result.Deleted) != 0 || len(result.Failures) != 1 || !strings.Contains(result.Failures[0].Reason, "Supplement upload files must be managed through their supplement record") {
		t.Fatalf("result = %+v, want supplement consistency failure", result)
	}
	if len(workbenchRepo.deletedFiles) != 0 || len(workbenchRepo.updatedItems) != 0 {
		t.Fatalf("supplement file must remain unchanged: files=%v items=%v", workbenchRepo.deletedFiles, workbenchRepo.updatedItems)
	}
}

func TestBatchDeleteFilesRejectsPartialPricedWorkSelection(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99, IsFolderUpload: true},
			502: {ID: 502, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99, IsFolderUpload: true},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	result, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{FileIDs: []int64{501}, Reason: "重复上传"})
	if appErr != nil {
		t.Fatalf("BatchDeleteFiles() appErr = %+v", appErr)
	}
	if len(result.Deleted) != 0 || len(result.Failures) != 1 || result.Failures[0].FileID != 501 || !strings.Contains(result.Failures[0].Reason, "selected together") {
		t.Fatalf("result = %+v, want one complete-work selection failure", result)
	}
	if workbenchRepo.files[501] == nil || workbenchRepo.files[502] == nil || len(workbenchRepo.events) != 0 {
		t.Fatalf("partial priced work must remain untouched: files=%+v events=%+v", workbenchRepo.files, workbenchRepo.events)
	}
}

func TestBatchDeleteFilesDeletesCompleteFolderPricedWorkAsOneItem(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99, IsFolderUpload: true},
			502: {ID: 502, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99, IsFolderUpload: true},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	result, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{FileIDs: []int64{501, 502}, Reason: "重复上传"})
	if appErr != nil {
		t.Fatalf("BatchDeleteFiles() appErr = %+v", appErr)
	}
	if len(result.Deleted) != 2 || len(result.Failures) != 0 {
		t.Fatalf("result = %+v, want both folder files deleted", result)
	}
	if len(workbenchRepo.events) != 3 || workbenchRepo.events[2].EventType != domain.AssetWorkbenchEventItemVoided {
		t.Fatalf("events = %+v, want two file events and one item void", workbenchRepo.events)
	}
	if len(workbenchRepo.refreshed) != 1 || workbenchRepo.refreshed[0] != 9001 {
		t.Fatalf("refreshed = %+v, want one submission refresh", workbenchRepo.refreshed)
	}
}

func TestBatchDeleteFilesRollsBackFileDeleteWhenItemReconcileFails(t *testing.T) {
	workbenchRepo := &batchFileMutationRepo{
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99, ObjectKey: "asset-workbench/uploads/a.psd"},
		},
		failVoid: true,
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, rollbackBatchFileTxRunner{repo: workbenchRepo}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}

	result, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{
		FileIDs: []int64{501},
		Reason:  "重复文件",
	})
	if appErr != nil {
		t.Fatalf("BatchDeleteFiles() appErr = %+v", appErr)
	}
	if len(result.Deleted) != 0 || len(result.Failures) != 1 || result.Failures[0].FileID != 501 {
		t.Fatalf("result = %+v, want one per-file failure and no deleted ids", result)
	}
	if workbenchRepo.files[501] == nil {
		t.Fatalf("file 501 should remain when item reconcile fails")
	}
	if len(workbenchRepo.deletedFiles) != 0 || len(workbenchRepo.refreshed) != 0 || len(workbenchRepo.events) != 0 {
		t.Fatalf("write side effects should roll back: deleted=%+v refreshed=%+v events=%+v", workbenchRepo.deletedFiles, workbenchRepo.refreshed, workbenchRepo.events)
	}
}

func TestBatchDeleteFilesRejectsSettlementLockedItem(t *testing.T) {
	batchID := int64(7001)
	workbenchRepo := &batchFileMutationRepo{
		files: map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99, ObjectKey: "asset-workbench/uploads/a.psd"},
		},
		items: map[int64]*domain.AssetWorkbenchSubmissionItem{
			8001: {
				ID:                       8001,
				SubmissionID:             9001,
				PayeeUserID:              99,
				OrderNo:                  "AWF",
				DifficultyClass:          "A",
				PageCount:                1,
				ItemCount:                1,
				BusinessMonth:            "2026-07",
				SubmittedAt:              time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC),
				SettlementStatus:         domain.AssetWorkbenchSettlementStatusInBatch,
				CurrentSettlementBatchID: &batchID,
				QCStatus:                 domain.AssetWorkbenchSubmissionStatusSubmitted,
			},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSubmitter}}

	result, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{
		FileIDs: []int64{501},
		Reason:  "重复文件",
	})
	if appErr != nil {
		t.Fatalf("BatchDeleteFiles() appErr = %+v", appErr)
	}
	if len(result.Deleted) != 0 || len(result.Failures) != 1 || result.Failures[0].FileID != 501 {
		t.Fatalf("result = %+v, want one settlement-lock failure and no deleted ids", result)
	}
	if workbenchRepo.files[501] == nil {
		t.Fatalf("file 501 should remain when item is settlement locked")
	}
	if len(workbenchRepo.deletedFiles) != 0 || len(workbenchRepo.refreshed) != 0 || len(workbenchRepo.events) != 0 {
		t.Fatalf("locked delete should not write side effects: deleted=%+v refreshed=%+v events=%+v", workbenchRepo.deletedFiles, workbenchRepo.refreshed, workbenchRepo.events)
	}
}

func TestBatchFileGroupMutationRollsBackWhenSettlementLocksAfterValidation(t *testing.T) {
	newItem := func() *domain.AssetWorkbenchSubmissionItem {
		return &domain.AssetWorkbenchSubmissionItem{
			ID:                 8001,
			SubmissionID:       9001,
			PayeeUserID:        99,
			OrderNo:            "AWF",
			DifficultyClass:    "C",
			Finalized:          true,
			PageCount:          1,
			ItemCount:          1,
			BusinessMonth:      "2026-07",
			SubmittedAt:        time.Date(2026, 7, 3, 8, 0, 0, 0, time.UTC),
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeParttime,
			JobGradeSnapshot:   "P1",
			SettlementStatus:   domain.AssetWorkbenchSettlementStatusUnsettled,
			QCStatus:           domain.AssetWorkbenchSubmissionStatusSubmitted,
		}
	}
	newFiles := func() map[int64]*domain.AssetWorkbenchSubmissionFile {
		return map[int64]*domain.AssetWorkbenchSubmissionFile{
			501: {
				ID: 501, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99,
				ObjectKey: "asset-workbench/uploads/source/a.jpg", OriginalFilename: "a.jpg", IsFolderUpload: true,
			},
			502: {
				ID: 502, SubmissionID: 9001, SubmissionItemID: 8001, OwnerUserID: 99,
				ObjectKey: "asset-workbench/uploads/source/b.jpg", OriginalFilename: "b.jpg", IsFolderUpload: true,
			},
		}
	}

	t.Run("move removes copied targets and rolls back database writes", func(t *testing.T) {
		transport := useRecordingOSSTransport(t)
		workbenchRepo := &batchFileMutationRepo{
			directories: map[int64]*domain.AssetWorkbenchUploadDirectory{
				11: {ID: 11, Name: "B类", OSSPrefix: "class-b", DifficultyClass: "B", Enabled: true},
			},
			files: newFiles(),
			items: map[int64]*domain.AssetWorkbenchSubmissionItem{8001: newItem()},
			price: &domain.AssetWorkbenchPriceMatrix{
				ID: 88, WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "P1", DifficultyClass: "B", UnitPrice: 0.63,
				EffectiveFrom: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
			},
		}
		txRunner := settlementLockRaceTxRunner{repo: workbenchRepo, itemID: 8001, batchID: 7001}
		svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, txRunner), WithOSSDirect(testWorkbenchOSSDirect()))
		actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}

		result, appErr := svc.BatchMoveFiles(context.Background(), actor, BatchMoveFilesParams{
			FileIDs:           []int64{501, 502},
			UploadDirectoryID: 11,
			Reason:            "改为B类",
		})
		if appErr != nil {
			t.Fatalf("BatchMoveFiles() appErr = %+v", appErr)
		}
		if len(result.Files) != 0 || len(result.Failures) != 2 {
			t.Fatalf("result = %+v, want whole group rejected", result)
		}
		if workbenchRepo.files[501].ObjectKey != "asset-workbench/uploads/source/a.jpg" || workbenchRepo.files[502].ObjectKey != "asset-workbench/uploads/source/b.jpg" {
			t.Fatalf("file locations changed after rollback: files=%+v", workbenchRepo.files)
		}
		if len(workbenchRepo.updatedFiles) != 0 || len(workbenchRepo.updatedItems) != 0 || len(workbenchRepo.refreshed) != 0 || len(workbenchRepo.events) != 0 {
			t.Fatalf("database side effects should roll back: files=%+v items=%+v refreshed=%+v events=%+v", workbenchRepo.updatedFiles, workbenchRepo.updatedItems, workbenchRepo.refreshed, workbenchRepo.events)
		}
		locked := workbenchRepo.items[8001]
		if locked.SettlementStatus != domain.AssetWorkbenchSettlementStatusInBatch || locked.CurrentSettlementBatchID == nil || *locked.CurrentSettlementBatchID != 7001 {
			t.Fatalf("concurrent settlement lock should remain: item=%+v", locked)
		}

		requests := transport.snapshot()
		putPaths := map[string]bool{}
		deletePaths := map[string]bool{}
		for _, request := range requests {
			switch request.Method {
			case http.MethodPut:
				if request.CopySource == "" {
					t.Fatalf("copy request is missing source: %+v", request)
				}
				putPaths[request.Path] = true
			case http.MethodDelete:
				deletePaths[request.Path] = true
			}
		}
		if len(putPaths) != 2 || len(deletePaths) != 2 {
			t.Fatalf("OSS requests = %+v, want two copies and cleanup of both targets", requests)
		}
		for path := range deletePaths {
			if !putPaths[path] {
				t.Fatalf("deleted non-target object %q; requests=%+v", path, requests)
			}
		}
	})

	t.Run("delete rolls back the complete file group", func(t *testing.T) {
		workbenchRepo := &batchFileMutationRepo{
			files: newFiles(),
			items: map[int64]*domain.AssetWorkbenchSubmissionItem{8001: newItem()},
		}
		txRunner := settlementLockRaceTxRunner{repo: workbenchRepo, itemID: 8001, batchID: 7002}
		svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, txRunner))
		actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSubmitter}}

		result, appErr := svc.BatchDeleteFiles(context.Background(), actor, BatchDeleteFilesParams{
			FileIDs: []int64{501, 502},
			Reason:  "重复上传",
		})
		if appErr != nil {
			t.Fatalf("BatchDeleteFiles() appErr = %+v", appErr)
		}
		if len(result.Deleted) != 0 || len(result.Failures) != 2 {
			t.Fatalf("result = %+v, want whole group rejected", result)
		}
		if workbenchRepo.files[501] == nil || workbenchRepo.files[502] == nil {
			t.Fatalf("complete file group should remain after rollback: files=%+v", workbenchRepo.files)
		}
		if len(workbenchRepo.deletedFiles) != 0 || len(workbenchRepo.updatedItems) != 0 || len(workbenchRepo.refreshed) != 0 || len(workbenchRepo.events) != 0 {
			t.Fatalf("database side effects should roll back: deleted=%+v items=%+v refreshed=%+v events=%+v", workbenchRepo.deletedFiles, workbenchRepo.updatedItems, workbenchRepo.refreshed, workbenchRepo.events)
		}
		locked := workbenchRepo.items[8001]
		if locked.SettlementStatus != domain.AssetWorkbenchSettlementStatusInBatch || locked.CurrentSettlementBatchID == nil || *locked.CurrentSettlementBatchID != 7002 {
			t.Fatalf("concurrent settlement lock should remain: item=%+v", locked)
		}
	})
}

func TestRepriceSubmissionItemUsesFrozenWorkerSnapshot(t *testing.T) {
	submittedAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	workbenchRepo := &itemActionRepo{
		items: map[int64]*domain.AssetWorkbenchSubmissionItem{
			501: {
				ID:                 501,
				SubmissionID:       9001,
				PayeeUserID:        77,
				OrderNo:            "ORD-501",
				DifficultyClass:    "A",
				Finalized:          true,
				PageCount:          3,
				ItemCount:          1,
				BusinessMonth:      "2026-06",
				SubmittedAt:        submittedAt,
				WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
				JobGradeSnapshot:   "P1",
				GrossAmount:        15,
				PricingStatus:      domain.AssetWorkbenchPricingStatusPriced,
				QCStatus:           domain.AssetWorkbenchSubmissionStatusSubmitted,
				SettlementStatus:   domain.AssetWorkbenchSettlementStatusUnsettled,
			},
		},
		price: &domain.AssetWorkbenchPriceMatrix{
			ID:              88,
			WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
			JobGrade:        "P1",
			DifficultyClass: "A",
			UnitPrice:       20,
			EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	updated, appErr := svc.RepriceSubmissionItem(context.Background(), actor, 501, RepriceSubmissionItemParams{Reason: "补价"})
	if appErr != nil {
		t.Fatalf("RepriceSubmissionItem() error = %+v", appErr)
	}
	if updated.GrossAmount != 60 || updated.BasePriceRuleID == nil || *updated.BasePriceRuleID != 88 {
		t.Fatalf("repriced item = %+v", updated)
	}
	if workbenchRepo.refreshCalls != 1 {
		t.Fatalf("refreshCalls = %d, want 1", workbenchRepo.refreshCalls)
	}
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventItemRepriced {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
}

func TestRepriceSubmissionItemUsesCurrentProfileWhenGradeSnapshotMissing(t *testing.T) {
	submittedAt := time.Date(2026, 7, 1, 14, 56, 13, 0, time.UTC)
	workbenchRepo := &itemActionRepo{
		items: map[int64]*domain.AssetWorkbenchSubmissionItem{
			501: {
				ID:               501,
				SubmissionID:     9001,
				PayeeUserID:      77,
				OrderNo:          "6954064249637049871",
				DifficultyClass:  "A",
				Finalized:        true,
				PageCount:        3,
				ItemCount:        1,
				BusinessMonth:    "2026-07",
				SubmittedAt:      submittedAt,
				PricingStatus:    domain.AssetWorkbenchPricingStatusPendingGrade,
				QCStatus:         domain.AssetWorkbenchSubmissionStatusSubmitted,
				SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled,
			},
		},
		profile: &domain.AssetWorkbenchProfile{
			UserID:     77,
			WorkerType: domain.AssetWorkbenchWorkerTypeParttime,
			JobGrade:   "J1",
		},
		price: &domain.AssetWorkbenchPriceMatrix{
			ID:              99,
			WorkerType:      domain.AssetWorkbenchWorkerTypeParttime,
			JobGrade:        "J1",
			DifficultyClass: "A",
			UnitPrice:       1.5,
			EffectiveFrom:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	updated, appErr := svc.RepriceSubmissionItem(context.Background(), actor, 501, RepriceSubmissionItemParams{Reason: "定级后重计价"})
	if appErr != nil {
		t.Fatalf("RepriceSubmissionItem() error = %+v", appErr)
	}
	if updated.WorkerTypeSnapshot != domain.AssetWorkbenchWorkerTypeParttime || updated.JobGradeSnapshot != "J1" {
		t.Fatalf("snapshots = %q/%q, want parttime/J1", updated.WorkerTypeSnapshot, updated.JobGradeSnapshot)
	}
	if updated.PricingStatus != domain.AssetWorkbenchPricingStatusPriced || updated.GrossAmount != 4.5 {
		t.Fatalf("pricing = %s gross=%v, want priced 4.5", updated.PricingStatus, updated.GrossAmount)
	}
	if updated.BasePriceRuleID == nil || *updated.BasePriceRuleID != 99 {
		t.Fatalf("base price rule = %v, want 99", updated.BasePriceRuleID)
	}
}

func TestBuildSubmissionItemFreezesGrossOnly(t *testing.T) {
	submittedAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &priceOnlyRepo{price: &domain.AssetWorkbenchPriceMatrix{
		ID:              42,
		WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
		JobGrade:        "P1",
		DifficultyClass: "A",
		UnitPrice:       12.5,
		EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}}

	item, appErr := svc.buildSubmissionItem(context.Background(), 1001, 2002, submittedAt, "2026-06", &domain.AssetWorkbenchProfile{
		UserID:     1001,
		WorkerType: domain.AssetWorkbenchWorkerTypeFulltime,
		JobGrade:   "P1",
	}, CreateSubmissionItemParams{
		OrderNo:         "ORD-1",
		DifficultyClass: "A",
		PageCount:       3,
	})
	if appErr != nil {
		t.Fatalf("buildSubmissionItem returned app error: %v", appErr)
	}
	if item.GrossAmount != 37.5 {
		t.Fatalf("gross amount = %v, want 37.5", item.GrossAmount)
	}
	if item.PricingStatus != domain.AssetWorkbenchPricingStatusPriced {
		t.Fatalf("pricing status = %q", item.PricingStatus)
	}

	var snapshot map[string]interface{}
	if err := json.Unmarshal(item.PricingSnapshot, &snapshot); err != nil {
		t.Fatalf("pricing snapshot json: %v", err)
	}
	if snapshot["deduction_timing"] != "settlement_time" {
		t.Fatalf("deduction_timing = %v, want settlement_time", snapshot["deduction_timing"])
	}
	if _, exists := snapshot["deduction_amount"]; exists {
		t.Fatalf("submission pricing snapshot must not freeze deduction_amount: %#v", snapshot)
	}
}

func TestBuildSubmissionItemGeneratesInternalOrderNoWhenOmitted(t *testing.T) {
	submittedAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &priceOnlyRepo{price: &domain.AssetWorkbenchPriceMatrix{
		ID:              42,
		WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
		JobGrade:        "P1",
		DifficultyClass: "A",
		UnitPrice:       12.5,
		EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}}

	item, appErr := svc.buildSubmissionItem(context.Background(), 1001, 2002, submittedAt, "2026-06", &domain.AssetWorkbenchProfile{
		UserID:     1001,
		WorkerType: domain.AssetWorkbenchWorkerTypeFulltime,
		JobGrade:   "P1",
	}, CreateSubmissionItemParams{
		DifficultyClass: "A",
		PageCount:       1,
	})
	if appErr != nil {
		t.Fatalf("buildSubmissionItem returned app error: %v", appErr)
	}
	if !strings.HasPrefix(item.OrderNo, "AWF20260625103000") {
		t.Fatalf("generated order no = %q, want AWF timestamp prefix", item.OrderNo)
	}
	if item.GrossAmount != 12.5 {
		t.Fatalf("gross amount = %v, want 12.5", item.GrossAmount)
	}
}

func TestBuildSubmissionItemAppliesSingleWinnerPromoToGross(t *testing.T) {
	submittedAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	fixedPrice := 9.0
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &priceOnlyRepo{
		price: &domain.AssetWorkbenchPriceMatrix{
			ID:              42,
			WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
			JobGrade:        "P1",
			DifficultyClass: "A",
			UnitPrice:       12.5,
			EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		coupons: []*domain.AssetWorkbenchPromoCoupon{
			{
				ID:              8,
				CouponCode:      "618-A",
				CouponName:      "618 A 类一口价",
				Mode:            domain.AssetWorkbenchPromoModeFixedPrice,
				Amount:          &fixedPrice,
				Priority:        1,
				WorkerType:      domain.AssetWorkbenchWorkerTypeAll,
				JobGrade:        domain.AssetWorkbenchWorkerTypeAll,
				DifficultyClass: "A",
				Enabled:         true,
			},
		},
	}

	item, appErr := svc.buildSubmissionItem(context.Background(), 1001, 2002, submittedAt, "2026-06", &domain.AssetWorkbenchProfile{
		UserID:     1001,
		WorkerType: domain.AssetWorkbenchWorkerTypeFulltime,
		JobGrade:   "P1",
	}, CreateSubmissionItemParams{
		OrderNo:         "ORD-1",
		DifficultyClass: "A",
		PageCount:       3,
	})
	if appErr != nil {
		t.Fatalf("buildSubmissionItem returned app error: %v", appErr)
	}
	if item.GrossAmount != 27 {
		t.Fatalf("gross amount = %v, want 27", item.GrossAmount)
	}
	if item.PromoCouponID == nil || *item.PromoCouponID != 8 {
		t.Fatalf("promo coupon id = %v, want 8", item.PromoCouponID)
	}
	var snapshot map[string]interface{}
	if err := json.Unmarshal(item.PricingSnapshot, &snapshot); err != nil {
		t.Fatalf("pricing snapshot json: %v", err)
	}
	if snapshot["promo_applied"] != true {
		t.Fatalf("promo_applied = %v, want true", snapshot["promo_applied"])
	}
	if snapshot["deduction_timing"] != "settlement_time" {
		t.Fatalf("deduction_timing = %v, want settlement_time", snapshot["deduction_timing"])
	}
}

func TestBuildSettlementPreviewCalculatesDeductionsAtSettlementTime(t *testing.T) {
	submittedAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &deductionOnlyRepo{rule: &domain.AssetWorkbenchDeductionRule{
		ID:              7,
		WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
		JobGrade:        "P1",
		DifficultyClass: "A",
		DeductionAmount: 4,
		EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}}
	itemID := int64(501)
	preview, appErr := svc.buildSettlementPreview(context.Background(), "2026-06", []*domain.AssetWorkbenchSubmissionItem{
		{
			ID:                 itemID,
			PayeeUserID:        1001,
			OrderNo:            "ORD-1",
			DifficultyClass:    "A",
			PageCount:          2,
			GrossAmount:        25,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			SubmittedAt:        submittedAt,
		},
	}, []*domain.AssetWorkbenchErrorRecord{
		{
			SubmissionItemID: &itemID,
			OrderNo:          "ORD-1",
			ErrorCount:       3,
		},
	}, nil)
	if appErr != nil {
		t.Fatalf("buildSettlementPreview returned app error: %v", appErr)
	}
	if preview.Totals.GrossAmount != 25 {
		t.Fatalf("gross total = %v, want 25", preview.Totals.GrossAmount)
	}
	if preview.Totals.DeductionAmount != 12 {
		t.Fatalf("deduction total = %v, want 12", preview.Totals.DeductionAmount)
	}
	if preview.Totals.NetAmount != 13 {
		t.Fatalf("net total = %v, want 13", preview.Totals.NetAmount)
	}
	if len(preview.PayrollRows) != 2 {
		t.Fatalf("payroll row count = %d, want 2", len(preview.PayrollRows))
	}
	if preview.PayrollRows[0].RowType != domain.AssetWorkbenchPayrollRowTypeNormalPiecework || preview.PayrollRows[0].NetAmount != 13 {
		t.Fatalf("normal payroll row = %+v, want net 13", preview.PayrollRows[0])
	}
	if preview.PayrollRows[1].RowType != domain.AssetWorkbenchPayrollRowTypeSupplementPiecework || preview.PayrollRows[1].NetAmount != 0 {
		t.Fatalf("supplement payroll row = %+v, want zero supplement row", preview.PayrollRows[1])
	}
}

func TestBuildSettlementPreviewCalculatesQualityErrorDeductionByDifficulty(t *testing.T) {
	payeeID := int64(1001)
	occurred := time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC)
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &settlementReportRepo{
		rule: &domain.AssetWorkbenchDeductionRule{
			ID:              9,
			WorkerType:      domain.AssetWorkbenchWorkerTypeAll,
			JobGrade:        domain.AssetWorkbenchWorkerTypeAll,
			DifficultyClass: "A",
			DeductionAmount: 10,
			EffectiveFrom:   time.Date(2026, 7, 1, 0, 0, 0, 0, time.UTC),
		},
		profiles: map[int64]*domain.AssetWorkbenchProfile{
			payeeID: {UserID: payeeID, RealName: "张三", WorkerType: domain.AssetWorkbenchWorkerTypeParttime, JobGrade: "J1"},
		},
	}
	preview, appErr := svc.buildSettlementPreview(context.Background(), "2026-07", nil, []*domain.AssetWorkbenchErrorRecord{
		{
			ID:              801,
			BusinessMonth:   "2026-07",
			PayeeUserID:     &payeeID,
			OrderNo:         "3310254339917022991",
			DifficultyClass: "A",
			OccurredDate:    &occurred,
			ErrorCount:      8,
			MatchStatus:     domain.AssetWorkbenchErrorMatchStatusMatched,
		},
	}, nil)
	if appErr != nil {
		t.Fatalf("buildSettlementPreview returned app error: %v", appErr)
	}
	if preview.Totals.ErrorCount != 8 || preview.Totals.DeductionAmount != 80 || preview.Totals.NetAmount != -80 {
		t.Fatalf("totals = %+v, want 8 errors, 80 deduction, -80 net", preview.Totals)
	}
	if len(preview.PayrollRows) != 2 || preview.PayrollRows[0].PayeeUserID != payeeID || preview.PayrollRows[0].DeductionAmount != 80 {
		t.Fatalf("payroll rows = %+v, want quality deduction in normal row", preview.PayrollRows)
	}
}

func TestBuildSettlementPreviewKeepsSupplementInSecondPayrollRow(t *testing.T) {
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	preview, appErr := svc.buildSettlementPreview(context.Background(), "2026-06", []*domain.AssetWorkbenchSubmissionItem{
		{
			ID:              501,
			PayeeUserID:     1001,
			OrderNo:         "ORD-1",
			DifficultyClass: "A",
			PageCount:       2,
			GrossAmount:     25,
		},
	}, nil, []*domain.AssetWorkbenchSettlementSupplement{
		{
			ID:            77,
			PayeeUserID:   1001,
			BusinessMonth: "2026-06",
			OrderNo:       "SUP-1",
			PageCount:     3,
			GrossAmount:   18,
			Status:        domain.AssetWorkbenchSupplementStatusApproved,
		},
	})
	if appErr != nil {
		t.Fatalf("buildSettlementPreview returned app error: %v", appErr)
	}
	if len(preview.PayrollRows) != 2 {
		t.Fatalf("payroll row count = %d, want fixed normal+supplement rows", len(preview.PayrollRows))
	}
	normal := preview.PayrollRows[0]
	supplement := preview.PayrollRows[1]
	if normal.RowType != domain.AssetWorkbenchPayrollRowTypeNormalPiecework || normal.NetAmount != 25 || normal.SupplementAmount != 0 {
		t.Fatalf("normal payroll row = %+v, want normal net 25 without supplement amount", normal)
	}
	if supplement.RowType != domain.AssetWorkbenchPayrollRowTypeSupplementPiecework || supplement.NetAmount != 18 || supplement.SupplementAmount != 18 {
		t.Fatalf("supplement payroll row = %+v, want supplement net 18", supplement)
	}
	if preview.Totals.NetAmount != 43 || preview.Totals.SupplementAmount != 18 {
		t.Fatalf("totals = %+v, want net 43 and supplement 18", preview.Totals)
	}
}

func TestMatchedErrorCountIgnoresUnmatchedAndAmbiguousRecords(t *testing.T) {
	itemID := int64(501)
	item := &domain.AssetWorkbenchSubmissionItem{
		ID:          itemID,
		PayeeUserID: 1001,
		OrderNo:     "ORD-1",
	}

	count := matchedErrorCount([]*domain.AssetWorkbenchErrorRecord{
		{
			SubmissionItemID: &itemID,
			OrderNo:          "ORD-1",
			ErrorCount:       2,
			MatchStatus:      domain.AssetWorkbenchErrorMatchStatusMatched,
		},
		{
			OrderNo:     "ORD-1",
			ErrorCount:  3,
			MatchStatus: domain.AssetWorkbenchErrorMatchStatusAmbiguous,
		},
		{
			OrderNo:     "ORD-1",
			ErrorCount:  4,
			MatchStatus: domain.AssetWorkbenchErrorMatchStatusUnmatched,
		},
		{
			OrderNo:     "ORD-1",
			ErrorCount:  1,
			MatchStatus: "",
		},
	}, item)
	if count != 3 {
		t.Fatalf("matched error count = %d, want matched + legacy fallback only", count)
	}
}

func TestBuildSettlementPreviewAddsWelfareAndSupplementsByPersonMonth(t *testing.T) {
	submittedAt := time.Date(2026, 6, 25, 10, 30, 0, 0, time.UTC)
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &deductionOnlyRepo{
		rule: &domain.AssetWorkbenchDeductionRule{
			ID:              7,
			WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
			JobGrade:        "P1",
			DifficultyClass: "A",
			DeductionAmount: 0,
			EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		welfareRules: []*domain.AssetWorkbenchWelfareRule{
			{
				ID:         9,
				RuleName:   "无差错奖",
				WorkerType: domain.AssetWorkbenchWorkerTypeAll,
				JobGrade:   domain.AssetWorkbenchWorkerTypeAll,
				RuleType:   "manual",
				Amount:     30,
				Enabled:    true,
			},
		},
	}
	preview, appErr := svc.buildSettlementPreview(context.Background(), "2026-06", []*domain.AssetWorkbenchSubmissionItem{
		{
			ID:                 501,
			PayeeUserID:        1001,
			OrderNo:            "ORD-1",
			DifficultyClass:    "A",
			PageCount:          2,
			GrossAmount:        25,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			SubmittedAt:        submittedAt,
		},
	}, nil, []*domain.AssetWorkbenchSettlementSupplement{
		{
			ID:            11,
			PayeeUserID:   1001,
			BusinessMonth: "2026-06",
			Status:        domain.AssetWorkbenchSupplementStatusApproved,
			OrderNo:       "MISS-1",
			PageCount:     1,
			GrossAmount:   15,
		},
	})
	if appErr != nil {
		t.Fatalf("buildSettlementPreview returned app error: %v", appErr)
	}
	if preview.Totals.WelfareAmount != 30 {
		t.Fatalf("welfare total = %v, want 30", preview.Totals.WelfareAmount)
	}
	if preview.Totals.SupplementAmount != 15 {
		t.Fatalf("supplement total = %v, want 15", preview.Totals.SupplementAmount)
	}
	if preview.Totals.NetAmount != 70 {
		t.Fatalf("net total = %v, want 70", preview.Totals.NetAmount)
	}
	if len(preview.PayrollRows) != 2 {
		t.Fatalf("payroll row count = %d, want 2", len(preview.PayrollRows))
	}
	normal := preview.PayrollRows[0]
	supplement := preview.PayrollRows[1]
	if normal.RowType != domain.AssetWorkbenchPayrollRowTypeNormalPiecework || normal.NetAmount != 55 || normal.SupplementAmount != 0 {
		t.Fatalf("normal payroll row = %+v, want normal net 55 without supplement", normal)
	}
	if supplement.RowType != domain.AssetWorkbenchPayrollRowTypeSupplementPiecework || supplement.NetAmount != 15 || supplement.SupplementAmount != 15 {
		t.Fatalf("supplement payroll row = %+v, want supplement net 15", supplement)
	}
}

func TestBuildSettlementPreviewEmitsTwoPayrollRowsForSupplementOnlyPayee(t *testing.T) {
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &deductionOnlyRepo{}

	preview, appErr := svc.buildSettlementPreview(context.Background(), "2026-06", nil, nil, []*domain.AssetWorkbenchSettlementSupplement{
		{
			ID:            11,
			PayeeUserID:   1002,
			BusinessMonth: "2026-06",
			Status:        domain.AssetWorkbenchSupplementStatusApproved,
			OrderNo:       "MISS-2",
			PageCount:     3,
			GrossAmount:   42,
		},
	})
	if appErr != nil {
		t.Fatalf("buildSettlementPreview returned app error: %v", appErr)
	}
	if len(preview.PayrollRows) != 2 {
		t.Fatalf("payroll row count = %d, want 2", len(preview.PayrollRows))
	}
	normal := preview.PayrollRows[0]
	supplement := preview.PayrollRows[1]
	if normal.PayeeUserID != 1002 || normal.RowType != domain.AssetWorkbenchPayrollRowTypeNormalPiecework || normal.NetAmount != 0 {
		t.Fatalf("normal payroll row = %+v, want zero normal row for supplement-only payee", normal)
	}
	if supplement.PayeeUserID != 1002 || supplement.RowType != domain.AssetWorkbenchPayrollRowTypeSupplementPiecework || supplement.PageCount != 3 || supplement.NetAmount != 42 {
		t.Fatalf("supplement payroll row = %+v, want supplement net 42", supplement)
	}
}

func TestBuildSettlementReportSplitsDifficultyAndDistinctOrders(t *testing.T) {
	submittedAt := time.Date(2026, 6, 1, 16, 30, 0, 0, time.UTC)
	itemAID := int64(501)
	itemBID := int64(502)
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = &settlementReportRepo{
		rule: &domain.AssetWorkbenchDeductionRule{
			ID:              7,
			WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
			JobGrade:        "P1",
			DifficultyClass: "A",
			DeductionAmount: 4,
			EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		profiles: map[int64]*domain.AssetWorkbenchProfile{
			1001: {UserID: 1001, RealName: "Alice", JobGrade: "P1"},
			1002: {UserID: 1002, RealName: "Bob", JobGrade: "P2"},
		},
	}

	report, appErr := svc.buildSettlementReport(context.Background(), "2026-06", []*domain.AssetWorkbenchSubmissionItem{
		{
			ID:                 itemAID,
			PayeeUserID:        1001,
			OrderNo:            "ORD-1",
			DifficultyClass:    "A",
			PageCount:          2,
			GrossAmount:        20,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			SubmittedAt:        submittedAt,
		},
		{
			ID:                 itemBID,
			PayeeUserID:        1001,
			OrderNo:            "ORD-1",
			DifficultyClass:    "B",
			PageCount:          3,
			GrossAmount:        30,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			SubmittedAt:        submittedAt.Add(25 * time.Hour),
		},
		{
			ID:                 503,
			PayeeUserID:        1002,
			OrderNo:            "ORD-2",
			DifficultyClass:    "A",
			PageCount:          5,
			GrossAmount:        50,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P2",
			SubmittedAt:        submittedAt,
		},
	}, []*domain.AssetWorkbenchErrorRecord{
		{SubmissionItemID: &itemAID, OrderNo: "ORD-1", ErrorCount: 2, MatchStatus: domain.AssetWorkbenchErrorMatchStatusMatched},
	}, []*domain.AssetWorkbenchSettlementSupplement{
		{
			ID:              11,
			PayeeUserID:     1001,
			BusinessMonth:   "2026-06",
			Status:          domain.AssetWorkbenchSupplementStatusApproved,
			OrderNo:         "SUP-1",
			DifficultyClass: "C",
			PageCount:       1,
			GrossAmount:     12,
			CreatedAt:       submittedAt,
		},
	})
	if appErr != nil {
		t.Fatalf("buildSettlementReport returned app error: %v", appErr)
	}
	if strings.Join(report.DifficultyClasses, ",") != "A,B,C" {
		t.Fatalf("difficulty classes = %+v, want A,B,C", report.DifficultyClasses)
	}
	if report.Totals.OrderCount != 3 || report.Totals.PageCount != 11 || report.Totals.ErrorCount != 2 || report.Totals.DeductionAmount != 8 || report.Totals.NetAmount != 104 {
		t.Fatalf("totals = %+v, want distinct orders=3 pages=11 errors=2 deduction=8 net=104", report.Totals)
	}
	var normal *SettlementReportRow
	var supplement *SettlementReportRow
	for index := range report.Rows {
		row := &report.Rows[index]
		if row.PayeeUserID == 1001 && row.RowType == domain.AssetWorkbenchPayrollRowTypeNormalPiecework {
			normal = row
		}
		if row.PayeeUserID == 1001 && row.RowType == domain.AssetWorkbenchPayrollRowTypeSupplementPiecework {
			supplement = row
		}
	}
	if normal == nil {
		t.Fatalf("missing normal report row: %+v", report.Rows)
	}
	if normal.CreatorName != "Alice" || normal.JobGrade != "P1" || normal.CreatedDate != "2026-06-02" || normal.CreatedDateEnd != "2026-06-03" {
		t.Fatalf("normal identity fields = %+v, want Alice/P1/2026-06-02 through 2026-06-03", normal)
	}
	if normal.OrderCount != 1 || normal.ItemCount != 2 || normal.PageCount != 5 || normal.GrossAmount != 50 || normal.ErrorCount != 2 || normal.DeductionAmount != 8 || normal.NetAmount != 42 {
		t.Fatalf("normal row = %+v, want distinct order count and net after deduction", normal)
	}
	if normal.PageCountShare != float64(5)/float64(11) || normal.ErrorRate != float64(2)/float64(5) || normal.MonthAmountShare != float64(42)/float64(104) {
		t.Fatalf("normal ratios = %+v, want page/error/month shares", normal)
	}
	if len(normal.DifficultyMetrics) != 2 || normal.DifficultyMetrics[0].DifficultyClass != "A" || normal.DifficultyMetrics[0].PageCount != 2 || normal.DifficultyMetrics[0].DeductionAmount != 8 {
		t.Fatalf("normal difficulty metrics = %+v, want A/B split with deduction on A", normal.DifficultyMetrics)
	}
	if supplement == nil || supplement.OrderCount != 1 || supplement.SupplementAmount != 12 || supplement.NetAmount != 12 || len(supplement.DifficultyMetrics) != 1 || supplement.DifficultyMetrics[0].DifficultyClass != "C" {
		t.Fatalf("supplement row = %+v, want C supplement metric", supplement)
	}
}

func TestBuildSettlementReportCachesDeductionRulesByDimensionDate(t *testing.T) {
	submittedAt := time.Date(2026, 6, 3, 10, 0, 0, 0, time.UTC)
	firstItemID := int64(601)
	secondItemID := int64(602)
	workbenchRepo := &settlementReportRepo{
		rule: &domain.AssetWorkbenchDeductionRule{
			ID:              7,
			WorkerType:      domain.AssetWorkbenchWorkerTypeFulltime,
			JobGrade:        "P1",
			DifficultyClass: "A",
			DeductionAmount: 4,
			EffectiveFrom:   time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
		},
		profiles: map[int64]*domain.AssetWorkbenchProfile{
			1001: {UserID: 1001, RealName: "Alice", JobGrade: "P1"},
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"})
	svc.repo = workbenchRepo

	report, appErr := svc.buildSettlementReport(context.Background(), "2026-06", []*domain.AssetWorkbenchSubmissionItem{
		{
			ID:                 firstItemID,
			PayeeUserID:        1001,
			OrderNo:            "ORD-601",
			DifficultyClass:    "A",
			PageCount:          1,
			GrossAmount:        10,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			SubmittedAt:        submittedAt,
		},
		{
			ID:                 secondItemID,
			PayeeUserID:        1001,
			OrderNo:            "ORD-602",
			DifficultyClass:    "A",
			PageCount:          1,
			GrossAmount:        10,
			WorkerTypeSnapshot: domain.AssetWorkbenchWorkerTypeFulltime,
			JobGradeSnapshot:   "P1",
			SubmittedAt:        submittedAt.Add(30 * time.Minute),
		},
	}, []*domain.AssetWorkbenchErrorRecord{
		{SubmissionItemID: &firstItemID, OrderNo: "ORD-601", ErrorCount: 1, MatchStatus: domain.AssetWorkbenchErrorMatchStatusMatched},
		{SubmissionItemID: &secondItemID, OrderNo: "ORD-602", ErrorCount: 2, MatchStatus: domain.AssetWorkbenchErrorMatchStatusMatched},
	}, nil)
	if appErr != nil {
		t.Fatalf("buildSettlementReport returned app error: %v", appErr)
	}
	if workbenchRepo.deductionCalls != 1 {
		t.Fatalf("deduction rule lookups = %d, want 1", workbenchRepo.deductionCalls)
	}
	if report.Totals.DeductionAmount != 12 {
		t.Fatalf("deduction total = %v, want 12", report.Totals.DeductionAmount)
	}
}

func TestBuildSettlementPayrollRowsFromBatchItems(t *testing.T) {
	rows := buildSettlementPayrollRowsFromItems("2026-06", []*domain.AssetWorkbenchSettlementItem{
		{ItemType: domain.AssetWorkbenchItemTypeGrossPiecework, PayeeUserID: 1001, BusinessMonth: "2026-06", Amount: 100, Quantity: 4, Direction: "credit"},
		{ItemType: domain.AssetWorkbenchItemTypeErrorDeduction, PayeeUserID: 1001, BusinessMonth: "2026-06", Amount: 12, Quantity: 3, Direction: "debit"},
		{ItemType: domain.AssetWorkbenchItemTypeWelfare, PayeeUserID: 1001, BusinessMonth: "2026-06", Amount: 30, Quantity: 1, Direction: "credit"},
		{ItemType: domain.AssetWorkbenchItemTypeSupplement, PayeeUserID: 1001, BusinessMonth: "2026-06", Amount: 15, Quantity: 1, Direction: "credit"},
	})
	if len(rows) != 2 {
		t.Fatalf("payroll row count = %d, want 2", len(rows))
	}
	if rows[0].RowType != domain.AssetWorkbenchPayrollRowTypeNormalPiecework || rows[0].NetAmount != 118 {
		t.Fatalf("normal payroll row = %+v, want net 118", rows[0])
	}
	if rows[1].RowType != domain.AssetWorkbenchPayrollRowTypeSupplementPiecework || rows[1].NetAmount != 15 {
		t.Fatalf("supplement payroll row = %+v, want net 15", rows[1])
	}
}

func TestParseErrorRecordsExcelSupportsChineseHeaders(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	if err := f.SetSheetRow(sheet, "A1", &[]interface{}{"订单号", "出错数", "人员ID"}); err != nil {
		t.Fatalf("set header row: %v", err)
	}
	if err := f.SetSheetRow(sheet, "A2", &[]interface{}{"ORD-1", 3, 1001}); err != nil {
		t.Fatalf("set data row: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	records, appErr := parseErrorRecordsExcel(bytes.NewReader(buf.Bytes()))
	if appErr != nil {
		t.Fatalf("parseErrorRecordsExcel appErr = %v", appErr)
	}
	if len(records) != 1 {
		t.Fatalf("records len = %d, want 1", len(records))
	}
	if records[0].OrderNo != "ORD-1" || records[0].ErrorCount != 3 {
		t.Fatalf("record = %+v", records[0])
	}
	if records[0].PayeeUserID == nil || *records[0].PayeeUserID != 1001 {
		t.Fatalf("payee = %v, want 1001", records[0].PayeeUserID)
	}
}

func TestParseErrorRecordsExcelDefaultsFormalTemplateErrorCountToOne(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	if err := f.SetSheetRow(sheet, "A1", &[]interface{}{"说明：", "格式", "", "关联我们的分类，用于计价的", "绑定系统全职/兼职注册人"}); err != nil {
		t.Fatalf("set description row: %v", err)
	}
	if err := f.SetSheetRow(sheet, "A2", &[]interface{}{"导入模板：", "日期", "线上订单号", "分类", "出错人", "问题描述", "抽查/售后", "处理方法", "登记人", "备注"}); err != nil {
		t.Fatalf("set header row: %v", err)
	}
	if err := f.SetSheetRow(sheet, "A3", &[]interface{}{"", "2026-07-01", "", "C类", "张三", "年龄做错了", "", "", "", ""}); err != nil {
		t.Fatalf("set data row: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	records, appErr := parseErrorRecordsExcel(bytes.NewReader(buf.Bytes()))
	if appErr != nil {
		t.Fatalf("parseErrorRecordsExcel appErr = %v", appErr)
	}
	if len(records) != 1 || records[0].ErrorCount != 1 || records[0].OrderNo != "" || records[0].DifficultyClass != "C类" || records[0].PayeeName != "张三" {
		t.Fatalf("records = %+v, want one formal row defaulting to one error", records)
	}
}

func TestParseErrorRecordsExcelSupportsFormalTemplateWithoutOrderColumn(t *testing.T) {
	f := excelize.NewFile()
	sheet := f.GetSheetName(f.GetActiveSheetIndex())
	if err := f.SetSheetRow(sheet, "A1", &[]interface{}{"日期", "出错人", "出错分类", "出错张数", "问题描述"}); err != nil {
		t.Fatalf("set header row: %v", err)
	}
	if err := f.SetSheetRow(sheet, "A2", &[]interface{}{"2026-07-01", "李四", "B类", 3, "文件尺寸不对"}); err != nil {
		t.Fatalf("set data row: %v", err)
	}
	var buf bytes.Buffer
	if err := f.Write(&buf); err != nil {
		t.Fatalf("write workbook: %v", err)
	}
	records, appErr := parseErrorRecordsExcel(bytes.NewReader(buf.Bytes()))
	if appErr != nil {
		t.Fatalf("parseErrorRecordsExcel appErr = %v", appErr)
	}
	if len(records) != 1 || records[0].OrderNo != "" || records[0].DifficultyClass != "B类" || records[0].PayeeName != "李四" || records[0].ErrorCount != 3 {
		t.Fatalf("records = %+v, want one quality row without order_no", records)
	}
}

func TestWorkbenchOperationalMaterialVisibilityRestrictsQuarkRoots(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{path: "/quark", want: true},
		{path: "/quark/海报/2026/a.jpg", want: true},
		{path: "/quark/kt板/a.jpg", want: true},
		{path: "/quark/电视投屏/a.jpg", want: true},
		{path: "/quark/闲置kt板/a.jpg", want: true},
		{path: "/quark/其他目录/a.jpg", want: false},
		{path: "/p3/仓库素材区/a.jpg", want: true},
		{path: "/系统资源/a.jpg", want: true},
	}
	for _, tc := range cases {
		if got := assetWorkbenchOperationalMaterialPathVisible(tc.path); got != tc.want {
			t.Fatalf("assetWorkbenchOperationalMaterialPathVisible(%q) = %v, want %v", tc.path, got, tc.want)
		}
	}
}

func TestBrowseMaterialsVirtualQuarkRootUsesVisibleFolderCounts(t *testing.T) {
	provider := &externalMaterialProviderStub{browseResults: map[string]*assetcenter.MaterialBrowseResult{
		"": {
			Path: "",
			Folders: []assetcenter.MaterialFolder{
				{Path: "/quark", Name: "quark", SourceType: string(domain.AssetResourceSourceExternal), FileCount: 106351},
			},
			Page: 1,
			Size: 100,
		},
		workbenchQuarkMaterialActualBase: {
			Path: workbenchQuarkMaterialActualBase,
			Folders: []assetcenter.MaterialFolder{
				{Path: workbenchQuarkMaterialActualBase + "/电视投屏", Name: "电视投屏", SourceType: string(domain.AssetResourceSourceExternal), FileCount: 620, DirectFileCount: 56},
				{Path: workbenchQuarkMaterialActualBase + "/海报", Name: "海报", SourceType: string(domain.AssetResourceSourceExternal), FileCount: 512, DirectFileCount: 15},
				{Path: workbenchQuarkMaterialActualBase + "/kt板", Name: "kt板", SourceType: string(domain.AssetResourceSourceExternal), FileCount: 1770, DirectFileCount: 21},
				{Path: workbenchQuarkMaterialActualBase + "/闲置kt板", Name: "闲置kt板", SourceType: string(domain.AssetResourceSourceExternal), FileCount: 162, DirectFileCount: 1},
			},
			Page: 1,
			Size: 100,
		},
	}}
	groups := &resourceGroupMaterialProviderStub{items: []domain.TaskAssetGroup{testResourceGroupMaterial(1, time.Now().UTC())}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetSearcher(provider), WithResourceGroupMaterialSearcher(groups))
	actor := assetCapabilityActor(1, domain.PermissionAssetView)

	root, appErr := svc.BrowseMaterials(context.Background(), actor, "", 1, 100, "all", "all", "")
	if appErr != nil {
		t.Fatalf("BrowseMaterials(root) error = %+v", appErr)
	}
	if len(root.Folders) != 2 || root.Folders[0].Path != "/quark" || root.Folders[0].FileCount != 3064 || root.Folders[0].DirectFileCount != 0 || root.Folders[1].Path != workbenchMaterialSystemRoot {
		t.Fatalf("root folders = %+v, want /quark plus finalized resource-group root", root.Folders)
	}
	if len(root.Files) != 1 || root.Files[0].SourceType != "task_resource_group" {
		t.Fatalf("root files = %+v, want finalized resource groups and no legacy system files", root.Files)
	}

	quark, appErr := svc.BrowseMaterials(context.Background(), actor, "/quark", 1, 100, "all", "all", "")
	if appErr != nil {
		t.Fatalf("BrowseMaterials(/quark) error = %+v", appErr)
	}
	if quark.Total != 3064 {
		t.Fatalf("/quark total = %d, want visible folder total 3064", quark.Total)
	}
	if len(quark.Folders) != 4 {
		t.Fatalf("/quark folders = %+v, want four virtual folders", quark.Folders)
	}
	for _, folder := range quark.Folders {
		if !strings.HasPrefix(folder.Path, "/quark/") {
			t.Fatalf("folder path = %q, want virtual /quark path", folder.Path)
		}
	}
}

func TestWorkbenchMaterialAssetVisibilityKeepsSystemAssets(t *testing.T) {
	if !assetWorkbenchMaterialAssetVisible(&assetcenter.AssetDetail{SourceType: string(domain.AssetResourceSourceSystem), OriginPath: "/quark/其他目录/a.jpg"}) {
		t.Fatalf("system asset should remain visible even when filename resembles a hidden quark path")
	}
	if assetWorkbenchMaterialAssetVisible(&assetcenter.AssetDetail{SourceType: string(domain.AssetResourceSourceExternal), OriginPath: "/quark/其他目录/a.jpg"}) {
		t.Fatalf("external asset under hidden quark root should be filtered")
	}
}

func TestSystemSearchAllSourcesKeepsSystemAndVisibleExternalAssets(t *testing.T) {
	now := time.Now().UTC()
	groups := &resourceGroupMaterialProviderStub{items: []domain.TaskAssetGroup{testResourceGroupMaterial(1, now)}}
	provider := &externalMaterialProviderStub{pagedSearch: []*assetcenter.AssetDetail{
		{ID: 2, SourceType: string(domain.AssetResourceSourceExternal), OriginPath: "/quark/海报/external.png", UpdatedAt: now.Add(-time.Minute)},
		{ID: 3, SourceType: string(domain.AssetResourceSourceExternal), OriginPath: "/quark/其他目录/hidden.png", UpdatedAt: now.Add(-2 * time.Minute)},
	}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetSearcher(provider), WithResourceGroupMaterialSearcher(groups))
	actor := assetCapabilityActor(1, domain.PermissionAssetView)

	result, appErr := svc.SystemSearch(context.Background(), actor, "png", 1, 50, "all", "all", "")
	if appErr != nil {
		t.Fatalf("SystemSearch() error = %+v", appErr)
	}
	if len(result.Items) != 2 || result.Total != 2 {
		t.Fatalf("SystemSearch() items/total = %d/%d, want visible system plus external", len(result.Items), result.Total)
	}
	if result.Items[0].SourceType != "task_resource_group" || result.Items[1].OriginPath != "/quark/海报/external.png" {
		t.Fatalf("SystemSearch() items = %+v, want system and visible external assets", result.Items)
	}
	if len(provider.searchQueries) != 1 || provider.searchQueries[0].Source != domain.AssetResourceSourceExternal {
		t.Fatalf("search queries = %+v, want external-only union branch", provider.searchQueries)
	}
}

func testResourceGroupMaterial(id int64, updatedAt time.Time) domain.TaskAssetGroup {
	fileSize := int64(1024)
	return domain.TaskAssetGroup{
		ID: id, TaskID: id + 1000, TaskNo: fmt.Sprintf("RW-%04d", id), SKUCode: fmt.Sprintf("SKU-%04d", id),
		BusinessLane: domain.TaskBusinessLaneNormal, CreatedAt: updatedAt, UpdatedAt: updatedAt,
		FinalizedRevision: &domain.TaskAssetGroupRevision{
			ID: id + 10000, Status: domain.TaskAssetGroupRevisionFinalized, Mode: domain.TaskAssetGroupModeSingle,
			Items: []domain.TaskAssetGroupRevisionItem{{
				ID: id + 20000, SortOrder: 1,
				File: &domain.TaskResourceFile{TaskAssetID: id + 30000, FileName: fmt.Sprintf("group-%04d.png", id), MimeType: "image/png", FileSize: &fileSize},
			}},
		},
	}
}

func TestSystemSearchPushesResourceGroupLaneAndFormatFiltersIntoCountAndPage(t *testing.T) {
	base := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	normalImage := testResourceGroupMaterial(1, base)
	customDesign := testResourceGroupMaterial(2, base.Add(-time.Minute))
	customDesign.BusinessLane = domain.TaskBusinessLaneCustomization
	customDesign.FinalizedRevision.SourceFile = &domain.TaskResourceFile{TaskAssetID: 9002, FileName: "custom.psd", MimeType: "image/vnd.adobe.photoshop"}
	groups := &resourceGroupMaterialProviderStub{items: []domain.TaskAssetGroup{normalImage, customDesign}}
	external := &externalMaterialProviderStub{pagedSearch: []*assetcenter.AssetDetail{{
		ID: 3, SourceType: string(domain.AssetResourceSourceExternal), OriginPath: "/quark/海报/external.psd", UpdatedAt: base,
	}}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetSearcher(external), WithResourceGroupMaterialSearcher(groups))
	actor := assetCapabilityActor(1, domain.PermissionAssetView)

	for _, source := range []string{"system", "all"} {
		result, appErr := svc.SystemSearch(context.Background(), actor, "", 1, 50, source, "design", string(domain.TaskBusinessLaneCustomization))
		if appErr != nil {
			t.Fatalf("SystemSearch(%s) error = %+v", source, appErr)
		}
		if result.Total != 1 || len(result.Items) != 1 || result.Items[0].ID != customDesign.ID {
			t.Fatalf("SystemSearch(%s) = total %d items %+v, want only custom design group", source, result.Total, result.Items)
		}
	}
	imageResult, appErr := svc.SystemSearch(context.Background(), actor, "", 1, 50, "system", "image", string(domain.TaskBusinessLaneCustomization))
	if appErr != nil || imageResult.Total != 1 || len(imageResult.Items) != 1 || imageResult.Items[0].ID != customDesign.ID {
		t.Fatalf("source PSD + final PNG image filter = %+v/%+v", imageResult, appErr)
	}
	pdfResult, appErr := svc.SystemSearch(context.Background(), actor, "", 1, 50, "system", "pdf", string(domain.TaskBusinessLaneCustomization))
	if appErr != nil || pdfResult.Total != 0 || len(pdfResult.Items) != 0 {
		t.Fatalf("unmatched PDF filter = %+v/%+v", pdfResult, appErr)
	}
	if len(groups.queries) != 4 {
		t.Fatalf("resource group queries = %+v", groups.queries)
	}
	for index, query := range groups.queries {
		if query.BusinessLane != domain.TaskBusinessLaneCustomization {
			t.Fatalf("resource group filters not pushed down: %+v", query)
		}
		if index < 2 && query.FormatCategory != domain.AssetFormatCategoryDesign {
			t.Fatalf("design filter not pushed down: %+v", query)
		}
	}
}

func TestSystemSearchDeepPagesUseConstantChunksWithoutGapsOrDuplicates(t *testing.T) {
	base := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	groupItems := make([]domain.TaskAssetGroup, 0, 310)
	externalItems := make([]*assetcenter.AssetDetail, 0, 310)
	for index := 0; index < 310; index++ {
		groupItems = append(groupItems, testResourceGroupMaterial(int64(index+1), base.Add(-time.Duration(index*2)*time.Minute)))
		externalItems = append(externalItems, &assetcenter.AssetDetail{
			ID: int64(index + 1), SourceType: string(domain.AssetResourceSourceExternal),
			OriginPath: fmt.Sprintf("/quark/海报/external-%04d.png", index+1), UpdatedAt: base.Add(-time.Duration(index*2+1) * time.Minute),
		})
	}

	tests := []struct {
		name          string
		source        string
		wantFirstType string
		wantFirstID   int64
	}{
		{name: "system", source: "system", wantFirstType: "task_resource_group", wantFirstID: 151},
		{name: "external", source: "external", wantFirstType: string(domain.AssetResourceSourceExternal), wantFirstID: 151},
		{name: "all", source: "all", wantFirstType: "task_resource_group", wantFirstID: 76},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			groups := &resourceGroupMaterialProviderStub{items: groupItems}
			external := &externalMaterialProviderStub{pagedSearch: externalItems}
			svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetSearcher(external), WithResourceGroupMaterialSearcher(groups))
			result, appErr := svc.SystemSearch(context.Background(), assetCapabilityActor(1, domain.PermissionAssetView), "", 3, 75, tc.source, "all", "")
			if appErr != nil {
				t.Fatalf("SystemSearch() error = %+v", appErr)
			}
			if len(result.Items) != 75 || result.Items[0].SourceType != tc.wantFirstType || result.Items[0].ID != tc.wantFirstID {
				t.Fatalf("page result len/first = %d/%s:%d, want 75/%s:%d", len(result.Items), result.Items[0].SourceType, result.Items[0].ID, tc.wantFirstType, tc.wantFirstID)
			}
			seen := map[string]struct{}{}
			for _, item := range result.Items {
				key := fmt.Sprintf("%s:%d", item.SourceType, item.ID)
				if _, exists := seen[key]; exists {
					t.Fatalf("duplicate item %s", key)
				}
				seen[key] = struct{}{}
			}
			for _, params := range groups.queries {
				if params.PageSize != 200 {
					t.Fatalf("resource group page size changed across pages: %+v", groups.queries)
				}
			}
			for _, query := range external.searchQueries {
				if query.Size != 100 {
					t.Fatalf("external page size changed across pages: %+v", external.searchQueries)
				}
			}
		})
	}
}

func TestSystemSearchExternalVisibilityIsAppliedBeforeCountAndPaging(t *testing.T) {
	base := time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC)
	raw := make([]*assetcenter.AssetDetail, 0, 250)
	visibleIDs := make([]int64, 0, 205)
	for index := 0; index < 250; index++ {
		hidden := index < 15 || (index >= 100 && index < 115) || index >= 235
		origin := fmt.Sprintf("/quark/海报/visible-%03d.png", index)
		if hidden {
			origin = fmt.Sprintf("/quark/其他目录/hidden-%03d.png", index)
		} else {
			visibleIDs = append(visibleIDs, int64(index+1))
		}
		raw = append(raw, &assetcenter.AssetDetail{ID: int64(index + 1), SourceType: string(domain.AssetResourceSourceExternal), OriginPath: origin, UpdatedAt: base.Add(-time.Duration(index) * time.Minute)})
	}
	provider := &externalMaterialProviderStub{pagedSearch: raw}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithSystemAssetSearcher(provider))
	result, appErr := svc.SystemSearch(context.Background(), assetCapabilityActor(1, domain.PermissionAssetView), "", 3, 75, "external", "all", "")
	if appErr != nil {
		t.Fatalf("SystemSearch() error = %+v", appErr)
	}
	if result.Total != int64(len(visibleIDs)) || len(result.Items) != len(visibleIDs)-150 {
		t.Fatalf("visible total/page = %d/%d, want %d/%d", result.Total, len(result.Items), len(visibleIDs), len(visibleIDs)-150)
	}
	for index, item := range result.Items {
		if item.ID != visibleIDs[index+150] || strings.Contains(item.OriginPath, "/其他目录/") {
			t.Fatalf("page item[%d] = %+v, want visible id %d", index, item, visibleIDs[index+150])
		}
	}
	for _, query := range provider.searchQueries {
		if !query.OperationalVisibleOnly || query.Size != 100 {
			t.Fatalf("external visibility/count filter not pushed down: %+v", provider.searchQueries)
		}
	}
}
