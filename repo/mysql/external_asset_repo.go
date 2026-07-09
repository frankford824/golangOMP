package mysqlrepo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"errors"
	"fmt"
	"path"
	"strings"
	"time"
	"unicode"

	"workflow/domain"
	"workflow/repo"
)

type externalAssetRepo struct{ db *DB }

type sqlExecContext interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

type sqlQueryRowContext interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func retryExternalAssetLockConflict(ctx context.Context, run func() error) error {
	const maxAttempts = 4
	var lastErr error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		lastErr = run()
		if lastErr == nil {
			return nil
		}
		if !isMySQLLockConflict(lastErr) || attempt == maxAttempts || ctx.Err() != nil {
			return lastErr
		}
		timer := time.NewTimer(time.Duration(attempt) * 150 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return lastErr
}

func NewExternalAssetRepo(db *DB) repo.ExternalAssetRepo { return &externalAssetRepo{db: db} }

const externalAssetSelectColumns = `
	SELECT id, provider, kind, driver, mount_path, origin_path_hash, origin_path, parent_path,
	       file_name, file_ext, mime_type, file_size, is_dir, status, raw_url, raw_url_expires_at,
	       direct_url_status, oss_original_key, oss_preview_key, oss_thumb_key, oss_sync_status,
	       preview_status, last_seen_at, last_scanned_at, last_link_checked_at, last_prepare_error,
	       searchable_text, created_at, updated_at`

const externalAssetSelect = externalAssetSelectColumns + `
	  FROM external_asset_records`

const externalAssetBrowseSelect = externalAssetSelectColumns + `
	  FROM external_asset_records FORCE INDEX (idx_external_asset_browse_parent)`

func (r *externalAssetRepo) Search(ctx context.Context, query domain.ExternalAssetSearchQuery) ([]*domain.ExternalAssetRecord, int64, error) {
	query = query.Normalized()
	where, args, orderBy := buildExternalAssetWhere(query)
	var total int64
	usedFallback := false
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_asset_records`+where, args...).Scan(&total); err != nil {
		if strings.TrimSpace(query.Keyword) == "" || !isMySQLFullTextIndexMissing(err) {
			return nil, 0, fmt.Errorf("count external assets: %w", err)
		}
		where, args, orderBy = buildExternalAssetLikeWhere(query)
		usedFallback = true
		if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_asset_records`+where, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count external assets fallback: %w", err)
		}
	}
	if !usedFallback && total == 0 && strings.TrimSpace(query.Keyword) != "" && externalAssetBooleanQuery(query.Keyword) != "" {
		where, args, orderBy = buildExternalAssetLikeWhere(query)
		if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_asset_records`+where, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count external assets fallback: %w", err)
		}
	}
	args = append(args, (query.Page-1)*query.Size, query.Size)
	rows, err := r.db.db.QueryContext(ctx, externalAssetSelect+where+orderBy+`
		LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("search external assets: %w", err)
	}
	defer rows.Close()
	items, err := scanExternalAssetRows(rows)
	return items, total, err
}

func (r *externalAssetRepo) ListDirectoryChildren(ctx context.Context, parentPath string, mountPaths []string, limit int, formatCategory domain.AssetFormatCategoryFilter) ([]domain.ExternalAssetDirectoryEntry, error) {
	parentPath = cleanExternalAssetBrowsePath(parentPath)
	if limit <= 0 || limit > 2000 {
		limit = 1000
	}
	normalizedFormat := (domain.ExternalAssetSearchQuery{FormatCategory: formatCategory}).Normalized().FormatCategory
	clauses, args := externalAssetDirectoryClauses(parentPath, mountPaths)
	args = append(args, limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT path, name, descendant_file_count, direct_file_count
		  FROM external_asset_directory_index
		 WHERE `+strings.Join(clauses, " AND ")+`
		 ORDER BY name ASC, path ASC
		 LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list external asset directory children: %w", err)
	}
	defer rows.Close()
	out := []domain.ExternalAssetDirectoryEntry{}
	for rows.Next() {
		var childPath string
		var name string
		var fileCount int64
		var directFileCount int64
		if err := rows.Scan(&childPath, &name, &fileCount, &directFileCount); err != nil {
			return nil, fmt.Errorf("scan external asset directory child: %w", err)
		}
		out = append(out, domain.ExternalAssetDirectoryEntry{
			Path:            cleanExternalAssetBrowsePath(childPath),
			Name:            name,
			FileCount:       fileCount,
			DirectFileCount: directFileCount,
		})
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate external asset directory children: %w", err)
	}
	if normalizedFormat != domain.AssetFormatCategoryAll {
		counts, err := r.countDirectoryChildrenByFormat(ctx, parentPath, mountPaths, normalizedFormat)
		if err != nil {
			return nil, err
		}
		for idx := range out {
			count := counts[out[idx].Path]
			out[idx].FileCount = count.FileCount
			out[idx].DirectFileCount = count.DirectFileCount
		}
	}
	return out, nil
}

