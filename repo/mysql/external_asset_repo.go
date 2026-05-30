package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type externalAssetRepo struct{ db *DB }

func NewExternalAssetRepo(db *DB) repo.ExternalAssetRepo { return &externalAssetRepo{db: db} }

const externalAssetSelect = `
	SELECT id, provider, kind, driver, mount_path, origin_path_hash, origin_path, parent_path,
	       file_name, file_ext, mime_type, file_size, is_dir, status, raw_url, raw_url_expires_at,
	       direct_url_status, oss_original_key, oss_preview_key, oss_thumb_key, oss_sync_status,
	       preview_status, last_seen_at, last_scanned_at, last_link_checked_at, last_prepare_error,
	       searchable_text, created_at, updated_at
	  FROM external_asset_records`

func (r *externalAssetRepo) Search(ctx context.Context, query domain.ExternalAssetSearchQuery) ([]*domain.ExternalAssetRecord, int64, error) {
	query = query.Normalized()
	where, args := buildExternalAssetWhere(query)
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_asset_records`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count external assets: %w", err)
	}
	args = append(args, (query.Page-1)*query.Size, query.Size)
	rows, err := r.db.db.QueryContext(ctx, externalAssetSelect+where+`
		ORDER BY updated_at DESC, id DESC
		LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search external assets: %w", err)
	}
	defer rows.Close()
	items, err := scanExternalAssetRows(rows)
	return items, total, err
}

func buildExternalAssetWhere(query domain.ExternalAssetSearchQuery) (string, []interface{}) {
	clauses := []string{
		`status <> 'missing'`,
		`is_dir = 0`,
		`origin_path NOT LIKE '%/@eaDir/%'`,
		`origin_path NOT LIKE '%/#recycle/%'`,
		`file_name NOT LIKE '%@Syno%'`,
	}
	args := []interface{}{}
	if query.Keyword != "" {
		like := "%" + strings.TrimSpace(query.Keyword) + "%"
		clauses = append(clauses, `(file_name LIKE ? OR origin_path LIKE ? OR parent_path LIKE ? OR searchable_text LIKE ?)`)
		args = append(args, like, like, like, like)
	}
	if query.Kind != "" {
		clauses = append(clauses, `kind = ?`)
		args = append(args, string(query.Kind))
	}
	if query.MountPath != "" {
		clauses = append(clauses, `mount_path = ?`)
		args = append(args, query.MountPath)
	}
	if query.CreatedFrom != nil {
		clauses = append(clauses, `updated_at >= ?`)
		args = append(args, *query.CreatedFrom)
	}
	if query.CreatedTo != nil {
		clauses = append(clauses, `updated_at <= ?`)
		args = append(args, *query.CreatedTo)
	}
	clauses, args = appendAssetFormatCategoryWhere(
		clauses,
		args,
		[]string{`LOWER(file_name)`, `LOWER(COALESCE(file_ext, ''))`},
		`LOWER(COALESCE(mime_type, ''))`,
		query.FormatCategory,
	)
	return " WHERE " + strings.Join(clauses, " AND "), args
}

func (r *externalAssetRepo) Upsert(ctx context.Context, item domain.ExternalAssetUpsert) (*domain.ExternalAssetRecord, error) {
	item = item.Normalized()
	if item.OriginPath == "" || item.FileName == "" {
		return nil, fmt.Errorf("external asset origin_path and file_name are required")
	}
	hash := domain.ExternalAssetOriginHash(item.Provider, item.MountPath, item.OriginPath)
	_, err := r.db.db.ExecContext(ctx, `
		INSERT INTO external_asset_records (
		  provider, kind, driver, mount_path, origin_path_hash, origin_path, parent_path,
		  file_name, file_ext, mime_type, file_size, is_dir, status, raw_url,
		  oss_sync_status, preview_status, last_seen_at, last_scanned_at, searchable_text
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, 'indexed', ?, 'none', 'none', ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  kind = VALUES(kind),
		  driver = VALUES(driver),
		  mount_path = VALUES(mount_path),
		  origin_path = VALUES(origin_path),
		  parent_path = VALUES(parent_path),
		  file_name = VALUES(file_name),
		  file_ext = VALUES(file_ext),
		  mime_type = VALUES(mime_type),
		  file_size = VALUES(file_size),
		  is_dir = VALUES(is_dir),
		  status = 'indexed',
		  raw_url = CASE WHEN VALUES(raw_url) <> '' THEN VALUES(raw_url) ELSE raw_url END,
		  last_seen_at = VALUES(last_seen_at),
		  last_scanned_at = VALUES(last_scanned_at),
		  searchable_text = VALUES(searchable_text)`,
		item.Provider, string(item.Kind), item.Driver, item.MountPath, hash, item.OriginPath, item.ParentPath,
		item.FileName, item.FileExt, item.MimeType, item.FileSize, item.IsDir, item.RawURL,
		item.ScannedAt, item.ScannedAt, item.SearchableText)
	if err != nil {
		return nil, fmt.Errorf("upsert external asset: %w", err)
	}
	return r.getByHash(ctx, hash)
}

