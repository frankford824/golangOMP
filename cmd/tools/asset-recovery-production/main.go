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
	"io"
	"net"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"workflow/service"
)

const (
	planVersion       = 1
	confirmPhrase     = "PRODUCTION_ASSET_RECOVERY_2807"
	targetEnvironment = "production"
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
	DSN               string
	Mode              string
	PlanFile          string
	SourceRoot        string
	ReportFile        string
	CutoverMarker     string
	ConfirmDatabase   string
	ConfirmHost       string
	ConfirmRunID      string
	ConfirmRelease    string
	ConfirmCommit     string
	ConfirmProduction string
}

type recoveryPlan struct {
	Version                  int             `json:"version"`
	Status                   string          `json:"status"`
	RunID                    string          `json:"run_id"`
	TargetEnvironment        string          `json:"target_environment"`
	ProductionRelease        string          `json:"production_release"`
	MappingSHA256            string          `json:"mapping_sha256"`
	DatabaseWritesExecuted   bool            `json:"database_writes_executed"`
	ProductionWritesExecuted bool            `json:"production_writes_executed"`
	EvidenceSHA256           string          `json:"evidence_sha256"`
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
	Environment                  string    `json:"environment"`
	Release                      string    `json:"release"`
	RunID                        string    `json:"run_id"`
	Database                     string    `json:"database"`
	Host                         string    `json:"host"`
	ApprovedCommit               string    `json:"approved_commit"`
	PlanSHA256                   string    `json:"plan_sha256"`
	ExecutedAt                   time.Time `json:"executed_at"`
	ChangedEntries               int       `json:"changed_entries"`
	AlreadyInTargetStateEntries  int       `json:"already_in_target_state_entries"`
	DatabaseTransactionCommitted bool      `json:"database_transaction_committed"`
	ObjectStorageWritesExecuted  bool      `json:"object_storage_writes_executed"`
	ObjectStorageDeletesExecuted bool      `json:"object_storage_deletes_executed"`
}

type objectStore interface {
	UploadObjectFromReader(context.Context, string, string, io.Reader) error
	StatObject(context.Context, string) (*service.OSSObjectInfo, bool, error)
	OpenObject(context.Context, string) (io.ReadCloser, error)
	DeleteObject(context.Context, string) error
}

type transaction interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
	ExecContext(context.Context, string, ...any) (sql.Result, error)
	Commit() error
	Rollback() error
}

