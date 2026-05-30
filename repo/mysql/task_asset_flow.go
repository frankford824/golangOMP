package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func (r *taskAssetRepo) MarkAssetVersionSuperseded(ctx context.Context, tx repo.Tx, versionID, supersededByVersionID int64, supersededAt, cleanupAfterAt time.Time) error {
	if versionID <= 0 || supersededByVersionID <= 0 || versionID == supersededByVersionID {
		return nil
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE task_assets
		   SET flow_review_status = ?,
		       superseded_by_version_id = ?,
		       superseded_at = ?,
		       cleanup_after_at = ?
		 WHERE id = ?
		   AND deleted_at IS NULL
		   AND cleaned_at IS NULL`,
		string(domain.TaskAssetFlowReviewStatusSuperseded), supersededByVersionID, supersededAt, cleanupAfterAt, versionID)
	if err != nil {
		return fmt.Errorf("mark task asset version superseded: %w", err)
	}
	if _, err := res.RowsAffected(); err != nil {
		return err
	}
	return nil
}

func (r *taskAssetRepo) MarkCurrentDeliveryVersionsApprovedForTask(ctx context.Context, tx repo.Tx, taskID, actorID int64, approvedAt time.Time) (int64, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE task_assets ta
		JOIN design_assets da ON da.id = ta.asset_id AND da.current_version_id = ta.id
		   SET ta.flow_review_status = ?,
		       ta.approved_at = ?,
		       ta.approved_by = ?,
		       ta.rejected_at = NULL,
		       ta.rejected_by = NULL
		 WHERE ta.task_id = ?
		   AND ta.asset_type = ?
		   AND ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL`,
		string(domain.TaskAssetFlowReviewStatusApproved), approvedAt, actorID, taskID, string(domain.TaskAssetTypeDelivery))
	if err != nil {
		return 0, fmt.Errorf("mark current delivery versions approved: %w", err)
	}
	return res.RowsAffected()
}

func (r *taskAssetRepo) MarkCurrentDeliveryVersionsRejectedForTask(ctx context.Context, tx repo.Tx, taskID, actorID int64, rejectedAt time.Time) (int64, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE task_assets ta
		JOIN design_assets da ON da.id = ta.asset_id AND da.current_version_id = ta.id
		   SET ta.flow_review_status = ?,
		       ta.rejected_at = ?,
		       ta.rejected_by = ?
		 WHERE ta.task_id = ?
		   AND ta.asset_type = ?
		   AND ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL`,
		string(domain.TaskAssetFlowReviewStatusRejected), rejectedAt, actorID, taskID, string(domain.TaskAssetTypeDelivery))
	if err != nil {
		return 0, fmt.Errorf("mark current delivery versions rejected: %w", err)
	}
	return res.RowsAffected()
}

func (r *taskRepo) CASUpdateStatus(ctx context.Context, tx repo.Tx, id int64, expected, next domain.TaskStatus) (bool, error) {
	res, err := Unwrap(tx).ExecContext(ctx,
		`UPDATE tasks SET task_status = ? WHERE id = ? AND task_status = ?`,
		string(next), id, string(expected),
	)
	if err != nil {
		return false, fmt.Errorf("cas update task status: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	if err := reindexTaskSearchDocument(ctx, Unwrap(tx), id); err != nil {
		return false, err
	}
	return true, nil
}

func (r *taskAssetRepo) CountUnapprovedCurrentDeliveryVersions(ctx context.Context, taskID int64) (int64, error) {
	var n int64
	err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		  FROM task_assets ta
		  JOIN design_assets da ON da.id = ta.asset_id AND da.current_version_id = ta.id
		 WHERE ta.task_id = ?
		   AND ta.asset_type = ?
		   AND ta.deleted_at IS NULL
		   AND ta.cleaned_at IS NULL
		   AND ta.flow_review_status <> ?`,
		taskID, string(domain.TaskAssetTypeDelivery), string(domain.TaskAssetFlowReviewStatusApproved)).Scan(&n)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("count unapproved current delivery versions: %w", err)
	}
	return n, nil
}

func (r *taskAssetRepo) ListWarehouseAutoReleaseCandidates(ctx context.Context, cutoff time.Time, limit int) ([]int64, error) {
	if limit <= 0 || limit > 1000 {
		limit = 100
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT t.id
		  FROM tasks t
		  LEFT JOIN warehouse_receipts wr ON wr.task_id = t.id
		 WHERE t.task_status IN (?, ?, ?)
		   AND t.task_type IN (?, ?, ?, ?)
		   AND COALESCE(t.sku_code, '') <> ''
		   AND t.updated_at <= ?
		   AND (wr.id IS NULL OR wr.status IN (?, ?))
		   AND EXISTS (
		        SELECT 1
		          FROM task_assets ta
		          JOIN design_assets da ON da.id = ta.asset_id AND da.current_version_id = ta.id
		         WHERE ta.task_id = t.id
		           AND ta.asset_type IN (?, ?, ?, ?, ?)
		           AND ta.deleted_at IS NULL
		           AND ta.cleaned_at IS NULL
		   )
		   AND NOT EXISTS (
		        SELECT 1
		          FROM task_assets ta
		          JOIN design_assets da ON da.id = ta.asset_id AND da.current_version_id = ta.id
		         WHERE ta.task_id = t.id
		           AND ta.asset_type IN (?, ?, ?, ?, ?)
		           AND ta.deleted_at IS NULL
		           AND ta.cleaned_at IS NULL
		           AND COALESCE(ta.flow_review_status, '') <> ?
		   )
		 ORDER BY t.updated_at ASC, t.id ASC
		 LIMIT ?`,
		string(domain.TaskStatusPendingWarehouseReceive), string(domain.TaskStatusPendingProductionTransfer), string(domain.TaskStatusPendingClose),
		string(domain.TaskTypeOriginalProductDevelopment), string(domain.TaskTypeNewProductDevelopment), string(domain.TaskTypeCustomerCustomization), string(domain.TaskTypeRegularCustomization),
		cutoff,
		string(domain.WarehouseReceiptStatusReceived), string(domain.WarehouseReceiptStatusCompleted),
		string(domain.TaskAssetTypeDelivery), string(domain.TaskAssetTypeDraft), string(domain.TaskAssetTypeRevised), string(domain.TaskAssetTypeFinal), string(domain.TaskAssetTypeOutsourceReturn),
		string(domain.TaskAssetTypeDelivery), string(domain.TaskAssetTypeDraft), string(domain.TaskAssetTypeRevised), string(domain.TaskAssetTypeFinal), string(domain.TaskAssetTypeOutsourceReturn),
		string(domain.TaskAssetFlowReviewStatusApproved),
		limit)
	if err != nil {
		return nil, fmt.Errorf("list warehouse auto release candidates: %w", err)
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan warehouse auto release candidate: %w", err)
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}