func (r *externalAssetRepo) getByHash(ctx context.Context, hash string) (*domain.ExternalAssetRecord, error) {
	row := r.db.db.QueryRowContext(ctx, externalAssetSelect+` WHERE origin_path_hash = ?`, hash)
	return scanExternalAssetRow(row)
}

func (r *externalAssetRepo) GetByID(ctx context.Context, id int64) (*domain.ExternalAssetRecord, error) {
	row := r.db.db.QueryRowContext(ctx, externalAssetSelect+` WHERE id = ?`, id)
	return scanExternalAssetRow(row)
}

func (r *externalAssetRepo) CreateSyncRun(ctx context.Context, run *domain.ExternalAssetSyncRun) (int64, error) {
	if run == nil {
		return 0, fmt.Errorf("external asset sync run is required")
	}
	status := strings.TrimSpace(run.Status)
	if status == "" {
		status = domain.ExternalAssetSyncRunStatusRunning
	}
	startedAt := run.StartedAt
	if startedAt.IsZero() {
		startedAt = time.Now().UTC()
	}
	result, err := r.db.db.ExecContext(ctx, `
		INSERT INTO external_asset_sync_runs (
		  run_type, mount_path, keyword, status, scanned_count, upserted_count, error_message, started_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(run.RunType), strings.TrimSpace(run.MountPath), strings.TrimSpace(run.Keyword), status,
		run.ScannedCount, run.UpsertedCount, nullableString(run.ErrorMessage), startedAt)
	if err != nil {
		return 0, fmt.Errorf("create external asset sync run: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("create external asset sync run id: %w", err)
	}
	return id, nil
}

func (r *externalAssetRepo) FinishSyncRun(ctx context.Context, id int64, status string, scannedCount, upsertedCount int, errorMessage string) error {
	if id <= 0 {
		return nil
	}
	status = strings.TrimSpace(status)
	if status == "" {
		status = domain.ExternalAssetSyncRunStatusCompleted
	}
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_sync_runs
		   SET status = ?, scanned_count = ?, upserted_count = ?, error_message = ?, finished_at = UTC_TIMESTAMP()
		 WHERE id = ?`,
		status, scannedCount, upsertedCount, nullableString(errorMessage), id)
	if err != nil {
		return fmt.Errorf("finish external asset sync run: %w", err)
	}
	return nil
}

func (r *externalAssetRepo) MarkMountMissingBefore(ctx context.Context, mountPath string, scannedBefore time.Time) error {
	mountPath = strings.TrimSpace(mountPath)
	if mountPath == "" || scannedBefore.IsZero() {
		return nil
	}
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET status = 'missing'
		 WHERE mount_path = ?
		   AND status <> 'missing'
		   AND (last_scanned_at IS NULL OR last_scanned_at < ?)`,
		mountPath, scannedBefore)
	if err != nil {
		return fmt.Errorf("mark external mount missing: %w", err)
	}
	return nil
}

func (r *externalAssetRepo) UpdateDirectURL(ctx context.Context, id int64, rawURL string, expiresAt *time.Time, status string) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET raw_url = ?, raw_url_expires_at = ?, direct_url_status = ?, last_link_checked_at = UTC_TIMESTAMP()
		 WHERE id = ?`, strings.TrimSpace(rawURL), expiresAt, strings.TrimSpace(status), id)
	if err != nil {
		return fmt.Errorf("update external direct url: %w", err)
	}
	return nil
}

func (r *externalAssetRepo) MarkOSSPreparePending(ctx context.Context, id int64) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET oss_sync_status = CASE WHEN oss_sync_status = 'ready' THEN oss_sync_status ELSE 'pending' END,
		       last_prepare_error = NULL
		 WHERE id = ?`, id)
	return wrapExternalAssetUpdate(err, "mark external oss pending")
}

func (r *externalAssetRepo) MarkPreviewPreparePending(ctx context.Context, id int64) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET preview_status = CASE WHEN preview_status = 'ready' THEN preview_status ELSE 'pending' END,
		       last_prepare_error = NULL
		 WHERE id = ?`, id)
	return wrapExternalAssetUpdate(err, "mark external preview pending")
}

func (r *externalAssetRepo) ListDirectURLRefreshCandidates(ctx context.Context, limit int, staleBefore time.Time) ([]*domain.ExternalAssetRecord, error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	rows, err := r.db.db.QueryContext(ctx, externalAssetSelect+`
		WHERE kind = 'netdisk'
		  AND is_dir = 0
		  AND (
		    raw_url IS NULL
		    OR raw_url = ''
		    OR direct_url_status IN ('missing', 'failed')
		    OR last_link_checked_at IS NULL
		    OR last_link_checked_at <= ?
		  )
		ORDER BY COALESCE(last_link_checked_at, '1970-01-01') ASC, updated_at DESC, id DESC
		LIMIT ?`, staleBefore.UTC(), limit)
	if err != nil {
		return nil, fmt.Errorf("list external direct url refresh candidates: %w", err)
	}
	defer rows.Close()
	return scanExternalAssetRows(rows)
}