type externalAssetDirectoryChildCount struct {
	FileCount       int64
	DirectFileCount int64
}

func (r *externalAssetRepo) countDirectoryChildrenByFormat(ctx context.Context, parentPath string, mountPaths []string, formatCategory domain.AssetFormatCategoryFilter) (map[string]externalAssetDirectoryChildCount, error) {
	parentPath = cleanExternalAssetBrowsePath(parentPath)
	clauses, args := externalAssetVisibleClauses(mountPaths)
	if parentPath != "" {
		clauses = append(clauses, `origin_path LIKE ?`)
		args = append(args, parentPath+"/%")
	}
	clauses, args = appendAssetFormatCategoryWhere(
		clauses,
		args,
		[]string{`LOWER(file_name)`, `LOWER(COALESCE(file_ext, ''))`},
		`LOWER(COALESCE(mime_type, ''))`,
		formatCategory,
	)
	childExpr := `CONCAT('/', SUBSTRING_INDEX(TRIM(LEADING '/' FROM origin_path), '/', 1))`
	queryArgs := []interface{}{}
	if parentPath != "" {
		childExpr = `CONCAT(?, '/', SUBSTRING_INDEX(SUBSTRING(origin_path, CHAR_LENGTH(?) + 2), '/', 1))`
		queryArgs = append(queryArgs, parentPath, parentPath)
	}
	queryArgs = append(queryArgs, args...)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT child_path, COUNT(*) AS file_count, COALESCE(SUM(CASE WHEN parent_path = child_path THEN 1 ELSE 0 END), 0) AS direct_file_count
		  FROM (
			SELECT `+childExpr+` AS child_path, parent_path
			  FROM external_asset_records
			 WHERE `+strings.Join(clauses, " AND ")+`
		  ) filtered
		 WHERE child_path <> ''
		 GROUP BY child_path`, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("count external asset directory children by format: %w", err)
	}
	defer rows.Close()
	counts := map[string]externalAssetDirectoryChildCount{}
	for rows.Next() {
		var childPath string
		var count externalAssetDirectoryChildCount
		if err := rows.Scan(&childPath, &count.FileCount, &count.DirectFileCount); err != nil {
			return nil, fmt.Errorf("scan external asset directory child format count: %w", err)
		}
		counts[cleanExternalAssetBrowsePath(childPath)] = count
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate external asset directory child format counts: %w", err)
	}
	return counts, nil
}

func (r *externalAssetRepo) ListDirectoryFiles(ctx context.Context, parentPath string, mountPaths []string, page int, size int, formatCategory domain.AssetFormatCategoryFilter) ([]*domain.ExternalAssetRecord, int64, error) {
	parentPath = cleanExternalAssetBrowsePath(parentPath)
	if page <= 0 {
		page = 1
	}
	if size <= 0 {
		size = 50
	}
	if size > 100 {
		size = 100
	}
	clauses, args := externalAssetVisibleClauses(mountPaths)
	clauses = append(clauses, `parent_path = ?`)
	args = append(args, parentPath)
	if normalizedFormat := (domain.ExternalAssetSearchQuery{FormatCategory: formatCategory}).Normalized().FormatCategory; normalizedFormat != domain.AssetFormatCategoryAll {
		clauses, args = appendAssetFormatCategoryWhere(
			clauses,
			args,
			[]string{`LOWER(file_name)`, `LOWER(COALESCE(file_ext, ''))`},
			`LOWER(COALESCE(mime_type, ''))`,
			normalizedFormat,
		)
	}
	where := ` WHERE ` + strings.Join(clauses, " AND ")
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM external_asset_records FORCE INDEX (idx_external_asset_browse_parent)`+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count external asset directory files: %w", err)
	}
	args = append(args, (page-1)*size, size)
	rows, err := r.db.db.QueryContext(ctx, externalAssetBrowseSelect+where+`
		ORDER BY file_name ASC, updated_at DESC, id DESC
		LIMIT ?, ?`, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list external asset directory files: %w", err)
	}
	defer rows.Close()
	items, err := scanExternalAssetRows(rows)
	return items, total, err
}

func buildExternalAssetWhere(query domain.ExternalAssetSearchQuery) (string, []interface{}, string) {
	return buildExternalAssetWhereWithMode(query, true)
}

func buildExternalAssetLikeWhere(query domain.ExternalAssetSearchQuery) (string, []interface{}, string) {
	return buildExternalAssetWhereWithMode(query, false)
}

