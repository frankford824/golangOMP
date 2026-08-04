package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestValidateOptions(t *testing.T) {
	if err := validateOptions(options{DryRun: true, BatchSize: 500}); err != nil {
		t.Fatalf("dry-run: %v", err)
	}
	if err := validateOptions(options{Apply: true, Rollback: true, BatchSize: 500, SnapshotDir: "x", MappingFile: "x"}); err == nil {
		t.Fatal("expected mutually exclusive error")
	}
	if err := validateOptions(options{Apply: true, BatchSize: 500}); err == nil {
		t.Fatal("expected snapshot/mapping error")
	}
}

func TestReadSnapshotRejectsV88AppliedSnapshotAsHistorical(t *testing.T) {
	path := filepath.Join(t.TempDir(), "workflow-groups-snapshot.json")
	raw := fmt.Sprintf(
		`{"version":%d,"tool_version":%q,"schema_version":%q,"apply_state":"applied"}`,
		previousSnapshotVersion,
		previousToolVersion,
		workflowGroupsSchemaVersion,
	)
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := readSnapshot(path, "clone_b", mappingFile{})
	if err == nil || !strings.Contains(err.Error(), "predates lossless v9 rollback") {
		t.Fatalf("readSnapshot() error = %v, want historical v8.8 rejection", err)
	}
}

