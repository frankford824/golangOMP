package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"

	"workflow/domain"
	"workflow/repo"
)

type kpiAnalysisRepo struct{ db *DB }

func NewKPIAnalysisRepo(db *DB) repo.KPIAnalysisRepo { return &kpiAnalysisRepo{db: db} }

func (r *kpiAnalysisRepo) ListTaskEvents(ctx context.Context, filter repo.KPIAnalysisFilter) ([]domain.KPIAnalysisEvent, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 300
	}
	query := `
		SELECT
		  tel.id,
		  tel.task_id,
		  COALESCE(t.task_no, ''),
		  COALESCE(t.sku_code, ''),
		  COALESCE(t.product_name_snapshot, ''),
		  COALESCE(CAST(t.task_type AS CHAR), ''),
		  COALESCE(CAST(t.business_lane AS CHAR), ''),
		  COALESCE(td.category_name, ''),
		  tel.event_type,
		  tel.operator_id,
		  COALESCE(NULLIF(u.display_name, ''), u.username, ''),
		  COALESCE(u.department, ''),
		  COALESCE(u.team, ''),
		  COALESCE(CAST(tel.payload AS CHAR), '{}'),
		  tel.created_at
		FROM task_event_logs tel
		JOIN tasks t ON t.id = tel.task_id
		LEFT JOIN task_details td ON td.task_id = t.id
		LEFT JOIN users u ON u.id = tel.operator_id
		WHERE tel.created_at >= ? AND tel.created_at < ?
		  AND tel.event_type IN (
		    'task.created',
		    'task.batch_items_created',
		    'task.assigned',
		    'task.reassigned',
		    'task.batch_assigned',
		    'task.design.submitted',
		    'task.audit.approved',
		    'task.audit.rejected'
		  )
		ORDER BY tel.created_at DESC
		LIMIT ?`
	rows, err := r.db.db.QueryContext(ctx, query, filter.From, filter.To, limit)
	if err != nil {
		return nil, fmt.Errorf("list kpi task events: %w", err)
	}
	defer rows.Close()

	out := make([]domain.KPIAnalysisEvent, 0, limit)
	for rows.Next() {
		var item domain.KPIAnalysisEvent
		var operatorID sql.NullInt64
		var payload string
		if err := rows.Scan(
			&item.ID,
			&item.TaskID,
			&item.TaskNo,
			&item.SKUCode,
			&item.ProductName,
			&item.TaskType,
			&item.BusinessLane,
			&item.CategoryName,
			&item.EventType,
			&operatorID,
			&item.OperatorName,
			&item.OperatorDepartment,
			&item.OperatorTeam,
			&payload,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan kpi task event: %w", err)
		}
		if operatorID.Valid {
			id := operatorID.Int64
			item.OperatorID = &id
		}
		if payload != "" {
			item.Payload = []byte(payload)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *kpiAnalysisRepo) ListTaskAssets(ctx context.Context, filter repo.KPIAnalysisFilter) ([]domain.KPIAnalysisAsset, error) {
	limit := filter.Limit
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	query := `
		SELECT
		  ta.id,
		  ta.task_id,
		  COALESCE(t.task_no, ''),
		  COALESCE(t.product_name_snapshot, ''),
		  COALESCE(CAST(t.task_type AS CHAR), ''),
		  COALESCE(CAST(t.business_lane AS CHAR), ''),
		  COALESCE(ta.asset_type, ''),
		  COALESCE(ta.file_name, ''),
		  COALESCE(ta.original_filename, ''),
		  ta.uploaded_by,
		  COALESCE(NULLIF(u.display_name, ''), u.username, ''),
		  ta.created_at
		FROM task_assets ta
		JOIN tasks t ON t.id = ta.task_id
		LEFT JOIN users u ON u.id = ta.uploaded_by
		WHERE ta.created_at >= ? AND ta.created_at < ?
		  AND ta.asset_type IN ('reference', 'draft', 'revised', 'final', 'outsource_return')
		ORDER BY ta.created_at DESC
		LIMIT ?`
	rows, err := r.db.db.QueryContext(ctx, query, filter.From, filter.To, limit)
	if err != nil {
		return nil, fmt.Errorf("list kpi task assets: %w", err)
	}
	defer rows.Close()

	out := make([]domain.KPIAnalysisAsset, 0, limit)
	for rows.Next() {
		var item domain.KPIAnalysisAsset
		if err := rows.Scan(
			&item.ID,
			&item.TaskID,
			&item.TaskNo,
			&item.ProductName,
			&item.TaskType,
			&item.BusinessLane,
			&item.AssetType,
			&item.FileName,
			&item.OriginalName,
			&item.UploadedBy,
			&item.UploadedByName,
			&item.CreatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan kpi task asset: %w", err)
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
