package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"
)

const (
	planVersion      = 1
	guardEnvironment = "clone_b"
)

type exactRecovery struct {
	sourceID int64
	size     int64
}

var exactRecoveries = map[int64]exactRecovery{
	23989: {sourceID: 24034, size: 683001},
	23990: {sourceID: 24033, size: 689291},
	23991: {sourceID: 24040, size: 686447},
}

type options struct {
	DSN             string
	Mode            string
	PlanFile        string
	FixtureRoot     string
	ReportFile      string
	ConfirmDatabase string
	ConfirmHost     string
	ConfirmRunID    string
}

type recoveryPlan struct {
	Version                  int             `json:"version"`
	Status                   string          `json:"status"`
	RunID                    string          `json:"run_id"`
	MappingSHA256            string          `json:"mapping_sha256"`
	DatabaseWritesExecuted   bool            `json:"database_writes_executed"`
	ProductionWritesExecuted bool            `json:"production_writes_executed"`
	Entries                  []recoveryEntry `json:"entries"`
}

type recoveryEntry struct {
	MissingTaskAssetID int64  `json:"missing_task_asset_id"`
	SourceTaskAssetID  int64  `json:"source_task_asset_id"`
	SourceSHA256       string `json:"source_sha256"`
	SourceSize         int64  `json:"source_size"`
	TargetStorageRefID string `json:"target_storage_ref_id"`
	TargetObjectKey    string `json:"target_object_key"`
	DBApplyPlan        struct {
		InsertStorageRef map[string]any `json:"insert_asset_storage_ref"`
		UpdateTaskAsset  rowMutation    `json:"update_task_asset"`
		UpdateUpload     rowMutation    `json:"update_upload_request"`
	} `json:"db_apply_plan"`
	Rollback struct {
		RestoreTaskAsset   map[string]any `json:"restore_task_asset"`
		RestoreUpload      map[string]any `json:"restore_upload_request"`
		OriginalStorageRef map[string]any `json:"original_storage_ref"`
		DeleteStorageRefID string         `json:"delete_created_storage_ref_id"`
		DeleteObjectKey    string         `json:"delete_fixture_object_key"`
		ExpectedObjectSHA  string         `json:"expected_fixture_sha256"`
		DBRollbackPlan     struct {
			RestoreTaskAsset rowMutation `json:"restore_task_asset"`
			RestoreUpload    rowMutation `json:"restore_upload_request"`
			DeleteStorageRef rowMutation `json:"delete_asset_storage_ref"`
		} `json:"db_rollback_plan"`
	} `json:"rollback_registry"`
}

type rowMutation struct {
	Where map[string]any `json:"where"`
	Set   map[string]any `json:"set"`
}

type executionReport struct {
	Version                      int       `json:"version"`
	Mode                         string    `json:"mode"`
	RunID                        string    `json:"run_id"`
	Database                     string    `json:"database"`
	Host                         string    `json:"host"`
	PlanSHA256                   string    `json:"plan_sha256"`
	ExecutedAt                   time.Time `json:"executed_at"`
	ChangedEntries               int       `json:"changed_entries"`
	AlreadyInTargetStateEntries  int       `json:"already_in_target_state_entries"`
	DatabaseTransactionCommitted bool      `json:"database_transaction_committed"`
	ObjectStorageWritesExecuted  bool      `json:"object_storage_writes_executed"`
}

type transaction interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

