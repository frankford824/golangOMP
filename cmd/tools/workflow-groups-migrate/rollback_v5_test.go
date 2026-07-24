package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func validAutoIncrementStates(next int64) []autoIncrementSnapshot {
	states := make([]autoIncrementSnapshot, 0, len(workflowGroupsAutoIncrementTables))
	for _, table := range workflowGroupsAutoIncrementTables {
		states = append(states, autoIncrementSnapshot{Table: table, NextValue: next})
	}
	return states
}

func TestAssetBindingSnapshotScopeRoundTripPreservesNullAndEmpty(t *testing.T) {
	tests := []struct {
		name  string
		scope *string
	}{
		{name: "null"},
		{name: "empty", scope: func() *string { value := ""; return &value }()},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(assetBindingSnapshot{ID: 1, ScopeSKUCode: tt.scope})
			if err != nil {
				t.Fatal(err)
			}
			var decoded assetBindingSnapshot
			if err := json.Unmarshal(raw, &decoded); err != nil {
				t.Fatal(err)
			}
			if (decoded.ScopeSKUCode == nil) != (tt.scope == nil) {
				t.Fatalf("scope nullability changed: %s", raw)
			}
			if decoded.ScopeSKUCode != nil && *decoded.ScopeSKUCode != "" {
				t.Fatalf("scope value = %q", *decoded.ScopeSKUCode)
			}
		})
	}
}

func TestValidateAutoIncrementStatesRequiresExactAllowlist(t *testing.T) {
	if err := validateAutoIncrementStates(validAutoIncrementStates(10)); err != nil {
		t.Fatalf("valid states: %v", err)
	}
	wrongOrder := validAutoIncrementStates(10)
	wrongOrder[0], wrongOrder[1] = wrongOrder[1], wrongOrder[0]
	if err := validateAutoIncrementStates(wrongOrder); err == nil || !strings.Contains(err.Error(), "expected") {
		t.Fatalf("wrong order error = %v", err)
	}
	invalidValue := validAutoIncrementStates(10)
	invalidValue[0].NextValue = 0
	if err := validateAutoIncrementStates(invalidValue); err == nil || !strings.Contains(err.Error(), "invalid") {
		t.Fatalf("invalid value error = %v", err)
	}
}

func TestRestoreAutoIncrementStatesIsIdempotentAtBeforeState(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	before := validAutoIncrementStates(100)
	after := validAutoIncrementStates(200)
	for _, state := range before {
		mock.ExpectQuery("SELECT @@SESSION.information_schema_stats_expiry,").
			WithArgs(state.Table).
			WillReturnRows(sqlmock.NewRows([]string{
				"information_schema_stats_expiry",
				"auto_increment_increment",
				"auto_increment_offset",
				"AUTO_INCREMENT",
			}).AddRow(0, 1, 1, state.NextValue))
	}
	if err := restoreAutoIncrementStates(context.Background(), db, before, after); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestAutoIncrementRecoveryCeilingsBoundPotentialWrites(t *testing.T) {
	aliasOrigin := int64(99)
	mapping := mappingFile{
		Resources: []resourceMapping{
			{
				V2Declared: true,
				History: []resourceRevisionMapping{
					{
						SourceAliasFrom: &aliasOrigin,
						FinalAssetIDs:   []int64{1, 2},
						ReferenceIDs:    []int64{10},
					},
					{
						FinalAssetIDs: []int64{3},
						ReferenceIDs:  []int64{11, 12},
					},
				},
			},
		},
		Planning: []planningMapping{{Items: []planningItemMapping{{}, {}}}},
	}
	ceilings, err := autoIncrementRecoveryCeilings(validAutoIncrementStates(100), mapping)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, state := range ceilings {
		got[state.Table] = state.NextValue
	}
	want := map[string]int64{
		"task_assets":                          101,
		"task_asset_groups":                    101,
		"task_asset_group_revisions":           102,
		"task_asset_group_revision_items":      103,
		"task_asset_group_revision_references": 103,
		"task_planning_sku_revisions":          102,
	}
	for table, next := range want {
		if got[table] != next {
			t.Fatalf("%s ceiling=%d want=%d", table, got[table], next)
		}
	}
}

func TestNeedsPreCommitAutoIncrementRecovery(t *testing.T) {
	tests := []struct {
		name          string
		state         string
		rowsMatch     bool
		countersMatch bool
		want          bool
	}{
		{name: "prepared drift", state: "prepared", rowsMatch: true, want: true},
		{name: "commit pending drift", state: "commit_pending", rowsMatch: true, want: true},
		{name: "prepared clean", state: "prepared", rowsMatch: true, countersMatch: true},
		{name: "rows differ", state: "prepared"},
		{name: "applied", state: "applied", rowsMatch: true},
		{name: "rollback dml", state: "rollback_dml_pending", rowsMatch: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := needsPreCommitAutoIncrementRecovery(
				tt.state,
				tt.rowsMatch,
				tt.countersMatch,
			)
			if got != tt.want {
				t.Fatalf("got=%v want=%v", got, tt.want)
			}
		})
	}
}

