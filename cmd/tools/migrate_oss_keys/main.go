package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/service"
)

var asciiSafeKeyPattern = regexp.MustCompile(`^[A-Za-z0-9._/-]+$`)
var extensionPattern = regexp.MustCompile(`^[A-Za-z0-9]{1,10}$`)

type migrationRow struct {
	ID               int64
	TaskID           int64
	TaskRef          string
	AssetNo          string
	VersionNo        int
	AssetType        string
	StorageKey       string
	OriginalFilename string
	CreatedAt        time.Time
}

type migrationSummary struct {
	Scanned             int `json:"scanned"`
	Migrated            int `json:"migrated"`
	Planned             int `json:"planned,omitempty"`
	SkippedAlreadyASCII int `json:"skipped_already_ascii"`
	OrphanNoOSSObject   int `json:"orphan_no_oss_object"`
	Errors              int `json:"errors"`
}

type migrationEvent struct {
	RowID       int64  `json:"row_id,omitempty"`
	OldKey      string `json:"old_key,omitempty"`
	NewKey      string `json:"new_key,omitempty"`
	Status      string `json:"status"`
	Error       string `json:"error,omitempty"`
	DryRun      bool   `json:"dry_run,omitempty"`
	DeletedOld  bool   `json:"deleted_old,omitempty"`
	RowsUpdated int64  `json:"rows_updated,omitempty"`
	MigratedAt  string `json:"migrated_at,omitempty"`
}

type lockedRow interface {
	Row() migrationRow
	UpdateStorageKey(ctx context.Context, oldKey, newKey string) (bool, error)
	Commit() error
	Rollback() error
}

type migrationStore interface {
	ListDirtyRows(ctx context.Context, limit int) ([]migrationRow, error)
	LockRow(ctx context.Context, id int64) (lockedRow, error)
}

type ossClient interface {
	HeadObject(ctx context.Context, objectKey string) (bool, error)
	CopyObject(ctx context.Context, srcKey, dstKey string) error
	DeleteObject(ctx context.Context, objectKey string) error
}

type migrator struct {
	store     migrationStore
	oss       ossClient
	dryRun    bool
	limit     int
	sleep     time.Duration
	deleteOld bool
	verbose   bool
	logEvent  func(migrationEvent)
}