func main() {
	var o options
	flag.StringVar(&o.DSN, "dsn", os.Getenv("CLONE_B_MYSQL_DSN"), "Clone B MySQL DSN; MYSQL_DSN is deliberately ignored")
	flag.StringVar(&o.Mode, "mode", "", "apply or rollback")
	flag.StringVar(&o.PlanFile, "plan", "", "materialized plan from prepare_asset_recovery.py")
	flag.StringVar(&o.FixtureRoot, "fixture-root", "", "run-scoped materialized object root")
	flag.StringVar(&o.ReportFile, "report-file", "", "new execution report path")
	flag.StringVar(&o.ConfirmDatabase, "confirm-database", "", "must exactly match the Clone B database")
	flag.StringVar(&o.ConfirmHost, "confirm-host", "", "must exactly match the DSN host")
	flag.StringVar(&o.ConfirmRunID, "confirm-run-id", "", "must exactly match the plan and DB guard run_id")
	flag.Parse()

	if err := run(context.Background(), o); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(ctx context.Context, o options) error {
	cfg, host, err := validateOptions(o)
	if err != nil {
		return err
	}
	planBytes, err := os.ReadFile(o.PlanFile)
	if err != nil {
		return err
	}
	planSHA := sha256Hex(planBytes)
	var plan recoveryPlan
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		return fmt.Errorf("decode recovery plan: %w", err)
	}
	if err := validatePlan(plan, o.ConfirmRunID, o.FixtureRoot); err != nil {
		return err
	}
	sort.Slice(plan.Entries, func(i, j int) bool {
		return plan.Entries[i].MissingTaskAssetID < plan.Entries[j].MissingTaskAssetID
	})

	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect Clone B: %w", err)
	}
	var database string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&database); err != nil {
		return fmt.Errorf("read database identity: %w", err)
	}
	if database != o.ConfirmDatabase || database != cfg.DBName {
		return fmt.Errorf("database identity mismatch: connected=%q confirmed=%q dsn=%q", database, o.ConfirmDatabase, cfg.DBName)
	}

	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := validateCloneGuard(ctx, tx, plan.RunID, planSHA); err != nil {
		return err
	}

	changed := 0
	already := 0
	for _, entry := range plan.Entries {
		var didChange bool
		if o.Mode == "apply" {
			didChange, err = applyEntry(ctx, tx, entry)
		} else {
			didChange, err = rollbackEntry(ctx, tx, entry)
		}
		if err != nil {
			return fmt.Errorf("task_asset %d: %w", entry.MissingTaskAssetID, err)
		}
		if didChange {
			changed++
		} else {
			already++
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	report := executionReport{
		Version:                      1,
		Mode:                         o.Mode,
		RunID:                        plan.RunID,
		Database:                     database,
		Host:                         host,
		PlanSHA256:                   planSHA,
		ExecutedAt:                   time.Now().UTC(),
		ChangedEntries:               changed,
		AlreadyInTargetStateEntries:  already,
		DatabaseTransactionCommitted: true,
		ObjectStorageWritesExecuted:  false,
	}
	if err := writeNewJSON(o.ReportFile, report); err != nil {
		if o.Mode == "apply" && changed > 0 {
			compensationErr := compensateCommittedApply(
				ctx, db, plan.RunID, planSHA, plan.Entries,
			)
			if compensationErr != nil {
				return fmt.Errorf(
					"write apply report: %v; committed database apply compensation failed: %w",
					err, compensationErr,
				)
			}
			return fmt.Errorf(
				"write apply report: %w; committed database apply was compensated",
				err,
			)
		}
		return err
	}
	return nil
}

func compensateCommittedApply(
	ctx context.Context,
	db *sql.DB,
	runID string,
	planSHA string,
	entries []recoveryEntry,
) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if err := validateCloneGuard(ctx, tx, runID, planSHA); err != nil {
		return err
	}
	for index := len(entries) - 1; index >= 0; index-- {
		changed, err := rollbackEntry(ctx, tx, entries[index])
		if err != nil {
			return fmt.Errorf(
				"task_asset %d compensation: %w",
				entries[index].MissingTaskAssetID,
				err,
			)
		}
		if !changed {
			return fmt.Errorf(
				"task_asset %d compensation found no committed apply",
				entries[index].MissingTaskAssetID,
			)
		}
	}
	return tx.Commit()
}