func buildExternalAssetWhereWithMode(query domain.ExternalAssetSearchQuery, preferFullText bool) (string, []interface{}, string) {
	clauses := []string{
		`status <> 'missing'`,
		`is_dir = 0`,
		`origin_path NOT LIKE '%/@eaDir/%'`,
		`origin_path NOT LIKE '%/#recycle/%'`,
		`file_name NOT LIKE '%@Syno%'`,
	}
	args := []interface{}{}
	if query.Keyword != "" {
		if fullText := externalAssetBooleanQuery(query.Keyword); preferFullText && fullText != "" {
			clauses = append(clauses, `MATCH(file_name, origin_path, parent_path, searchable_text) AGAINST (? IN BOOLEAN MODE)`)
			args = append(args, fullText)
		} else {
			like := "%" + strings.TrimSpace(query.Keyword) + "%"
			clauses = append(clauses, `(file_name LIKE ? OR origin_path LIKE ? OR parent_path LIKE ? OR searchable_text LIKE ?)`)
			args = append(args, like, like, like, like)
		}
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
	return " WHERE " + strings.Join(clauses, " AND "), args, `
		ORDER BY updated_at DESC, id DESC`
}

func externalAssetBooleanQuery(keyword string) string {
	parts := strings.FieldsFunc(strings.TrimSpace(keyword), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r)
	})
	terms := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.Trim(part, `"' +-*~<>`)
		if part == "" {
			continue
		}
		terms = append(terms, "+"+part+"*")
	}
	if len(terms) == 0 {
		return ""
	}
	return strings.Join(terms, " ")
}

func externalAssetVisibleClauses(mountPaths []string) ([]string, []interface{}) {
	clauses := []string{
		`status <> 'missing'`,
		`is_dir = 0`,
		`origin_path NOT LIKE '%/@eaDir/%'`,
		`origin_path NOT LIKE '%/#recycle/%'`,
		`file_name NOT LIKE '%@Syno%'`,
	}
	args := []interface{}{}
	cleanMounts := make([]string, 0, len(mountPaths))
	seen := map[string]struct{}{}
	for _, raw := range mountPaths {
		mount := cleanExternalAssetBrowsePath(raw)
		if mount == "" {
			continue
		}
		if _, ok := seen[mount]; ok {
			continue
		}
		seen[mount] = struct{}{}
		cleanMounts = append(cleanMounts, mount)
	}
	if len(cleanMounts) > 0 {
		placeholders := make([]string, 0, len(cleanMounts))
		for _, mount := range cleanMounts {
			placeholders = append(placeholders, "?")
			args = append(args, mount)
		}
		clauses = append(clauses, `mount_path IN (`+strings.Join(placeholders, ",")+`)`)
	}
	return clauses, args
}

func externalAssetDirectoryClauses(parentPath string, mountPaths []string) ([]string, []interface{}) {
	clauses := []string{
		`status <> 'missing'`,
		`parent_path_hash = ?`,
	}
	args := []interface{}{externalAssetParentPathHash(parentPath)}
	cleanMounts := cleanExternalAssetMountPaths(mountPaths)
	if len(cleanMounts) > 0 {
		placeholders := make([]string, 0, len(cleanMounts))
		for _, mount := range cleanMounts {
			placeholders = append(placeholders, "?")
			args = append(args, mount)
		}
		clauses = append(clauses, `mount_path IN (`+strings.Join(placeholders, ",")+`)`)
	}
	return clauses, args
}

func cleanExternalAssetMountPaths(mountPaths []string) []string {
	cleanMounts := make([]string, 0, len(mountPaths))
	seen := map[string]struct{}{}
	for _, raw := range mountPaths {
		mount := cleanExternalAssetBrowsePath(raw)
		if mount == "" {
			continue
		}
		if _, ok := seen[mount]; ok {
			continue
		}
		seen[mount] = struct{}{}
		cleanMounts = append(cleanMounts, mount)
	}
	return cleanMounts
}

func cleanExternalAssetBrowsePath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" || value == "/" {
		return ""
	}
	cleaned := path.Clean("/" + strings.TrimLeft(value, "/"))
	if cleaned == "." || cleaned == "/" {
		return ""
	}
	return cleaned
}

func normalizeExternalAssetOriginPrefixes(prefixes []repo.ExternalAssetOriginPrefix) []repo.ExternalAssetOriginPrefix {
	out := make([]repo.ExternalAssetOriginPrefix, 0, len(prefixes))
	seen := map[string]struct{}{}
	for _, prefix := range prefixes {
		mountPath := cleanExternalAssetBrowsePath(prefix.MountPath)
		originPath := cleanExternalAssetBrowsePath(prefix.OriginPath)
		if mountPath == "" || originPath == "" {
			continue
		}
		if originPath != mountPath && !strings.HasPrefix(originPath, mountPath+"/") {
			continue
		}
		key := mountPath + "\x00" + originPath
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, repo.ExternalAssetOriginPrefix{MountPath: mountPath, OriginPath: originPath})
	}
	return out
}

func externalAssetOriginPrefixWhere(prefixes []repo.ExternalAssetOriginPrefix) (string, []interface{}) {
	prefixes = normalizeExternalAssetOriginPrefixes(prefixes)
	if len(prefixes) == 0 {
		return "", nil
	}
	clauses := make([]string, 0, len(prefixes))
	args := make([]interface{}, 0, len(prefixes)*3)
	for _, prefix := range prefixes {
		clauses = append(clauses, `(mount_path = ? AND (origin_path = ? OR origin_path LIKE ?))`)
		args = append(args, prefix.MountPath, prefix.OriginPath, prefix.OriginPath+"/%")
	}
	return strings.Join(clauses, " OR "), args
}