func main() {
	var o options
	flag.StringVar(&o.DSN, "dsn", os.Getenv("MYSQL_DSN"), "production MySQL DSN")
	flag.StringVar(&o.Mode, "mode", "", "apply or rollback")
	flag.StringVar(&o.PlanFile, "plan", "", "production plan from prepare_asset_recovery.py")
	flag.StringVar(&o.SourceRoot, "source-root", "", "directory containing the three frozen source files")
	flag.StringVar(&o.ReportFile, "report-file", "", "new execution report path")
	flag.StringVar(&o.CutoverMarker, "cutover-marker", "", "exact-commit cutover marker")
	flag.StringVar(&o.ConfirmDatabase, "confirm-database", "", "must exactly match the production database")
	flag.StringVar(&o.ConfirmHost, "confirm-host", "", "must exactly match the DSN host")
	flag.StringVar(&o.ConfirmRunID, "confirm-run-id", "", "must exactly match the plan run_id")
	flag.StringVar(&o.ConfirmRelease, "confirm-release", "", "must exactly match the plan release")
	flag.StringVar(&o.ConfirmCommit, "confirm-commit", "", "40-character approved commit")
	flag.StringVar(&o.ConfirmProduction, "confirm-production", "", "required production confirmation phrase")
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
	if err := validateCutoverMarker(o.CutoverMarker, o.ConfirmCommit); err != nil {
		return err
	}
	planBytes, err := os.ReadFile(o.PlanFile)
	if err != nil {
		return err
	}
	planSHA := sha256Hex(planBytes)
	var plan recoveryPlan
	if err := json.Unmarshal(planBytes, &plan); err != nil {
		return fmt.Errorf("decode production recovery plan: %w", err)
	}
	if err := validatePlanBytes(planBytes, plan); err != nil {
		return err
	}
	if err := validatePlan(plan, o); err != nil {
		return err
	}
	sort.Slice(plan.Entries, func(i, j int) bool {
		return plan.Entries[i].MissingTaskAssetID < plan.Entries[j].MissingTaskAssetID
	})

	oss, err := productionOSS()
	if err != nil {
		return err
	}
	db, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("connect production database: %w", err)
	}
	var database string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&database); err != nil {
		return fmt.Errorf("read database identity: %w", err)
	}
	if database != o.ConfirmDatabase || database != cfg.DBName {
		return fmt.Errorf("database identity mismatch: connected=%q confirmed=%q dsn=%q", database, o.ConfirmDatabase, cfg.DBName)
	}

	changed, already, objectWrites, objectDeletes, err := execute(
		ctx, db, oss, plan, o,
	)
	if err != nil {
		return err
	}
	report := executionReport{
		Version:                      1,
		Mode:                         o.Mode,
		Environment:                  targetEnvironment,
		Release:                      plan.ProductionRelease,
		RunID:                        plan.RunID,
		Database:                     database,
		Host:                         host,
		ApprovedCommit:               o.ConfirmCommit,
		PlanSHA256:                   planSHA,
		ExecutedAt:                   time.Now().UTC(),
		ChangedEntries:               changed,
		AlreadyInTargetStateEntries:  already,
		DatabaseTransactionCommitted: true,
		ObjectStorageWritesExecuted:  objectWrites,
		ObjectStorageDeletesExecuted: objectDeletes,
	}
	if err := writeNewJSON(o.ReportFile, report); err != nil {
		if o.Mode == "apply" && changed > 0 {
			rollbackOptions := o
			rollbackOptions.Mode = "rollback"
			_, _, _, _, compensationErr := execute(
				ctx, db, oss, plan, rollbackOptions,
			)
			if compensationErr != nil {
				return fmt.Errorf(
					"write production apply report: %v; committed apply compensation failed: %w",
					err,
					compensationErr,
				)
			}
			return fmt.Errorf(
				"write production apply report: %w; committed database and OSS apply was compensated",
				err,
			)
		}
		return err
	}
	return nil
}