func validateOptions(o options) (*mysql.Config, string, error) {
	if o.Mode != "apply" && o.Mode != "rollback" {
		return nil, "", errors.New("--mode must be apply or rollback")
	}
	for name, value := range map[string]string{
		"--dsn/CLONE_B_MYSQL_DSN": o.DSN,
		"--plan":                  o.PlanFile,
		"--fixture-root":          o.FixtureRoot,
		"--report-file":           o.ReportFile,
		"--confirm-database":      o.ConfirmDatabase,
		"--confirm-host":          o.ConfirmHost,
		"--confirm-run-id":        o.ConfirmRunID,
	} {
		if strings.TrimSpace(value) == "" {
			return nil, "", fmt.Errorf("%s is required", name)
		}
	}
	cfg, err := mysql.ParseDSN(o.DSN)
	if err != nil {
		return nil, "", fmt.Errorf("parse Clone B DSN: %w", err)
	}
	if cfg.Net != "tcp" {
		return nil, "", errors.New("Clone B executor only accepts a TCP DSN")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return nil, "", fmt.Errorf("Clone B DSN address must include an explicit port: %w", err)
	}
	if !isLoopbackHost(host) {
		return nil, "", fmt.Errorf("refusing non-loopback database host %q", host)
	}
	if host != o.ConfirmHost {
		return nil, "", fmt.Errorf("host confirmation mismatch: dsn=%q confirmed=%q", host, o.ConfirmHost)
	}
	if !isSafeCloneBDatabaseName(cfg.DBName) {
		return nil, "", fmt.Errorf("refusing unmarked database %q; name must start with ab_ and end with _b", cfg.DBName)
	}
	if cfg.DBName != o.ConfirmDatabase {
		return nil, "", fmt.Errorf("database confirmation mismatch: dsn=%q confirmed=%q", cfg.DBName, o.ConfirmDatabase)
	}
	cfg.ParseTime = true
	cfg.MultiStatements = false
	return cfg, host, nil
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isSafeCloneBDatabaseName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "ab_") && strings.HasSuffix(lower, "_b")
}

func validatePlan(plan recoveryPlan, confirmRunID, fixtureRoot string) error {
	if plan.Version != planVersion || plan.Status != "MATERIALIZED" {
		return errors.New("plan must be version 1 with status MATERIALIZED")
	}
	if plan.DatabaseWritesExecuted || plan.ProductionWritesExecuted {
		return errors.New("plan write flags must both be false")
	}
	if plan.RunID == "" || plan.RunID != confirmRunID {
		return errors.New("plan run_id does not match --confirm-run-id")
	}
	if len(plan.Entries) != len(exactRecoveries) {
		return errors.New("plan must contain the three exact task 2807 recoveries")
	}
	seen := map[int64]bool{}
	for _, entry := range plan.Entries {
		exact, ok := exactRecoveries[entry.MissingTaskAssetID]
		if !ok || entry.SourceTaskAssetID != exact.sourceID || entry.SourceSize != exact.size || seen[entry.MissingTaskAssetID] {
			return errors.New("plan recovery set differs from the frozen task 2807 allowlist")
		}
		seen[entry.MissingTaskAssetID] = true
		if err := validateEntry(entry, fixtureRoot); err != nil {
			return fmt.Errorf("task_asset %d: %w", entry.MissingTaskAssetID, err)
		}
	}
	return nil
}