func externalAssetOriginPrefixPriorityOrder(prefixes []repo.ExternalAssetOriginPrefix) (string, []interface{}) {
	where, args := externalAssetOriginPrefixWhere(prefixes)
	if where == "" {
		return "", nil
	}
	return "CASE WHEN (" + where + ") THEN 0 ELSE 1 END, ", args
}

func parentExternalAssetBrowsePath(value string) string {
	value = cleanExternalAssetBrowsePath(value)
	if value == "" {
		return ""
	}
	idx := strings.LastIndex(value, "/")
	if idx <= 0 {
		return ""
	}
	return value[:idx]
}

func externalAssetDirectoryPaths(parentPath, mountPath string) []string {
	parentPath = cleanExternalAssetBrowsePath(parentPath)
	mountPath = cleanExternalAssetBrowsePath(mountPath)
	if parentPath == "" || mountPath == "" {
		return nil
	}
	if parentPath != mountPath && !strings.HasPrefix(parentPath, mountPath+"/") {
		return nil
	}
	paths := []string{}
	for current := parentPath; current != ""; current = parentExternalAssetBrowsePath(current) {
		paths = append(paths, current)
		if current == mountPath {
			break
		}
	}
	for left, right := 0, len(paths)-1; left < right; left, right = left+1, right-1 {
		paths[left], paths[right] = paths[right], paths[left]
	}
	return paths
}

func externalAssetDirectoryPathHash(provider, mountPath, dirPath string) string {
	return domain.ExternalAssetOriginHash(provider, mountPath, dirPath)
}

func externalAssetParentPathHash(parentPath string) string {
	sum := sha256.Sum256([]byte(cleanExternalAssetBrowsePath(parentPath)))
	return hex.EncodeToString(sum[:])
}

func joinExternalAssetBrowsePath(parentPath, name string) string {
	name = strings.Trim(strings.ReplaceAll(name, "\\", "/"), "/")
	if name == "" {
		return cleanExternalAssetBrowsePath(parentPath)
	}
	parentPath = cleanExternalAssetBrowsePath(parentPath)
	if parentPath == "" {
		return "/" + name
	}
	return path.Join(parentPath, name)
}

type externalAssetCountState struct {
	Provider       string
	Kind           domain.ExternalAssetKind
	Driver         string
	MountPath      string
	ParentPath     string
	FileSize       int64
	IsDir          bool
	Status         domain.ExternalAssetStatus
	OSSSyncStatus  domain.ExternalAssetOSSStatus
	PreviewStatus  domain.ExternalAssetPreviewStatus
	OSSOriginalKey string
	OSSPreviewKey  string
	OSSThumbKey    string
}

type externalAssetSourceFingerprintState struct {
	FileSize         int64
	SourceModifiedAt *time.Time
}

func getExternalAssetCountStateForUpdate(ctx context.Context, q sqlQueryRowContext, hash string) (*externalAssetCountState, error) {
	row := q.QueryRowContext(ctx, `
		SELECT provider, kind, driver, mount_path, parent_path, file_size, is_dir, status,
		       oss_sync_status, preview_status, oss_original_key, oss_preview_key, oss_thumb_key
		  FROM external_asset_records
		 WHERE origin_path_hash = ?
		 FOR UPDATE`, hash)
	var state externalAssetCountState
	var kind string
	var status string
	var ossStatus string
	var previewStatus string
	if err := row.Scan(
		&state.Provider, &kind, &state.Driver, &state.MountPath, &state.ParentPath, &state.FileSize,
		&state.IsDir, &status, &ossStatus, &previewStatus, &state.OSSOriginalKey, &state.OSSPreviewKey, &state.OSSThumbKey,
	); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load external asset count state: %w", err)
	}
	state.Kind = domain.ExternalAssetKind(kind)
	state.Status = domain.ExternalAssetStatus(status)
	state.OSSSyncStatus = domain.ExternalAssetOSSStatus(ossStatus)
	state.PreviewStatus = domain.ExternalAssetPreviewStatus(previewStatus)
	return &state, nil
}

func getExternalAssetSourceFingerprintForUpdate(ctx context.Context, q sqlQueryRowContext, hash string) (*externalAssetSourceFingerprintState, error) {
	row := q.QueryRowContext(ctx, `
		SELECT file_size, source_modified_at
		  FROM external_asset_source_fingerprints
		 WHERE origin_path_hash = ?
		 FOR UPDATE`, hash)
	var state externalAssetSourceFingerprintState
	var sourceModifiedAt sql.NullTime
	if err := row.Scan(&state.FileSize, &sourceModifiedAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("load external asset source fingerprint: %w", err)
	}
	state.SourceModifiedAt = fromNullTime(sourceModifiedAt)
	return &state, nil
}