func execute(
	ctx context.Context,
	db *sql.DB,
	objects objectStore,
	plan recoveryPlan,
	o options,
) (changed, already int, objectWrites, objectDeletes bool, err error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return 0, 0, false, false, err
	}
	defer tx.Rollback()

	states := make([]lockedState, len(plan.Entries))
	allBefore, allAfter := true, true
	for index, entry := range plan.Entries {
		states[index], err = readLockedState(ctx, tx, entry)
		if err != nil {
			return 0, 0, false, false, fmt.Errorf("task_asset %d: %w", entry.MissingTaskAssetID, err)
		}
		allBefore = allBefore && states[index].equals(expectedBefore(entry))
		allAfter = allAfter && states[index].equals(expectedAfter(entry))
	}
	if !allBefore && !allAfter {
		return 0, 0, false, false, errors.New("recovery rows are in a mixed or drifted state")
	}

	if o.Mode == "apply" {
		if allAfter {
			for _, entry := range plan.Entries {
				if err := verifyRemoteObject(ctx, objects, entry); err != nil {
					return 0, 0, false, false, err
				}
			}
			if err := tx.Commit(); err != nil {
				return 0, 0, false, false, err
			}
			return 0, len(plan.Entries), false, false, nil
		}
		created := make([]recoveryEntry, 0, len(plan.Entries))
		commitAttempted := false
		defer func() {
			if err == nil || commitAttempted {
				return
			}
			for index := len(created) - 1; index >= 0; index-- {
				_ = deleteVerifiedObject(ctx, objects, created[index], true)
			}
		}()
		for _, entry := range plan.Entries {
			_, exists, statErr := objects.StatObject(ctx, entry.TargetObjectKey)
			if statErr != nil {
				return 0, 0, false, false, statErr
			}
			if exists {
				return 0, 0, false, false, fmt.Errorf(
					"task_asset %d target object already exists before first apply",
					entry.MissingTaskAssetID,
				)
			}
			sourceFile := sourcePath(o.SourceRoot, entry)
			if err := verifyLocalSource(sourceFile, entry); err != nil {
				return 0, 0, false, false, err
			}
			source, openErr := os.Open(sourceFile)
			if openErr != nil {
				return 0, 0, false, false, openErr
			}
			mimeType := fmt.Sprint(entry.DBApplyPlan.InsertStorageRef["mime_type"])
			uploadErr := objects.UploadObjectFromReader(
				ctx, entry.TargetObjectKey, mimeType, source,
			)
			closeErr := source.Close()
			if uploadErr != nil {
				return 0, 0, false, false, uploadErr
			}
			if closeErr != nil {
				return 0, 0, false, false, closeErr
			}
			created = append(created, entry)
			if err := verifyRemoteObject(ctx, objects, entry); err != nil {
				return 0, 0, false, false, err
			}
		}
		objectWrites = len(created) > 0
		for _, entry := range plan.Entries {
			if err := insertStorageRef(ctx, tx, entry.DBApplyPlan.InsertStorageRef); err != nil {
				return 0, 0, objectWrites, false, err
			}
			if err := updateTaskAsset(ctx, tx, entry.DBApplyPlan.UpdateTaskAsset); err != nil {
				return 0, 0, objectWrites, false, err
			}
			if err := updateUpload(ctx, tx, entry.DBApplyPlan.UpdateUpload); err != nil {
				return 0, 0, objectWrites, false, err
			}
		}
		commitAttempted = true
		if err := tx.Commit(); err != nil {
			return 0, 0, objectWrites, false, fmt.Errorf(
				"production commit result is ambiguous; preserve exact OSS objects and reconcile DB state before retry: %w",
				err,
			)
		}
		created = nil
		return len(plan.Entries), 0, objectWrites, false, nil
	}

	for _, entry := range plan.Entries {
		_, exists, statErr := objects.StatObject(ctx, entry.TargetObjectKey)
		if statErr != nil {
			return 0, 0, false, false, statErr
		}
		if exists {
			if err := verifyRemoteObject(ctx, objects, entry); err != nil {
				return 0, 0, false, false, err
			}
		} else if allAfter {
			return 0, 0, false, false, fmt.Errorf(
				"task_asset %d recovery object is missing before rollback",
				entry.MissingTaskAssetID,
			)
		}
	}
	if allAfter {
		for _, entry := range plan.Entries {
			if err := updateTaskAsset(ctx, tx, entry.Rollback.DBRollbackPlan.RestoreTaskAsset); err != nil {
				return 0, 0, false, false, err
			}
			if err := updateUpload(ctx, tx, entry.Rollback.DBRollbackPlan.RestoreUpload); err != nil {
				return 0, 0, false, false, err
			}
			result, deleteErr := tx.ExecContext(
				ctx,
				`DELETE FROM asset_storage_refs WHERE ref_id=?`,
				entry.TargetStorageRefID,
			)
			if deleteErr != nil {
				return 0, 0, false, false, deleteErr
			}
			if err := requireOne(result, "delete created storage ref"); err != nil {
				return 0, 0, false, false, err
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, 0, false, false, err
	}
	for _, entry := range plan.Entries {
		deleted, deleteErr := deleteVerifiedObjectIfPresent(ctx, objects, entry)
		if deleteErr != nil {
			return 0, 0, false, objectDeletes, deleteErr
		}
		objectDeletes = objectDeletes || deleted
	}
	if allAfter {
		return len(plan.Entries), 0, false, objectDeletes, nil
	}
	return 0, len(plan.Entries), false, objectDeletes, nil
}

func validateOptions(o options) (*mysql.Config, string, error) {
	if o.Mode != "apply" && o.Mode != "rollback" {
		return nil, "", errors.New("--mode must be apply or rollback")
	}
	for name, value := range map[string]string{
		"--dsn/MYSQL_DSN":    o.DSN,
		"--plan":             o.PlanFile,
		"--source-root":      o.SourceRoot,
		"--report-file":      o.ReportFile,
		"--cutover-marker":   o.CutoverMarker,
		"--confirm-database": o.ConfirmDatabase,
		"--confirm-host":     o.ConfirmHost,
		"--confirm-run-id":   o.ConfirmRunID,
		"--confirm-release":  o.ConfirmRelease,
		"--confirm-commit":   o.ConfirmCommit,
	} {
		if strings.TrimSpace(value) == "" {
			return nil, "", fmt.Errorf("%s is required", name)
		}
	}
	if o.ConfirmProduction != confirmPhrase {
		return nil, "", errors.New("exact --confirm-production phrase is required")
	}
	sourceInfo, err := os.Lstat(o.SourceRoot)
	if err != nil || !sourceInfo.IsDir() || sourceInfo.Mode()&os.ModeSymlink != 0 {
		return nil, "", errors.New("--source-root must be an existing real directory")
	}
	reportParent := filepath.Dir(o.ReportFile)
	if info, err := os.Lstat(reportParent); err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return nil, "", errors.New("--report-file parent must be an existing real directory")
	}
	if _, err := os.Lstat(o.ReportFile); err == nil || !errors.Is(err, os.ErrNotExist) {
		return nil, "", errors.New("--report-file must not already exist")
	}
	if len(o.ConfirmCommit) != 40 {
		return nil, "", errors.New("--confirm-commit must be a full 40-character commit")
	}
	if _, err := hex.DecodeString(o.ConfirmCommit); err != nil {
		return nil, "", errors.New("--confirm-commit is not hexadecimal")
	}
	cfg, err := mysql.ParseDSN(o.DSN)
	if err != nil {
		return nil, "", fmt.Errorf("parse production DSN: %w", err)
	}
	if cfg.Net != "tcp" {
		return nil, "", errors.New("production executor only accepts a TCP DSN")
	}
	host, _, err := net.SplitHostPort(cfg.Addr)
	if err != nil {
		return nil, "", fmt.Errorf("production DSN address must include an explicit port: %w", err)
	}
	if !isLoopbackHost(host) {
		return nil, "", fmt.Errorf("refusing non-loopback database host %q", host)
	}
	if host != o.ConfirmHost {
		return nil, "", fmt.Errorf("host confirmation mismatch: dsn=%q confirmed=%q", host, o.ConfirmHost)
	}
	if isCloneDatabaseName(cfg.DBName) {
		return nil, "", fmt.Errorf("refusing Clone B database %q", cfg.DBName)
	}
	if cfg.DBName != o.ConfirmDatabase {
		return nil, "", fmt.Errorf("database confirmation mismatch: dsn=%q confirmed=%q", cfg.DBName, o.ConfirmDatabase)
	}
	cfg.ParseTime = true
	cfg.MultiStatements = false
	return cfg, host, nil
}