func TestMigrateStatesAppliesReviewedDecisionBeforeGenericLegacyMapping(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	mock.ExpectExec("UPDATE tasks\\s+SET task_status=\\?,workflow_revision=workflow_revision\\+1\\s+WHERE id=\\? AND task_status=\\?").
		WithArgs("InProgress", int64(449), "PendingWarehouseReceive").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE tasks SET task_status=CASE").
		WillReturnResult(sqlmock.NewResult(0, 10))

	mapping := mappingFile{TaskDecisions: []taskStateDecisionMapping{{
		TaskID: 449, FromStatus: "PendingWarehouseReceive", TargetStatus: "InProgress",
	}}}
	if err := migrateStates(context.Background(), tx, mapping); err != nil {
		t.Fatalf("migrateStates() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateStatesTreatsReviewedTargetAsIdempotentNoOp(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	mock.ExpectExec("UPDATE tasks\\s+SET task_status=\\?,workflow_revision=workflow_revision\\+1\\s+WHERE id=\\? AND task_status=\\?").
		WithArgs("InProgress", int64(449), "PendingWarehouseReceive").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT task_status FROM tasks WHERE id=\\? FOR UPDATE").
		WithArgs(int64(449)).
		WillReturnRows(sqlmock.NewRows([]string{"task_status"}).AddRow("InProgress"))
	mock.ExpectExec("UPDATE tasks SET task_status=CASE").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mapping := mappingFile{TaskDecisions: []taskStateDecisionMapping{{
		TaskID: 449, FromStatus: "PendingWarehouseReceive", TargetStatus: "InProgress",
	}}}
	if err := migrateStates(context.Background(), tx, mapping); err != nil {
		t.Fatalf("migrateStates() idempotent target error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateStatesReopensReviewedRetouchModule(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	confirmedAt := time.Date(2026, 7, 25, 8, 30, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE tasks\\s+SET task_status=\\?,workflow_revision=workflow_revision\\+1\\s+WHERE id=\\? AND task_status=\\?").
		WithArgs("InProgress", int64(981), "Completed").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE task_modules\\s+SET state='in_progress',terminal_at=NULL,updated_at=\\?\\s+WHERE task_id=\\? AND module_key='retouch'").
		WithArgs(confirmedAt, int64(981)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT state,terminal_at\\s+FROM task_modules\\s+WHERE task_id=\\? AND module_key='retouch'\\s+FOR UPDATE").
		WithArgs(int64(981)).
		WillReturnRows(sqlmock.NewRows([]string{"state", "terminal_at"}).
			AddRow("in_progress", nil))
	mock.ExpectExec("UPDATE tasks SET task_status=CASE").
		WillReturnResult(sqlmock.NewResult(0, 0))

	mapping := mappingFile{TaskDecisions: []taskStateDecisionMapping{{
		TaskID:          981,
		FromStatus:      "Completed",
		TargetStatus:    "InProgress",
		ReviewPolicyIDs: []string{reviewPolicyLegacyRetouchPrematurePartial},
		ConfirmedAt:     confirmedAt,
	}}}
	if err := migrateStates(context.Background(), tx, mapping); err != nil {
		t.Fatalf("migrateStates() reviewed retouch reopen error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateStatesRejectsAmbiguousReviewedRetouchModule(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	confirmedAt := time.Date(2026, 7, 25, 8, 30, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE tasks\\s+SET task_status=\\?,workflow_revision=workflow_revision\\+1\\s+WHERE id=\\? AND task_status=\\?").
		WithArgs("InProgress", int64(981), "Completed").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE task_modules\\s+SET state='in_progress',terminal_at=NULL,updated_at=\\?\\s+WHERE task_id=\\? AND module_key='retouch'").
		WithArgs(confirmedAt, int64(981)).
		WillReturnResult(sqlmock.NewResult(0, 2))
	mock.ExpectQuery("SELECT state,terminal_at\\s+FROM task_modules\\s+WHERE task_id=\\? AND module_key='retouch'\\s+FOR UPDATE").
		WithArgs(int64(981)).
		WillReturnRows(sqlmock.NewRows([]string{"state", "terminal_at"}).
			AddRow("in_progress", nil).
			AddRow("in_progress", nil))

	mapping := mappingFile{TaskDecisions: []taskStateDecisionMapping{{
		TaskID:          981,
		FromStatus:      "Completed",
		TargetStatus:    "InProgress",
		ReviewPolicyIDs: []string{reviewPolicyLegacyRetouchPrematurePartial},
		ConfirmedAt:     confirmedAt,
	}}}
	err = migrateStates(context.Background(), tx, mapping)
	if err == nil || !strings.Contains(err.Error(), "requires exactly one active non-terminal retouch module") {
		t.Fatalf("unexpected ambiguous reviewed retouch module error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateStatesRejectsReviewedRetouchModuleIterationError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	confirmedAt := time.Date(2026, 7, 25, 8, 30, 0, 0, time.UTC)
	mock.ExpectExec("UPDATE tasks\\s+SET task_status=\\?,workflow_revision=workflow_revision\\+1\\s+WHERE id=\\? AND task_status=\\?").
		WithArgs("InProgress", int64(981), "Completed").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE task_modules\\s+SET state='in_progress',terminal_at=NULL,updated_at=\\?\\s+WHERE task_id=\\? AND module_key='retouch'").
		WithArgs(confirmedAt, int64(981)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery("SELECT state,terminal_at\\s+FROM task_modules\\s+WHERE task_id=\\? AND module_key='retouch'\\s+FOR UPDATE").
		WithArgs(int64(981)).
		WillReturnRows(
			sqlmock.NewRows([]string{"state", "terminal_at"}).
				AddRow("in_progress", nil).
				AddRow("in_progress", nil).
				RowError(1, errors.New("driver iteration failed")),
		)

	mapping := mappingFile{TaskDecisions: []taskStateDecisionMapping{{
		TaskID:          981,
		FromStatus:      "Completed",
		TargetStatus:    "InProgress",
		ReviewPolicyIDs: []string{reviewPolicyLegacyRetouchPrematurePartial},
		ConfirmedAt:     confirmedAt,
	}}}
	err = migrateStates(context.Background(), tx, mapping)
	if err == nil || !strings.Contains(err.Error(), "iterate reviewed retouch module rows") {
		t.Fatalf("unexpected iteration error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMigrateStatesRejectsUnexpectedReviewedDecisionDrift(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()

	mock.ExpectExec("UPDATE tasks\\s+SET task_status=\\?,workflow_revision=workflow_revision\\+1\\s+WHERE id=\\? AND task_status=\\?").
		WithArgs("InProgress", int64(449), "PendingWarehouseReceive").
		WillReturnResult(sqlmock.NewResult(0, 0))
	mock.ExpectQuery("SELECT task_status FROM tasks WHERE id=\\? FOR UPDATE").
		WithArgs(int64(449)).
		WillReturnRows(sqlmock.NewRows([]string{"task_status"}).AddRow("Completed"))

	mapping := mappingFile{TaskDecisions: []taskStateDecisionMapping{{
		TaskID: 449, FromStatus: "PendingWarehouseReceive", TargetStatus: "InProgress",
	}}}
	if err := migrateStates(context.Background(), tx, mapping); err == nil || !strings.Contains(err.Error(), "no longer matches reviewed state decision") {
		t.Fatalf("unexpected drift error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyHardAbortsBeforeJournalForEveryPreflightBlocker(t *testing.T) {
	tests := []struct {
		name       string
		orgRows    *sqlmock.Rows
		accessRows *sqlmock.Rows
		taskRows   *sqlmock.Rows
		groupRows  *sqlmock.Rows
	}{
		{
			name:       "ambiguous organization",
			orgRows:    sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}).AddRow("user", 7, "department name missing or ambiguous"),
			accessRows: sqlmock.NewRows([]string{"user_id", "role"}),
			taskRows:   sqlmock.NewRows([]string{"id", "task_type", "task_status"}),
			groupRows:  sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}),
		},
		{
			name:       "manual access",
			orgRows:    sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}),
			accessRows: sqlmock.NewRows([]string{"user_id", "role"}).AddRow(8, "Warehouse"),
			taskRows:   sqlmock.NewRows([]string{"id", "task_type", "task_status"}),
			groupRows:  sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}),
		},
		{
			name:       "unmapped resource group",
			orgRows:    sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}),
			accessRows: sqlmock.NewRows([]string{"user_id", "role"}),
			taskRows:   sqlmock.NewRows([]string{"id", "task_type", "task_status"}),
			groupRows:  sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}).AddRow(9, 10, "sku", 11),
		},
		{
			name:       "unmapped purchase task",
			orgRows:    sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}),
			accessRows: sqlmock.NewRows([]string{"user_id", "role"}),
			taskRows:   sqlmock.NewRows([]string{"id", "task_type", "task_status"}).AddRow(12, "purchase_task", "InProgress"),
			groupRows:  sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}),
		},
		{
			name:       "warehouse rejection",
			orgRows:    sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}),
			accessRows: sqlmock.NewRows([]string{"user_id", "role"}),
			taskRows:   sqlmock.NewRows([]string{"id", "task_type", "task_status"}).AddRow(13, "normal", "RejectedByWarehouse"),
			groupRows:  sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT subject_type,subject_id,reason").WillReturnRows(tt.orgRows)
			mock.ExpectQuery("SELECT ur.user_id,ur.role").WillReturnRows(tt.accessRows)
			mock.ExpectQuery("SELECT id,task_type,task_status").WillReturnRows(tt.taskRows)
			mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").WillReturnRows(tt.groupRows)
			dir := t.TempDir()
			err = apply(context.Background(), db, "unit_test", options{SnapshotDir: dir}, mappingFile{})
			if err == nil || !strings.Contains(err.Error(), "cutover preflight blocked") {
				t.Fatalf("apply() error = %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "workflow-groups-snapshot.json")); !os.IsNotExist(statErr) {
				t.Fatalf("manifest should not exist, stat error = %v", statErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestMappedPurchaseTaskIsNotAPreflightBlocker(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT subject_type,subject_id,reason").WillReturnRows(sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}))
	mock.ExpectQuery("SELECT ur.user_id,ur.role").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
	mock.ExpectQuery("SELECT id,task_type,task_status").WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status"}).AddRow(14, "purchase_task", "InProgress"))
	mock.ExpectQuery("SELECT id FROM task_sku_items WHERE task_id=\\?").WithArgs(int64(14)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(22))
	mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}))
	blockers, err := queryCutoverBlockers(context.Background(), db, mappingFile{Planning: []planningMapping{{
		TaskID: 14, CodeRuleRevisionID: 20, CreatedBy: 21,
		Items: []planningItemMapping{{TaskSKUItemID: 22, DescriptionSpec: "规格", Quantity: 1}},
	}}})
	if err != nil {
		t.Fatal(err)
	}
	if !blockers.Empty() {
		t.Fatalf("mapped purchase task should not be blocked: %+v", blockers)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestMappedIncompleteUATPlanningTombstoneIsNotAPreflightBlocker(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectQuery("SELECT subject_type,subject_id,reason").
		WillReturnRows(sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}))
	mock.ExpectQuery("SELECT ur.user_id,ur.role").
		WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
	mock.ExpectQuery("SELECT id,task_type,task_status").
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status"}).
			AddRow(497, "purchase_task", "InProgress"))
	mock.ExpectQuery("SELECT id FROM task_sku_items WHERE task_id=\\?").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(380))
	mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").
		WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}))
	blockers, err := queryCutoverBlockers(context.Background(), db, mappingFile{
		Version:  workflowGroupsMappingV2,
		Planning: []planningMapping{validIncompleteUATPlanningTombstone(t)},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !blockers.Empty() {
		t.Fatalf("reviewed UAT planning tombstone should not be blocked: %+v", blockers)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPurchasePlanningPreflightRejectsPartialDuplicateAndCrossTaskSKUIdsBeforeJournal(t *testing.T) {
	tests := []struct {
		name       string
		mappingIDs []int64
	}{
		{name: "partial", mappingIDs: []int64{101}},
		{name: "duplicate", mappingIDs: []int64{101, 101}},
		{name: "cross task", mappingIDs: []int64{101, 999}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT subject_type,subject_id,reason").WillReturnRows(sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}))
			mock.ExpectQuery("SELECT ur.user_id,ur.role").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
			mock.ExpectQuery("SELECT id,task_type,task_status").WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status"}).AddRow(20, "purchase_task", "InProgress"))
			mock.ExpectQuery("SELECT id FROM task_sku_items WHERE task_id=\\?").WithArgs(int64(20)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(101).AddRow(102))
			mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}))
			items := make([]planningItemMapping, 0, len(tt.mappingIDs))
			for _, id := range tt.mappingIDs {
				items = append(items, planningItemMapping{TaskSKUItemID: id, DescriptionSpec: "规格", Quantity: 1})
			}
			mapping := mappingFile{Planning: []planningMapping{{TaskID: 20, CodeRuleRevisionID: 30, CreatedBy: 40, Items: items}}}
			dir := t.TempDir()
			err = apply(context.Background(), db, "unit_test", options{SnapshotDir: dir}, mapping)
			if err == nil || !strings.Contains(err.Error(), "cutover preflight blocked") {
				t.Fatalf("apply() error = %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "workflow-groups-snapshot.json")); !os.IsNotExist(statErr) {
				t.Fatalf("manifest should not exist, stat error = %v", statErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlanningPreflightRejectsNonPurchaseAndMissingTasksBeforeJournal(t *testing.T) {
	tests := []struct {
		name     string
		taskRows *sqlmock.Rows
	}{
		{name: "non purchase", taskRows: sqlmock.NewRows([]string{"task_type", "task_status"}).AddRow("normal", "InProgress")},
		{name: "missing", taskRows: sqlmock.NewRows([]string{"task_type", "task_status"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT subject_type,subject_id,reason").WillReturnRows(sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}))
			mock.ExpectQuery("SELECT ur.user_id,ur.role").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
			mock.ExpectQuery("SELECT id,task_type,task_status").WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status"}))
			mock.ExpectQuery("SELECT task_type,task_status FROM tasks WHERE id=\\?").WithArgs(int64(55)).WillReturnRows(tt.taskRows)
			mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}))
			mapping := mappingFile{Planning: []planningMapping{{
				TaskID: 55, CodeRuleRevisionID: 56, CreatedBy: 57,
				Items: []planningItemMapping{{TaskSKUItemID: 58, DescriptionSpec: "规格", Quantity: 1}},
			}}}
			dir := t.TempDir()
			err = apply(context.Background(), db, "unit_test", options{SnapshotDir: dir}, mapping)
			if err == nil || !strings.Contains(err.Error(), "cutover preflight blocked") {
				t.Fatalf("apply() error = %v", err)
			}
			if _, statErr := os.Stat(filepath.Join(dir, "workflow-groups-snapshot.json")); !os.IsNotExist(statErr) {
				t.Fatalf("manifest should not exist, stat error = %v", statErr)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExistingPlanningTaskRequiresCompleteSameMappingBeforeJournal(t *testing.T) {
	tests := []struct {
		name        string
		expectState func(sqlmock.Sqlmock)
		wantBlocked bool
	}{
		{
			name: "missing settings",
			expectState: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status", "code_rule_revision_id", "created_by"}))
			},
			wantBlocked: true,
		},
		{
			name: "different settings",
			expectState: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status", "code_rule_revision_id", "created_by"}).AddRow("sku_planning", "Completed", 999, 72))
			},
			wantBlocked: true,
		},
		{
			name: "different revision fields",
			expectState: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status", "code_rule_revision_id", "created_by"}).AddRow("sku_planning", "Completed", 71, 72))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_sku_items").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT r.description_spec,r.quantity").WithArgs(int64(70), int64(73)).WillReturnRows(planningRevisionRows().AddRow("different", 1, nil, "", "", "", "", nil))
			},
			wantBlocked: true,
		},
		{
			name: "different image",
			expectState: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status", "code_rule_revision_id", "created_by"}).AddRow("sku_planning", "Completed", 71, 72))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_sku_items").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT r.description_spec,r.quantity").WithArgs(int64(70), int64(73)).WillReturnRows(planningRevisionRows().AddRow("规格", 1, nil, "", "", "", "", "storage-other"))
			},
			wantBlocked: true,
		},
		{
			name: "exact mapping",
			expectState: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status", "code_rule_revision_id", "created_by"}).AddRow("sku_planning", "Completed", 71, 72))
				mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_sku_items").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
				mock.ExpectQuery("SELECT r.description_spec,r.quantity").WithArgs(int64(70), int64(73)).WillReturnRows(planningRevisionRows().AddRow("规格", 1, nil, "", "", "", "", nil))
			},
			wantBlocked: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT subject_type,subject_id,reason").WillReturnRows(sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}))
			mock.ExpectQuery("SELECT ur.user_id,ur.role").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
			mock.ExpectQuery("SELECT id,task_type,task_status").WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status"}))
			mock.ExpectQuery("SELECT task_type,task_status FROM tasks WHERE id=\\?").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status"}).AddRow("sku_planning", "Completed"))
			mock.ExpectQuery("SELECT id FROM task_sku_items WHERE task_id=\\?").WithArgs(int64(70)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(73))
			tt.expectState(mock)
			mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}))
			mapping := mappingFile{Planning: []planningMapping{{
				TaskID: 70, TargetTaskStatus: "Completed", CodeRuleRevisionID: 71, CreatedBy: 72,
				Items: []planningItemMapping{{TaskSKUItemID: 73, DescriptionSpec: "规格", Quantity: 1}},
			}}}
			blockers, err := queryCutoverBlockers(context.Background(), db, mapping)
			if err != nil {
				t.Fatal(err)
			}
			if (len(blockers.Tasks) > 0) != tt.wantBlocked {
				t.Fatalf("task blockers = %+v, want blocked=%v", blockers.Tasks, tt.wantBlocked)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func planningRevisionRows() *sqlmock.Rows {
	return sqlmock.NewRows([]string{"description_spec", "quantity", "target_price", "note", "reference_url", "erp_product_i_id", "erp_product_name", "storage_ref_id"})
}

func TestPreflightLocksEveryTaskBeforeOrganizationRecheck(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT id FROM tasks ORDER BY id FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2))
	mock.ExpectQuery("SELECT id FROM org_departments ORDER BY id FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT id FROM org_teams ORDER BY id FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT id FROM users ORDER BY id FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"id"}))
	mock.ExpectQuery("SELECT user_id,role FROM user_roles ORDER BY user_id,role FOR UPDATE").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
	if err := lockPreflightRows(context.Background(), tx); err != nil {
		t.Fatal(err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestSnapshotIntegrityDatabaseAndMappingAreBound(t *testing.T) {
	mapping := mappingFile{Resources: []resourceMapping{{TaskID: 1, ScopeKind: "task", ScopeRefID: 0, TargetStatus: "shell", CreatedBy: 2}}}
	mappingSHA256, err := mappingDigest(mapping)
	if err != nil {
		t.Fatal(err)
	}
	s := snapshot{
		Version: workflowGroupsSnapshotVersion, ToolVersion: workflowGroupsToolVersion,
		SchemaVersion: workflowGroupsSchemaVersion, Database: "unit_test", MappingSHA256: mappingSHA256,
		ApplyState: "applied", CreatedAt: time.Now().UTC(), AppliedAt: timePointer(time.Now().UTC()),
		Tasks: []taskSnapshot{}, AfterTasks: []taskSnapshot{}, ResourceGroups: []resourceGroupSnapshot{}, AfterResourceGroups: []resourceGroupSnapshot{},
		AssetBindings: []assetBindingSnapshot{}, AfterAssetBindings: []assetBindingSnapshot{}, SKUOrigins: []skuOriginSnapshot{}, AfterSKUOrigins: []skuOriginSnapshot{},
		PlanningBefore: []planningStateSnapshot{}, PlanningAfter: []planningStateSnapshot{},
	}
	path := filepath.Join(t.TempDir(), "workflow-groups-snapshot.json")
	if err := writeSnapshot(path, s); err != nil {
		t.Fatal(err)
	}
	if _, err := readSnapshot(path, "unit_test", mapping); err != nil {
		t.Fatalf("read valid snapshot: %v", err)
	}
	if _, err := readSnapshot(path, "other_database", mapping); err == nil {
		t.Fatal("expected database mismatch")
	}
	if _, err := readSnapshot(path, "unit_test", mappingFile{}); err == nil {
		t.Fatal("expected mapping mismatch")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var tampered map[string]interface{}
	if err := json.Unmarshal(raw, &tampered); err != nil {
		t.Fatal(err)
	}
	tampered["apply_state"] = "rolled_back"
	raw, err = json.Marshal(tampered)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := readSnapshot(path, "unit_test", mapping); err == nil || !strings.Contains(err.Error(), "integrity") {
		t.Fatalf("expected integrity failure, got %v", err)
	}
}

func TestExactFileDigestBindsRawMappingBytes(t *testing.T) {
	raw := []byte("{\"version\":2}\n")
	path := filepath.Join(t.TempDir(), "mapping.json")
	if err := os.WriteFile(path, raw, 0o600); err != nil {
		t.Fatal(err)
	}
	want := fmt.Sprintf("%x", sha256.Sum256(raw))
	got, err := exactFileDigest(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want {
		t.Fatalf("exact file digest = %q, want %q", got, want)
	}
	if got, err := exactFileDigest(""); err != nil || got != "" {
		t.Fatalf("empty mapping digest = %q, %v", got, err)
	}
}

func TestExplicitAccessMigrationUsesOnlyUniqueStableOrgMatches(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", "124_explicit_access_platform.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(raw)
	if strings.Contains(sqlText, "d.id IS NULL OR") {
		t.Fatal("migration must not match a team globally when department lookup fails")
	}
	if strings.Count(sqlText, "HAVING COUNT(*) = 1") < 5 {
		t.Fatal("department and team backfills must require unique stable matches")
	}
	for _, mapping := range []string{
		"WHEN 'DepartmentAdmin' THEN 'department_admin'",
		"WHEN 'TeamLead' THEN 'team_lead'",
		"WHEN 'DesignDirector' THEN 'design_director'",
	} {
		if !strings.Contains(sqlText, mapping) {
			t.Fatalf("missing explicit legacy role mapping %q", mapping)
		}
	}
	if strings.Contains(sqlText, "WHEN 'OrgAdmin' THEN") {
		t.Fatal("OrgAdmin must remain an explicit manual blocker, not an inferred grant")
	}
}

func TestResourceGroupMigrationCreatesAdapterAwareDeletionOutbox(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "..", "db", "migrations", "125_task_resource_groups.sql"))
	if err != nil {
		t.Fatal(err)
	}
	sqlText := string(raw)
	for _, required := range []string{
		"CREATE TABLE asset_object_deletion_outbox",
		"storage_ref_id VARCHAR(64) NULL",
		"storage_adapter VARCHAR(32) NOT NULL",
		"storage_is_placeholder TINYINT(1) NOT NULL DEFAULT 0",
		"UNIQUE KEY uq_asset_object_deletion_dedupe (dedupe_key)",
		"DROP TABLE IF EXISTS asset_object_deletion_outbox",
	} {
		if !strings.Contains(sqlText, required) {
			t.Fatalf("migration 125 is missing adapter-aware outbox contract %q", required)
		}
	}
}

func timePointer(value time.Time) *time.Time { return &value }

func TestValidateMapping(t *testing.T) {
	valid := mappingFile{Resources: []resourceMapping{{TaskID: 1, ScopeKind: "sku", ScopeRefID: 2, Mode: "set", FinalAssetIDs: []int64{3, 4}, CreatedBy: 5, TargetStatus: "finalized"}}, Planning: []planningMapping{{TaskID: 6, CodeRuleRevisionID: 7, CreatedBy: 5, Items: []planningItemMapping{{TaskSKUItemID: 8, DescriptionSpec: "规格", Quantity: 1}}}}}
	if err := validateMapping(valid); err != nil {
		t.Fatalf("valid mapping: %v", err)
	}
	invalid := mappingFile{Resources: []resourceMapping{{TaskID: 1, ScopeKind: "sku", ScopeRefID: 2, Mode: "single", FinalAssetIDs: []int64{3, 4}, CreatedBy: 5, TargetStatus: "finalized"}}}
	if err := validateMapping(invalid); err == nil {
		t.Fatal("expected single cardinality error")
	}
}

func TestReferenceMatchesResourceScope(t *testing.T) {
	null := sql.NullInt64{}
	sku10 := sql.NullInt64{Int64: 10, Valid: true}
	sku11 := sql.NullInt64{Int64: 11, Valid: true}
	retouch20 := sql.NullInt64{Int64: 20, Valid: true}
	retouch21 := sql.NullInt64{Int64: 21, Valid: true}
	tests := []struct {
		name       string
		scopeKind  string
		scopeRefID int64
		sku        sql.NullInt64
		retouch    sql.NullInt64
		want       bool
	}{
		{name: "task accepts task reference", scopeKind: "task", sku: null, retouch: null, want: true},
		{name: "task rejects sku reference", scopeKind: "task", sku: sku10, retouch: null},
		{name: "task rejects retouch reference", scopeKind: "task", sku: null, retouch: retouch20},
		{name: "sku inherits task reference", scopeKind: "sku", scopeRefID: 10, sku: null, retouch: null, want: true},
		{name: "sku accepts same sku", scopeKind: "sku", scopeRefID: 10, sku: sku10, retouch: null, want: true},
		{name: "sku rejects other sku", scopeKind: "sku", scopeRefID: 10, sku: sku11, retouch: null},
		{name: "sku rejects retouch", scopeKind: "sku", scopeRefID: 10, sku: null, retouch: retouch20},
		{name: "retouch inherits task reference", scopeKind: "retouch_requirement", scopeRefID: 20, sku: null, retouch: null, want: true},
		{name: "retouch accepts same requirement", scopeKind: "retouch_requirement", scopeRefID: 20, sku: null, retouch: retouch20, want: true},
		{name: "retouch rejects other requirement", scopeKind: "retouch_requirement", scopeRefID: 20, sku: null, retouch: retouch21},
		{name: "retouch rejects sku", scopeKind: "retouch_requirement", scopeRefID: 20, sku: sku10, retouch: null},
		{name: "dual discriminator always rejected", scopeKind: "retouch_requirement", scopeRefID: 20, sku: sku10, retouch: retouch20},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := referenceMatchesResourceScope(tt.scopeKind, tt.scopeRefID, tt.sku, tt.retouch); got != tt.want {
				t.Fatalf("referenceMatchesResourceScope() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResourceMappingPreflightConstrainsRetouchReferenceScope(t *testing.T) {
	tests := []struct {
		name        string
		refRetouch  int64
		wantBlocked bool
	}{
		{name: "matching requirement", refRetouch: 50},
		{name: "different requirement", refRetouch: 51, wantBlocked: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT task_type,task_status FROM tasks").WithArgs(int64(10)).
				WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status"}).AddRow("retouch_task", "InProgress"))
			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM users").WithArgs(int64(20)).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_retouch_requirements").WithArgs(int64(50), int64(10)).
				WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
			mock.ExpectQuery("SELECT id FROM task_asset_groups").WithArgs(int64(10), "retouch_requirement", int64(50)).
				WillReturnRows(sqlmock.NewRows([]string{"id"}))
			mock.ExpectQuery("SELECT task_id FROM task_assets").WithArgs(int64(30)).
				WillReturnRows(sqlmock.NewRows([]string{"task_id"}).AddRow(10))
			mock.ExpectQuery("SELECT task_id,sku_item_id,retouch_requirement_id FROM reference_file_refs").WithArgs(int64(40)).
				WillReturnRows(sqlmock.NewRows([]string{"task_id", "sku_item_id", "retouch_requirement_id"}).AddRow(10, nil, tt.refRetouch))

			issue, err := validateResourceMappingPreflight(context.Background(), db, resourceMapping{
				TaskID: 10, ScopeKind: "retouch_requirement", ScopeRefID: 50,
				Mode: "single", FinalAssetIDs: []int64{30}, ReferenceIDs: []int64{40},
				CreatedBy: 20, TargetStatus: "draft",
			})
			if err != nil {
				t.Fatal(err)
			}
			if (issue != nil) != tt.wantBlocked {
				t.Fatalf("issue = %+v, wantBlocked %v", issue, tt.wantBlocked)
			}
			if issue != nil && !strings.Contains(issue.Reason, "does not belong to the mapped task scope") {
				t.Fatalf("unexpected issue: %+v", issue)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPlanningAndResourceMappingOverlapHardAbortsBeforeJournal(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mapping := mappingFile{
		Resources: []resourceMapping{{TaskID: 10, ScopeKind: "task", ScopeRefID: 0, CreatedBy: 20, TargetStatus: "shell"}},
		Planning:  []planningMapping{{TaskID: 10, CodeRuleRevisionID: 30, CreatedBy: 20, Items: []planningItemMapping{{TaskSKUItemID: 40, DescriptionSpec: "规格", Quantity: 1}}}},
	}
	if err := validateMapping(mapping); err == nil || !strings.Contains(err.Error(), "cannot migrate both planning data and design resources") {
		t.Fatalf("validateMapping() error = %v", err)
	}
	dir := t.TempDir()
	err = apply(context.Background(), db, "unit_test", options{SnapshotDir: dir}, mapping)
	if err == nil || !strings.Contains(err.Error(), "invalid reviewed mapping") {
		t.Fatalf("apply() error = %v", err)
	}
	if _, statErr := os.Stat(filepath.Join(dir, "workflow-groups-snapshot.json")); !os.IsNotExist(statErr) {
		t.Fatalf("manifest should not exist, stat error = %v", statErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestInsertMigratedReferenceSnapshotFreezesAllScopeKinds(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherFunc(func(expected, actual string) error {
		for _, token := range strings.Split(expected, "&&") {
			if !strings.Contains(actual, token) {
				return fmt.Errorf("query missing %q: %s", token, actual)
			}
		}
		return nil
	})))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for index, referenceID := range []int64{101, 102, 103} {
		mock.ExpectExec("INSERT INTO task_asset_group_revision_references&&ref_id_snapshot&&file_name_snapshot&&scope_snapshot&&retouch_requirement_id IS NOT NULL&&CONCAT('retouch_requirement:'&&sku_item_id IS NOT NULL&&CONCAT('sku:'&&ELSE 'task'").
			WithArgs(int64(200), index, referenceID).
			WillReturnResult(sqlmock.NewResult(0, 1))
		if err := insertMigratedReferenceSnapshot(context.Background(), tx, 200, referenceID, index); err != nil {
			t.Fatalf("insert reference %d: %v", referenceID, err)
		}
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestVerifyResourceMappingChecksMigratedReferenceSnapshots(t *testing.T) {
	tests := []struct {
		name          string
		storedRef     string
		storedName    string
		storedScope   string
		expectedScope string
		wantErr       bool
	}{
		{name: "task snapshot", storedRef: "ref-40", storedName: "ref.png", storedScope: "task", expectedScope: "task"},
		{name: "sku snapshot", storedRef: "ref-40", storedName: "ref.png", storedScope: "sku:50", expectedScope: "sku:50"},
		{name: "retouch snapshot", storedRef: "ref-40", storedName: "ref.png", storedScope: "retouch_requirement:50", expectedScope: "retouch_requirement:50"},
		{name: "empty snapshot", expectedScope: "task", wantErr: true},
		{name: "wrong scope snapshot", storedRef: "ref-40", storedName: "ref.png", storedScope: "retouch_requirement:51", expectedScope: "retouch_requirement:50", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatal(err)
			}
			defer db.Close()
			mock.ExpectQuery("SELECT working_revision_id FROM task_asset_groups").WithArgs(int64(10)).
				WillReturnRows(sqlmock.NewRows([]string{"working_revision_id"}).AddRow(20))
			mock.ExpectQuery("SELECT status,mode,source_task_asset_id,source_stage FROM task_asset_group_revisions").WithArgs(int64(20), int64(10)).
				WillReturnRows(sqlmock.NewRows([]string{"status", "mode", "source_task_asset_id", "source_stage"}).AddRow("draft", "single", nil, "migration"))
			mock.ExpectQuery("SELECT task_asset_id FROM task_asset_group_revision_items").WithArgs(int64(20)).
				WillReturnRows(sqlmock.NewRows([]string{"task_asset_id"}).AddRow(30))
			mock.ExpectQuery("SELECT rr.reference_file_ref_id,rr.ref_id_snapshot").WithArgs(int64(20)).
				WillReturnRows(sqlmock.NewRows([]string{
					"reference_file_ref_id", "ref_id_snapshot", "file_name_snapshot", "scope_snapshot",
					"expected_ref_id", "expected_file_name", "expected_scope_snapshot",
				}).AddRow(40, tt.storedRef, tt.storedName, tt.storedScope, "ref-40", "ref.png", tt.expectedScope))
			err = verifyResourceMappingQuery(context.Background(), db, 10, resourceMapping{
				Mode: "single", FinalAssetIDs: []int64{30}, ReferenceIDs: []int64{40}, TargetStatus: "draft",
			})
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), "invalid frozen snapshot") {
					t.Fatalf("verifyResourceMappingQuery() error = %v", err)
				}
			} else if err != nil {
				t.Fatalf("verifyResourceMappingQuery() error = %v", err)
			}
			if err := mock.ExpectationsWereMet(); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestValidateCutoverStateRejectsPlanningTasksWithResourceGroups(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 6; i++ {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\)").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM tasks t JOIN task_asset_groups").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	err = validateCutoverState(context.Background(), tx, mappingFile{})
	if err == nil || !strings.Contains(err.Error(), "sku_planning tasks still own design resource groups") {
		t.Fatalf("validateCutoverState() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCutoverStateVerifiesEveryConfirmedPlanningMapping(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mapping := mappingFile{
		Version: workflowGroupsMappingV2,
		Planning: []planningMapping{{
			TaskID:     123,
			Confidence: "confirmed_auto",
		}},
	}
	mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").
		WithArgs(int64(123)).
		WillReturnError(fmt.Errorf("postcondition drift"))

	err = validateCutoverState(context.Background(), tx, mapping)
	if err == nil || !strings.Contains(err.Error(), "verified planning state differs from mapping") {
		t.Fatalf("validateCutoverState() error = %v", err)
	}
	mock.ExpectRollback()
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCutoverStateAcceptsOnlyVerifiedPlanningTombstoneException(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mapping := mappingFile{
		Version:  workflowGroupsMappingV2,
		Planning: []planningMapping{validIncompleteUATPlanningTombstone(t)},
	}

	expectPlanningTombstoneVerification(mock)
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_asset_groups WHERE migration_incomplete=1").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM tasks WHERE task_type='purchase_task'").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM tasks WHERE task_status IN").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_modules tm JOIN tasks t").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("WHERE t.task_type='sku_planning' AND t.id <> 497 AND").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("WITH expected_scopes AS").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM tasks t JOIN task_asset_groups").
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

	if err := validateCutoverState(context.Background(), tx, mapping); err != nil {
		t.Fatalf("validateCutoverState() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestValidateCutoverStateBlocksUnmappedPurchaseTaskBeforeCommit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_asset_groups").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM tasks WHERE task_type='purchase_task'").WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	err = validateCutoverState(context.Background(), tx, mappingFile{})
	if err == nil || !strings.Contains(err.Error(), "legacy purchase_task rows remain") {
		t.Fatalf("validateCutoverState() error = %v", err)
	}
	mock.ExpectRollback()
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyPlanningRejectsInconsistentRerunBeforeMutation(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectQuery("SELECT task_type,task_status FROM tasks").WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status"}).AddRow("sku_planning", "Completed"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_planning_settings").WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").WithArgs(int64(10)).WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status", "code_rule_revision_id", "created_by"}).AddRow("sku_planning", "Completed", int64(999), int64(7)))
	inserted, err := applyPlanning(context.Background(), tx, planningMapping{
		TaskID: 10, TargetTaskStatus: "Completed", CodeRuleRevisionID: 20, CreatedBy: 7,
		Items: []planningItemMapping{{TaskSKUItemID: 30, DescriptionSpec: "规格", Quantity: 1}},
	})
	if err == nil || inserted || !strings.Contains(err.Error(), "settings differ") {
		t.Fatalf("applyPlanning() inserted/error = %v/%v", inserted, err)
	}
	mock.ExpectRollback()
	if rollbackErr := tx.Rollback(); rollbackErr != nil {
		t.Fatal(rollbackErr)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyPlanningTombstoneCreatesNoRevisionAndRerunsIdempotently(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mapping := validIncompleteUATPlanningTombstone(t)

	mock.ExpectQuery("SELECT task_type,task_status FROM tasks").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status"}).AddRow("purchase_task", "InProgress"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_planning_settings").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectExec("INSERT INTO task_planning_settings").WithArgs(int64(497), int64(9), "migration-497", int64(1)).
		WillReturnResult(sqlmock.NewResult(1, 1))
	mock.ExpectExec("UPDATE tasks SET task_type='sku_planning'").WithArgs("Cancelled", int64(497)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE task_sku_items SET sku_origin='legacy_migration'").WithArgs(int64(497)).
		WillReturnResult(sqlmock.NewResult(0, 1))
	inserted, err := applyPlanning(context.Background(), tx, mapping)
	if err != nil || !inserted {
		t.Fatalf("initial applyPlanning() inserted/error = %v/%v", inserted, err)
	}

	mock.ExpectQuery("SELECT task_type,task_status FROM tasks").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status"}).AddRow("sku_planning", "Cancelled"))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_planning_settings").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	expectPlanningTombstoneVerification(mock)
	inserted, err = applyPlanning(context.Background(), tx, mapping)
	if err != nil || inserted {
		t.Fatalf("rerun applyPlanning() inserted/error = %v/%v", inserted, err)
	}

	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPlanningTombstoneRollbackJournalTracksOnlyCreatedSettings(t *testing.T) {
	created := diffPlanningCreated(
		[]planningStateSnapshot{{
			TaskID:           497,
			SettingsExists:   false,
			Details:          []planningDetailSnapshot{},
			RevisionIDs:      []int64{},
			ImageRevisionIDs: []int64{},
		}},
		[]planningStateSnapshot{{
			TaskID:           497,
			SettingsExists:   true,
			Details:          []planningDetailSnapshot{},
			RevisionIDs:      []int64{},
			ImageRevisionIDs: []int64{},
		}},
	)
	if len(created) != 1 || !created[0].SettingsCreated ||
		len(created[0].DetailIDs) != 0 ||
		len(created[0].RevisionIDs) != 0 ||
		len(created[0].ImageRevisionIDs) != 0 {
		t.Fatalf("diffPlanningCreated() = %+v", created)
	}
}

func expectPlanningTombstoneVerification(mock sqlmock.Sqlmock) {
	mock.ExpectQuery("SELECT t.task_type,t.task_status,s.code_rule_revision_id,s.created_by").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"task_type", "task_status", "code_rule_revision_id", "created_by"}).
			AddRow("sku_planning", "Cancelled", int64(9), int64(1)))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_sku_items WHERE task_id=\\?").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_sku_items WHERE task_id=\\? AND id=\\?").WithArgs(int64(497), int64(380)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_planning_sku_details").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_planning_sku_revisions").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\) FROM task_planning_sku_revision_images").WithArgs(int64(497)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))
}

func TestPlanningTargetStatusPreservesTerminalAndAllowsExplicitActiveStates(t *testing.T) {
	for _, status := range []string{"PendingAssign", "Completed", "Cancelled", "Archived"} {
		if err := validatePlanningTargetTransition(status, status); err != nil {
			t.Fatalf("preserve %s: %v", status, err)
		}
	}
	for _, terminal := range []string{"Cancelled", "Archived"} {
		if err := validatePlanningTargetTransition(terminal, "Completed"); err == nil || !strings.Contains(err.Error(), "must be preserved") {
			t.Fatalf("terminal %s transition error = %v", terminal, err)
		}
	}
}

func TestApplyHistoricalUnavailableRecoveryIsExactAndPointerSafe(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery := historicalUnavailableRecoveryFixture()
	expectAssetStorageRefStatus(mock, recovery, "recorded")
	expectHistoricalUnavailableRecoveryEvidence(mock, recovery, 0, "recorded")
	mock.ExpectExec("UPDATE asset_storage_refs").
		WithArgs(recovery.OriginalStorageRefID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := applyAssetRecoveries(context.Background(), tx, []assetRecoveryMapping{recovery}); err != nil {
		t.Fatalf("applyAssetRecoveries() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyHistoricalUnavailableRecoveryTreatsExactTargetAsIdempotent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery := historicalUnavailableRecoveryFixture()
	expectAssetStorageRefStatus(mock, recovery, "historical_unavailable")
	expectHistoricalUnavailableRecoveryEvidence(
		mock, recovery, 0, "historical_unavailable",
	)
	if err := applyAssetRecoveries(
		context.Background(), tx, []assetRecoveryMapping{recovery},
	); err != nil {
		t.Fatalf("idempotent applyAssetRecoveries() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyPrematerializedRecoveryOnlyValidatesExecutorAfterState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery := prematerializedRecoveryFixture()
	expectPrematerializedRecoveryEvidence(mock, recovery, false)
	if err := applyAssetRecoveries(context.Background(), tx, []assetRecoveryMapping{recovery}); err != nil {
		t.Fatalf("applyAssetRecoveries() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPrematerializedRecoveryRejectsUploadBindingDrift(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	recovery := prematerializedRecoveryFixture()
	expectPrematerializedRecoveryEvidence(mock, recovery, true)
	if err := validatePrematerializedAssetRecoveryEvidence(context.Background(), db, recovery); err == nil || !strings.Contains(err.Error(), "upload request") {
		t.Fatalf("validatePrematerializedAssetRecoveryEvidence() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func expectPrematerializedRecoveryEvidence(mock sqlmock.Sqlmock, recovery assetRecoveryMapping, driftUpload bool) {
	const runID = "recovery-materialization-20260723-25"
	const targetRef = "11111111-1111-5111-8111-111111111111"
	const uploadRequestID = "e79e96b2-e2bf-48ed-bb66-a29e17b72018"
	objectKey := "v8-ab/" + runID + "/recovered/task-2807/task-asset-23989/" + recovery.RecoverySourceSHA256 + ".bin"
	mock.ExpectQuery("SELECT environment,run_id,plan_sha256").
		WillReturnRows(sqlmock.NewRows([]string{"environment", "run_id", "plan_sha256"}).
			AddRow("clone_b", runID, strings.Repeat("a", 64)))
	mock.ExpectQuery("SELECT task_id,asset_id,file_size,upload_request_id").
		WithArgs(int64(23989)).
		WillReturnRows(sqlmock.NewRows([]string{
			"task_id", "asset_id", "file_size", "upload_request_id",
			"storage_ref_id", "storage_key", "whole_hash", "upload_status", "access_revoked_reason",
			"deleted_at", "cleaned_at", "object_deleted_at", "access_revoked_at",
		}).AddRow(
			int64(2807), int64(22001), int64(683001), uploadRequestID,
			targetRef, objectKey, recovery.RecoverySourceSHA256, "uploaded", "",
			nil, nil, nil, nil,
		))
	mock.ExpectQuery("SELECT asset_id,owner_type,owner_id,upload_request_id,storage_adapter,ref_type").
		WithArgs(targetRef).
		WillReturnRows(sqlmock.NewRows([]string{
			"asset_id", "owner_type", "owner_id", "upload_request_id", "storage_adapter", "ref_type",
			"ref_key", "file_size", "is_placeholder", "checksum_hint", "status",
		}).AddRow(
			int64(22001), "task_asset", int64(23989), uploadRequestID, "local", "task_asset_object",
			objectKey, int64(683001), int64(0), recovery.RecoverySourceSHA256, "recorded",
		))
	boundRef := targetRef
	if driftUpload {
		boundRef = "wrong-ref"
	}
	mock.ExpectQuery("SELECT request_id,COALESCE\\(bound_ref_id,''\\),COALESCE\\(checksum_hint,''\\)").
		WithArgs(uploadRequestID).
		WillReturnRows(sqlmock.NewRows([]string{
			"request_id", "bound_ref_id", "checksum_hint", "file_size", "status", "session_status",
		}).AddRow(uploadRequestID, boundRef, recovery.RecoverySourceSHA256, int64(683001), "bound", "completed"))
}

func expectAssetStorageRefStatus(
	mock sqlmock.Sqlmock, recovery assetRecoveryMapping, status string,
) {
	mock.ExpectQuery("SELECT status\\s+FROM asset_storage_refs\\s+WHERE ref_id=\\?\\s+FOR UPDATE").
		WithArgs(recovery.OriginalStorageRefID).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(status))
}

func expectHistoricalUnavailableRecoveryEvidence(mock sqlmock.Sqlmock, recovery assetRecoveryMapping, currentReferences int, storageStatus string) {
	mock.ExpectQuery("SELECT id,task_id,asset_id,file_size,COALESCE\\(storage_ref_id,''\\)").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "asset_id", "file_size", "storage_ref_id",
			"superseded_by_version_id", "upload_status", "deleted_at", "cleaned_at", "object_deleted_at",
		}).
			AddRow(int64(12323), int64(2199), int64(12401), int64(17755216), recovery.OriginalStorageRefID, int64(14510), "uploaded", nil, nil, nil).
			AddRow(int64(14510), int64(2199), int64(12401), int64(17595421), "58aebabe-355c-4d24-814a-d6dca306b73d", int64(14514), "uploaded", nil, nil, nil).
			AddRow(int64(14514), int64(2199), int64(12401), int64(11275123), "6e6cd051-f261-424d-8b55-49dd6868be9a", nil, "uploaded", nil, nil, nil))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM task_assets.*WHERE task_id=\\? AND asset_id=\\?").
		WithArgs(int64(2199), int64(12401)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	expectHistoricalUnavailableSourceAliases(mock, nil)
	mock.ExpectQuery("SELECT asset_id,owner_type,owner_id,ref_key,status,file_size").
		WithArgs(recovery.OriginalStorageRefID).
		WillReturnRows(sqlmock.NewRows([]string{"asset_id", "owner_type", "owner_id", "ref_key", "status", "file_size"}).
			AddRow(int64(12323), "task_asset", int64(12323),
				"tasks/RW-20260709-A-002196/assets/AST-0002/v1/delivery/1783575756672661314_d97ed925.psd",
				storageStatus, int64(17755216)))
	mock.ExpectQuery("SELECT task_id,file_size,COALESCE\\(storage_ref_id,''\\),COALESCE\\(upload_status,''\\)").
		WithArgs(int64(12323)).
		WillReturnRows(sqlmock.NewRows([]string{"task_id", "file_size", "storage_ref_id", "upload_status", "deleted_at", "cleaned_at", "object_deleted_at"}).
			AddRow(int64(2199), int64(17755216), recovery.OriginalStorageRefID, "uploaded", nil, nil, nil))
	for _, derivative := range []struct {
		assetType string
		wholeHash string
	}{
		{"preview", recovery.PreviewWholeHash},
		{"design_thumb", recovery.DesignThumbWholeHash},
	} {
		mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM task_assets").
			WithArgs(int64(2199), int64(12323), derivative.assetType, derivative.wholeHash).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(1))
	}
	mock.ExpectQuery("WITH RECURSIVE asset_lineage").
		WithArgs(int64(12323), int64(12323)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(currentReferences))
}

func expectHistoricalUnavailableSourceAliases(mock sqlmock.Sqlmock, extraOrigins []int64) {
	rows := sqlmock.NewRows([]string{"id", "bound_group_id", "bound_role", "binding_state", "remark"}).
		AddRow(int64(25564), int64(3101), "source", "bound", sourceAliasRemark(3101, 12323)).
		AddRow(int64(25565), int64(3101), "source", "bound", sourceAliasRemark(3101, 14510)).
		AddRow(int64(25566), int64(3101), "source", "bound", sourceAliasRemark(3101, 14514))
	for index, originID := range extraOrigins {
		rows.AddRow(int64(25567+index), int64(3101), "source", "bound", sourceAliasRemark(3101, originID))
	}
	mock.ExpectQuery("SELECT id,bound_group_id,COALESCE\\(bound_role,''\\),COALESCE\\(binding_state,''\\),remark").
		WithArgs(int64(2199), int64(12401)).
		WillReturnRows(rows)
}

func prematerializedRecoveryFixture() assetRecoveryMapping {
	recovery := assetRecoveryMapping{
		TaskID:                     2807,
		MissingTaskAssetID:         23989,
		RecoverySourceTaskAssetID:  24034,
		Strategy:                   "verified_oss_recovery_v1",
		OriginalStorageRefID:       "f511c5d4-507f-4a69-bf10-70bae369429d",
		RecoverySourceStorageRefID: "983a746c-c674-4f5c-8812-073be989b194",
		ExpectedFileSize:           683001,
		PreviewWholeHash:           "471739776f4c230a80ae5514e83e92fd3f1e104d203ced3ac793c65c25a525e4",
		DesignThumbWholeHash:       "3442c0ac91eb61371d4057d6c0de232f8ba4f3c25cb6b68cff63142aa155e6ef",
		ControlledReadProtocol:     "controlled-asset-read-v1",
		ControlledReadEvidenceHash: "b39e0d9d26e6fdd35941b195bdc413eb12dd6795e23276a48c9b9bd49f829b08",
		RecoverySourceSHA256:       "d0558b1a9d4a7afed5a03b6b97d4a765d34050866686e396ab0acf9f08f0dec5",
		Confidence:                 "confirmed_auto",
		ReviewPolicyIDs:            []string{reviewPolicyLegacyDeletedAssetRecovery},
		ConfirmedBy:                1,
		ConfirmedAt:                time.Date(2026, 7, 23, 10, 0, 0, 0, time.UTC),
		ConfirmationNote:           "reviewed controlled read receipt and Clone B recovery executor after-state",
	}
	recovery.ManifestRowHash, _ = assetRecoveryManifestRowHash(recovery)
	return recovery
}

func TestApplyHistoricalUnavailableRecoveryRejectsCurrentPointer(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery := historicalUnavailableRecoveryFixture()
	expectAssetStorageRefStatus(mock, recovery, "recorded")
	expectHistoricalUnavailableRecoveryEvidence(mock, recovery, 1, "recorded")
	if err := applyAssetRecoveries(context.Background(), tx, []assetRecoveryMapping{recovery}); err == nil || !strings.Contains(err.Error(), "current working/finalized") {
		t.Fatalf("applyAssetRecoveries() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCurrentPointerAssetReferenceQueryCoversEveryIndirectPath(t *testing.T) {
	for _, fragment := range []string{
		"SELECT seed.id,seed.storage_ref_id",
		"source_asset_version_id=parent.id",
		"v8-source-alias:group=",
		"r.source_task_asset_id",
		"task_asset_group_revision_items",
		"rr.formal_task_asset_id",
		"reference_file_refs",
		"live_ref.asset_id",
		"live_ref.owner_id",
		"frozen_ref.asset_id",
		"frozen_ref.owner_id",
		"task_reference_asset_bindings",
		"live_ref.owner_type='task_asset'",
		"frozen_ref.owner_type='task_asset'",
	} {
		if !strings.Contains(currentPointerAssetReferencesSQL, fragment) {
			t.Fatalf("current pointer query does not cover %q", fragment)
		}
	}
}

func TestCurrentPointerAssetReferenceQueryAvoidsOptimizerHostileORJoins(t *testing.T) {
	for _, fragment := range []string{
		"ON r.id=g.working_revision_id OR r.id=g.finalized_revision_id",
		"ON lineage.id=rr.formal_task_asset_id\n\t\t          OR",
		"WHERE seed.id=?\n\t\t     OR",
	} {
		if strings.Contains(currentPointerAssetReferencesSQL, fragment) {
			t.Fatalf("current pointer query retained optimizer-hostile fragment %q", fragment)
		}
	}
	if count := strings.Count(currentPointerAssetReferencesSQL, "SELECT current.revision_id"); count != 8 {
		t.Fatalf("current pointer query has %d explicit reference paths; want 8", count)
	}
}

func TestHistoricalUnavailablePostApplyEvidenceAcceptsOnlyTombstoneStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	recovery := historicalUnavailableRecoveryFixture()
	expectHistoricalUnavailableRecoveryEvidence(mock, recovery, 0, "historical_unavailable")
	if err := validateHistoricalUnavailableRecoveryEvidence(
		context.Background(), db, recovery, "historical_unavailable",
	); err != nil {
		t.Fatalf("validateHistoricalUnavailableRecoveryEvidence() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestHistoricalUnavailableRecoveryRejectsUnexpectedMigrationAlias(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	recovery := historicalUnavailableRecoveryFixture()
	mock.ExpectQuery("SELECT id,task_id,asset_id,file_size,COALESCE\\(storage_ref_id,''\\)").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "asset_id", "file_size", "storage_ref_id",
			"superseded_by_version_id", "upload_status", "deleted_at", "cleaned_at", "object_deleted_at",
		}).
			AddRow(int64(12323), int64(2199), int64(12401), int64(17755216), recovery.OriginalStorageRefID, int64(14510), "uploaded", nil, nil, nil).
			AddRow(int64(14510), int64(2199), int64(12401), int64(17595421), "58aebabe-355c-4d24-814a-d6dca306b73d", int64(14514), "uploaded", nil, nil, nil).
			AddRow(int64(14514), int64(2199), int64(12401), int64(11275123), "6e6cd051-f261-424d-8b55-49dd6868be9a", nil, "uploaded", nil, nil, nil))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM task_assets.*WHERE task_id=\\? AND asset_id=\\?").
		WithArgs(int64(2199), int64(12401)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	expectHistoricalUnavailableSourceAliases(mock, []int64{99999})
	if err := validateHistoricalUnavailableRecoveryEvidence(
		context.Background(), db, recovery, "recorded",
	); err == nil || !strings.Contains(err.Error(), "outside the frozen") {
		t.Fatalf("unexpected migration alias error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyHistoricalUnavailableRecoveryRejectsLineageDrift(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery := historicalUnavailableRecoveryFixture()
	expectAssetStorageRefStatus(mock, recovery, "recorded")
	mock.ExpectQuery("SELECT id,task_id,asset_id,file_size,COALESCE\\(storage_ref_id,''\\)").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "asset_id", "file_size", "storage_ref_id",
			"superseded_by_version_id", "upload_status", "deleted_at", "cleaned_at", "object_deleted_at",
		}).
			AddRow(int64(12323), int64(2199), int64(12401), int64(17755216), recovery.OriginalStorageRefID, int64(14514), "uploaded", nil, nil, nil).
			AddRow(int64(14510), int64(2199), int64(12401), int64(17595421), "58aebabe-355c-4d24-814a-d6dca306b73d", int64(14514), "uploaded", nil, nil, nil).
			AddRow(int64(14514), int64(2199), int64(12401), int64(11275123), "6e6cd051-f261-424d-8b55-49dd6868be9a", nil, "uploaded", nil, nil, nil))
	if err := applyAssetRecoveries(context.Background(), tx, []assetRecoveryMapping{recovery}); err == nil || !strings.Contains(err.Error(), "lineage row 12323") {
		t.Fatalf("applyAssetRecoveries() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyHistoricalUnavailableRecoveryRejectsStorageRefIdentityDrift(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	recovery := historicalUnavailableRecoveryFixture()
	expectAssetStorageRefStatus(mock, recovery, "recorded")
	mock.ExpectQuery("SELECT id,task_id,asset_id,file_size,COALESCE\\(storage_ref_id,''\\)").
		WillReturnRows(sqlmock.NewRows([]string{
			"id", "task_id", "asset_id", "file_size", "storage_ref_id",
			"superseded_by_version_id", "upload_status", "deleted_at", "cleaned_at", "object_deleted_at",
		}).
			AddRow(int64(12323), int64(2199), int64(12401), int64(17755216), recovery.OriginalStorageRefID, int64(14510), "uploaded", nil, nil, nil).
			AddRow(int64(14510), int64(2199), int64(12401), int64(17595421), "58aebabe-355c-4d24-814a-d6dca306b73d", int64(14514), "uploaded", nil, nil, nil).
			AddRow(int64(14514), int64(2199), int64(12401), int64(11275123), "6e6cd051-f261-424d-8b55-49dd6868be9a", nil, "uploaded", nil, nil, nil))
	mock.ExpectQuery("SELECT COUNT\\(\\*\\).*FROM task_assets.*WHERE task_id=\\? AND asset_id=\\?").
		WithArgs(int64(2199), int64(12401)).
		WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(3))
	expectHistoricalUnavailableSourceAliases(mock, nil)
	mock.ExpectQuery("SELECT asset_id,owner_type,owner_id,ref_key,status,file_size").
		WithArgs(recovery.OriginalStorageRefID).
		WillReturnRows(sqlmock.NewRows([]string{"asset_id", "owner_type", "owner_id", "ref_key", "status", "file_size"}).
			AddRow(int64(12323), "task_asset", int64(12323), "wrong/key.psd", "recorded", int64(17755216)))
	if err := applyAssetRecoveries(context.Background(), tx, []assetRecoveryMapping{recovery}); err == nil || !strings.Contains(err.Error(), "storage ref identity") {
		t.Fatalf("applyAssetRecoveries() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRestoreAssetStorageRefStatesRestoresExactStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	mock.ExpectExec("UPDATE asset_storage_refs SET status=\\? WHERE ref_id=\\?").
		WithArgs("recorded", "ref-original").
		WillReturnResult(sqlmock.NewResult(0, 1))
	if err := restoreAssetStorageRefStates(context.Background(), tx, []assetStorageRefStatusSnapshot{{
		RefID:  "ref-original",
		Status: "recorded",
	}}); err != nil {
		t.Fatalf("restoreAssetStorageRefStates() error = %v", err)
	}
	mock.ExpectRollback()
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func historicalUnavailableRecoveryFixture() assetRecoveryMapping {
	recovery := assetRecoveryMapping{
		TaskID:                     2199,
		MissingTaskAssetID:         12323,
		RejectedSourceTaskAssetIDs: []int64{14510, 14514},
		Strategy:                   "historical_unavailable_tombstone_v1",
		OriginalStorageRefID:       "c0a135a1-080f-46a0-a41a-461aef0ea0fb",
		ExpectedFileSize:           17755216,
		PreviewWholeHash:           "82b35a045540d27f9656d6d02c99eb2814a62e9d048d33b20823fb8c0017aa4c",
		DesignThumbWholeHash:       "54dbf569874243a212c11c3e83e80f19944c2581f12c9473a793bc273ec666a3",
		ObjectProbeResult:          "not_found",
		ObjectProbeInputSHA256:     "3f17b37296d2670235ca9bfcfd4388823b81adecf8fbac0826e6f241923579c7",
		ObjectProbeEvidenceHash:    "f1c78819e1f3d5f4e7a4b25ff3d173368574a5639f4c6df45c8aae5482d047b8",
		ObjectProbeObjectKeySHA256: "e732f6cd269a93d6bac168b0852dbcf8480af8966847278cb073cd6905b0efdd",
		ObjectProbeReadOnlyGETs:    1,
	}
	recovery.ManifestRowHash, _ = assetRecoveryManifestRowHash(recovery)
	return recovery
}