func upsertExternalAssetSourceFingerprint(ctx context.Context, exec sqlExecContext, hash string, item domain.ExternalAssetUpsert) error {
	if item.IsDir {
		return nil
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO external_asset_source_fingerprints (
		  origin_path_hash, file_size, source_modified_at, last_scanned_at
		) VALUES (?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  file_size = VALUES(file_size),
		  source_modified_at = VALUES(source_modified_at),
		  last_scanned_at = VALUES(last_scanned_at)`,
		hash, item.FileSize, item.SourceModifiedAt, item.ScannedAt)
	if err != nil {
		return fmt.Errorf("upsert external asset source fingerprint: %w", err)
	}
	return nil
}

func maintainExternalAssetDirectoryIndex(ctx context.Context, exec sqlExecContext, existing *externalAssetCountState, item domain.ExternalAssetUpsert) error {
	if item.IsDir {
		return upsertExternalAssetDirectoryPresence(ctx, exec, item.Provider, item.Kind, item.Driver, item.MountPath, item.OriginPath, item.ScannedAt)
	}
	if existing != nil && !existing.IsDir && existing.Status != domain.ExternalAssetStatusMissing {
		sameLocation := existing.Provider == item.Provider &&
			existing.MountPath == item.MountPath &&
			existing.ParentPath == item.ParentPath
		if sameLocation {
			return upsertExternalAssetDirectoryPresence(ctx, exec, item.Provider, item.Kind, item.Driver, item.MountPath, item.ParentPath, item.ScannedAt)
		}
		if err := applyExternalAssetDirectoryCountDelta(ctx, exec, existing.Provider, existing.Kind, existing.Driver, existing.MountPath, existing.ParentPath, item.ScannedAt, -1); err != nil {
			return err
		}
	}
	return applyExternalAssetDirectoryCountDelta(ctx, exec, item.Provider, item.Kind, item.Driver, item.MountPath, item.ParentPath, item.ScannedAt, 1)
}

func upsertExternalAssetDirectoryPresence(ctx context.Context, exec sqlExecContext, provider string, kind domain.ExternalAssetKind, driver, mountPath, directoryPath string, scannedAt time.Time) error {
	for _, dirPath := range externalAssetDirectoryPaths(directoryPath, mountPath) {
		if err := upsertExternalAssetDirectory(ctx, exec, provider, kind, driver, mountPath, dirPath, scannedAt, 0, 0); err != nil {
			return err
		}
	}
	return nil
}

func applyExternalAssetDirectoryCountDelta(ctx context.Context, exec sqlExecContext, provider string, kind domain.ExternalAssetKind, driver, mountPath, parentPath string, scannedAt time.Time, delta int64) error {
	if delta == 0 {
		return upsertExternalAssetDirectoryPresence(ctx, exec, provider, kind, driver, mountPath, parentPath, scannedAt)
	}
	for _, dirPath := range externalAssetDirectoryPaths(parentPath, mountPath) {
		directDelta := int64(0)
		if cleanExternalAssetBrowsePath(dirPath) == cleanExternalAssetBrowsePath(parentPath) {
			directDelta = delta
		}
		if delta > 0 {
			if err := upsertExternalAssetDirectory(ctx, exec, provider, kind, driver, mountPath, dirPath, scannedAt, delta, directDelta); err != nil {
				return err
			}
			continue
		}
		if err := decrementExternalAssetDirectory(ctx, exec, provider, mountPath, dirPath, delta, directDelta); err != nil {
			return err
		}
	}
	return nil
}

func upsertExternalAssetDirectory(ctx context.Context, exec sqlExecContext, provider string, kind domain.ExternalAssetKind, driver, mountPath, dirPath string, scannedAt time.Time, descendantDelta, directDelta int64) error {
	dirPath = cleanExternalAssetBrowsePath(dirPath)
	mountPath = cleanExternalAssetBrowsePath(mountPath)
	if dirPath == "" || mountPath == "" {
		return nil
	}
	parentPath := parentExternalAssetBrowsePath(dirPath)
	name := path.Base(dirPath)
	_, err := exec.ExecContext(ctx, `
		INSERT INTO external_asset_directory_index (
		  provider, kind, driver, mount_path, path_hash, parent_path_hash, path, parent_path, name,
		  status, descendant_file_count, direct_file_count, last_seen_at, last_scanned_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, 'indexed', ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
		  kind = VALUES(kind),
		  driver = VALUES(driver),
		  mount_path = VALUES(mount_path),
		  parent_path_hash = VALUES(parent_path_hash),
		  parent_path = VALUES(parent_path),
		  name = VALUES(name),
		  status = 'indexed',
		  descendant_file_count = GREATEST(descendant_file_count + VALUES(descendant_file_count), 0),
		  direct_file_count = GREATEST(direct_file_count + VALUES(direct_file_count), 0),
		  last_seen_at = VALUES(last_seen_at),
		  last_scanned_at = VALUES(last_scanned_at)`,
		provider, string(kind), driver, mountPath, externalAssetDirectoryPathHash(provider, mountPath, dirPath),
		externalAssetParentPathHash(parentPath), dirPath, parentPath, name, descendantDelta, directDelta, scannedAt, scannedAt)
	if err != nil {
		return fmt.Errorf("upsert external asset directory index: %w", err)
	}
	return nil
}

func decrementExternalAssetDirectory(ctx context.Context, exec sqlExecContext, provider, mountPath, dirPath string, descendantDelta, directDelta int64) error {
	dirPath = cleanExternalAssetBrowsePath(dirPath)
	mountPath = cleanExternalAssetBrowsePath(mountPath)
	if dirPath == "" || mountPath == "" {
		return nil
	}
	pathHash := externalAssetDirectoryPathHash(provider, mountPath, dirPath)
	_, err := exec.ExecContext(ctx, `
		UPDATE external_asset_directory_index
		   SET descendant_file_count = GREATEST(descendant_file_count + ?, 0),
		       direct_file_count = GREATEST(direct_file_count + ?, 0)
		 WHERE path_hash = ?`,
		descendantDelta, directDelta, pathHash)
	if err != nil {
		return fmt.Errorf("decrement external asset directory index: %w", err)
	}
	if _, err := exec.ExecContext(ctx, `
		DELETE FROM external_asset_directory_index
		 WHERE path_hash = ?
		   AND descendant_file_count = 0
		   AND direct_file_count = 0`, pathHash); err != nil {
		return fmt.Errorf("delete empty external asset directory index: %w", err)
	}
	return nil
}

func (r *externalAssetRepo) Upsert(ctx context.Context, item domain.ExternalAssetUpsert) (*domain.ExternalAssetRecord, error) {
	item = item.Normalized()
	if item.OriginPath == "" || item.FileName == "" {
		return nil, fmt.Errorf("external asset origin_path and file_name are required")
	}
	hash := domain.ExternalAssetOriginHash(item.Provider, item.MountPath, item.OriginPath)
	if err := retryExternalAssetLockConflict(ctx, func() error {
		return r.upsertExternalAsset(ctx, item, hash)
	}); err != nil {
		return nil, err
	}
	return r.getByHash(ctx, hash)
}

func (r *externalAssetRepo) upsertExternalAsset(ctx context.Context, item domain.ExternalAssetUpsert, hash string) error {
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin external asset upsert: %w", err)
	}
	defer tx.Rollback()
	existing, err := getExternalAssetCountStateForUpdate(ctx, tx, hash)
	if err != nil {
		return err
	}
	sourceFingerprint, err := getExternalAssetSourceFingerprintForUpdate(ctx, tx, hash)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `
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
		return fmt.Errorf("upsert external asset: %w", err)
	}
	if err := maintainExternalAssetDirectoryIndex(ctx, tx, existing, item); err != nil {
		return err
	}
	if externalAssetNeedsOSSRealignment(existing, sourceFingerprint, item) {
		if err := resetExternalAssetOSSStateForRealignment(ctx, tx, hash); err != nil {
			return err
		}
	}
	if err := upsertExternalAssetSourceFingerprint(ctx, tx, hash, item); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit external asset upsert: %w", err)
	}
	return nil
}

func externalAssetNeedsOSSRealignment(existing *externalAssetCountState, sourceFingerprint *externalAssetSourceFingerprintState, item domain.ExternalAssetUpsert) bool {
	if existing == nil || item.Kind != domain.ExternalAssetKindNASLocal || item.IsDir || existing.IsDir {
		return false
	}
	if existing.Status == domain.ExternalAssetStatusMissing {
		return false
	}
	previousSize := existing.FileSize
	var previousModifiedAt *time.Time
	if sourceFingerprint != nil {
		previousSize = sourceFingerprint.FileSize
		previousModifiedAt = sourceFingerprint.SourceModifiedAt
	}
	if previousSize == item.FileSize && externalAssetSameSourceModifiedAt(previousModifiedAt, item.SourceModifiedAt) {
		return false
	}
	return existing.OSSOriginalKey != "" ||
		existing.OSSPreviewKey != "" ||
		existing.OSSThumbKey != "" ||
		existing.OSSSyncStatus == domain.ExternalAssetOSSStatusReady ||
		existing.OSSSyncStatus == domain.ExternalAssetOSSStatusUploading ||
		existing.PreviewStatus == domain.ExternalAssetPreviewStatusReady
}

func externalAssetSameSourceModifiedAt(left, right *time.Time) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return left.UTC().Truncate(time.Second).Equal(right.UTC().Truncate(time.Second))
}

