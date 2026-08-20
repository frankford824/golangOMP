package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// driveOwnerClause appends an owner filter used to scope non-admin users to their
// own uploads. Returns the SQL fragment (may be empty) and the extra args.
func driveOwnerClause(filter repo.AssetWorkbenchDriveFilter) (string, []interface{}) {
	if filter.OwnerUserID != nil {
		return " AND f.owner_user_id = ?", []interface{}{*filter.OwnerUserID}
	}
	return "", nil
}

// driveDirectoryClause scopes queries to a specific virtual folder (upload directory)
// or to the "unassigned" bucket (files without an upload directory).
func driveDirectoryClause(filter repo.AssetWorkbenchDriveFilter) (string, []interface{}) {
	if filter.Unassigned {
		return " AND f.upload_directory_id IS NULL", nil
	}
	if filter.UploadDirectoryID != nil {
		return " AND f.upload_directory_id = ?", []interface{}{*filter.UploadDirectoryID}
	}
	return "", nil
}

func (r *assetWorkbenchRepo) DriveListDirectories(ctx context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveDirectory, error) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	query := `SELECT f.upload_directory_id AS directory_id,
		MAX(COALESCE(NULLIF(f.upload_directory_name, ''), '未分类')) AS name,
		MAX(COALESCE(f.upload_directory_prefix, '')) AS prefix,
		MAX(COALESCE(f.upload_directory_difficulty_class, '')) AS difficulty_class,
		COUNT(f.id) AS file_count,
		COUNT(DISTINCT i.order_no) AS order_count
	FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	WHERE f.deleted_at IS NULL` + ownerSQL + `
	GROUP BY f.upload_directory_id
	ORDER BY name ASC`
	rows, err := r.db.db.QueryContext(ctx, query, ownerArgs...)
	if err != nil {
		return nil, fmt.Errorf("drive list directories: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchDriveDirectory{}
	for rows.Next() {
		item := &domain.AssetWorkbenchDriveDirectory{}
		if err := rows.Scan(&item.DirectoryID, &item.Name, &item.Prefix, &item.DifficultyClass, &item.FileCount, &item.OrderCount); err != nil {
			return nil, fmt.Errorf("scan drive directory: %w", err)
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) DriveListOrders(ctx context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveOrder, error) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	dirSQL, dirArgs := driveDirectoryClause(filter)
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, dirArgs...)
	query := `SELECT i.order_no AS order_no,
		MIN(i.id) AS submission_item_id,
		COUNT(f.id) AS file_count,
		MAX(f.created_at) AS latest_at
	FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	WHERE f.deleted_at IS NULL` + ownerSQL + dirSQL + `
	GROUP BY i.order_no
	ORDER BY latest_at DESC, i.order_no ASC`
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("drive list orders: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchDriveOrder{}
	for rows.Next() {
		item := &domain.AssetWorkbenchDriveOrder{}
		var itemID sql.NullInt64
		if err := rows.Scan(&item.OrderNo, &itemID, &item.FileCount, &item.LatestAt); err != nil {
			return nil, fmt.Errorf("scan drive order: %w", err)
		}
		item.SubmissionItemID = fromNullInt64(itemID)
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if err := r.hydrateDriveOrderItemIDs(ctx, filter, items); err != nil {
		return nil, err
	}
	return items, nil
}

func (r *assetWorkbenchRepo) hydrateDriveOrderItemIDs(ctx context.Context, filter repo.AssetWorkbenchDriveFilter, orders []*domain.AssetWorkbenchDriveOrder) error {
	if len(orders) == 0 {
		return nil
	}
	byOrderNo := make(map[string]*domain.AssetWorkbenchDriveOrder, len(orders))
	orderNos := make([]interface{}, 0, len(orders))
	for _, order := range orders {
		if order == nil {
			continue
		}
		order.SubmissionItemIDs = nil
		byOrderNo[order.OrderNo] = order
		orderNos = append(orderNos, order.OrderNo)
	}
	if len(orderNos) == 0 {
		return nil
	}
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	dirSQL, dirArgs := driveDirectoryClause(filter)
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, dirArgs...)
	placeholders := make([]string, 0, len(orderNos))
	for range orderNos {
		placeholders = append(placeholders, "?")
	}
	args = append(args, orderNos...)
	query := `SELECT DISTINCT i.order_no, i.id
	FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	WHERE f.deleted_at IS NULL` + ownerSQL + dirSQL + `
	  AND i.order_no IN (` + strings.Join(placeholders, ",") + `)
	ORDER BY i.order_no ASC, i.id ASC`
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("drive list order item ids: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var orderNo string
		var itemID int64
		if err := rows.Scan(&orderNo, &itemID); err != nil {
			return fmt.Errorf("scan drive order item id: %w", err)
		}
		if order := byOrderNo[orderNo]; order != nil {
			order.SubmissionItemIDs = append(order.SubmissionItemIDs, itemID)
		}
	}
	return rows.Err()
}

func driveFileColumns() string {
	return `f.id, f.submission_id, f.submission_item_id, s.submission_no, f.owner_user_id,
		COALESCE(NULLIF(p.real_name, ''), NULLIF(u.display_name, ''), NULLIF(u.username, ''), CONCAT('用户 ', f.owner_user_id)) AS owner_name,
		COALESCE(u.username, '') AS owner_username,
		f.upload_directory_id,
		COALESCE(NULLIF(f.upload_directory_name, ''), '未分类') AS upload_directory_name,
		i.difficulty_class, i.order_no,
		f.original_filename, COALESCE(NULLIF(f.display_name, ''), f.original_filename) AS display_name,
		f.relative_path, f.upload_batch_id, f.is_folder_upload,
		f.file_type, f.mime_type, f.file_size, f.preview_status,
		i.qc_status, i.pricing_status, i.settlement_status, i.page_count,
		i.gross_amount, i.business_month,
		CASE
			WHEN i.entry_kind = 'supplement' AND s.submitter_user_id = i.payee_user_id THEN 'client_supplement'
			WHEN i.entry_kind = 'supplement' THEN 'admin_supplement'
			ELSE 'normal_upload'
		END AS operation_source,
		f.created_at`
}

func scanDriveFile(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchDriveFile, error) {
	item := &domain.AssetWorkbenchDriveFile{}
	if err := scanner.Scan(
		&item.ID, &item.SubmissionID, &item.SubmissionItemID, &item.SubmissionNo, &item.OwnerUserID,
		&item.OwnerName, &item.OwnerUsername,
		&item.UploadDirectoryID, &item.UploadDirectoryName, &item.DifficultyClass, &item.OrderNo,
		&item.OriginalFilename, &item.DisplayName, &item.RelativePath, &item.UploadBatchID, &item.IsFolderUpload,
		&item.FileType, &item.MimeType, &item.FileSize, &item.PreviewStatus,
		&item.QCStatus, &item.PricingStatus, &item.SettlementStatus, &item.PageCount,
		&item.GrossAmount, &item.BusinessMonth, &item.OperationSource, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	return item, nil
}

func driveListFileOrderBy(filter repo.AssetWorkbenchDriveFilter, orderNo string) string {
	if orderNo != "" {
		return `f.sort_order ASC, f.id ASC`
	}
	dir := "DESC"
	if strings.EqualFold(strings.TrimSpace(filter.SortDir), "asc") {
		dir = "ASC"
	}
	switch strings.TrimSpace(filter.SortBy) {
	case "owner", "creator":
		return `owner_name ` + dir + `, f.created_at DESC, f.id DESC`
	case "directory", "category":
		return `upload_directory_name ` + dir + `, f.created_at DESC, f.id DESC`
	case "name", "display_name":
		return `display_name ` + dir + `, f.created_at DESC, f.id DESC`
	case "format", "file_type":
		return `f.file_type ` + dir + `, f.mime_type ` + dir + `, f.created_at DESC, f.id DESC`
	case "created_at", "":
		return `f.created_at ` + dir + `, f.id ` + dir
	default:
		return `f.created_at DESC, f.id DESC`
	}
}

func (r *assetWorkbenchRepo) DriveListFiles(ctx context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	dirSQL, dirArgs := driveDirectoryClause(filter)
	orderNo := strings.TrimSpace(filter.OrderNo)
	where := ""
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, dirArgs...)
	if orderNo != "" {
		where = ` AND i.order_no = ?`
		args = append(args, orderNo)
	}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		like := "%" + keyword + "%"
		where += ` AND (f.original_filename LIKE ? OR f.display_name LIKE ? OR f.relative_path LIKE ? OR f.file_type LIKE ? OR f.mime_type LIKE ? OR COALESCE(f.upload_directory_name, '') LIKE ? OR COALESCE(p.real_name, '') LIKE ? OR COALESCE(u.display_name, '') LIKE ? OR COALESCE(u.username, '') LIKE ? OR i.order_no LIKE ? OR s.submission_no LIKE ?)`
		args = append(args, like, like, like, like, like, like, like, like, like, like, like)
	}
	if owner := strings.TrimSpace(filter.OwnerKeyword); owner != "" {
		like := "%" + owner + "%"
		where += ` AND (COALESCE(p.real_name, '') LIKE ? OR COALESCE(u.display_name, '') LIKE ? OR COALESCE(u.username, '') LIKE ?)`
		args = append(args, like, like, like)
	}
	if filter.CreatedFrom != nil {
		where += ` AND f.created_at >= ?`
		args = append(args, *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		where += ` AND f.created_at <= ?`
		args = append(args, *filter.CreatedTo)
	}
	orderBy := driveListFileOrderBy(filter, orderNo)

	base := `FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	LEFT JOIN users u ON u.id = f.owner_user_id
	LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id
	WHERE f.deleted_at IS NULL` + ownerSQL + dirSQL + where

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count drive files: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+driveFileColumns()+` `+base+`
	ORDER BY `+orderBy+`
	LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("drive list files: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchDriveFile{}
	for rows.Next() {
		item, err := scanDriveFile(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan drive file: %w", err)
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) DriveSearchFiles(ctx context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	keyword := strings.TrimSpace(filter.Keyword)
	base, args := buildDriveSearchBase(filter, true)
	var total int64
	usedFallback := false
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base, args...).Scan(&total); err != nil {
		if keyword == "" || !isMySQLFullTextIndexMissing(err) {
			return nil, 0, fmt.Errorf("count drive search files: %w", err)
		}
		base, args = buildDriveSearchBase(filter, false)
		usedFallback = true
		if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count drive search files fallback: %w", err)
		}
	}
	if !usedFallback && total == 0 && keyword != "" && externalAssetBooleanQuery(keyword) != "" {
		base, args = buildDriveSearchBase(filter, false)
		if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count drive search files fallback: %w", err)
		}
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+driveFileColumns()+` `+base+`
	ORDER BY `+driveListFileOrderBy(filter, "")+`
	LIMIT ? OFFSET ?`, listArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("drive search files: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchDriveFile{}
	for rows.Next() {
		item, err := scanDriveFile(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan drive search file: %w", err)
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func buildDriveSearchBase(filter repo.AssetWorkbenchDriveFilter, preferFullText bool) (string, []interface{}) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	dirSQL, dirArgs := driveDirectoryClause(filter)
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, dirArgs...)
	extraSQL, extraArgs := driveSearchExtraClauses(filter)
	args = append(args, extraArgs...)
	keyword := strings.TrimSpace(filter.Keyword)
	keywordSQL := ""
	if keyword != "" {
		if fullText := externalAssetBooleanQuery(keyword); preferFullText && fullText != "" {
			idQuery, idArgs := buildDriveSearchFullTextIDQuery(filter, fullText)
			base := `FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	LEFT JOIN users u ON u.id = f.owner_user_id
	LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id
	WHERE f.id IN (` + idQuery + `)`
			return base, idArgs
		} else {
			like := "%" + keyword + "%"
			keywordSQL = `
	  AND (f.original_filename LIKE ? OR f.display_name LIKE ? OR f.relative_path LIKE ? OR i.order_no LIKE ? OR s.submission_no LIKE ? OR COALESCE(p.real_name, '') LIKE ? OR COALESCE(u.display_name, '') LIKE ? OR COALESCE(u.username, '') LIKE ?)`
			args = append(args, like, like, like, like, like, like, like, like)
		}
	}
	base := `FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	LEFT JOIN users u ON u.id = f.owner_user_id
	LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id
	WHERE f.deleted_at IS NULL` + ownerSQL + dirSQL + extraSQL + keywordSQL
	return base, args
}

func buildDriveSearchFullTextIDQuery(filter repo.AssetWorkbenchDriveFilter, fullText string) (string, []interface{}) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	dirSQL, dirArgs := driveDirectoryClause(filter)
	extraSQL, extraArgs := driveSearchExtraClauses(filter)
	queries := []string{
		`SELECT f.id
		FROM asset_workbench_submission_files f
		JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
		JOIN asset_workbench_submissions s ON s.id = f.submission_id
		LEFT JOIN users u ON u.id = f.owner_user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id
		WHERE f.deleted_at IS NULL` + ownerSQL + dirSQL + extraSQL + ` AND ` + assetWorkbenchFileFullTextMatch,
		`SELECT f.id
		FROM asset_workbench_submission_items i
		JOIN asset_workbench_submission_files f ON f.submission_item_id = i.id
		JOIN asset_workbench_submissions s ON s.id = f.submission_id
		LEFT JOIN users u ON u.id = f.owner_user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id
		WHERE i.voided_at IS NULL AND f.deleted_at IS NULL` + ownerSQL + dirSQL + extraSQL + ` AND ` + assetWorkbenchItemFullTextMatch,
		`SELECT f.id
		FROM asset_workbench_submissions s
		JOIN asset_workbench_submission_files f ON f.submission_id = s.id
		JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
		LEFT JOIN users u ON u.id = f.owner_user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id
		WHERE f.deleted_at IS NULL` + ownerSQL + dirSQL + extraSQL + ` AND ` + assetWorkbenchSubmissionFullTextMatch,
	}
	filterArgs := append([]interface{}{}, ownerArgs...)
	filterArgs = append(filterArgs, dirArgs...)
	filterArgs = append(filterArgs, extraArgs...)
	args := make([]interface{}, 0, len(filterArgs)*len(queries)+len(queries))
	for range queries {
		args = append(args, filterArgs...)
		args = append(args, fullText)
	}
	return strings.Join(queries, `
		UNION DISTINCT
		`), args
}

func driveSearchExtraClauses(filter repo.AssetWorkbenchDriveFilter) (string, []interface{}) {
	args := []interface{}{}
	clauses := []string{}
	if owner := strings.TrimSpace(filter.OwnerKeyword); owner != "" {
		like := "%" + owner + "%"
		clauses = append(clauses, `(COALESCE(p.real_name, '') LIKE ? OR COALESCE(u.display_name, '') LIKE ? OR COALESCE(u.username, '') LIKE ?)`)
		args = append(args, like, like, like)
	}
	if filter.CreatedFrom != nil {
		clauses = append(clauses, `f.created_at >= ?`)
		args = append(args, *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		clauses = append(clauses, `f.created_at <= ?`)
		args = append(args, *filter.CreatedTo)
	}
	if len(clauses) == 0 {
		return "", nil
	}
	return " AND " + strings.Join(clauses, " AND "), args
}

func (r *assetWorkbenchRepo) DriveLocateFile(ctx context.Context, filter repo.AssetWorkbenchDriveFilter, fileID int64) (*domain.AssetWorkbenchDriveFile, error) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, fileID)
	query := `SELECT ` + driveFileColumns() + `
	FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	LEFT JOIN users u ON u.id = f.owner_user_id
	LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id
	WHERE f.deleted_at IS NULL` + ownerSQL + `
	  AND f.id = ?
	LIMIT 1`
	row := r.db.db.QueryRowContext(ctx, query, args...)
	file, err := scanDriveFile(row)
	if err != nil {
		return nil, err
	}
	if err := r.hydrateDriveLocatePage(ctx, filter, file, 60); err != nil {
		return nil, err
	}
	return file, nil
}

func (r *assetWorkbenchRepo) hydrateDriveLocatePage(ctx context.Context, filter repo.AssetWorkbenchDriveFilter, file *domain.AssetWorkbenchDriveFile, pageSize int) error {
	if file == nil {
		return nil
	}
	if pageSize <= 0 {
		pageSize = 60
	}
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	args := append([]interface{}{}, ownerArgs...)
	dirSQL := " AND f.upload_directory_id IS NULL"
	if file.UploadDirectoryID != nil {
		dirSQL = " AND f.upload_directory_id = ?"
		args = append(args, *file.UploadDirectoryID)
	}
	args = append(args, file.CreatedAt, file.CreatedAt, file.ID)
	query := `SELECT COUNT(*)
	FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	WHERE f.deleted_at IS NULL` + ownerSQL + dirSQL + `
	  AND (f.created_at > ? OR (f.created_at = ? AND f.id > ?))`
	var before int64
	if err := r.db.db.QueryRowContext(ctx, query, args...).Scan(&before); err != nil {
		return fmt.Errorf("count drive locate page: %w", err)
	}
	file.LocatePageSize = pageSize
	file.LocatePage = int(before/int64(pageSize)) + 1
	return nil
}
