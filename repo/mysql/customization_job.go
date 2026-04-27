package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type customizationJobRepo struct{ db *DB }

func NewCustomizationJobRepo(db *DB) repo.CustomizationJobRepo { return &customizationJobRepo{db: db} }

func (r *customizationJobRepo) Create(ctx context.Context, tx repo.Tx, job *domain.CustomizationJob) (int64, error) {
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO customization_jobs
		  (task_id, order_no, source_asset_id, current_asset_id, customization_level_code, customization_level_name,
		   review_reference_unit_price, review_reference_weight_factor, unit_price, weight_factor,
		   note, customization_review_decision, decision_type,
		   assigned_operator_id, last_operator_id, pricing_worker_type, status, warehouse_reject_reason, warehouse_reject_category)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		job.TaskID,
		job.OrderNo,
		toNullInt64(job.SourceAssetID),
		toNullInt64(job.CurrentAssetID),
		job.CustomizationLevelCode,
		job.CustomizationLevelName,
		toNullFloat64(job.ReviewReferenceUnitPrice),
		toNullFloat64(job.ReviewReferenceWeightFactor),
		toNullFloat64(job.UnitPrice),
		toNullFloat64(job.WeightFactor),
		job.Note,
		string(job.ReviewDecision),
		string(job.DecisionType),
		toNullInt64(job.AssignedOperatorID),
		toNullInt64(job.LastOperatorID),
		string(job.PricingWorkerType),
		string(job.Status),
		job.WarehouseRejectReason,
		job.WarehouseRejectCategory,
	)
	if err != nil {
		return 0, fmt.Errorf("insert customization_job: %w", err)
	}
	return res.LastInsertId()
}

func (r *customizationJobRepo) GetByID(ctx context.Context, id int64) (*domain.CustomizationJob, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, order_no, source_asset_id, current_asset_id, customization_level_code, customization_level_name,
		       review_reference_unit_price, review_reference_weight_factor, unit_price, weight_factor,
		       note, customization_review_decision, decision_type,
		       assigned_operator_id, last_operator_id, pricing_worker_type, status, warehouse_reject_reason, warehouse_reject_category,
		       created_at, updated_at
		FROM customization_jobs WHERE id = ?`, id)
	return scanCustomizationJob(row)
}

func (r *customizationJobRepo) GetLatestByTaskID(ctx context.Context, taskID int64) (*domain.CustomizationJob, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, task_id, order_no, source_asset_id, current_asset_id, customization_level_code, customization_level_name,
		       review_reference_unit_price, review_reference_weight_factor, unit_price, weight_factor,
		       note, customization_review_decision, decision_type,
		       assigned_operator_id, last_operator_id, pricing_worker_type, status, warehouse_reject_reason, warehouse_reject_category,
		       created_at, updated_at
		FROM customization_jobs
		WHERE task_id = ?
		ORDER BY id DESC
		LIMIT 1`, taskID)
	return scanCustomizationJob(row)
}

