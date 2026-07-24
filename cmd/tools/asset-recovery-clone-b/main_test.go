package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
)

func testEntry(t *testing.T, root string) recoveryEntry {
	t.Helper()
	body := []byte("controlled clone-b bytes")
	digest := sha256.Sum256(body)
	wholeHash := hex.EncodeToString(digest[:])
	objectKey := "v8-ab/run-test/recovered/task-2807/task-asset-23989/object.bin"
	objectPath, err := containedFixturePath(root, objectKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectPath, body, 0o600); err != nil {
		t.Fatal(err)
	}

	beforeTask := map[string]any{
		"id":                    float64(23989),
		"task_id":               float64(2807),
		"asset_id":              float64(24989),
		"upload_request_id":     "request-23989",
		"storage_ref_id":        "missing-ref-23989",
		"storage_key":           "deleted/23989",
		"whole_hash":            nil,
		"upload_status":         "uploaded",
		"deleted_at":            "2026-07-22T00:00:00Z",
		"cleaned_at":            nil,
		"object_deleted_at":     nil,
		"access_revoked_at":     nil,
		"access_revoked_reason": "",
	}
	beforeUpload := map[string]any{
		"request_id":     "request-23989",
		"bound_ref_id":   "missing-ref-23989",
		"checksum_hint":  "",
		"file_size":      float64(len(body)),
		"status":         "bound",
		"session_status": "completed",
	}
	targetRef := map[string]any{
		"ref_id":            "target-ref-23989",
		"asset_id":          float64(24989),
		"owner_type":        "task_asset",
		"owner_id":          float64(23989),
		"upload_request_id": "request-23989",
		"storage_adapter":   "local",
		"ref_type":          "task_asset_object",
		"ref_key":           objectKey,
		"file_name":         "23989.jpg",
		"mime_type":         "image/jpeg",
		"file_size":         float64(len(body)),
		"is_placeholder":    float64(0),
		"checksum_hint":     wholeHash,
		"status":            "recorded",
	}
	entry := recoveryEntry{
		MissingTaskAssetID: 23989,
		SourceTaskAssetID:  24034,
		SourceSHA256:       wholeHash,
		SourceSize:         int64(len(body)),
		TargetStorageRefID: "target-ref-23989",
		TargetObjectKey:    objectKey,
	}
	entry.DBApplyPlan.InsertStorageRef = targetRef
	entry.DBApplyPlan.UpdateTaskAsset = rowMutation{
		Where: map[string]any{"id": float64(23989)},
		Set: map[string]any{
			"storage_ref_id":        "target-ref-23989",
			"storage_key":           objectKey,
			"whole_hash":            wholeHash,
			"upload_status":         "uploaded",
			"deleted_at":            nil,
			"cleaned_at":            nil,
			"object_deleted_at":     nil,
			"access_revoked_at":     nil,
			"access_revoked_reason": "",
		},
	}
	entry.DBApplyPlan.UpdateUpload = rowMutation{
		Where: map[string]any{"request_id": "request-23989"},
		Set: map[string]any{
			"bound_ref_id":   "target-ref-23989",
			"checksum_hint":  wholeHash,
			"file_size":      float64(len(body)),
			"status":         "bound",
			"session_status": "completed",
		},
	}
	entry.Rollback.RestoreTaskAsset = beforeTask
	entry.Rollback.RestoreUpload = beforeUpload
	entry.Rollback.OriginalStorageRef = map[string]any{
		"ref_id": "missing-ref-23989",
		"status": "recorded",
	}
	entry.Rollback.DeleteStorageRefID = entry.TargetStorageRefID
	entry.Rollback.DeleteObjectKey = objectKey
	entry.Rollback.ExpectedObjectSHA = wholeHash
	entry.Rollback.DBRollbackPlan.RestoreTaskAsset = rowMutation{
		Where: entry.DBApplyPlan.UpdateTaskAsset.Where,
		Set:   selectFields(beforeTask, taskAssetFields()),
	}
	entry.Rollback.DBRollbackPlan.RestoreUpload = rowMutation{
		Where: entry.DBApplyPlan.UpdateUpload.Where,
		Set:   selectFields(beforeUpload, uploadFields()),
	}
	entry.Rollback.DBRollbackPlan.DeleteStorageRef = rowMutation{
		Where: map[string]any{"ref_id": entry.TargetStorageRefID},
	}
	return entry
}

