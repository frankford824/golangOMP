package main

import (
	"context"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

func TestMigratorHappyPath(t *testing.T) {
	store := newFakeMigrationStore([]migrationRow{dirtyTestRow()})
	oss := &fakeMigrationOSS{existing: map[string]bool{dirtyTestRow().StorageKey: true}}
	var events []migrationEvent
	m := &migrator{
		store:    store,
		oss:      oss,
		dryRun:   false,
		logEvent: func(event migrationEvent) { events = append(events, event) },
	}

	summary, err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Scanned != 1 || summary.Migrated != 1 || summary.Errors != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	row := store.rows[1]
	if !asciiSafeKeyPattern.MatchString(row.StorageKey) {
		t.Fatalf("storage key was not migrated to ASCII key: %q", row.StorageKey)
	}
	if oss.copyCalls != 1 {
		t.Fatalf("copyCalls = %d, want 1", oss.copyCalls)
	}
	if len(events) != 1 || events[0].Status != "migrated" {
		t.Fatalf("events = %+v", events)
	}
}

func TestMigratorOrphanLeavesDBUntouched(t *testing.T) {
	row := dirtyTestRow()
	store := newFakeMigrationStore([]migrationRow{row})
	oss := &fakeMigrationOSS{existing: map[string]bool{}}
	m := &migrator{store: store, oss: oss, dryRun: false}

	summary, err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.OrphanNoOSSObject != 1 || summary.Migrated != 0 || summary.Errors != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if store.rows[row.ID].StorageKey != row.StorageKey {
		t.Fatalf("storage key changed for orphan: %q", store.rows[row.ID].StorageKey)
	}
	if oss.copyCalls != 0 {
		t.Fatalf("copyCalls = %d, want 0", oss.copyCalls)
	}
}

func TestMigratorIdempotentAlreadyASCII(t *testing.T) {
	row := dirtyTestRow()
	row.StorageKey = "tasks/RW-1/assets/AST-0001/v1/source/migrated_abc123def4567890.psd"
	store := newFakeMigrationStore([]migrationRow{row})
	oss := &fakeMigrationOSS{existing: map[string]bool{row.StorageKey: true}}
	m := &migrator{store: store, oss: oss, dryRun: false}

	summary, err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.SkippedAlreadyASCII != 1 || summary.Migrated != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if oss.headCalls != 0 || oss.copyCalls != 0 {
		t.Fatalf("oss calls head=%d copy=%d, want 0", oss.headCalls, oss.copyCalls)
	}
}

func TestMigratorDryRunPlansWithoutCopyOrDBWrite(t *testing.T) {
	row := dirtyTestRow()
	store := newFakeMigrationStore([]migrationRow{row})
	oss := &fakeMigrationOSS{existing: map[string]bool{row.StorageKey: true}}
	var events []migrationEvent
	m := &migrator{
		store:    store,
		oss:      oss,
		dryRun:   true,
		logEvent: func(event migrationEvent) { events = append(events, event) },
	}

	summary, err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Planned != 1 || summary.Migrated != 0 || summary.Errors != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if store.rows[row.ID].StorageKey != row.StorageKey {
		t.Fatalf("storage key changed in dry-run: %q", store.rows[row.ID].StorageKey)
	}
	if oss.copyCalls != 0 {
		t.Fatalf("copyCalls = %d, want 0", oss.copyCalls)
	}
	if len(events) != 1 || events[0].Status != "planned_migrate" || events[0].NewKey == "" {
		t.Fatalf("events = %+v", events)
	}
}

func TestUpdateSQLMatchesSchema(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("sqlmock.New() error = %v", err)
	}
	defer db.Close()

	mock.ExpectBegin()
	tx, err := db.BeginTx(context.Background(), nil)
	if err != nil {
		t.Fatalf("BeginTx() error = %v", err)
	}
	row := dirtyTestRow()
	locked := &mysqlLockedRow{tx: tx, row: row}
	newKey := "tasks/RW-1/assets/AST-0001/v1/source/migrated_fixed.psd"
	mock.ExpectExec(regexp.QuoteMeta("UPDATE task_assets SET storage_key = ? WHERE id = ? AND storage_key = ?")).
		WithArgs(newKey, row.ID, row.StorageKey).
		WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectRollback()

	updated, err := locked.UpdateStorageKey(context.Background(), row.StorageKey, newKey)
	if err != nil {
		t.Fatalf("UpdateStorageKey() error = %v", err)
	}
	if !updated {
		t.Fatal("UpdateStorageKey() updated = false, want true")
	}
	if err := locked.Rollback(); err != nil {
		t.Fatalf("Rollback() error = %v", err)
	}
	if err := mock.ExpectationsWereMet(); err != nil {
		t.Fatalf("SQL expectations were not met; UPDATE must not reference updated_at: %v", err)
	}
}