func validateEntry(entry recoveryEntry, fixtureRoot string) error {
	if len(entry.SourceSHA256) != 64 || entry.SourceSize <= 0 ||
		entry.TargetStorageRefID == "" || entry.TargetObjectKey == "" ||
		entry.Rollback.DeleteStorageRefID != entry.TargetStorageRefID ||
		entry.Rollback.DeleteObjectKey != entry.TargetObjectKey ||
		entry.Rollback.ExpectedObjectSHA != entry.SourceSHA256 {
		return errors.New("plan entry identity or rollback registry is incomplete")
	}
	decodedSHA, err := hex.DecodeString(entry.SourceSHA256)
	if err != nil || len(decodedSHA) != sha256.Size {
		return errors.New("source SHA-256 is invalid")
	}
	if entry.TargetStorageRefID == fmt.Sprint(entry.Rollback.OriginalStorageRef["ref_id"]) {
		return errors.New("target storage ref must not reuse the deleted original ref")
	}
	if err := requireMapFields(entry.DBApplyPlan.UpdateTaskAsset.Set, taskAssetFields(), "task asset apply set"); err != nil {
		return err
	}
	if err := requireMapFields(entry.Rollback.RestoreTaskAsset, taskAssetFields(), "task asset before state"); err != nil {
		return err
	}
	if err := requireMapFields(entry.DBApplyPlan.UpdateUpload.Set, uploadFields(), "upload apply set"); err != nil {
		return err
	}
	if err := requireMapFields(entry.Rollback.RestoreUpload, uploadFields(), "upload before state"); err != nil {
		return err
	}
	if err := requireMapFields(entry.DBApplyPlan.InsertStorageRef, storageRefFields(), "storage ref insert"); err != nil {
		return err
	}
	if !reflect.DeepEqual(entry.DBApplyPlan.UpdateTaskAsset.Where, map[string]any{"id": float64(entry.MissingTaskAssetID)}) ||
		!reflect.DeepEqual(entry.DBRollbackTaskWhere(), entry.DBApplyPlan.UpdateTaskAsset.Where) {
		return errors.New("task asset where clause is not exact")
	}
	requestID := entry.DBApplyPlan.UpdateUpload.Where["request_id"]
	if requestID == nil || requestID == "" ||
		!reflect.DeepEqual(entry.Rollback.DBRollbackPlan.RestoreUpload.Where, entry.DBApplyPlan.UpdateUpload.Where) {
		return errors.New("upload request where clause is not exact")
	}
	if entry.DBApplyPlan.InsertStorageRef["ref_id"] != entry.TargetStorageRefID ||
		entry.DBApplyPlan.InsertStorageRef["owner_type"] != "task_asset" ||
		asInt64(entry.DBApplyPlan.InsertStorageRef["owner_id"]) != entry.MissingTaskAssetID ||
		entry.DBApplyPlan.InsertStorageRef["storage_adapter"] != "local" ||
		entry.DBApplyPlan.InsertStorageRef["ref_key"] != entry.TargetObjectKey {
		return errors.New("storage ref insert does not match the recovery identity")
	}
	taskAfter := entry.DBApplyPlan.UpdateTaskAsset.Set
	if taskAfter["storage_ref_id"] != entry.TargetStorageRefID ||
		taskAfter["storage_key"] != entry.TargetObjectKey ||
		taskAfter["whole_hash"] != entry.SourceSHA256 ||
		taskAfter["upload_status"] != "uploaded" ||
		taskAfter["deleted_at"] != nil ||
		taskAfter["cleaned_at"] != nil ||
		taskAfter["object_deleted_at"] != nil ||
		taskAfter["access_revoked_at"] != nil {
		return errors.New("task asset target state differs from the frozen recovery contract")
	}
	uploadAfter := entry.DBApplyPlan.UpdateUpload.Set
	if uploadAfter["bound_ref_id"] != entry.TargetStorageRefID ||
		uploadAfter["checksum_hint"] != entry.SourceSHA256 ||
		asInt64(uploadAfter["file_size"]) != entry.SourceSize ||
		uploadAfter["status"] != "bound" ||
		uploadAfter["session_status"] != "completed" {
		return errors.New("upload request target state differs from the frozen recovery contract")
	}
	if entry.DBApplyPlan.InsertStorageRef["upload_request_id"] != requestID ||
		entry.Rollback.RestoreTaskAsset["storage_ref_id"] != entry.Rollback.OriginalStorageRef["ref_id"] ||
		entry.Rollback.RestoreUpload["bound_ref_id"] != entry.Rollback.OriginalStorageRef["ref_id"] {
		return errors.New("before/after rows do not share one exact upload and storage identity")
	}
	if entry.Rollback.DBRollbackPlan.DeleteStorageRef.Where["ref_id"] != entry.TargetStorageRefID {
		return errors.New("storage ref rollback where clause is not exact")
	}
	if !reflect.DeepEqual(
		normalizeMap(selectFields(entry.Rollback.RestoreTaskAsset, taskAssetFields())),
		normalizeMap(entry.Rollback.DBRollbackPlan.RestoreTaskAsset.Set),
	) {
		return errors.New("task asset rollback set differs from frozen before state")
	}
	if !reflect.DeepEqual(
		normalizeMap(selectFields(entry.Rollback.RestoreUpload, uploadFields())),
		normalizeMap(entry.Rollback.DBRollbackPlan.RestoreUpload.Set),
	) {
		return errors.New("upload rollback set differs from frozen before state")
	}
	if fmt.Sprint(entry.Rollback.OriginalStorageRef["ref_id"]) == "" ||
		fmt.Sprint(entry.Rollback.OriginalStorageRef["status"]) == "" {
		return errors.New("original storage ref before state is incomplete")
	}
	target, err := containedFixturePath(fixtureRoot, entry.TargetObjectKey)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		return fmt.Errorf("materialized object is not readable: %w", err)
	}
	if info.Size() != entry.SourceSize {
		return errors.New("materialized object size differs from plan")
	}
	digest, err := sha256File(target)
	if err != nil {
		return err
	}
	if digest != entry.SourceSHA256 {
		return errors.New("materialized object SHA-256 differs from plan")
	}
	return nil
}