func resetExternalAssetOSSStateForRealignment(ctx context.Context, exec sqlExecContext, hash string) error {
	_, err := exec.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET oss_original_key = '',
		       oss_preview_key = '',
		       oss_thumb_key = '',
		       oss_sync_status = 'pending',
		       preview_status = CASE WHEN preview_status = 'ready' THEN 'pending' ELSE preview_status END,
		       last_prepare_error = NULL
		 WHERE origin_path_hash = ?`, hash)
	if err != nil {
		return fmt.Errorf("reset external asset oss state for realignment: %w", err)
	}
	return nil
}

func (r *externalAssetRepo) getByHash(ctx context.Context, hash string) (*domain.ExternalAssetRecord, error) {
	row := r.db.db.QueryRowContext(ctx, externalAssetSelect+` WHERE origin_path_hash = ?`, hash)
	return scanExternalAssetRow(row)
}

func (r *externalAssetRepo) GetByID(ctx context.Context, id int64) (*domain.ExternalAssetRecord, error) {
	row := r.db.db.QueryRowContext(ctx, externalAssetSelect+` WHERE id = ?`, id)
	return scanExternalAssetRow(row)
}

func (r *externalAssetRepo) MarkOriginPathMissing(ctx context.Context, provider, mountPath, originPath string) error {
	item := domain.ExternalAssetUpsert{
		Provider:   provider,
		MountPath:  mountPath,
		OriginPath: originPath,
	}.Normalized()
	if item.OriginPath == "" {
		return nil
	}
	hash := domain.ExternalAssetOriginHash(item.Provider, item.MountPath, item.OriginPath)
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin external origin missing update: %w", err)
	}
	defer tx.Rollback()
	existing, err := getExternalAssetCountStateForUpdate(ctx, tx, hash)
	if err != nil {
		return err
	}
	if existing == nil || existing.Status == domain.ExternalAssetStatusMissing {
		return nil
	}
	_, err = tx.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET status = 'missing'
		 WHERE origin_path_hash = ?
		   AND status <> 'missing'`,
		hash)
	if err != nil {
		return fmt.Errorf("mark external origin missing: %w", err)
	}
	if !existing.IsDir {
		if err := applyExternalAssetDirectoryCountDelta(ctx, tx, existing.Provider, existing.Kind, existing.Driver, existing.MountPath, existing.ParentPath, time.Now().UTC(), -1); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit external origin missing update: %w", err)
	}
	return nil
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
	return retryExternalAssetLockConflict(ctx, func() error {
		return r.markMountMissingBefore(ctx, mountPath, scannedBefore)
	})
}