func TestIdempotentReRunAfterPartialFailure(t *testing.T) {
	row := dirtyTestRow()
	row.ID = 188
	newKey := deterministicASCIIKey(row)
	store := newFakeMigrationStore([]migrationRow{row})
	oss := &fakeMigrationOSS{existing: map[string]bool{
		row.StorageKey: true,
		newKey:         true,
	}}
	var events []migrationEvent
	m := &migrator{
		store:    store,
		oss:      oss,
		dryRun:   false,
		logEvent: func(event migrationEvent) { events = append(events, event) },
	}

	summary, err := m.Run(context.Background())
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if summary.Migrated != 1 || summary.Errors != 0 {
		t.Fatalf("summary = %+v", summary)
	}
	if got := store.rows[row.ID].StorageKey; got != newKey {
		t.Fatalf("storage key = %q, want %q", got, newKey)
	}
	if oss.copyCalls != 1 || oss.lastCopySrc != row.StorageKey || oss.lastCopyDst != newKey {
		t.Fatalf("copy src=%q dst=%q calls=%d, want overwrite copy to deterministic key", oss.lastCopySrc, oss.lastCopyDst, oss.copyCalls)
	}
	if len(events) != 1 || events[0].Status != "migrated" || events[0].MigratedAt == "" {
		t.Fatalf("events = %+v", events)
	}
}

func dirtyTestRow() migrationRow {
	return migrationRow{
		ID:               1,
		TaskID:           1001,
		TaskRef:          "RW-20260420-A-000001",
		AssetNo:          "AST-0001",
		VersionNo:        1,
		AssetType:        "source",
		StorageKey:       "tasks/RW-20260420-A-000001/assets/AST-0001/v1/source/【4条装】毕业手持横幅组合A.psd",
		OriginalFilename: "【4条装】毕业手持横幅组合A.psd",
		CreatedAt:        time.Date(2026, 4, 20, 1, 0, 0, 0, time.UTC),
	}
}

type fakeMigrationStore struct {
	rows map[int64]migrationRow
}

func newFakeMigrationStore(rows []migrationRow) *fakeMigrationStore {
	store := &fakeMigrationStore{rows: map[int64]migrationRow{}}
	for _, row := range rows {
		store.rows[row.ID] = row
	}
	return store
}

func (s *fakeMigrationStore) ListDirtyRows(context.Context, int) ([]migrationRow, error) {
	rows := make([]migrationRow, 0, len(s.rows))
	for _, row := range s.rows {
		rows = append(rows, row)
	}
	return rows, nil
}

func (s *fakeMigrationStore) LockRow(_ context.Context, id int64) (lockedRow, error) {
	return &fakeLockedRow{store: s, row: s.rows[id]}, nil
}

type fakeLockedRow struct {
	store *fakeMigrationStore
	row   migrationRow
}

func (r *fakeLockedRow) Row() migrationRow {
	return r.row
}

func (r *fakeLockedRow) UpdateStorageKey(_ context.Context, oldKey, newKey string) (bool, error) {
	current := r.store.rows[r.row.ID]
	if current.StorageKey != oldKey {
		return false, nil
	}
	current.StorageKey = newKey
	r.store.rows[r.row.ID] = current
	return true, nil
}

func (r *fakeLockedRow) Commit() error {
	return nil
}

func (r *fakeLockedRow) Rollback() error {
	return nil
}

type fakeMigrationOSS struct {
	existing    map[string]bool
	headCalls   int
	copyCalls   int
	deleteCalls int
	lastCopySrc string
	lastCopyDst string
}

func (o *fakeMigrationOSS) HeadObject(_ context.Context, key string) (bool, error) {
	o.headCalls++
	return o.existing[key], nil
}

func (o *fakeMigrationOSS) CopyObject(_ context.Context, srcKey, dstKey string) error {
	o.copyCalls++
	o.lastCopySrc = srcKey
	o.lastCopyDst = dstKey
	o.existing[dstKey] = true
	return nil
}

func (o *fakeMigrationOSS) DeleteObject(_ context.Context, key string) error {
	o.deleteCalls++
	delete(o.existing, key)
	return nil
}