func validateCutoverMarker(path, commit string) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("cutover marker must be a real regular file")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	expected := "APPROVED_COMMIT=" + commit + "\n"
	if string(raw) != expected {
		return errors.New("cutover marker does not exactly bind the approved commit")
	}
	return nil
}

func validatePlanBytes(raw []byte, plan recoveryPlan) error {
	var object map[string]any
	if err := json.Unmarshal(raw, &object); err != nil {
		return err
	}
	evidence, ok := object["evidence_sha256"].(string)
	if !ok || len(evidence) != 64 {
		return errors.New("production plan evidence_sha256 is missing")
	}
	delete(object, "evidence_sha256")
	canonical, err := json.Marshal(object)
	if err != nil {
		return err
	}
	if sha256Hex(canonical) != evidence || plan.EvidenceSHA256 != evidence {
		return errors.New("production plan evidence_sha256 is stale")
	}
	return nil
}

func validatePlan(plan recoveryPlan, o options) error {
	if plan.Version != planVersion || plan.Status != "PREPARED" {
		return errors.New("production plan must be version 1 with status PREPARED")
	}
	if plan.TargetEnvironment != targetEnvironment ||
		plan.ProductionRelease != o.ConfirmRelease ||
		plan.RunID != o.ConfirmRunID {
		return errors.New("production plan environment, release, or run_id confirmation mismatch")
	}
	if plan.DatabaseWritesExecuted || plan.ProductionWritesExecuted {
		return errors.New("production plan write flags must both be false")
	}
	if len(plan.Entries) != len(exactRecoveries) {
		return errors.New("production plan must contain the three exact task 2807 recoveries")
	}
	seen := map[int64]bool{}
	for _, entry := range plan.Entries {
		exact, ok := exactRecoveries[entry.MissingTaskAssetID]
		if !ok || seen[entry.MissingTaskAssetID] ||
			entry.SourceTaskAssetID != exact.sourceID ||
			entry.SourceSize != exact.size {
			return errors.New("production plan recovery set differs from the frozen allowlist")
		}
		seen[entry.MissingTaskAssetID] = true
		if err := validateEntry(entry, plan); err != nil {
			return fmt.Errorf("task_asset %d: %w", entry.MissingTaskAssetID, err)
		}
	}
	return nil
}

