package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

func int64SliceFromJSON(raw string) []int64 {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []int64
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

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
	WHERE 1=1` + ownerSQL + `
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
		CONCAT('[', COALESCE(GROUP_CONCAT(DISTINCT i.id ORDER BY i.id), ''), ']') AS submission_item_ids_json,
		COUNT(f.id) AS file_count,
		MAX(f.created_at) AS latest_at
	FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	WHERE 1=1` + ownerSQL + dirSQL + `
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
		var itemIDsJSON sql.NullString
		if err := rows.Scan(&item.OrderNo, &itemID, &itemIDsJSON, &item.FileCount, &item.LatestAt); err != nil {
			return nil, fmt.Errorf("scan drive order: %w", err)
		}
		item.SubmissionItemID = fromNullInt64(itemID)
		item.SubmissionItemIDs = int64SliceFromJSON(itemIDsJSON.String)
		items = append(items, item)
	}
	return items, rows.Err()
}

func driveFileColumns() string {
	return `f.id, f.submission_id, f.submission_item_id, s.submission_no, f.owner_user_id,
		f.upload_directory_id,
		COALESCE(NULLIF(f.upload_directory_name, ''), '未分类') AS upload_directory_name,
		i.difficulty_class, i.order_no,
		f.original_filename, f.file_type, f.mime_type, f.file_size, f.preview_status,
		i.qc_status, i.pricing_status, i.settlement_status, i.page_count,
		i.business_month, f.created_at`
}

func scanDriveFile(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchDriveFile, error) {
	item := &domain.AssetWorkbenchDriveFile{}
	if err := scanner.Scan(
		&item.ID, &item.SubmissionID, &item.SubmissionItemID, &item.SubmissionNo, &item.OwnerUserID,
		&item.UploadDirectoryID, &item.UploadDirectoryName, &item.DifficultyClass, &item.OrderNo,
		&item.OriginalFilename, &item.FileType, &item.MimeType, &item.FileSize, &item.PreviewStatus,
		&item.QCStatus, &item.PricingStatus, &item.SettlementStatus, &item.PageCount,
		&item.BusinessMonth, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	return item, nil
}

func (r *assetWorkbenchRepo) DriveListFiles(ctx context.Context, filter repo.AssetWorkbenchDriveFilter) ([]*domain.AssetWorkbenchDriveFile, int64, error) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	dirSQL, dirArgs := driveDirectoryClause(filter)
	where := ` AND i.order_no = ?`
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, dirArgs...)
	args = append(args, filter.OrderNo)

	base := `FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	WHERE 1=1` + ownerSQL + dirSQL + where

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count drive files: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+driveFileColumns()+` `+base+`
	ORDER BY f.sort_order ASC, f.id ASC
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
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	keyword := strings.TrimSpace(filter.Keyword)
	like := "%" + keyword + "%"
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, like, like, like)
	base := `FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id AND i.voided_at IS NULL
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	WHERE 1=1` + ownerSQL + `
	  AND (f.original_filename LIKE ? OR i.order_no LIKE ? OR s.submission_no LIKE ?)`

	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) `+base, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count drive search files: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	listArgs := append([]interface{}{}, args...)
	listArgs = append(listArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+driveFileColumns()+` `+base+`
	ORDER BY f.created_at DESC, f.id DESC
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

func (r *assetWorkbenchRepo) DriveLocateFile(ctx context.Context, filter repo.AssetWorkbenchDriveFilter, fileID int64) (*domain.AssetWorkbenchDriveFile, error) {
	ownerSQL, ownerArgs := driveOwnerClause(filter)
	args := append([]interface{}{}, ownerArgs...)
	args = append(args, fileID)
	query := `SELECT ` + driveFileColumns() + `
	FROM asset_workbench_submission_files f
	JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id
	JOIN asset_workbench_submissions s ON s.id = f.submission_id
	WHERE 1=1` + ownerSQL + `
	  AND f.id = ?
	LIMIT 1`
	row := r.db.db.QueryRowContext(ctx, query, args...)
	return scanDriveFile(row)
}