func (r *externalAssetRepo) markMountMissingBefore(ctx context.Context, mountPath string, scannedBefore time.Time) error {
	mountPath = strings.TrimSpace(mountPath)
	if mountPath == "" || scannedBefore.IsZero() {
		return nil
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin external mount missing update: %w", err)
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET status = 'missing'
		 WHERE mount_path = ?
		   AND status <> 'missing'
		   AND (last_scanned_at IS NULL OR last_scanned_at < ?)`,
		mountPath, scannedBefore)
	if err != nil {
		return fmt.Errorf("mark external mount missing: %w", err)
	}
	if err := rebuildExternalAssetDirectoryIndexForMount(ctx, tx, mountPath); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit external mount missing update: %w", err)
	}
	return nil
}

func (r *externalAssetRepo) MarkOriginPrefixesMissingBefore(ctx context.Context, prefixes []repo.ExternalAssetOriginPrefix, scannedBefore time.Time) error {
	return retryExternalAssetLockConflict(ctx, func() error {
		return r.markOriginPrefixesMissingBefore(ctx, prefixes, scannedBefore)
	})
}

func (r *externalAssetRepo) markOriginPrefixesMissingBefore(ctx context.Context, prefixes []repo.ExternalAssetOriginPrefix, scannedBefore time.Time) error {
	prefixes = normalizeExternalAssetOriginPrefixes(prefixes)
	if len(prefixes) == 0 || scannedBefore.IsZero() {
		return nil
	}
	where, args := externalAssetOriginPrefixWhere(prefixes)
	if where == "" {
		return nil
	}
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin external prefix missing update: %w", err)
	}
	defer tx.Rollback()
	updateArgs := append([]interface{}{}, args...)
	updateArgs = append(updateArgs, scannedBefore)
	_, err = tx.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET status = 'missing'
		 WHERE status <> 'missing'
		   AND (`+where+`)
		   AND (last_scanned_at IS NULL OR last_scanned_at < ?)`,
		updateArgs...)
	if err != nil {
		return fmt.Errorf("mark external prefixes missing: %w", err)
	}
	seenMounts := map[string]struct{}{}
	for _, prefix := range prefixes {
		if _, ok := seenMounts[prefix.MountPath]; ok {
			continue
		}
		seenMounts[prefix.MountPath] = struct{}{}
		if err := rebuildExternalAssetDirectoryIndexForMount(ctx, tx, prefix.MountPath); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit external prefix missing update: %w", err)
	}
	return nil
}