func (r *customizationJobRepo) List(ctx context.Context, filter repo.CustomizationJobListFilter) ([]*domain.CustomizationJob, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.TaskID != nil {
		where = append(where, "task_id = ?")
		args = append(args, *filter.TaskID)
	}
	if filter.Status != nil {
		where = append(where, "status = ?")
		args = append(args, string(*filter.Status))
	}
	if filter.OperatorID != nil {
		where = append(where, "(assigned_operator_id = ? OR last_operator_id = ?)")
		args = append(args, *filter.OperatorID, *filter.OperatorID)
	}
	whereSQL := strings.Join(where, " AND ")

	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM customization_jobs WHERE %s`, whereSQL)
	var total int64
	if err := r.db.db.QueryRowContext(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count customization_jobs: %w", err)
	}

	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	offset := (page - 1) * pageSize

	query := fmt.Sprintf(`
		SELECT id, task_id, order_no, source_asset_id, current_asset_id, customization_level_code, customization_level_name,
		       review_reference_unit_price, review_reference_weight_factor, unit_price, weight_factor,
		       note, customization_review_decision, decision_type,
		       assigned_operator_id, last_operator_id, pricing_worker_type, status, warehouse_reject_reason, warehouse_reject_category,
		       created_at, updated_at
		FROM customization_jobs
		WHERE %s
		ORDER BY id DESC
		LIMIT ? OFFSET ?`, whereSQL)
	args = append(args, pageSize, offset)
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list customization_jobs: %w", err)
	}
	defer rows.Close()

	items := make([]*domain.CustomizationJob, 0)
	for rows.Next() {
		item, err := scanCustomizationJob(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, err
	}
	return items, total, nil
}

func (r *customizationJobRepo) Update(ctx context.Context, tx repo.Tx, job *domain.CustomizationJob) error {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE customization_jobs
		SET source_asset_id = ?,
		    current_asset_id = ?,
		    order_no = ?,
		    customization_level_code = ?,
		    customization_level_name = ?,
		    review_reference_unit_price = ?,
		    review_reference_weight_factor = ?,
		    unit_price = ?,
		    weight_factor = ?,
		    note = ?,
		    customization_review_decision = ?,
		    decision_type = ?,
		    assigned_operator_id = ?,
		    last_operator_id = ?,
		    pricing_worker_type = ?,
		    status = ?,
		    warehouse_reject_reason = ?,
		    warehouse_reject_category = ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		toNullInt64(job.SourceAssetID),
		toNullInt64(job.CurrentAssetID),
		job.OrderNo,
		job.CustomizationLevelCode,
		job.CustomizationLevelName,
		toNullFloat64(job.ReviewReferenceUnitPrice),
		toNullFloat64(job.ReviewReferenceWeightFactor),
		toNullFloat64(job.UnitPrice),
		toNullFloat64(job.WeightFactor),
		job.Note,
		string(job.ReviewDecision),
		string(job.DecisionType),
		toNullInt64(job.AssignedOperatorID),
		toNullInt64(job.LastOperatorID),
		string(job.PricingWorkerType),
		string(job.Status),
		job.WarehouseRejectReason,
		job.WarehouseRejectCategory,
		job.ID,
	)
	if err != nil {
		return fmt.Errorf("update customization_job: %w", err)
	}
	return nil
}

func scanCustomizationJob(scanner interface{ Scan(...interface{}) error }) (*domain.CustomizationJob, error) {
	var item domain.CustomizationJob
	var sourceAssetID, currentAssetID, assignedOperatorID, lastOperatorID sql.NullInt64
	var reviewReferenceUnitPrice, reviewReferenceWeightFactor sql.NullFloat64
	var unitPrice, weightFactor sql.NullFloat64
	err := scanner.Scan(
		&item.ID,
		&item.TaskID,
		&item.OrderNo,
		&sourceAssetID,
		&currentAssetID,
		&item.CustomizationLevelCode,
		&item.CustomizationLevelName,
		&reviewReferenceUnitPrice,
		&reviewReferenceWeightFactor,
		&unitPrice,
		&weightFactor,
		&item.Note,
		&item.ReviewDecision,
		&item.DecisionType,
		&assignedOperatorID,
		&lastOperatorID,
		&item.PricingWorkerType,
		&item.Status,
		&item.WarehouseRejectReason,
		&item.WarehouseRejectCategory,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan customization_job: %w", err)
	}
	item.SourceAssetID = fromNullInt64(sourceAssetID)
	item.CurrentAssetID = fromNullInt64(currentAssetID)
	item.ReviewReferenceUnitPrice = fromNullFloat64(reviewReferenceUnitPrice)
	item.ReviewReferenceWeightFactor = fromNullFloat64(reviewReferenceWeightFactor)
	item.UnitPrice = fromNullFloat64(unitPrice)
	item.WeightFactor = fromNullFloat64(weightFactor)
	item.AssignedOperatorID = fromNullInt64(assignedOperatorID)
	item.LastOperatorID = fromNullInt64(lastOperatorID)
	return &item, nil
}
