package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
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
	mock.ExpectQuery("SELECT subject_type,subject_id,reason").WillReturnRows(sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}))
	mock.ExpectQuery("SELECT ur.user_id,ur.role").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
	mock.ExpectQuery("SELECT id,task_type,task_status").WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status"}))
	mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}))
	for i := 0; i < 5; i++ {
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
	mock.ExpectQuery("SELECT subject_type,subject_id,reason").WillReturnRows(sqlmock.NewRows([]string{"subject_type", "subject_id", "reason"}))
	mock.ExpectQuery("SELECT ur.user_id,ur.role").WillReturnRows(sqlmock.NewRows([]string{"user_id", "role"}))
	mock.ExpectQuery("SELECT id,task_type,task_status").WillReturnRows(sqlmock.NewRows([]string{"id", "task_type", "task_status"}))
	mock.ExpectQuery("SELECT id,task_id,scope_kind,scope_ref_id").WillReturnRows(sqlmock.NewRows([]string{"id", "task_id", "scope_kind", "scope_ref_id"}))
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
