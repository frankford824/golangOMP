package assetworkbench

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"strconv"
	"strings"
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

func containsString(values []string, target string) bool {
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
	items   []*domain.AssetWorkbenchSubmissionItem
	batch   *domain.AssetWorkbenchErrorImportBatch
	records []*domain.AssetWorkbenchErrorRecord
	events  []*domain.AssetWorkbenchEvent
}

func (r *errorImportRepo) ListSubmissionItemsByMonth(context.Context, string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	return r.items, nil
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
	events          []*domain.AssetWorkbenchEvent
}

func (r *supplementRepo) GetDifficultyClass(_ context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	return testDifficultyClass(code)
}

func (r *supplementRepo) ListSubmissionItemsByMonth(context.Context, string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	return r.items, nil
}

func (r *supplementRepo) ListSettlementSupplements(_ context.Context, filter repo.AssetWorkbenchSettlementSupplementFilter) ([]*domain.AssetWorkbenchSettlementSupplement, int64, error) {
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

func (r *supplementRepo) GetSupplementPermission(_ context.Context, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error) {
	for _, item := range r.permissions {
		if item != nil && item.PayeeUserID == payeeUserID && item.BusinessMonth == businessMonth {
			return item, nil
		}
	}
	return nil, sql.ErrNoRows
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

type profileListRepo struct {
	repo.AssetWorkbenchRepo
	items        []*domain.AssetWorkbenchProfile
	saved        *domain.AssetWorkbenchProfile
	gradePeriods []*domain.AssetWorkbenchGradePeriod
	events       []*domain.AssetWorkbenchEvent
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
	profile       *domain.AssetWorkbenchProfile
	price         *domain.AssetWorkbenchPriceMatrix
	session       *domain.AssetWorkbenchUploadSession
	submission    *domain.AssetWorkbenchSubmission
	item          *domain.AssetWorkbenchSubmissionItem
	files         []*domain.AssetWorkbenchSubmissionFile
	sessionStatus string
	events        []*domain.AssetWorkbenchEvent
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

type clientMaterialRepo struct {
	repo.AssetWorkbenchRepo
	materials map[int64]*domain.AssetWorkbenchClientMaterial
	events    []*domain.AssetWorkbenchEvent
}

func (r *clientMaterialRepo) GetClientMaterial(_ context.Context, materialID int64) (*domain.AssetWorkbenchClientMaterial, error) {
	item := r.materials[materialID]
	if item == nil {
		return nil, sql.ErrNoRows
	}
	copyItem := *item
	return &copyItem, nil
}

func (r *clientMaterialRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	copyEvent := *event
	copyEvent.ID = int64(len(r.events) + 1)
	r.events = append(r.events, &copyEvent)
	return &copyEvent, nil
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

type itemActionRepo struct {
	repo.AssetWorkbenchRepo
	items        map[int64]*domain.AssetWorkbenchSubmissionItem
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
	blockedDelete map[int64]bool
	updatedFiles  []*domain.AssetWorkbenchSubmissionFile
	deletedFiles  []int64
	refreshed     []int64
	events        []*domain.AssetWorkbenchEvent
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

func (r *batchFileMutationRepo) UpdateSubmissionFileLocation(_ context.Context, _ repo.Tx, file *domain.AssetWorkbenchSubmissionFile) (*domain.AssetWorkbenchSubmissionFile, error) {
	if file == nil || r.files[file.ID] == nil {
		return nil, sql.ErrNoRows
	}
	copyFile := *file
	r.files[file.ID] = &copyFile
	r.updatedFiles = append(r.updatedFiles, &copyFile)
	return &copyFile, nil
}

func (r *batchFileMutationRepo) DeleteSubmissionFile(_ context.Context, _ repo.Tx, fileID int64) error {
	if r.blockedDelete[fileID] {
		return domain.NewAppError(domain.ErrCodeConflict, "blocked file", nil)
	}
	if r.files[fileID] == nil {
		return sql.ErrNoRows
	}
	delete(r.files, fileID)
	r.deletedFiles = append(r.deletedFiles, fileID)
	return nil
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
		Province:      "浙江",
		City:          "杭州",
		IDCard:        "330100199001010000",
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
}

func TestUpsertMyProfileCreatesMissingPIINotification(t *testing.T) {
	workbenchRepo := &profileListRepo{}
	notifier := &profileNotificationStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithNotificationCreator(notifier),
	)

	saved, appErr := svc.UpsertMyProfile(context.Background(), domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}, UpsertProfileParams{
		RealName: "计件人员",
		Province: "浙江",
		City:     "杭州",
	})
	if appErr != nil {
		t.Fatalf("UpsertMyProfile() error = %+v", appErr)
	}
	if saved.PIICompleted {
		t.Fatalf("profile should be incomplete: %+v", saved)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("notification calls = %+v, want one", notifier.calls)
	}
	call := notifier.calls[0]
	if call.userID != 77 || call.ntype != domain.NotificationTypeSystemBroadcast {
		t.Fatalf("notification call = %+v", call)
	}
	if call.dedupeScope != "asset_workbench_profile_completion" || call.dedupeKey != "asset_workbench_profile_completion:77" {
		t.Fatalf("dedupe = %q/%q", call.dedupeScope, call.dedupeKey)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(call.payload, &payload); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	if payload["reason"] != "missing_pii" || payload["action"] != "complete_profile" {
		t.Fatalf("payload = %+v", payload)
	}
	missing, ok := payload["missing_fields"].([]interface{})
	if !ok || len(missing) != 3 {
		t.Fatalf("missing_fields = %+v", payload["missing_fields"])
	}
}

func TestUpsertMyProfileRequiresAlipayForPIICompletion(t *testing.T) {
	workbenchRepo := &profileListRepo{}
	notifier := &profileNotificationStub{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"},
		WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}),
		WithNotificationCreator(notifier),
	)

	saved, appErr := svc.UpsertMyProfile(context.Background(), domain.RequestActor{ID: 78, Roles: []domain.Role{domain.RoleAssetSubmitter}}, UpsertProfileParams{
		RealName: "计件人员",
		Phone:    "13800000078",
		IDCard:   "330100199001010078",
		Province: "浙江",
		City:     "杭州",
	})
	if appErr != nil {
		t.Fatalf("UpsertMyProfile() error = %+v", appErr)
	}
	if saved.PIICompleted {
		t.Fatalf("profile without alipay should be incomplete: %+v", saved)
	}
	if len(notifier.calls) != 1 {
		t.Fatalf("notification calls = %+v, want one", notifier.calls)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal(notifier.calls[0].payload, &payload); err != nil {
		t.Fatalf("payload unmarshal: %v", err)
	}
	missing, ok := payload["missing_fields"].([]interface{})
	if !ok || len(missing) != 1 || missing[0] != "alipay_account" {
		t.Fatalf("missing_fields = %+v, want alipay_account", payload["missing_fields"])
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
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	created, appErr := svc.CreateSettlementSupplement(context.Background(), actor, CreateSettlementSupplementParams{
		PayeeUserID:     1001,
		BusinessMonth:   "2026-06",
		OrderNo:         "ORD-1",
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
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventSupplementCreated {
		t.Fatalf("events = %+v", workbenchRepo.events)
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

func TestCreateSettlementSupplementRequiresOpenPermission(t *testing.T) {
	workbenchRepo := &supplementRepo{}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
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
	workbenchRepo := &supplementRepo{confirmedMonths: map[int64][]string{1001: []string{"2026-06"}}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
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

func TestUpsertSupplementPermissionRequiresConfirmedSettlementMonth(t *testing.T) {
	workbenchRepo := &supplementRepo{confirmedMonths: map[int64][]string{1001: []string{"2026-05"}}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
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
		t.Fatalf("permission should not be saved without confirmed settlement month: permissions=%+v events=%+v", workbenchRepo.permissions, workbenchRepo.events)
	}
}

func TestListSupplementEligibleMonthsReturnsConfirmedSettlementMonths(t *testing.T) {
	workbenchRepo := &supplementRepo{confirmedMonths: map[int64][]string{1001: []string{"2026-06", "2026-05"}}}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	actor := domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetSettlement}}

	months, appErr := svc.ListSupplementEligibleMonths(context.Background(), actor, 1001)
	if appErr != nil {
		t.Fatalf("ListSupplementEligibleMonths() error = %+v", appErr)
	}
	if len(months) != 2 || months[0] != "2026-06" || months[1] != "2026-05" {
		t.Fatalf("months = %+v, want confirmed months", months)
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
	key := svc.buildObjectKey(time.Date(2026, 6, 15, 0, 0, 0, 0, time.UTC), "session-1", "../final.psd", directory)
	if !strings.Contains(key, "/uploads/client-a/2026/06/session-1/final.psd") || strings.Contains(key, "..") {
		t.Fatalf("object key = %q", key)
	}
}

func TestCreateSubmissionInfersDifficultyFromUploadDirectorySnapshot(t *testing.T) {
	sessionID := "session-c"
	directoryID := int64(11)
	workbenchRepo := &submissionDirectoryDifficultyRepo{
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
		},
	}
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
	svc.nowFn = func() time.Time {
		return time.Date(2026, 7, 3, 2, 0, 0, 0, time.UTC)
	}
	actor := domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}

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
	if workbenchRepo.sessionStatus != domain.AssetWorkbenchUploadStatusSubmitted {
		t.Fatalf("session status = %q, want submitted", workbenchRepo.sessionStatus)
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
	actor := domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}

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
	actor := domain.RequestActor{ID: 77, Roles: []domain.Role{domain.RoleAssetSubmitter}}

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

	meta, appErr := svc.SystemAssetPreview(context.Background(), domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}}, 1001)
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
		domain.RequestActor{ID: 99, Roles: []domain.Role{domain.RoleAssetManager}},
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
	svc := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(workbenchRepo, assetWorkbenchTestTxRunner{}))
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
	if len(workbenchRepo.events) != 1 || workbenchRepo.events[0].EventType != domain.AssetWorkbenchEventFileDeleted {
		t.Fatalf("events = %+v", workbenchRepo.events)
	}
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
			SubmittedAt:        submittedAt.Add(time.Hour),
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
	if normal.CreatorName != "Alice" || normal.JobGrade != "P1" || normal.CreatedDate != "2026-06-02" {
		t.Fatalf("normal identity fields = %+v, want Alice/P1/2026-06-02", normal)
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
