package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type businessTrendRepo struct{ db *DB }

func NewBusinessTrendRepo(db *DB) repo.BusinessTrendRepo { return &businessTrendRepo{db: db} }

func (r *businessTrendRepo) ListRecentTaskTexts(ctx context.Context, filter repo.BusinessTrendFilter) ([]domain.BusinessTrendTaskText, error) {
	limit := filter.Limit
	if limit <= 0 {
		limit = 240
	}
	if limit > 1000 {
		limit = 1000
	}
	query := `
		SELECT
		  t.id,
		  COALESCE(t.task_no, ''),
		  COALESCE(t.sku_code, ''),
		  COALESCE(t.product_name_snapshot, ''),
		  COALESCE(CAST(t.task_type AS CHAR), ''),
		  COALESCE(CAST(t.business_lane AS CHAR), ''),
		  COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), ''),
		  COALESCE(CAST(t.task_status AS CHAR), ''),
		  COALESCE(CAST(t.priority AS CHAR), ''),
		  COALESCE(NULLIF(creator.display_name, ''), creator.username, ''),
		  COALESCE(NULLIF(designer.display_name, ''), designer.username, ''),
		  COALESCE(td.demand_text, ''),
		  COALESCE(td.copy_text, ''),
		  COALESCE(td.remark, ''),
		  COALESCE(td.change_request, ''),
		  COALESCE(td.design_requirement, ''),
		  COALESCE(td.product_short_name, ''),
		  COALESCE(NULLIF(td.material, ''), NULLIF(td.material_other, ''), NULLIF(td.material_mode, ''), ''),
		  COALESCE(td.size_text, ''),
		  COALESCE(td.craft_text, ''),
		  t.created_at,
		  t.updated_at
		FROM tasks t
		LEFT JOIN task_details td ON td.task_id = t.id
		LEFT JOIN users creator ON creator.id = t.creator_id
		LEFT JOIN users designer ON designer.id = t.designer_id
		WHERE t.created_at >= ? AND t.created_at < ?
		ORDER BY t.created_at DESC
		LIMIT ?`
	rows, err := r.db.db.QueryContext(ctx, query, filter.From, filter.To, limit)
	if err != nil {
		return nil, fmt.Errorf("list business trend task texts: %w", err)
	}
	defer rows.Close()

	out := make([]domain.BusinessTrendTaskText, 0, limit)
	taskIDs := make([]int64, 0, limit)
	indexByTaskID := make(map[int64]int, limit)
	for rows.Next() {
		var item domain.BusinessTrendTaskText
		if err := rows.Scan(
			&item.ID,
			&item.TaskNo,
			&item.SKUCode,
			&item.ProductName,
			&item.TaskType,
			&item.BusinessLane,
			&item.CategoryName,
			&item.TaskStatus,
			&item.Priority,
			&item.CreatorName,
			&item.DesignerName,
			&item.DemandText,
			&item.CopyText,
			&item.Remark,
			&item.ChangeRequest,
			&item.DesignRequirement,
			&item.ProductShortName,
			&item.Material,
			&item.SizeText,
			&item.CraftText,
			&item.CreatedAt,
			&item.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan business trend task text: %w", err)
		}
		indexByTaskID[item.ID] = len(out)
		taskIDs = append(taskIDs, item.ID)
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if len(taskIDs) == 0 {
		return out, nil
	}
	if err := r.attachBusinessTrendBatchItems(ctx, out, indexByTaskID, taskIDs, filter.BatchItemLimit); err != nil {
		return nil, err
	}
	return out, nil
}

func (r *businessTrendRepo) attachBusinessTrendBatchItems(ctx context.Context, tasks []domain.BusinessTrendTaskText, indexByTaskID map[int64]int, taskIDs []int64, limit int) error {
	if limit <= 0 {
		limit = 600
	}
	if limit > 3000 {
		limit = 3000
	}
	placeholders := make([]string, len(taskIDs))
	args := make([]any, 0, len(taskIDs)+1)
	for i, id := range taskIDs {
		placeholders[i] = "?"
		args = append(args, id)
	}
	args = append(args, limit)
	query := `
		SELECT
		  id,
		  task_id,
		  sequence_no,
		  COALESCE(sku_code, ''),
		  COALESCE(product_name_snapshot, ''),
		  COALESCE(product_short_name, ''),
		  COALESCE(category_code, ''),
		  COALESCE(material_mode, ''),
		  COALESCE(design_requirement, ''),
		  quantity
		FROM task_sku_items
		WHERE task_id IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY task_id ASC, sequence_no ASC
		LIMIT ?`
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("list business trend batch items: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.BusinessTrendTaskSKUItem
		var quantity sql.NullInt64
		if err := rows.Scan(
			&item.ID,
			&item.TaskID,
			&item.SequenceNo,
			&item.SKUCode,
			&item.ProductName,
			&item.ProductShortName,
			&item.CategoryCode,
			&item.MaterialMode,
			&item.DesignRequirement,
			&quantity,
		); err != nil {
			return fmt.Errorf("scan business trend batch item: %w", err)
		}
		if quantity.Valid {
			q := quantity.Int64
			item.Quantity = &q
		}
		idx, ok := indexByTaskID[item.TaskID]
		if !ok {
			continue
		}
		tasks[idx].BatchItems = append(tasks[idx].BatchItems, item)
	}
	return rows.Err()
}