func TestRollbackRejectsV4BeforeDatabaseAccess(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mapping := mappingFile{}
	dir := t.TempDir()
	legacyRaw := []byte(`{
  "version": 4,
  "tool_version": "workflow-groups-migrate/v8.4",
  "schema_version": "124-126",
  "database": "unit_test",
  "mapping_sha256": "historical-v4-shape",
  "integrity_sha256": "historical-v4-integrity",
  "apply_state": "applied",
  "created_at": "2026-07-24T08:00:00Z",
  "tasks_before": [],
  "tasks_after": [],
  "resource_groups_before": [],
  "resource_groups_after": [],
  "asset_bindings_before": [{"id":1,"scope_sku_code":""}],
  "asset_bindings_after": [],
  "sku_origins_before": [],
  "sku_origins_after": [],
  "planning_before": [],
  "planning_after": [],
  "planning_created": [],
  "inserted_group_ids": [],
  "applied_revision_ids": []
}`)
	if err := os.WriteFile(filepath.Join(dir, "workflow-groups-snapshot.json"), legacyRaw, 0o600); err != nil {
		t.Fatal(err)
	}
	err = rollback(
		context.Background(),
		db,
		"unit_test",
		options{SnapshotDir: dir},
		mapping,
	)
	if err == nil || !strings.Contains(err.Error(), "snapshot v4") {
		t.Fatalf("rollback error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestPlanningDetailSnapshotPreservesUpdatedAt(t *testing.T) {
	expected := time.Date(2026, 7, 24, 8, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(planningDetailSnapshot{
		TaskSKUItemID: 1,
		UpdatedAt:     expected,
	})
	if err != nil {
		t.Fatal(err)
	}
	var decoded planningDetailSnapshot
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatal(err)
	}
	if !decoded.UpdatedAt.Equal(expected) {
		t.Fatalf("updated_at=%s want=%s", decoded.UpdatedAt, expected)
	}
}

func TestAutoIncrementCrashRecoveryMySQL(t *testing.T) {
	dsn := os.Getenv("WORKFLOW_GROUPS_MIGRATE_MYSQL_TEST_DSN")
	if dsn == "" {
		t.Skip("WORKFLOW_GROUPS_MIGRATE_MYSQL_TEST_DSN is not set")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Errorf("close database: %v", err)
		}
	})
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if _, err := db.Exec("SET SESSION information_schema_stats_expiry = 0"); err != nil {
		t.Fatal(err)
	}
	var database string
	if err := db.QueryRow("SELECT DATABASE()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(database, "ab_workflow_groups_migrate_test_") {
		t.Fatalf("refusing destructive integration test against database %q", database)
	}
	var existingTables int
	if err := db.QueryRow(
		"SELECT COUNT(*) FROM information_schema.TABLES WHERE TABLE_SCHEMA=DATABASE()",
	).Scan(&existingTables); err != nil {
		t.Fatal(err)
	}
	if existingTables != 0 {
		t.Fatalf("refusing non-empty integration test database %q", database)
	}
	t.Cleanup(func() {
		for _, table := range workflowGroupsAutoIncrementTables {
			statement := fmt.Sprintf("DROP TABLE IF EXISTS `%s`", table)
			if _, err := db.Exec(statement); err != nil {
				t.Errorf("cleanup %s: %v", table, err)
			}
		}
	})
	for _, table := range workflowGroupsAutoIncrementTables {
		statement := fmt.Sprintf("CREATE TABLE `%s` (id BIGINT NOT NULL AUTO_INCREMENT PRIMARY KEY)", table)
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
		statement = fmt.Sprintf("INSERT INTO `%s` VALUES (NULL)", table)
		if _, err := db.Exec(statement); err != nil {
			t.Fatal(err)
		}
	}
	before, err := captureAutoIncrementStates(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	for _, table := range workflowGroupsAutoIncrementTables {
		statement := fmt.Sprintf("INSERT INTO `%s` VALUES (NULL)", table)
		if _, err := tx.Exec(statement); err != nil {
			_ = tx.Rollback()
			t.Fatal(err)
		}
	}
	if err := tx.Rollback(); err != nil {
		t.Fatal(err)
	}
	drifted, err := captureAutoIncrementStates(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	if reflect.DeepEqual(before, drifted) {
		t.Fatal("rolled-back inserts did not exercise auto-increment drift")
	}
	ceilings := make([]autoIncrementSnapshot, len(before))
	for index, state := range before {
		ceilings[index] = autoIncrementSnapshot{
			Table:     state.Table,
			NextValue: state.NextValue + 1,
		}
	}
	if err := restoreAutoIncrementStatesWithinRecoveryCeilings(
		context.Background(),
		db,
		before,
		ceilings,
	); err != nil {
		t.Fatal(err)
	}
	matches, err := autoIncrementStatesMatch(context.Background(), db, before)
	if err != nil {
		t.Fatal(err)
	}
	if !matches {
		t.Fatal("auto-increment counters did not return to baseline")
	}
}

var _ snapshotQueryer = (*sql.DB)(nil)