func (entry recoveryEntry) DBRollbackTaskWhere() map[string]any {
	return entry.Rollback.DBRollbackPlan.RestoreTaskAsset.Where
}

func containedFixturePath(root, objectKey string) (string, error) {
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	target, err := filepath.Abs(filepath.Join(base, "objects", filepath.FromSlash(objectKey)))
	if err != nil {
		return "", err
	}
	prefix := base + string(os.PathSeparator)
	if target == base || !strings.HasPrefix(target, prefix) {
		return "", errors.New("object path escapes fixture root")
	}
	return target, nil
}

func validateCloneGuard(ctx context.Context, tx transaction, runID, planSHA string) error {
	// Clone provisioning owns this deliberately non-migrated guard table:
	//   CREATE TABLE v8_ab_clone_guard (
	//     singleton_id TINYINT PRIMARY KEY,
	//     environment VARCHAR(32) NOT NULL,
	//     run_id VARCHAR(81) NOT NULL,
	//     plan_sha256 CHAR(64) NOT NULL
	//   );
	// Exactly one singleton_id=1 row must be installed in Clone B after the
	// materialized plan is frozen. This tool never creates or updates the guard.
	var environment, guardedRunID, guardedPlanSHA string
	err := tx.QueryRowContext(ctx, `
		SELECT environment,run_id,plan_sha256
		FROM v8_ab_clone_guard
		WHERE singleton_id=1
		FOR UPDATE`).Scan(&environment, &guardedRunID, &guardedPlanSHA)
	if err != nil {
		return fmt.Errorf("Clone B guard missing or unreadable: %w", err)
	}
	if environment != guardEnvironment || guardedRunID != runID || guardedPlanSHA != planSHA {
		return errors.New("Clone B guard does not bind environment, run_id, and plan SHA-256")
	}
	return nil
}

func applyEntry(ctx context.Context, tx transaction, entry recoveryEntry) (bool, error) {
	state, err := readLockedState(ctx, tx, entry)
	if err != nil {
		return false, err
	}
	before := expectedBefore(entry)
	after := expectedAfter(entry)
	if state.equals(after) {
		return false, nil
	}
	if !state.equals(before) {
		return false, errors.New("before-state drift or partial prior apply")
	}
	if err := insertStorageRef(ctx, tx, entry.DBApplyPlan.InsertStorageRef); err != nil {
		return false, err
	}
	if err := updateTaskAsset(ctx, tx, entry.DBApplyPlan.UpdateTaskAsset); err != nil {
		return false, err
	}
	if err := updateUpload(ctx, tx, entry.DBApplyPlan.UpdateUpload); err != nil {
		return false, err
	}
	return true, nil
}