func validateEntry(entry recoveryEntry, plan recoveryPlan) error {
	if len(entry.SourceSHA256) != 64 ||
		entry.TargetStorageRefID == "" ||
		entry.Rollback.DeleteStorageRefID != entry.TargetStorageRefID ||
		entry.Rollback.DeleteObjectKey != entry.TargetObjectKey ||
		entry.Rollback.ExpectedObjectSHA != entry.SourceSHA256 {
		return errors.New("plan entry identity or rollback registry is incomplete")
	}
	decoded, err := hex.DecodeString(entry.SourceSHA256)
	if err != nil || len(decoded) != sha256.Size {
		return errors.New("source SHA-256 is invalid")
	}
	prefix := "v8-production/" + plan.ProductionRelease + "/" + plan.RunID + "/recovered/task-2807/"
	expectedObjectKey := fmt.Sprintf(
		"%stask-asset-%d/%s.bin",
		prefix,
		entry.MissingTaskAssetID,
		entry.SourceSHA256,
	)
	if entry.TargetObjectKey != expectedObjectKey ||
		entry.DBApplyPlan.InsertStorageRef["storage_adapter"] != "oss_upload_service" ||
		entry.DBApplyPlan.InsertStorageRef["ref_key"] != entry.TargetObjectKey ||
		entry.DBApplyPlan.InsertStorageRef["ref_id"] != entry.TargetStorageRefID ||
		asInt64(entry.DBApplyPlan.InsertStorageRef["owner_id"]) != entry.MissingTaskAssetID {
		return errors.New("production OSS storage identity is invalid")
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
	requestID := entry.DBApplyPlan.UpdateUpload.Where["request_id"]
	if !reflect.DeepEqual(entry.DBApplyPlan.UpdateTaskAsset.Where, map[string]any{"id": float64(entry.MissingTaskAssetID)}) ||
		!reflect.DeepEqual(entry.Rollback.DBRollbackPlan.RestoreTaskAsset.Where, entry.DBApplyPlan.UpdateTaskAsset.Where) ||
		requestID == nil || requestID == "" ||
		!reflect.DeepEqual(entry.Rollback.DBRollbackPlan.RestoreUpload.Where, entry.DBApplyPlan.UpdateUpload.Where) ||
		entry.Rollback.DBRollbackPlan.DeleteStorageRef.Where["ref_id"] != entry.TargetStorageRefID {
		return errors.New("production DB where clauses are not exact")
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
	if entry.DBApplyPlan.InsertStorageRef["owner_type"] != "task_asset" ||
		entry.DBApplyPlan.InsertStorageRef["upload_request_id"] != requestID ||
		entry.Rollback.RestoreTaskAsset["storage_ref_id"] != entry.Rollback.OriginalStorageRef["ref_id"] ||
		entry.Rollback.RestoreUpload["bound_ref_id"] != entry.Rollback.OriginalStorageRef["ref_id"] {
		return errors.New("before/after rows do not share one exact upload and storage identity")
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
	return nil
}

func productionOSS() (*service.OSSDirectService, error) {
	if strings.ToLower(strings.TrimSpace(os.Getenv("UPLOAD_STORAGE_PROVIDER"))) != "oss" {
		return nil, errors.New("production recovery requires UPLOAD_STORAGE_PROVIDER=oss")
	}
	oss := service.NewOSSDirectService(service.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        os.Getenv("OSS_ENDPOINT"),
		Bucket:          os.Getenv("OSS_BUCKET"),
		AccessKeyID:     os.Getenv("OSS_ACCESS_KEY_ID"),
		AccessKeySecret: os.Getenv("OSS_ACCESS_KEY_SECRET"),
		PublicEndpoint:  os.Getenv("OSS_PUBLIC_ENDPOINT"),
		HTTPTimeout:     10 * time.Minute,
	})
	if !oss.Enabled() {
		return nil, errors.New("production OSS configuration is incomplete")
	}
	return oss, nil
}

func sourcePath(root string, entry recoveryEntry) string {
	return filepath.Join(
		root,
		fmt.Sprintf(
			"task-asset-%d-%s.jpg",
			entry.SourceTaskAssetID,
			entry.SourceSHA256,
		),
	)
}

func verifyLocalSource(path string, entry recoveryEntry) error {
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Mode()&os.ModeSymlink != 0 ||
		info.Size() != entry.SourceSize {
		return fmt.Errorf(
			"task_asset %d frozen source file is missing or drifted",
			entry.MissingTaskAssetID,
		)
	}
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	digest := sha256.New()
	written, err := io.Copy(digest, file)
	if err != nil {
		return err
	}
	if written != entry.SourceSize ||
		hex.EncodeToString(digest.Sum(nil)) != entry.SourceSHA256 {
		return fmt.Errorf(
			"task_asset %d frozen source SHA-256 is drifted",
			entry.MissingTaskAssetID,
		)
	}
	return nil
}

func verifyRemoteObject(ctx context.Context, objects objectStore, entry recoveryEntry) error {
	info, exists, err := objects.StatObject(ctx, entry.TargetObjectKey)
	if err != nil {
		return err
	}
	if !exists || info.ContentLength != entry.SourceSize {
		return fmt.Errorf("task_asset %d production object size is missing or drifted", entry.MissingTaskAssetID)
	}
	reader, err := objects.OpenObject(ctx, entry.TargetObjectKey)
	if err != nil {
		return err
	}
	digest := sha256.New()
	written, copyErr := io.Copy(digest, reader)
	closeErr := reader.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written != entry.SourceSize ||
		hex.EncodeToString(digest.Sum(nil)) != entry.SourceSHA256 {
		return fmt.Errorf("task_asset %d production object SHA-256 is drifted", entry.MissingTaskAssetID)
	}
	return nil
}

func deleteVerifiedObjectIfPresent(ctx context.Context, objects objectStore, entry recoveryEntry) (bool, error) {
	_, exists, err := objects.StatObject(ctx, entry.TargetObjectKey)
	if err != nil || !exists {
		return false, err
	}
	if err := deleteVerifiedObject(ctx, objects, entry, false); err != nil {
		return false, err
	}
	return true, nil
}

func deleteVerifiedObject(ctx context.Context, objects objectStore, entry recoveryEntry, bestEffort bool) error {
	if err := verifyRemoteObject(ctx, objects, entry); err != nil {
		if bestEffort {
			return objects.DeleteObject(ctx, entry.TargetObjectKey)
		}
		return err
	}
	return objects.DeleteObject(ctx, entry.TargetObjectKey)
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
	return lockedState{
		taskAsset: taskAsset, uploadRequest: upload,
		originalStatus: originalStatus, targetStorageRef: target,
	}, nil
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
		SET bound_ref_id=?,checksum_hint=?,file_size=?,status=?,session_status=?,
		    updated_at=updated_at
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
		default:
			result[key] = typed
		}
	}
	return result
}

func normalizeTime(value string) string {
	value = strings.TrimSpace(value)
	for _, layout := range []string{
		"2006-01-02 15:04:05",
		time.RFC3339Nano,
	} {
		if parsed, err := time.Parse(layout, value); err == nil {
			return parsed.UTC().Format("2006-01-02 15:04:05")
		}
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
	case json.Number:
		parsed, _ := typed.Int64()
		return parsed
	case int64:
		return typed
	case int:
		return int64(typed)
	default:
		var parsed int64
		_, _ = fmt.Sscan(fmt.Sprint(value), &parsed)
		return parsed
	}
}

func isLoopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func isCloneDatabaseName(name string) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	return strings.HasPrefix(lower, "ab_") || strings.HasSuffix(lower, "_b")
}

func sha256Hex(value []byte) string {
	digest := sha256.Sum256(value)
	return hex.EncodeToString(digest[:])
}

func writeNewJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return err
	}
	if _, err := file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	return file.Close()
}