func (r *externalAssetRepo) ListPendingOSS(ctx context.Context, limit int) ([]*domain.ExternalAssetRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.db.QueryContext(ctx, externalAssetSelect+`
		WHERE kind = 'nas_local'
		  AND is_dir = 0
		  AND oss_sync_status IN ('pending', 'failed')
		ORDER BY CASE oss_sync_status WHEN 'pending' THEN 0 ELSE 1 END, updated_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list external oss pending: %w", err)
	}
	defer rows.Close()
	return scanExternalAssetRows(rows)
}

func (r *externalAssetRepo) ListPendingPreview(ctx context.Context, limit int) ([]*domain.ExternalAssetRecord, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}
	rows, err := r.db.db.QueryContext(ctx, externalAssetSelect+`
		WHERE is_dir = 0
		  AND preview_status IN ('pending', 'failed')
		ORDER BY CASE preview_status WHEN 'pending' THEN 0 ELSE 1 END, updated_at DESC, id DESC
		LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list external preview pending: %w", err)
	}
	defer rows.Close()
	return scanExternalAssetRows(rows)
}

func (r *externalAssetRepo) MarkOSSReady(ctx context.Context, id int64, objectKey string) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET oss_original_key = ?, oss_sync_status = 'ready', last_prepare_error = NULL
		 WHERE id = ?`, strings.TrimSpace(objectKey), id)
	return wrapExternalAssetUpdate(err, "mark external oss ready")
}

func (r *externalAssetRepo) MarkPreviewReady(ctx context.Context, id int64, previewKey string) error {
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET oss_preview_key = ?, preview_status = 'ready', last_prepare_error = NULL
		 WHERE id = ?`, strings.TrimSpace(previewKey), id)
	return wrapExternalAssetUpdate(err, "mark external preview ready")
}

func (r *externalAssetRepo) MarkPrepareFailed(ctx context.Context, id int64, target, message string) error {
	target = strings.TrimSpace(target)
	column := "oss_sync_status"
	if target == "preview" {
		column = "preview_status"
	}
	_, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET `+column+` = 'failed', last_prepare_error = ?
		 WHERE id = ?`, strings.TrimSpace(message), id)
	return wrapExternalAssetUpdate(err, "mark external prepare failed")
}

func wrapExternalAssetUpdate(err error, op string) error {
	if err != nil {
		return fmt.Errorf("%s: %w", op, err)
	}
	return nil
}

func scanExternalAssetRows(rows *sql.Rows) ([]*domain.ExternalAssetRecord, error) {
	out := []*domain.ExternalAssetRecord{}
	for rows.Next() {
		item, err := scanExternalAssetScanner(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func scanExternalAssetRow(row *sql.Row) (*domain.ExternalAssetRecord, error) {
	item, err := scanExternalAssetScanner(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	return item, err
}

type externalAssetScanner interface {
	Scan(dest ...interface{}) error
}

func scanExternalAssetScanner(s externalAssetScanner) (*domain.ExternalAssetRecord, error) {
	var item domain.ExternalAssetRecord
	var parentPath, rawURL, directStatus, ossOriginal, ossPreview, ossThumb, lastErr, searchable sql.NullString
	var rawExpires, lastSeen, lastScanned, lastLink sql.NullTime
	var isDir bool
	if err := s.Scan(
		&item.ID, &item.Provider, &item.Kind, &item.Driver, &item.MountPath, &item.OriginPathHash, &item.OriginPath, &parentPath,
		&item.FileName, &item.FileExt, &item.MimeType, &item.FileSize, &isDir, &item.Status, &rawURL, &rawExpires,
		&directStatus, &ossOriginal, &ossPreview, &ossThumb, &item.OSSSyncStatus,
		&item.PreviewStatus, &lastSeen, &lastScanned, &lastLink, &lastErr,
		&searchable, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan external asset: %w", err)
	}
	item.ResourceID = domain.ExternalAssetResourceID(item.ID)
	item.ParentPath = fromNullStringValue(parentPath)
	item.RawURL = fromNullStringValue(rawURL)
	item.RawURLExpiresAt = fromNullTime(rawExpires)
	item.DirectURLStatus = fromNullStringValue(directStatus)
	item.OSSOriginalKey = fromNullStringValue(ossOriginal)
	item.OSSPreviewKey = fromNullStringValue(ossPreview)
	item.OSSThumbKey = fromNullStringValue(ossThumb)
	item.LastSeenAt = fromNullTime(lastSeen)
	item.LastScannedAt = fromNullTime(lastScanned)
	item.LastLinkCheckedAt = fromNullTime(lastLink)
	item.LastPrepareError = fromNullStringValue(lastErr)
	item.SearchableText = fromNullStringValue(searchable)
	item.IsDir = isDir
	return &item, nil
}

func fromNullStringValue(value sql.NullString) string {
	if !value.Valid {
		return ""
	}
	return value.String
}

func nullableString(value string) interface{} {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return value
}