func rollbackEntry(ctx context.Context, tx transaction, entry recoveryEntry) (bool, error) {
	state, err := readLockedState(ctx, tx, entry)
	if err != nil {
		return false, err
	}
	before := expectedBefore(entry)
	after := expectedAfter(entry)
	if state.equals(before) {
		return false, nil
	}
	if !state.equals(after) {
		return false, errors.New("after-state drift or partial prior rollback")
	}
	if err := updateTaskAsset(ctx, tx, entry.Rollback.DBRollbackPlan.RestoreTaskAsset); err != nil {
		return false, err
	}
	if err := updateUpload(ctx, tx, entry.Rollback.DBRollbackPlan.RestoreUpload); err != nil {
		return false, err
	}
	result, err := tx.ExecContext(ctx, `DELETE FROM asset_storage_refs WHERE ref_id=?`, entry.TargetStorageRefID)
	if err != nil {
		return false, err
	}
	if err := requireOne(result, "delete created storage ref"); err != nil {
		return false, err
	}
	return true, nil
}

type lockedState struct {
	taskAsset        map[string]any
	uploadRequest    map[string]any
	originalStatus   string
	targetStorageRef map[string]any
}

func (s lockedState) equals(other lockedState) bool {
	return reflect.DeepEqual(normalizeMap(s.taskAsset), normalizeMap(other.taskAsset)) &&
		reflect.DeepEqual(normalizeMap(s.uploadRequest), normalizeMap(other.uploadRequest)) &&
		s.originalStatus == other.originalStatus &&
		reflect.DeepEqual(normalizeMap(s.targetStorageRef), normalizeMap(other.targetStorageRef))
}

func expectedBefore(entry recoveryEntry) lockedState {
	return lockedState{
		taskAsset:        selectFields(entry.Rollback.RestoreTaskAsset, taskAssetFields()),
		uploadRequest:    selectFields(entry.Rollback.RestoreUpload, uploadFields()),
		originalStatus:   fmt.Sprint(entry.Rollback.OriginalStorageRef["status"]),
		targetStorageRef: nil,
	}
}

func expectedAfter(entry recoveryEntry) lockedState {
	return lockedState{
		taskAsset:        selectFields(entry.DBApplyPlan.UpdateTaskAsset.Set, taskAssetFields()),
		uploadRequest:    selectFields(entry.DBApplyPlan.UpdateUpload.Set, uploadFields()),
		originalStatus:   fmt.Sprint(entry.Rollback.OriginalStorageRef["status"]),
		targetStorageRef: selectFields(entry.DBApplyPlan.InsertStorageRef, storageRefFields()),
	}
}

func readLockedState(ctx context.Context, tx transaction, entry recoveryEntry) (lockedState, error) {
	taskAsset, err := queryJSONMap(ctx, tx, taskAssetStateSQL, entry.MissingTaskAssetID)
	if err != nil {
		return lockedState{}, fmt.Errorf("lock/read task asset: %w", err)
	}
	requestID := entry.DBApplyPlan.UpdateUpload.Where["request_id"]
	upload, err := queryJSONMap(ctx, tx, uploadStateSQL, requestID)
	if err != nil {
		return lockedState{}, fmt.Errorf("lock/read upload request: %w", err)
	}
	var originalStatus string
	if err := tx.QueryRowContext(ctx, `SELECT status FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`, entry.Rollback.OriginalStorageRef["ref_id"]).Scan(&originalStatus); err != nil {
		return lockedState{}, fmt.Errorf("lock/read original storage ref: %w", err)
	}
	target, err := queryOptionalJSONMap(ctx, tx, storageRefStateSQL, entry.TargetStorageRefID)
	if err != nil {
		return lockedState{}, fmt.Errorf("lock/read target storage ref: %w", err)
	}
	return lockedState{taskAsset: taskAsset, uploadRequest: upload, originalStatus: originalStatus, targetStorageRef: target}, nil
}