func rebuildExternalAssetDirectoryIndexForMount(ctx context.Context, exec sqlExecContext, mountPath string) error {
	mountPath = cleanExternalAssetBrowsePath(mountPath)
	if mountPath == "" {
		return nil
	}
	if _, err := exec.ExecContext(ctx, `DELETE FROM external_asset_directory_index WHERE mount_path = ?`, mountPath); err != nil {
		return fmt.Errorf("clear external asset directory index: %w", err)
	}
	_, err := exec.ExecContext(ctx, `
		INSERT INTO external_asset_directory_index (
		  path_hash, parent_path_hash, provider, kind, driver, mount_path, path, parent_path, name,
		  status, descendant_file_count, direct_file_count, last_seen_at, last_scanned_at
		)
		WITH RECURSIVE file_dirs AS (
		  SELECT provider, kind, driver, mount_path, parent_path AS path, parent_path AS direct_parent_path,
		         last_seen_at, last_scanned_at
		    FROM external_asset_records
		   WHERE status <> 'missing'
		     AND is_dir = 0
		     AND mount_path = ?
		     AND parent_path <> ''
		     AND parent_path <> '/'
		     AND parent_path NOT LIKE '%/@eaDir/%'
		     AND parent_path NOT LIKE '%/#recycle/%'
		     AND file_name NOT LIKE '%@Syno%'
		  UNION ALL
		  SELECT provider, kind, driver, mount_path,
		         CASE
		           WHEN (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) <= 1 THEN ''
		           ELSE SUBSTRING_INDEX(path, '/', (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))))
		         END AS path,
		         direct_parent_path, last_seen_at, last_scanned_at
		    FROM file_dirs
		   WHERE path <> ''
		     AND (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) > 1
		)
		SELECT SHA2(CONCAT(LOWER(TRIM(provider)), '|', mount_path, '|', path), 256) AS path_hash,
		       SHA2(CASE
		         WHEN (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) <= 1 THEN ''
		         ELSE SUBSTRING_INDEX(path, '/', (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))))
		       END, 256) AS parent_path_hash,
		       provider,
		       kind,
		       driver,
		       mount_path,
		       path,
		       CASE
		         WHEN (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))) <= 1 THEN ''
		         ELSE SUBSTRING_INDEX(path, '/', (LENGTH(path) - LENGTH(REPLACE(path, '/', ''))))
		       END AS parent_path,
		       SUBSTRING_INDEX(path, '/', -1) AS name,
		       'indexed' AS status,
		       COUNT(*) AS descendant_file_count,
		       SUM(path = direct_parent_path) AS direct_file_count,
		       MAX(last_seen_at) AS last_seen_at,
		       MAX(last_scanned_at) AS last_scanned_at
		  FROM file_dirs
		 WHERE path <> ''
		 GROUP BY provider, kind, driver, mount_path, path
		ON DUPLICATE KEY UPDATE
		  kind = VALUES(kind),
		  driver = VALUES(driver),
		  parent_path_hash = VALUES(parent_path_hash),
		  parent_path = VALUES(parent_path),
		  name = VALUES(name),
		  status = VALUES(status),
		  descendant_file_count = VALUES(descendant_file_count),
		  direct_file_count = VALUES(direct_file_count),
		  last_seen_at = VALUES(last_seen_at),
		  last_scanned_at = VALUES(last_scanned_at)`, mountPath)
	if err != nil {
		return fmt.Errorf("rebuild external asset directory index: %w", err)
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

func (r *externalAssetRepo) MarkOSSPendingByOriginPrefixes(ctx context.Context, prefixes []repo.ExternalAssetOriginPrefix) (int64, error) {
	where, args := externalAssetOriginPrefixWhere(prefixes)
	if where == "" {
		return 0, nil
	}
	result, err := r.db.db.ExecContext(ctx, `
		UPDATE external_asset_records
		   SET oss_sync_status = 'pending',
		       last_prepare_error = NULL
		 WHERE kind = 'nas_local'
		   AND is_dir = 0
		   AND status <> 'missing'
		   AND oss_sync_status NOT IN ('ready', 'uploading')
		   AND (`+where+`)`, args...)
	if err != nil {
		return 0, fmt.Errorf("mark external oss pending by origin prefixes: %w", err)
	}
	affected, _ := result.RowsAffected()
	return affected, nil
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
	return r.ListPendingOSSPrioritized(ctx, nil, limit)
}

func (r *externalAssetRepo) ListPendingOSSPrioritized(ctx context.Context, prefixes []repo.ExternalAssetOriginPrefix, limit int) ([]*domain.ExternalAssetRecord, error) {
	limit = externalAssetPrepareLimit(limit)
	priorityOrder, priorityArgs := externalAssetOriginPrefixPriorityOrder(prefixes)
	args := append(priorityArgs, limit)
	rows, err := r.db.db.QueryContext(ctx, externalAssetSelect+`
		WHERE kind = 'nas_local'
		  AND is_dir = 0
		  AND oss_sync_status IN ('pending', 'failed')
		ORDER BY `+priorityOrder+`CASE oss_sync_status WHEN 'pending' THEN 0 ELSE 1 END, updated_at DESC, id DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("list external oss pending: %w", err)
	}
	defer rows.Close()
	return scanExternalAssetRows(rows)
}

func (r *externalAssetRepo) ListPendingPreview(ctx context.Context, limit int) ([]*domain.ExternalAssetRecord, error) {
	limit = externalAssetPrepareLimit(limit)
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

func externalAssetPrepareLimit(limit int) int {
	if limit <= 0 {
		return 20
	}
	if limit > 1000 {
		return 1000
	}
	return limit
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