func main() {
	var dsnFlag string
	var envPath string
	var dryRun bool
	var limit int
	var sleepMS int
	var deleteOld bool
	var verbose bool
	flag.StringVar(&dsnFlag, "dsn", "", "MySQL DSN. If empty, MYSQL_* or DB_* env is used.")
	flag.StringVar(&envPath, "env", "", "Optional env file to load before reading MYSQL/OSS config.")
	flag.BoolVar(&dryRun, "dry-run", true, "Dry-run mode. Must be explicitly set to false to write.")
	flag.IntVar(&limit, "limit", 0, "Maximum rows to scan; 0 means no limit.")
	flag.IntVar(&sleepMS, "sleep-ms", 50, "Throttle delay between rows.")
	flag.BoolVar(&deleteOld, "delete-old", false, "Delete old OSS object after successful copy and DB update.")
	flag.BoolVar(&verbose, "verbose", false, "Print per-row events to stderr.")
	flag.Parse()

	if envPath != "" {
		if err := loadEnvFile(envPath); err != nil {
			exitErr(fmt.Errorf("load env: %w", err))
		}
	}
	dsn := strings.TrimSpace(dsnFlag)
	if dsn == "" {
		var err error
		dsn, err = mysqlDSNFromEnv()
		if err != nil {
			exitErr(err)
		}
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		exitErr(err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		exitErr(fmt.Errorf("mysql ping: %w", err))
	}

	oss := service.NewOSSDirectService(service.OSSDirectConfig{
		Enabled:         true,
		Endpoint:        getenv("OSS_ENDPOINT", ""),
		PublicEndpoint:  getenv("OSS_PUBLIC_ENDPOINT", ""),
		Bucket:          getenv("OSS_BUCKET", ""),
		AccessKeyID:     getenv("OSS_ACCESS_KEY_ID", ""),
		AccessKeySecret: getenv("OSS_ACCESS_KEY_SECRET", ""),
		PresignExpiry:   15 * time.Minute,
		PartSize:        10 * 1024 * 1024,
	})
	if !oss.Enabled() {
		exitErr(fmt.Errorf("OSS config is incomplete"))
	}

	logPath := fmt.Sprintf("/tmp/oss_key_migration_%d.jsonl", time.Now().Unix())
	logFile, err := os.Create(logPath)
	if err != nil {
		exitErr(fmt.Errorf("create log file: %w", err))
	}
	defer logFile.Close()
	encoder := json.NewEncoder(logFile)
	logEvent := func(event migrationEvent) {
		_ = encoder.Encode(event)
		if verbose {
			raw, _ := json.Marshal(event)
			fmt.Fprintln(os.Stderr, string(raw))
		}
	}

	runner := &migrator{
		store:     &mysqlMigrationStore{db: db},
		oss:       oss,
		dryRun:    dryRun,
		limit:     limit,
		sleep:     time.Duration(sleepMS) * time.Millisecond,
		deleteOld: deleteOld,
		verbose:   verbose,
		logEvent:  logEvent,
	}
	summary, err := runner.Run(context.Background())
	if err != nil {
		exitErr(err)
	}
	raw, _ := json.Marshal(summary)
	fmt.Println(string(raw))
	fmt.Fprintf(os.Stderr, "migration_log=%s\n", logPath)
}

func (m *migrator) Run(ctx context.Context) (migrationSummary, error) {
	var summary migrationSummary
	rows, err := m.store.ListDirtyRows(ctx, m.limit)
	if err != nil {
		return summary, err
	}
	for _, listed := range rows {
		summary.Scanned++
		errorsBefore := summary.Errors
		orphansBefore := summary.OrphanNoOSSObject
		event := migrationEvent{RowID: listed.ID, OldKey: listed.StorageKey, DryRun: m.dryRun}
		lock, err := m.store.LockRow(ctx, listed.ID)
		if err != nil {
			summary.Errors++
			event.Status = "error_lock"
			event.Error = err.Error()
			m.emit(event)
			return summary, nil
		}
		committed := false
		func() {
			defer func() {
				if !committed {
					_ = lock.Rollback()
				}
			}()
			row := lock.Row()
			event.OldKey = row.StorageKey
			if asciiSafeKeyPattern.MatchString(row.StorageKey) {
				summary.SkippedAlreadyASCII++
				event.Status = "skipped_already_ascii"
				m.emit(event)
				return
			}
			newKey := deterministicASCIIKey(row)
			event.NewKey = newKey
			exists, err := m.oss.HeadObject(ctx, row.StorageKey)
			if err != nil {
				summary.Errors++
				event.Status = "error_head"
				event.Error = err.Error()
				m.emit(event)
				return
			}
			if !exists {
				summary.OrphanNoOSSObject++
				event.Status = "orphan_no_oss_object"
				m.emit(event)
				return
			}
			if m.dryRun {
				summary.Planned++
				event.Status = "planned_migrate"
				m.emit(event)
				return
			}
			if err := m.oss.CopyObject(ctx, row.StorageKey, newKey); err != nil {
				summary.Errors++
				event.Status = "error_copy"
				event.Error = err.Error()
				m.emit(event)
				return
			}
			updated, err := lock.UpdateStorageKey(ctx, row.StorageKey, newKey)
			if err != nil {
				summary.Errors++
				event.Status = "error_update"
				event.Error = err.Error()
				m.emit(event)
				return
			}
			if !updated {
				summary.Errors++
				event.Status = "conflict_update"
				m.emit(event)
				return
			}
			if m.deleteOld {
				if err := m.oss.DeleteObject(ctx, row.StorageKey); err != nil {
					summary.Errors++
					event.Status = "error_delete_old"
					event.Error = err.Error()
					m.emit(event)
					return
				}
				event.DeletedOld = true
			}
			if err := lock.Commit(); err != nil {
				summary.Errors++
				event.Status = "error_commit"
				event.Error = err.Error()
				m.emit(event)
				return
			}
			committed = true
			summary.Migrated++
			event.Status = "migrated"
			event.RowsUpdated = 1
			event.MigratedAt = time.Now().UTC().Format(time.RFC3339Nano)
			m.emit(event)
		}()
		if summary.Errors > errorsBefore || summary.OrphanNoOSSObject > orphansBefore {
			return summary, nil
		}
		if m.sleep > 0 {
			select {
			case <-ctx.Done():
				return summary, ctx.Err()
			case <-time.After(m.sleep):
			}
		}
	}
	return summary, nil
}

func (m *migrator) emit(event migrationEvent) {
	if m.logEvent != nil {
		m.logEvent(event)
	}
}

type mysqlMigrationStore struct {
	db *sql.DB
}

func (s *mysqlMigrationStore) ListDirtyRows(ctx context.Context, limit int) ([]migrationRow, error) {
	query := `
SELECT ta.id, ta.task_id, COALESCE(t.task_no, ''), COALESCE(da.asset_no, ''),
       COALESCE(ta.asset_version_no, ta.version_no, 1), ta.asset_type, ta.storage_key,
       COALESCE(ta.original_filename, ta.file_name, ''), ta.created_at
FROM task_assets ta
LEFT JOIN tasks t ON t.id = ta.task_id
LEFT JOIN design_assets da ON da.id = ta.asset_id
WHERE ta.storage_key IS NOT NULL AND ta.storage_key <> ''
  AND ta.storage_key REGEXP '[^A-Za-z0-9._/-]'
ORDER BY ta.created_at ASC, ta.id ASC`
	args := []interface{}{}
	if limit > 0 {
		query += " LIMIT ?"
		args = append(args, limit)
	}
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	return scanMigrationRows(rows)
}

func (s *mysqlMigrationStore) LockRow(ctx context.Context, id int64) (lockedRow, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	row := tx.QueryRowContext(ctx, `
SELECT ta.id, ta.task_id, COALESCE(t.task_no, ''), COALESCE(da.asset_no, ''),
       COALESCE(ta.asset_version_no, ta.version_no, 1), ta.asset_type, ta.storage_key,
       COALESCE(ta.original_filename, ta.file_name, ''), ta.created_at
FROM task_assets ta
LEFT JOIN tasks t ON t.id = ta.task_id
LEFT JOIN design_assets da ON da.id = ta.asset_id
WHERE ta.id = ?
FOR UPDATE`, id)
	migrationRow, err := scanMigrationRow(row)
	if err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	return &mysqlLockedRow{tx: tx, row: migrationRow}, nil
}

type mysqlLockedRow struct {
	tx  *sql.Tx
	row migrationRow
}

func (r *mysqlLockedRow) Row() migrationRow {
	return r.row
}

func (r *mysqlLockedRow) UpdateStorageKey(ctx context.Context, oldKey, newKey string) (bool, error) {
	res, err := r.tx.ExecContext(ctx, `UPDATE task_assets SET storage_key = ? WHERE id = ? AND storage_key = ?`, newKey, r.row.ID, oldKey)
	if err != nil {
		return false, err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	return affected == 1, nil
}

func (r *mysqlLockedRow) Commit() error {
	return r.tx.Commit()
}

func (r *mysqlLockedRow) Rollback() error {
	return r.tx.Rollback()
}

type rowScanner interface {
	Scan(dest ...interface{}) error
}

func scanMigrationRows(rows *sql.Rows) ([]migrationRow, error) {
	var result []migrationRow
	for rows.Next() {
		row, err := scanMigrationRow(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, rows.Err()
}

func scanMigrationRow(scanner rowScanner) (migrationRow, error) {
	var row migrationRow
	if err := scanner.Scan(
		&row.ID,
		&row.TaskID,
		&row.TaskRef,
		&row.AssetNo,
		&row.VersionNo,
		&row.AssetType,
		&row.StorageKey,
		&row.OriginalFilename,
		&row.CreatedAt,
	); err != nil {
		return row, err
	}
	return row, nil
}

func deterministicASCIIKey(row migrationRow) string {
	taskRef := safePathSegment(firstNonEmpty(row.TaskRef, fmt.Sprintf("task-%d", row.TaskID)))
	assetNo := safePathSegment(firstNonEmpty(row.AssetNo, fmt.Sprintf("asset-%d", row.ID)))
	versionNo := row.VersionNo
	if versionNo <= 0 {
		versionNo = 1
	}
	role := assetTypeToSubdir(domain.TaskAssetType(row.AssetType))
	sum := sha256.Sum256([]byte(row.StorageKey))
	base := "migrated_" + hex.EncodeToString(sum[:])[:16]
	if ext := validExtension(firstNonEmpty(row.OriginalFilename, row.StorageKey)); ext != "" {
		base += "." + ext
	}
	return fmt.Sprintf("tasks/%s/assets/%s/v%d/%s/%s", taskRef, assetNo, versionNo, role, base)
}

func assetTypeToSubdir(assetType domain.TaskAssetType) string {
	switch {
	case assetType.IsSource():
		return "source"
	case assetType.IsDelivery():
		return "delivery"
	case assetType.IsPreview():
		return "preview"
	case assetType.IsDesignThumb():
		return "design_thumb"
	case assetType.IsReference():
		return "derived"
	default:
		return "derived"
	}
}

func validExtension(filename string) string {
	filename = strings.TrimSpace(filename)
	idx := strings.LastIndexAny(filename, `/\`)
	if idx >= 0 {
		filename = filename[idx+1:]
	}
	dot := strings.LastIndex(filename, ".")
	if dot < 0 || dot == len(filename)-1 {
		return ""
	}
	ext := filename[dot+1:]
	if extensionPattern.MatchString(ext) {
		return ext
	}
	return ""
}

func safePathSegment(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return "unknown"
	}
	var b strings.Builder
	lastUnderscore := false
	for _, r := range value {
		ok := (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '.' || r == '_' || r == '-'
		if ok {
			b.WriteRune(r)
			lastUnderscore = false
			continue
		}
		if !lastUnderscore {
			b.WriteByte('_')
			lastUnderscore = true
		}
	}
	out := strings.Trim(b.String(), "._-")
	if out == "" {
		return "unknown"
	}
	return out
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func loadEnvFile(path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		value = strings.Trim(value, `"'`)
		if key != "" {
			_ = os.Setenv(key, value)
		}
	}
	return scanner.Err()
}

func mysqlDSNFromEnv() (string, error) {
	user := getenv("MYSQL_USER", getenv("DB_USER", ""))
	pass := getenv("MYSQL_PASSWORD", getenv("DB_PASS", ""))
	host := getenv("MYSQL_HOST", getenv("DB_HOST", "127.0.0.1"))
	port := getenv("MYSQL_PORT", getenv("DB_PORT", "3306"))
	db := getenv("MYSQL_DATABASE", getenv("DB_NAME", getenv("MYSQL_DB", "")))
	if user == "" || db == "" {
		return "", fmt.Errorf("missing MYSQL_USER/DB_USER or MYSQL_DATABASE/DB_NAME")
	}
	query := url.Values{}
	query.Set("parseTime", "true")
	query.Set("charset", "utf8mb4")
	query.Set("collation", "utf8mb4_unicode_ci")
	query.Set("multiStatements", "false")
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?%s", user, pass, host, port, db, query.Encode()), nil
}

func getenv(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func exitErr(err error) {
	fmt.Fprintln(os.Stderr, "error:", err)
	os.Exit(1)
}