func testExactEntry(t *testing.T, root string, missingID int64, exact exactRecovery) recoveryEntry {
	t.Helper()
	entry := testEntry(t, filepath.Join(root, fmt.Sprintf("seed-%d", missingID)))
	body := make([]byte, exact.size)
	for index := range body {
		body[index] = byte((index + int(missingID)) % 251)
	}
	digest := sha256.Sum256(body)
	wholeHash := hex.EncodeToString(digest[:])
	targetRef := fmt.Sprintf("target-ref-%d", missingID)
	objectKey := "v8-ab/run-test/recovered/task-2807/task-asset-" + fmt.Sprintf("%d", missingID) + "/object.bin"
	objectPath, err := containedFixturePath(root, objectKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(objectPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(objectPath, body, 0o600); err != nil {
		t.Fatal(err)
	}
	entry.MissingTaskAssetID = missingID
	entry.SourceTaskAssetID = exact.sourceID
	entry.SourceSize = exact.size
	entry.SourceSHA256 = wholeHash
	entry.TargetStorageRefID = targetRef
	entry.TargetObjectKey = objectKey
	entry.DBApplyPlan.UpdateTaskAsset.Where = map[string]any{"id": float64(missingID)}
	entry.Rollback.DBRollbackPlan.RestoreTaskAsset.Where = map[string]any{"id": float64(missingID)}
	entry.DBApplyPlan.UpdateTaskAsset.Set["storage_ref_id"] = targetRef
	entry.DBApplyPlan.UpdateTaskAsset.Set["storage_key"] = objectKey
	entry.DBApplyPlan.UpdateTaskAsset.Set["whole_hash"] = wholeHash
	entry.DBApplyPlan.UpdateUpload.Set["bound_ref_id"] = targetRef
	entry.DBApplyPlan.UpdateUpload.Set["checksum_hint"] = wholeHash
	entry.DBApplyPlan.UpdateUpload.Set["file_size"] = float64(exact.size)
	entry.DBApplyPlan.InsertStorageRef = cloneMap(entry.DBApplyPlan.InsertStorageRef)
	entry.DBApplyPlan.InsertStorageRef["ref_id"] = targetRef
	entry.DBApplyPlan.InsertStorageRef["owner_id"] = float64(missingID)
	entry.DBApplyPlan.InsertStorageRef["ref_key"] = objectKey
	entry.DBApplyPlan.InsertStorageRef["file_size"] = float64(exact.size)
	entry.DBApplyPlan.InsertStorageRef["checksum_hint"] = wholeHash
	entry.Rollback.DeleteStorageRefID = targetRef
	entry.Rollback.DeleteObjectKey = objectKey
	entry.Rollback.ExpectedObjectSHA = wholeHash
	entry.Rollback.DBRollbackPlan.DeleteStorageRef.Where = map[string]any{"ref_id": targetRef}
	return entry
}

func jsonRow(t *testing.T, value map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func expectLockedState(t *testing.T, mock sqlmock.Sqlmock, entry recoveryEntry, state lockedState) {
	t.Helper()
	mock.ExpectQuery(regexp.QuoteMeta(taskAssetStateSQL)).
		WithArgs(entry.MissingTaskAssetID).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(jsonRow(t, state.taskAsset)))
	mock.ExpectQuery(regexp.QuoteMeta(uploadStateSQL)).
		WithArgs(entry.DBApplyPlan.UpdateUpload.Where["request_id"]).
		WillReturnRows(sqlmock.NewRows([]string{"state"}).AddRow(jsonRow(t, state.uploadRequest)))
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT status FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`)).
		WithArgs(entry.Rollback.OriginalStorageRef["ref_id"]).
		WillReturnRows(sqlmock.NewRows([]string{"status"}).AddRow(state.originalStatus))
	targetRows := sqlmock.NewRows([]string{"state"})
	if state.targetStorageRef != nil {
		targetRows.AddRow(jsonRow(t, state.targetStorageRef))
	}
	mock.ExpectQuery(regexp.QuoteMeta(storageRefStateSQL)).
		WithArgs(entry.TargetStorageRefID).
		WillReturnRows(targetRows)
}

func TestValidateOptionsRejectsProductionAndUnmarkedDatabase(t *testing.T) {
	base := options{
		Mode:            "apply",
		PlanFile:        "plan.json",
		FixtureRoot:     "fixture",
		ReportFile:      "report.json",
		ConfirmRunID:    "run-test",
		ConfirmHost:     "127.0.0.1",
		ConfirmDatabase: "workflow_clone_b",
	}
	base.DSN = "user:pass@tcp(223.4.249.11:3306)/workflow_clone_b"
	if _, _, err := validateOptions(base); err == nil || !regexp.MustCompile("non-loopback").MatchString(err.Error()) {
		t.Fatalf("expected non-loopback rejection, got %v", err)
	}
	base.DSN = "user:pass@tcp(127.0.0.1:3306)/workflow"
	base.ConfirmDatabase = "workflow"
	if _, _, err := validateOptions(base); err == nil || !regexp.MustCompile("unmarked database").MatchString(err.Error()) {
		t.Fatalf("expected unmarked DB rejection, got %v", err)
	}
	base.DSN = "user:pass@tcp(127.0.0.1:3312)/ab_r20260723_01_b"
	base.ConfirmDatabase = "ab_r20260723_01_b"
	if _, _, err := validateOptions(base); err != nil {
		t.Fatalf("formal ab_*_b database should be accepted: %v", err)
	}
}

func TestValidatePlanRequiresMaterializedExactFixture(t *testing.T) {
	root := t.TempDir()
	entry := testEntry(t, root)
	plan := recoveryPlan{
		Version: planVersion,
		Status:  "MATERIALIZED",
		RunID:   "run-test",
		Entries: []recoveryEntry{entry, entry, entry},
	}
	if err := validatePlan(plan, "run-test", root); err == nil {
		t.Fatal("expected duplicate recovery rejection")
	}
	plan.Entries = nil
	for missingID, exact := range exactRecoveries {
		plan.Entries = append(plan.Entries, testExactEntry(t, root, missingID, exact))
	}
	if err := validatePlan(plan, "run-test", root); err != nil {
		t.Fatalf("valid materialized plan rejected: %v", err)
	}
	plan.Status = "PREPARED"
	if err := validatePlan(plan, "run-test", root); err == nil {
		t.Fatal("expected PREPARED plan rejection")
	}
}

func TestApplyEntryExactAndIdempotent(t *testing.T) {
	entry := testEntry(t, t.TempDir())
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
	expectLockedState(t, mock, entry, expectedBefore(entry))
	mock.ExpectExec("INSERT INTO asset_storage_refs").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE task_assets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upload_requests").
		WillReturnResult(sqlmock.NewResult(0, 1))
	changed, err := applyEntry(context.Background(), tx, entry)
	if err != nil || !changed {
		t.Fatalf("apply failed: changed=%v err=%v", changed, err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}

	db2, mock2, _ := sqlmock.New()
	defer db2.Close()
	mock2.ExpectBegin()
	tx2, _ := db2.Begin()
	expectLockedState(t, mock2, entry, expectedAfter(entry))
	changed, err = applyEntry(context.Background(), tx2, entry)
	if err != nil || changed {
		t.Fatalf("idempotent apply failed: changed=%v err=%v", changed, err)
	}
	mock2.ExpectRollback()
	if err := tx2.Rollback(); err != nil {
		t.Fatal(err)
	}
	if err := mock2.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestApplyRejectsBeforeStateDriftWithoutWrites(t *testing.T) {
	entry := testEntry(t, t.TempDir())
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectBegin()
	tx, _ := db.Begin()
	drift := expectedBefore(entry)
	drift.taskAsset = cloneMap(drift.taskAsset)
	drift.taskAsset["storage_key"] = "unexpected"
	expectLockedState(t, mock, entry, drift)
	if changed, err := applyEntry(context.Background(), tx, entry); err == nil || changed {
		t.Fatalf("expected drift rejection, got changed=%v err=%v", changed, err)
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestRollbackRestoresRowsThenDeletesCreatedRef(t *testing.T) {
	entry := testEntry(t, t.TempDir())
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectBegin()
	tx, _ := db.Begin()
	expectLockedState(t, mock, entry, expectedAfter(entry))
	mock.ExpectExec("UPDATE task_assets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upload_requests").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(regexp.QuoteMeta(`DELETE FROM asset_storage_refs WHERE ref_id=?`)).
		WithArgs(entry.TargetStorageRefID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	changed, err := rollbackEntry(context.Background(), tx, entry)
	if err != nil || !changed {
		t.Fatalf("rollback failed: changed=%v err=%v", changed, err)
	}
	mock.ExpectCommit()
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCloneGuardRequiresExactBinding(t *testing.T) {
	db, mock, _ := sqlmock.New()
	defer db.Close()
	mock.ExpectBegin()
	tx, _ := db.Begin()
	mock.ExpectQuery("SELECT environment,run_id,plan_sha256").
		WillReturnRows(sqlmock.NewRows([]string{"environment", "run_id", "plan_sha256"}).
			AddRow("clone_b", "other-run", "plan-sha"))
	if err := validateCloneGuard(context.Background(), tx, "run-test", "plan-sha"); err == nil {
		t.Fatal("expected mismatched guard rejection")
	}
	mock.ExpectRollback()
	_ = tx.Rollback()
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func TestCommittedApplyReportFailureCompensationRestoresDatabase(t *testing.T) {
	entry := testEntry(t, t.TempDir())
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery("SELECT environment,run_id,plan_sha256").
		WillReturnRows(
			sqlmock.NewRows(
				[]string{"environment", "run_id", "plan_sha256"},
			).AddRow("clone_b", "run-test", "plan-sha"),
		)
	expectLockedState(t, mock, entry, expectedAfter(entry))
	mock.ExpectExec("UPDATE task_assets").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec("UPDATE upload_requests").
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(
		regexp.QuoteMeta(
			`DELETE FROM asset_storage_refs WHERE ref_id=?`,
		),
	).WithArgs(entry.TargetStorageRefID).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	if err := compensateCommittedApply(
		context.Background(),
		db,
		"run-test",
		"plan-sha",
		[]recoveryEntry{entry},
	); err != nil {
		t.Fatalf("compensateCommittedApply() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatal(err)
	}
}

func cloneMap(value map[string]any) map[string]any {
	result := make(map[string]any, len(value))
	for key, item := range value {
		result[key] = item
	}
	return result
}
