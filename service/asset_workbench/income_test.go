package assetworkbench

import (
	"context"
	"database/sql"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type personalIncomeRepo struct {
	repo.AssetWorkbenchRepo
	sources        repo.AssetWorkbenchIncomeSources
	profile        *domain.AssetWorkbenchProfile
	readCalls      int
	visibilityUser int64
	visible        bool
	events         []*domain.AssetWorkbenchEvent
}

func (r *personalIncomeRepo) GetProfileByUserID(context.Context, int64) (*domain.AssetWorkbenchProfile, error) {
	return r.profile, nil
}
func (r *personalIncomeRepo) ReadPersonalIncome(_ context.Context, userID int64) (*repo.AssetWorkbenchIncomeSources, error) {
	if userID != 77 {
		panic("unscoped income query")
	}
	r.readCalls++
	return &r.sources, nil
}
func (r *personalIncomeRepo) GetSupplementPermission(context.Context, int64, string) (*domain.AssetWorkbenchSupplementPermission, error) {
	return nil, sql.ErrNoRows
}
func (r *personalIncomeRepo) FindActiveWelfareRules(context.Context, string, string, time.Time) ([]*domain.AssetWorkbenchWelfareRule, error) {
	return nil, nil
}
func (r *personalIncomeRepo) FindActiveDeductionRule(context.Context, string, string, string, time.Time) (*domain.AssetWorkbenchDeductionRule, error) {
	return &domain.AssetWorkbenchDeductionRule{DeductionAmount: 2}, nil
}
func (r *personalIncomeRepo) SetSettlementVisibility(_ context.Context, _ repo.Tx, userID int64, visible bool, _ int64) error {
	r.visibilityUser = userID
	r.visible = visible
	return nil
}
func (r *personalIncomeRepo) AppendEvent(_ context.Context, _ repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	r.events = append(r.events, event)
	return event, nil
}

func incomeFixture() (*Service, *personalIncomeRepo, domain.RequestActor) {
	r := &personalIncomeRepo{profile: &domain.AssetWorkbenchProfile{UserID: 77, WorkerType: "parttime", JobGrade: "J1"}}
	s := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	s.nowFn = func() time.Time { return time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC) }
	return s, r, assetCapabilityActor(77, domain.PermissionAssetWorkbenchUse, domain.PermissionAssetWorkbenchSubmit)
}
func incomeWork(id int64, month string, amount float64) *domain.AssetWorkbenchSubmissionItem {
	return &domain.AssetWorkbenchSubmissionItem{ID: id, PayeeUserID: 77, BusinessMonth: month, GrossAmount: amount, PageCount: 1, PricingStatus: domain.AssetWorkbenchPricingStatusPriced, QCStatus: domain.AssetWorkbenchSubmissionStatusSubmitted, SettlementStatus: domain.AssetWorkbenchSettlementStatusUnsettled}
}
func incomeLedger(id int64, month, kind, status string, amount float64) repo.AssetWorkbenchIncomeLedgerRow {
	return repo.AssetWorkbenchIncomeLedgerRow{BatchStatus: status, Item: &domain.AssetWorkbenchSettlementItem{ID: id, PayeeUserID: 77, BusinessMonth: month, ItemType: kind, Amount: amount, Quantity: 1, Direction: "credit"}}
}