const taskAssetStateSQL = `
	SELECT JSON_OBJECT(
		'storage_ref_id',storage_ref_id,'storage_key',storage_key,'whole_hash',whole_hash,
		'upload_status',upload_status,
		'deleted_at',IF(deleted_at IS NULL,NULL,DATE_FORMAT(deleted_at,'%Y-%m-%d %H:%i:%s')),
		'cleaned_at',IF(cleaned_at IS NULL,NULL,DATE_FORMAT(cleaned_at,'%Y-%m-%d %H:%i:%s')),
		'object_deleted_at',IF(object_deleted_at IS NULL,NULL,DATE_FORMAT(object_deleted_at,'%Y-%m-%d %H:%i:%s')),
		'access_revoked_at',IF(access_revoked_at IS NULL,NULL,DATE_FORMAT(access_revoked_at,'%Y-%m-%d %H:%i:%s')),
		'access_revoked_reason',access_revoked_reason)
	FROM task_assets WHERE id=? FOR UPDATE`

const uploadStateSQL = `
	SELECT JSON_OBJECT(
		'bound_ref_id',bound_ref_id,'checksum_hint',checksum_hint,'file_size',file_size,
		'status',status,'session_status',session_status)
	FROM upload_requests WHERE request_id=? FOR UPDATE`

const storageRefStateSQL = `
	SELECT JSON_OBJECT(
		'ref_id',ref_id,'asset_id',asset_id,'owner_type',owner_type,'owner_id',owner_id,
		'upload_request_id',upload_request_id,'storage_adapter',storage_adapter,'ref_type',ref_type,
		'ref_key',ref_key,'file_name',file_name,'mime_type',mime_type,'file_size',file_size,
		'is_placeholder',is_placeholder,'checksum_hint',checksum_hint,'status',status)
	FROM asset_storage_refs WHERE ref_id=? FOR UPDATE`

func queryJSONMap(ctx context.Context, tx transaction, query string, arg any) (map[string]any, error) {
	var raw []byte
	if err := tx.QueryRowContext(ctx, query, arg).Scan(&raw); err != nil {
		return nil, err
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, err
	}
	return value, nil
}

func queryOptionalJSONMap(ctx context.Context, tx transaction, query string, arg any) (map[string]any, error) {
	value, err := queryJSONMap(ctx, tx, query, arg)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return value, err
}

func insertStorageRef(ctx context.Context, tx transaction, values map[string]any) error {
	fields := storageRefFields()
	args := make([]any, 0, len(fields))
	for _, field := range fields {
		args = append(args, values[field])
	}
	result, err := tx.ExecContext(ctx, `
		INSERT INTO asset_storage_refs
			(ref_id,asset_id,owner_type,owner_id,upload_request_id,storage_adapter,ref_type,ref_key,file_name,mime_type,file_size,is_placeholder,checksum_hint,status)
		VALUES (?,?,?,?,?,?,?,?,?,?,?,?,?,?)`, args...)
	if err != nil {
		return err
	}
	return requireOne(result, "insert storage ref")
}

