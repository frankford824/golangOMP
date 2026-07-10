package main

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

func TestValidateRunConfigRequiresExactTaskForApply(t *testing.T) {
	if err := validateRunConfig(runConfig{Apply: true, Limit: 10, Mode: repairModeReviewState}); err == nil || !strings.Contains(err.Error(), "exact --task-no") {
		t.Fatalf("validateRunConfig(apply without task) error = %v", err)
	}
	if err := validateRunConfig(runConfig{Apply: true, Limit: 10, TaskNo: "  ", Mode: repairModeReviewState}); err == nil || !strings.Contains(err.Error(), "exact --task-no") {
		t.Fatalf("validateRunConfig(apply with blank task) error = %v", err)
	}
	if err := validateRunConfig(runConfig{Apply: true, Limit: 10, TaskNo: "RW-20260709-A-002196", Mode: repairModeReviewState}); err != nil {
		t.Fatalf("validateRunConfig(exact task) error = %v", err)
	}
	if err := validateRunConfig(runConfig{Apply: false, Limit: 10, Mode: repairModeForcedCloseCorrection}); err != nil {
		t.Fatalf("validateRunConfig(dry-run discovery) error = %v", err)
	}
	if err := validateRunConfig(runConfig{Apply: false, Limit: 10, Mode: "unknown"}); err == nil || !strings.Contains(err.Error(), "--mode") {
		t.Fatalf("validateRunConfig(unknown mode) error = %v", err)
	}
}

func TestCandidateDiscoveryUsesLatestJobLatestReviewAndReviewTimeCutoff(t *testing.T) {
	query := candidateDiscoveryQuery(false)
	required := []string{
		"ORDER BY cj2.id DESC",
		"ORDER BY tel2.sequence DESC",
		"cj.customization_review_decision IN ('approved', 'reviewer_fixed')",
		"JSON_UNQUOTE(JSON_EXTRACT(tel.payload, '$.customization_review_decision')) IN ('approved', 'reviewer_fixed')",
		"ta.created_at <= tel.created_at",
		"COALESCE(ta.is_archived, 0) = 0",
	}
	for _, fragment := range required {
		if !strings.Contains(query, fragment) {
			t.Errorf("candidate discovery query missing %q", fragment)
		}
	}
	latestEventPos := strings.Index(query, "ORDER BY tel2.sequence DESC")
	eventDecisionPos := strings.Index(query, "JSON_UNQUOTE(JSON_EXTRACT(tel.payload")
	if latestEventPos < 0 || eventDecisionPos < latestEventPos {
		t.Fatalf("event decision filtering must happen after selecting the latest review event")
	}
}

func TestForcedCloseCandidateDiscoveryRequiresRepairToolProvenance(t *testing.T) {
	query := forcedCloseCandidateDiscoveryQuery(false)
	required := []string{
		"tm.state = 'forcibly_closed'",
		"tme2.event_type = 'forcibly_closed'",
		"tme2.to_state = 'forcibly_closed'",
		"JSON_UNQUOTE(JSON_EXTRACT(tme2.payload, '$.source')) = 'repair_customization_review_state'",
		"tm2.task_id = t.id",
	}
	for _, fragment := range required {
		if !strings.Contains(query, fragment) {
			t.Errorf("forced-close discovery query missing %q", fragment)
		}
	}
}