func TestPersonalIncomeIncludesHistoricalUnsettledAndGeneratedWithoutGaps(t *testing.T) {
	s, r, actor := incomeFixture()
	r.sources.Items = []*domain.AssetWorkbenchSubmissionItem{incomeWork(1, "2026-09", 10), incomeWork(2, "2026-10", 20)}
	r.sources.Ledger = []repo.AssetWorkbenchIncomeLedgerRow{
		incomeLedger(3, "2026-10", domain.AssetWorkbenchItemTypeGrossPiecework, domain.AssetWorkbenchBatchStatusGenerated, 30),
		incomeLedger(4, "2026-10", domain.AssetWorkbenchItemTypeGrossPiecework, domain.AssetWorkbenchBatchStatusConfirmed, 40),
	}
	got, err := s.MySettlement(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if len(got.Months) != 2 || got.EstimatedNetAmount != 90 {
		t.Fatalf("income=%+v", got)
	}
	row := got.Months[0]
	if row.Confirmed || row.SettlementState != "partially_confirmed" || row.ItemCount != 3 || row.EstimatedAmount != 20 || row.PendingConfirmationAmount != 30 || row.ConfirmedNetAmount != 40 {
		t.Fatalf("row=%+v", row)
	}
	if got.Months[1].NetAmount != 10 || got.GeneratedAt.IsZero() || got.RefreshAfterSeconds != 30 {
		t.Fatalf("income=%+v", got)
	}
}

func TestPersonalIncomeSupplementStatusAndAllRows(t *testing.T) {
	s, r, actor := incomeFixture()
	for i := int64(1); i <= 53; i++ {
		r.sources.Supplements = append(r.sources.Supplements, &domain.AssetWorkbenchSettlementSupplement{ID: i, PayeeUserID: 77, BusinessMonth: "2026-10", Status: domain.AssetWorkbenchSupplementStatusApproved, PageCount: 1, GrossAmount: 1})
	}
	batchID := int64(5)
	for _, status := range []string{domain.AssetWorkbenchSupplementStatusDraft, domain.AssetWorkbenchSupplementStatusVoided, domain.AssetWorkbenchSupplementStatusInBatch, domain.AssetWorkbenchSupplementStatusSettled} {
		row := &domain.AssetWorkbenchSettlementSupplement{ID: 100, PayeeUserID: 77, BusinessMonth: "2026-10", Status: status, PageCount: 1, GrossAmount: 100}
		if status == domain.AssetWorkbenchSupplementStatusInBatch || status == domain.AssetWorkbenchSupplementStatusSettled {
			row.LinkedBatchID = &batchID
		}
		r.sources.Supplements = append(r.sources.Supplements, row)
	}
	r.sources.Ledger = []repo.AssetWorkbenchIncomeLedgerRow{incomeLedger(1, "2026-10", domain.AssetWorkbenchItemTypeSupplement, domain.AssetWorkbenchBatchStatusConfirmed, 5)}
	got, err := s.MySettlement(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	row := got.Months[0]
	if row.NetAmount != 58 || row.SupplementAmount != 58 || row.SupplementItemCount != 54 || row.ItemCount != 0 || row.PageCount != 0 || row.SupplementPageCount != 54 {
		t.Fatalf("row=%+v", row)
	}
	if len(got.Supplements) != 56 {
		t.Fatalf("current supplements=%d, want 53 approved + draft + in_batch + settled", len(got.Supplements))
	}
}

func TestPersonalIncomeUnpricedIsNotZeroSalaryAndConfirmedIsWholeMonth(t *testing.T) {
	s, r, actor := incomeFixture()
	r.sources.Ledger = []repo.AssetWorkbenchIncomeLedgerRow{incomeLedger(1, "2026-10", domain.AssetWorkbenchItemTypeGrossPiecework, domain.AssetWorkbenchBatchStatusConfirmed, 10)}
	work := incomeWork(2, "2026-10", 999)
	work.PricingStatus = domain.AssetWorkbenchPricingStatusPendingGrade
	r.sources.Items = []*domain.AssetWorkbenchSubmissionItem{work}
	got, err := s.MySettlement(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if row := got.Months[0]; row.Confirmed || row.PendingPricingCount != 1 || row.NetAmount != 10 || row.ItemCount != 1 {
		t.Fatalf("row=%+v", row)
	}
	r.sources.Items = nil
	got, err = s.MySettlement(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Months[0].Confirmed || got.Months[0].SettlementState != "confirmed" {
		t.Fatalf("row=%+v", got.Months[0])
	}
}

func TestPersonalIncomeDoesNotRepeatReservedQualityDeduction(t *testing.T) {
	s, r, actor := incomeFixture()
	payee := int64(77)
	errorID := int64(88)
	line := incomeLedger(1, "2026-10", domain.AssetWorkbenchItemTypeErrorDeduction, domain.AssetWorkbenchBatchStatusConfirmed, 4)
	line.Item.Direction = "debit"
	line.Item.SourceRefType = "error_record"
	line.Item.SourceRefID = &errorID
	r.sources.Ledger = []repo.AssetWorkbenchIncomeLedgerRow{line}
	r.sources.Errors = []*domain.AssetWorkbenchErrorRecord{{ID: 88, PayeeUserID: &payee, BusinessMonth: "2026-10", DifficultyClass: "A", ErrorCount: 2, MatchStatus: domain.AssetWorkbenchErrorMatchStatusMatched}}
	got, err := s.MySettlement(context.Background(), actor)
	if err != nil {
		t.Fatal(err)
	}
	if row := got.Months[0]; row.NetAmount != -4 || row.DeductionAmount != 4 || !row.Confirmed {
		t.Fatalf("row=%+v", row)
	}
}

func TestPersonalIncomeVisibilityIsServerEnforcedAndCannotBeSelfChanged(t *testing.T) {
	s, r, actor := incomeFixture()
	visible := false
	r.profile.SettlementVisible = &visible
	got, err := s.MySettlement(context.Background(), actor)
	if got != nil || err == nil || err.Code != domain.ErrCodePermissionDenied || r.readCalls != 0 {
		t.Fatalf("got=%+v err=%v calls=%d", got, err, r.readCalls)
	}
	if err := s.SetSettlementVisibility(context.Background(), actor, 77, true, "self"); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatal("submitter can change visibility")
	}
	admin := assetCapabilityActor(1, domain.PermissionAssetWorkbenchProfiles)
	if err := s.SetSettlementVisibility(context.Background(), admin, 77, false, "admin"); err != nil {
		t.Fatal(err)
	}
	if r.visibilityUser != 77 || r.visible || len(r.events) != 1 {
		t.Fatalf("visibility audit=%+v", r)
	}
}

func TestSettlementReservationKeysArePersonAndSourceScoped(t *testing.T) {
	person, ruleID, errorID := int64(77), int64(5), int64(8)
	keys := reservedSettlementSources([]*domain.AssetWorkbenchSettlementItem{
		{PayeeUserID: person, ItemType: domain.AssetWorkbenchItemTypeWelfare, SourceRefID: &ruleID},
		{PayeeUserID: person, ItemType: domain.AssetWorkbenchItemTypeErrorDeduction, SourceRefType: "error_record", SourceRefID: &errorID},
	})
	lines := keys.remainingWelfare([]welfareSettlementLine{{PayeeUserID: 77, RuleID: 5, Amount: 10}, {PayeeUserID: 78, RuleID: 5, Amount: 10}, {PayeeUserID: 77, RuleID: 6, Amount: 20}})
	if len(lines) != 2 {
		t.Fatalf("welfare=%+v", lines)
	}
	other := int64(78)
	records := keys.remainingErrors([]*domain.AssetWorkbenchErrorRecord{{ID: 8, PayeeUserID: &person, DifficultyClass: "A"}, {ID: 8, PayeeUserID: &other, DifficultyClass: "A"}, {ID: 9, PayeeUserID: &person, DifficultyClass: "A"}})
	if len(records) != 2 {
		t.Fatalf("errors=%+v", records)
	}
}

type reservedBatchIncomeRepo struct {
	settlementBatchRepo
	reserved []*domain.AssetWorkbenchSettlementItem
	errors   []*domain.AssetWorkbenchErrorRecord
}

func (r *reservedBatchIncomeRepo) ListReservedSettlementSources(context.Context, repo.Tx, string) ([]*domain.AssetWorkbenchSettlementItem, error) {
	return r.reserved, nil
}
func (r *reservedBatchIncomeRepo) ListErrorRecordsByMonth(context.Context, string) ([]*domain.AssetWorkbenchErrorRecord, error) {
	return r.errors, nil
}
func (r *reservedBatchIncomeRepo) FindActiveWelfareRules(context.Context, string, string, time.Time) ([]*domain.AssetWorkbenchWelfareRule, error) {
	return []*domain.AssetWorkbenchWelfareRule{{ID: 5, Amount: 10}}, nil
}
func (r *reservedBatchIncomeRepo) GetProfileByUserID(context.Context, int64) (*domain.AssetWorkbenchProfile, error) {
	return nil, sql.ErrNoRows
}

func TestGenerateSettlementDoesNotRepeatMonthlyWelfareOrImportedQualityErrors(t *testing.T) {
	person, ruleID, errorID := int64(77), int64(5), int64(8)
	work := incomeWork(1, "2026-10", 10)
	work.WorkerTypeSnapshot = "parttime"
	work.JobGradeSnapshot = "J1"
	r := &reservedBatchIncomeRepo{settlementBatchRepo: settlementBatchRepo{items: []*domain.AssetWorkbenchSubmissionItem{work}}, reserved: []*domain.AssetWorkbenchSettlementItem{
		{PayeeUserID: person, ItemType: domain.AssetWorkbenchItemTypeWelfare, SourceRefID: &ruleID},
		{PayeeUserID: person, ItemType: domain.AssetWorkbenchItemTypeErrorDeduction, SourceRefType: "error_record", SourceRefID: &errorID},
	}, errors: []*domain.AssetWorkbenchErrorRecord{{ID: 8, PayeeUserID: &person, DifficultyClass: "A", ErrorCount: 2, MatchStatus: domain.AssetWorkbenchErrorMatchStatusMatched}}}
	s := NewService(Config{Timezone: "Asia/Shanghai"}, WithRepository(r, assetWorkbenchTestTxRunner{}))
	batch, err := s.GenerateSettlementBatch(context.Background(), domain.RequestActor{ID: 1, Roles: []domain.Role{domain.RoleAssetSettlement}}, "2026-10")
	if err != nil {
		t.Fatal(err)
	}
	if batch.NetAmount != 10 || batch.WelfareAmount != 0 || batch.DeductionAmount != 0 || len(r.settlementItems) != 1 {
		t.Fatalf("batch=%+v lines=%+v", batch, r.settlementItems)
	}
}