func updateTaskAsset(ctx context.Context, tx transaction, mutation rowMutation) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE task_assets
		SET storage_ref_id=?,storage_key=?,whole_hash=?,upload_status=?,deleted_at=?,cleaned_at=?,
		    object_deleted_at=?,access_revoked_at=?,access_revoked_reason=?
		WHERE id=?`,
		mutation.Set["storage_ref_id"], mutation.Set["storage_key"], mutation.Set["whole_hash"],
		mutation.Set["upload_status"], dbMutationValue("deleted_at", mutation.Set["deleted_at"]), dbMutationValue("cleaned_at", mutation.Set["cleaned_at"]),
		dbMutationValue("object_deleted_at", mutation.Set["object_deleted_at"]), dbMutationValue("access_revoked_at", mutation.Set["access_revoked_at"]), mutation.Set["access_revoked_reason"],
		mutation.Where["id"])
	if err != nil {
		return err
	}
	return requireOne(result, "update task asset")
}

func updateUpload(ctx context.Context, tx transaction, mutation rowMutation) error {
	result, err := tx.ExecContext(ctx, `
		UPDATE upload_requests
		SET bound_ref_id=?,checksum_hint=?,file_size=?,status=?,session_status=?
		WHERE request_id=?`,
		mutation.Set["bound_ref_id"], mutation.Set["checksum_hint"], mutation.Set["file_size"],
		mutation.Set["status"], mutation.Set["session_status"], mutation.Where["request_id"])
	if err != nil {
		return err
	}
	return requireOne(result, "update upload request")
}

func requireOne(result sql.Result, action string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if affected != 1 {
		return fmt.Errorf("%s affected %d rows, expected 1", action, affected)
	}
	return nil
}

func taskAssetFields() []string {
	return []string{"storage_ref_id", "storage_key", "whole_hash", "upload_status", "deleted_at", "cleaned_at", "object_deleted_at", "access_revoked_at", "access_revoked_reason"}
}

func uploadFields() []string {
	return []string{"bound_ref_id", "checksum_hint", "file_size", "status", "session_status"}
}

func storageRefFields() []string {
	return []string{"ref_id", "asset_id", "owner_type", "owner_id", "upload_request_id", "storage_adapter", "ref_type", "ref_key", "file_name", "mime_type", "file_size", "is_placeholder", "checksum_hint", "status"}
}

func selectFields(values map[string]any, fields []string) map[string]any {
	result := make(map[string]any, len(fields))
	for _, field := range fields {
		result[field] = values[field]
	}
	return result
}

func requireMapFields(values map[string]any, fields []string, label string) error {
	for _, field := range fields {
		if _, ok := values[field]; !ok {
			return fmt.Errorf("%s lacks %q", label, field)
		}
	}
	return nil
}

func normalizeMap(value map[string]any) map[string]any {
	if value == nil {
		return nil
	}
	result := make(map[string]any, len(value))
	for key, item := range value {
		if strings.HasSuffix(key, "_at") && item != nil {
			result[key] = normalizeTime(fmt.Sprint(item))
			continue
		}
		if number, ok := item.(json.Number); ok {
			result[key] = number.String()
			continue
		}
		switch typed := item.(type) {
		case float64:
			result[key] = fmt.Sprintf("%.0f", typed)
		case int64:
			result[key] = fmt.Sprintf("%d", typed)
		case int:
			result[key] = fmt.Sprintf("%d", typed)
		default:
			result[key] = item
		}
	}
	return result
}

func normalizeTime(value string) string {
	value = strings.TrimSuffix(value, "Z")
	value = strings.Replace(value, "T", " ", 1)
	if dot := strings.IndexByte(value, '.'); dot >= 0 {
		value = value[:dot]
	}
	return value
}

func dbMutationValue(field string, value any) any {
	if value == nil || !strings.HasSuffix(field, "_at") {
		return value
	}
	return normalizeTime(fmt.Sprint(value))
}

func asInt64(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		return 0
	}
}

func sha256Hex(value []byte) string {
	sum := sha256.Sum256(value)
	return hex.EncodeToString(sum[:])
}

func sha256File(path string) (string, error) {
	value, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return sha256Hex(value), nil
}

func writeNewJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return fmt.Errorf("create report without overwrite: %w", err)
	}
	defer file.Close()
	if _, err := file.Write(data); err != nil {
		return err
	}
	return file.Sync()
}