func TestLockAndRevalidateRejectsLatestReturnedReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	reviewedAt := time.Date(2026, 7, 9, 12, 30, 0, 0, time.UTC)
	candidate := repairCandidate{
		TaskID:       2199,
		TaskNo:       "RW-20260709-A-002196",
		TaskStatus:   domain.TaskStatusPendingWarehouseReceive,
		ReviewerID:   77,
		ReviewedAt:   reviewedAt,
		VersionCount: 1,
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT task_no, task_status, customization_required.*FROM tasks.*FOR UPDATE`).
		WithArgs(candidate.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"task_no", "task_status", "customization_required"}).
			AddRow(candidate.TaskNo, candidate.TaskStatus, true))
	mock.ExpectQuery(`(?s)SELECT customization_review_decision.*FROM customization_jobs.*ORDER BY id DESC.*FOR UPDATE`).
		WithArgs(candidate.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"customization_review_decision"}).AddRow("approved"))
	mock.ExpectQuery(`(?s)SELECT JSON_UNQUOTE.*FROM task_event_logs.*ORDER BY sequence DESC.*FOR UPDATE`).
		WithArgs(candidate.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"decision", "operator_id", "created_at"}).
			AddRow("return_to_designer", candidate.ReviewerID, reviewedAt))
	mock.ExpectRollback()

	mdb := mysqlrepo.New(db)
	err = mdb.RunInTx(context.Background(), func(tx repo.Tx) error {
		_, lockErr := (mysqlRepairStateLocker{}).LockAndRevalidate(context.Background(), tx, candidate)
		return lockErr
	})
	if err == nil || !strings.Contains(err.Error(), "latest customization review event decision") {
		t.Fatalf("LockAndRevalidate error = %v, want latest review rejection", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestLockAndRevalidateRejectsCurrentDeliveryCreatedAfterReview(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	reviewedAt := time.Date(2026, 7, 9, 12, 30, 0, 0, time.UTC)
	candidate := repairCandidate{
		TaskID:       2199,
		TaskNo:       "RW-20260709-A-002196",
		TaskStatus:   domain.TaskStatusPendingWarehouseReceive,
		ReviewerID:   77,
		ReviewedAt:   reviewedAt,
		VersionCount: 1,
	}
	mock.ExpectBegin()
	mock.ExpectQuery(`(?s)SELECT task_no, task_status, customization_required.*FROM tasks.*FOR UPDATE`).
		WithArgs(candidate.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"task_no", "task_status", "customization_required"}).
			AddRow(candidate.TaskNo, candidate.TaskStatus, true))
	mock.ExpectQuery(`(?s)SELECT customization_review_decision.*FROM customization_jobs.*ORDER BY id DESC.*FOR UPDATE`).
		WithArgs(candidate.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"customization_review_decision"}).AddRow("reviewer_fixed"))
	mock.ExpectQuery(`(?s)SELECT JSON_UNQUOTE.*FROM task_event_logs.*ORDER BY sequence DESC.*FOR UPDATE`).
		WithArgs(candidate.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"decision", "operator_id", "created_at"}).
			AddRow("reviewer_fixed", candidate.ReviewerID, reviewedAt))
	mock.ExpectQuery(`(?s)SELECT ta.id, ta.flow_review_status, ta.created_at.*FROM task_assets.*FOR UPDATE`).
		WithArgs(candidate.TaskID).
		WillReturnRows(sqlmock.NewRows([]string{"id", "flow_review_status", "created_at"}).
			AddRow(501, domain.TaskAssetFlowReviewStatusPendingReview, reviewedAt.Add(time.Second)))
	mock.ExpectRollback()

	mdb := mysqlrepo.New(db)
	err = mdb.RunInTx(context.Background(), func(tx repo.Tx) error {
		_, lockErr := (mysqlRepairStateLocker{}).LockAndRevalidate(context.Background(), tx, candidate)
		return lockErr
	})
	if err == nil || !strings.Contains(err.Error(), "created after the approved review") {
		t.Fatalf("LockAndRevalidate error = %v, want review-time cutoff rejection", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("sql expectations: %v", err)
	}
}

func TestRepairOneRejectsAffectedRowMismatch(t *testing.T) {
	reviewedAt := time.Date(2026, 7, 9, 12, 30, 0, 0, time.UTC)
	candidate := repairCandidate{
		TaskID:       2199,
		TaskNo:       "RW-20260709-A-002196",
		TaskStatus:   domain.TaskStatusPendingWarehouseReceive,
		ReviewerID:   77,
		ReviewedAt:   reviewedAt,
		VersionCount: 2,
	}
	locker := &fakeRepairLocker{state: &lockedRepairState{
		TaskNo:       candidate.TaskNo,
		TaskStatus:   candidate.TaskStatus,
		ReviewerID:   candidate.ReviewerID,
		ReviewedAt:   reviewedAt,
		VersionCount: 2,
	}}
	modules := &fakeModuleStateRepo{}
	events := &fakeModuleEventWriter{}
	err := repairOne(context.Background(), fakeTxRunner{}, locker, &fakeAssetFlowRepo{updated: 1}, modules, events, candidate)
	if err == nil || !strings.Contains(err.Error(), "does not match locked candidate count") {
		t.Fatalf("repairOne error = %v, want affected-row mismatch", err)
	}
	if modules.updateCalls != 0 || modules.enterCalls != 0 || events.insertCalls != 0 {
		t.Fatalf("module mutations after affected-row mismatch: updates=%d enters=%d events=%d", modules.updateCalls, modules.enterCalls, events.insertCalls)
	}
}

func TestTransitionModuleRejectsDifferentTerminalState(t *testing.T) {
	candidate := repairCandidate{TaskID: 2199, TaskNo: "RW-20260709-A-002196", ReviewerID: 77}
	locker := &fakeRepairLocker{modules: map[string]*domain.TaskModule{
		domain.ModuleKeyDesign: {
			ID:        11,
			TaskID:    candidate.TaskID,
			ModuleKey: domain.ModuleKeyDesign,
			State:     domain.ModuleStateCompleted,
		},
	}}
	modules := &fakeModuleStateRepo{}
	events := &fakeModuleEventWriter{}
	err := transitionModule(context.Background(), fakeTx{}, locker, modules, events, candidate, domain.ModuleKeyDesign, domain.ModuleStateClosed, domain.ModuleEventClosed, json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "is terminal") {
		t.Fatalf("transitionModule error = %v, want terminal-state guard", err)
	}
	if modules.updateCalls != 0 || events.insertCalls != 0 {
		t.Fatalf("terminal module was mutated: updates=%d events=%d", modules.updateCalls, events.insertCalls)
	}
}

func TestTransitionModuleSameTargetIsIdempotent(t *testing.T) {
	candidate := repairCandidate{TaskID: 2199, TaskNo: "RW-20260709-A-002196", ReviewerID: 77}
	locker := &fakeRepairLocker{modules: map[string]*domain.TaskModule{
		domain.ModuleKeyCustomization: {
			ID:        12,
			TaskID:    candidate.TaskID,
			ModuleKey: domain.ModuleKeyCustomization,
			State:     domain.ModuleStateClosed,
		},
	}}
	modules := &fakeModuleStateRepo{}
	events := &fakeModuleEventWriter{}
	err := transitionModule(context.Background(), fakeTx{}, locker, modules, events, candidate, domain.ModuleKeyCustomization, domain.ModuleStateClosed, domain.ModuleEventClosed, json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("transitionModule(same target) error = %v", err)
	}
	if modules.updateCalls != 0 || events.insertCalls != 0 {
		t.Fatalf("same-target transition was not idempotent: updates=%d events=%d", modules.updateCalls, events.insertCalls)
	}
}

func TestCorrectForcedCloseOneWritesClosedCorrectionEvent(t *testing.T) {
	candidate := repairCandidate{
		TaskID:     2199,
		TaskNo:     "RW-20260709-A-002196",
		TaskStatus: domain.TaskStatusCompleted,
	}
	locker := &fakeRepairLocker{forcedModule: &domain.TaskModule{
		ID:        11,
		TaskID:    candidate.TaskID,
		ModuleKey: domain.ModuleKeyDesign,
		State:     domain.ModuleStateForciblyClosed,
	}}
	modules := &fakeModuleStateRepo{}
	events := &fakeModuleEventWriter{}
	if err := correctForcedCloseOne(context.Background(), fakeTxRunner{}, locker, modules, events, candidate); err != nil {
		t.Fatalf("correctForcedCloseOne() error = %v", err)
	}
	if modules.updateCalls != 1 || modules.lastState != domain.ModuleStateClosed {
		t.Fatalf("module updates = %d state=%s, want one closed update", modules.updateCalls, modules.lastState)
	}
	if events.insertCalls != 1 || events.lastEvent == nil || events.lastEvent.EventType != domain.ModuleEventClosed ||
		events.lastEvent.FromState == nil || *events.lastEvent.FromState != domain.ModuleStateForciblyClosed ||
		events.lastEvent.ToState == nil || *events.lastEvent.ToState != domain.ModuleStateClosed {
		t.Fatalf("correction event = %+v", events.lastEvent)
	}
	if !strings.Contains(string(events.lastEvent.Payload), "repair_customization_review_forced_close_correction") {
		t.Fatalf("correction payload = %s", events.lastEvent.Payload)
	}
}

func TestWarehouseModuleTarget(t *testing.T) {
	tests := []struct {
		status domain.TaskStatus
		state  domain.ModuleState
		event  domain.ModuleEventType
	}{
		{domain.TaskStatusPendingWarehouseReceive, domain.ModuleStatePending, domain.ModuleEventEntered},
		{domain.TaskStatusPendingProductionTransfer, domain.ModuleStateReceived, domain.ModuleEventReceived},
		{domain.TaskStatusPendingClose, domain.ModuleStateCompleted, domain.ModuleEventCompleted},
		{domain.TaskStatusCompleted, domain.ModuleStateCompleted, domain.ModuleEventCompleted},
	}
	for _, tc := range tests {
		state, event := warehouseModuleTarget(tc.status)
		if state != tc.state || event != tc.event {
			t.Fatalf("status %s target = %s/%s, want %s/%s", tc.status, state, event, tc.state, tc.event)
		}
	}
}

type fakeTx struct{}

func (fakeTx) IsTx() {}

type fakeTxRunner struct{}

func (fakeTxRunner) RunInTx(ctx context.Context, fn func(tx repo.Tx) error) error {
	return fn(fakeTx{})
}

type fakeRepairLocker struct {
	state        *lockedRepairState
	err          error
	modules      map[string]*domain.TaskModule
	forcedModule *domain.TaskModule
}

func (f *fakeRepairLocker) LockAndRevalidate(context.Context, repo.Tx, repairCandidate) (*lockedRepairState, error) {
	return f.state, f.err
}

func (f *fakeRepairLocker) LockModule(_ context.Context, _ repo.Tx, _ int64, moduleKey string) (*domain.TaskModule, error) {
	return f.modules[moduleKey], f.err
}

func (f *fakeRepairLocker) LockForcedCloseCorrection(context.Context, repo.Tx, repairCandidate) (*domain.TaskModule, error) {
	return f.forcedModule, f.err
}

type fakeAssetFlowRepo struct {
	updated int64
	err     error
}

func (f *fakeAssetFlowRepo) MarkCurrentDeliveryVersionsApprovedForTask(context.Context, repo.Tx, int64, int64, time.Time) (int64, error) {
	return f.updated, f.err
}

type fakeModuleStateRepo struct {
	updateCalls int
	enterCalls  int
	lastState   domain.ModuleState
}

func (f *fakeModuleStateRepo) Enter(_ context.Context, _ repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, _ *string, _ json.RawMessage) (*domain.TaskModule, error) {
	f.enterCalls++
	return &domain.TaskModule{ID: 99, TaskID: taskID, ModuleKey: moduleKey, State: state}, nil
}

func (f *fakeModuleStateRepo) UpdateState(_ context.Context, _ repo.Tx, _ int64, _ string, state domain.ModuleState, _ bool, _ json.RawMessage) error {
	f.updateCalls++
	f.lastState = state
	return nil
}

type fakeModuleEventWriter struct {
	insertCalls int
	lastEvent   *domain.TaskModuleEvent
}

func (f *fakeModuleEventWriter) Insert(_ context.Context, _ repo.Tx, event *domain.TaskModuleEvent) (int64, error) {
	f.insertCalls++
	f.lastEvent = event
	return int64(f.insertCalls), nil
}
