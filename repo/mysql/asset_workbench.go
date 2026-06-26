package mysqlrepo

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type assetWorkbenchRepo struct{ db *DB }

func NewAssetWorkbenchRepo(db *DB) repo.AssetWorkbenchRepo {
	return &assetWorkbenchRepo{db: db}
}

func (r *assetWorkbenchRepo) GetProfileByUserID(ctx context.Context, userID int64) (*domain.AssetWorkbenchProfile, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchProfileSelect()+` WHERE user_id = ?`, userID)
	return scanAssetWorkbenchProfile(row)
}

func (r *assetWorkbenchRepo) ListProfiles(ctx context.Context, filter repo.AssetWorkbenchProfileFilter) ([]*domain.AssetWorkbenchProfile, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.Keyword); v != "" {
		like := "%" + v + "%"
		where = append(where, "(real_name LIKE ? OR phone LIKE ? OR alipay_account LIKE ?)")
		args = append(args, like, like, like)
	}
	if v := strings.TrimSpace(filter.WorkerType); v != "" {
		where = append(where, "worker_type = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.JobGrade); v != "" {
		where = append(where, "job_grade = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Status); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_profiles WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench profiles: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchProfileSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY updated_at DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench profiles: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchProfile{}
	for rows.Next() {
		item, err := scanAssetWorkbenchProfile(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) UpsertProfile(ctx context.Context, tx repo.Tx, profile *domain.AssetWorkbenchProfile) (*domain.AssetWorkbenchProfile, error) {
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO asset_workbench_profiles (
			user_id, worker_type, job_grade, real_name, phone, province, city, id_card, gender,
			alipay_account, onboarded_at, grade_hidden, status, pii_completed, created_by, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			worker_type = VALUES(worker_type),
			job_grade = VALUES(job_grade),
			real_name = VALUES(real_name),
			phone = VALUES(phone),
			province = VALUES(province),
			city = VALUES(city),
			id_card = VALUES(id_card),
			gender = VALUES(gender),
			alipay_account = VALUES(alipay_account),
			onboarded_at = VALUES(onboarded_at),
			grade_hidden = VALUES(grade_hidden),
			status = VALUES(status),
			pii_completed = VALUES(pii_completed),
			updated_by = VALUES(updated_by),
			updated_at = CURRENT_TIMESTAMP`,
		profile.UserID,
		profile.WorkerType,
		profile.JobGrade,
		profile.RealName,
		toNullString(profile.Phone),
		profile.Province,
		profile.City,
		toNullString(profile.IDCard),
		profile.Gender,
		profile.AlipayAccount,
		toNullTime(profile.OnboardedAt),
		profile.GradeHidden,
		profile.Status,
		profile.PIICompleted,
		toNullInt64(profile.CreatedBy),
		toNullInt64(profile.UpdatedBy),
	)
	if err != nil {
		return nil, fmt.Errorf("upsert asset workbench profile: %w", err)
	}
	row := sqlTx.QueryRowContext(ctx, assetWorkbenchProfileSelect()+` WHERE user_id = ?`, profile.UserID)
	return scanAssetWorkbenchProfile(row)
}

func (r *assetWorkbenchRepo) AppendGradePeriod(ctx context.Context, tx repo.Tx, period *domain.AssetWorkbenchGradePeriod) (*domain.AssetWorkbenchGradePeriod, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_profile_grade_periods (
			profile_id, user_id, worker_type, job_grade, effective_from, effective_to, changed_by, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		period.ProfileID,
		period.UserID,
		period.WorkerType,
		period.JobGrade,
		period.EffectiveFrom,
		toNullTime(period.EffectiveTo),
		toNullInt64(period.ChangedBy),
		period.Reason,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench grade period: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench grade period last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchGradePeriodSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchGradePeriod(row)
}

func (r *assetWorkbenchRepo) ListPriceMatrix(ctx context.Context, filter repo.AssetWorkbenchPriceMatrixFilter) ([]*domain.AssetWorkbenchPriceMatrix, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.WorkerType); v != "" {
		where = append(where, "worker_type = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.JobGrade); v != "" {
		where = append(where, "job_grade = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.DifficultyClass); v != "" {
		where = append(where, "difficulty_class = ?")
		args = append(args, v)
	}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_price_matrix WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench price matrix: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchPriceMatrixSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY worker_type ASC, job_grade ASC, difficulty_class ASC, effective_from DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench price matrix: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchPriceMatrix{}
	for rows.Next() {
		item, err := scanAssetWorkbenchPriceMatrix(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) LockPriceMatrixDimension(ctx context.Context, tx repo.Tx, workerType, jobGrade, difficultyClass string) ([]*domain.AssetWorkbenchPriceMatrix, error) {
	sqlTx := Unwrap(tx)
	if err := lockAssetWorkbenchPriceMatrixDimension(ctx, sqlTx, workerType, jobGrade, difficultyClass); err != nil {
		return nil, err
	}
	rows, err := sqlTx.QueryContext(ctx, assetWorkbenchPriceMatrixSelect()+`
		WHERE worker_type = ? AND job_grade = ? AND difficulty_class = ?
		ORDER BY effective_from ASC, id ASC
		FOR UPDATE`, workerType, jobGrade, difficultyClass)
	if err != nil {
		return nil, fmt.Errorf("lock asset workbench price matrix dimension: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchPriceMatrix{}
	for rows.Next() {
		item, err := scanAssetWorkbenchPriceMatrix(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func lockAssetWorkbenchPriceMatrixDimension(ctx context.Context, tx *sql.Tx, workerType, jobGrade, difficultyClass string) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO asset_workbench_price_matrix_dimensions (
			worker_type, job_grade, difficulty_class
		) VALUES (?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = updated_at`, workerType, jobGrade, difficultyClass); err != nil {
		return fmt.Errorf("ensure asset workbench price matrix dimension: %w", err)
	}
	var id int64
	if err := tx.QueryRowContext(ctx, `
		SELECT id
		FROM asset_workbench_price_matrix_dimensions
		WHERE worker_type = ? AND job_grade = ? AND difficulty_class = ?
		FOR UPDATE`, workerType, jobGrade, difficultyClass).Scan(&id); err != nil {
		return fmt.Errorf("lock asset workbench price matrix dimension row: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) CreatePriceMatrix(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchPriceMatrix) (*domain.AssetWorkbenchPriceMatrix, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_price_matrix (
			worker_type, job_grade, difficulty_class, unit_price, effective_from, effective_to,
			enabled, revision_no, created_by, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.WorkerType,
		item.JobGrade,
		item.DifficultyClass,
		item.UnitPrice,
		item.EffectiveFrom,
		toNullTime(item.EffectiveTo),
		item.Enabled,
		item.RevisionNo,
		item.CreatedBy,
		item.Remark,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench price matrix: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench price matrix last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchPriceMatrixSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchPriceMatrix(row)
}

func (r *assetWorkbenchRepo) FindActivePrice(ctx context.Context, workerType, jobGrade, difficultyClass string, asOf time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	date := asOf.Format("2006-01-02")
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchPriceMatrixSelect()+`
		WHERE worker_type IN (?, ?)
		  AND job_grade IN (?, ?)
		  AND difficulty_class = ?
		  AND enabled = 1
		  AND effective_from <= ?
		  AND (effective_to IS NULL OR effective_to >= ?)
		ORDER BY (worker_type = ?) DESC, (job_grade = ?) DESC, effective_from DESC, id DESC
		LIMIT 1`,
		workerType, domain.AssetWorkbenchWorkerTypeAll,
		jobGrade, domain.AssetWorkbenchWorkerTypeAll,
		difficultyClass,
		date,
		date,
		workerType,
		jobGrade,
	)
	return scanAssetWorkbenchPriceMatrix(row)
}

func (r *assetWorkbenchRepo) ListDeductionRules(ctx context.Context, filter repo.AssetWorkbenchDeductionRuleFilter) ([]*domain.AssetWorkbenchDeductionRule, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.WorkerType); v != "" {
		where = append(where, "worker_type = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.JobGrade); v != "" {
		where = append(where, "job_grade = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.DifficultyClass); v != "" {
		where = append(where, "difficulty_class = ?")
		args = append(args, v)
	}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_deduction_rules WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench deduction rules: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchDeductionRuleSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY worker_type ASC, job_grade ASC, difficulty_class ASC, effective_from DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench deduction rules: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchDeductionRule{}
	for rows.Next() {
		item, err := scanAssetWorkbenchDeductionRule(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) LockDeductionRuleDimension(ctx context.Context, tx repo.Tx, workerType, jobGrade, difficultyClass string) ([]*domain.AssetWorkbenchDeductionRule, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, assetWorkbenchDeductionRuleSelect()+`
		WHERE worker_type = ? AND job_grade = ? AND difficulty_class = ?
		ORDER BY effective_from ASC, id ASC
		FOR UPDATE`, workerType, jobGrade, difficultyClass)
	if err != nil {
		return nil, fmt.Errorf("lock asset workbench deduction rule dimension: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchDeductionRule{}
	for rows.Next() {
		item, err := scanAssetWorkbenchDeductionRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) CreateDeductionRule(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchDeductionRule) (*domain.AssetWorkbenchDeductionRule, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_deduction_rules (
			worker_type, job_grade, difficulty_class, deduction_amount, effective_from, effective_to,
			enabled, revision_no, created_by, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.WorkerType,
		item.JobGrade,
		item.DifficultyClass,
		item.DeductionAmount,
		item.EffectiveFrom,
		toNullTime(item.EffectiveTo),
		item.Enabled,
		item.RevisionNo,
		item.CreatedBy,
		item.Remark,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench deduction rule: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench deduction rule last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchDeductionRuleSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchDeductionRule(row)
}

func (r *assetWorkbenchRepo) ListWelfareRules(ctx context.Context, filter repo.AssetWorkbenchWelfareRuleFilter) ([]*domain.AssetWorkbenchWelfareRule, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.WorkerType); v != "" {
		where = append(where, "worker_type = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.JobGrade); v != "" {
		where = append(where, "job_grade = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.RuleType); v != "" {
		where = append(where, "rule_type = ?")
		args = append(args, v)
	}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_welfare_rules WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench welfare rules: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchWelfareRuleSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY worker_type ASC, job_grade ASC, rule_type ASC, effective_from DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench welfare rules: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchWelfareRule{}
	for rows.Next() {
		item, err := scanAssetWorkbenchWelfareRule(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) CreateWelfareRule(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchWelfareRule) (*domain.AssetWorkbenchWelfareRule, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_welfare_rules (
			rule_name, worker_type, job_grade, rule_type, amount, config_json, effective_from, effective_to,
			enabled, created_by, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.RuleName,
		item.WorkerType,
		item.JobGrade,
		item.RuleType,
		item.Amount,
		nullableJSON(item.Config),
		item.EffectiveFrom,
		toNullTime(item.EffectiveTo),
		item.Enabled,
		item.CreatedBy,
		item.Remark,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench welfare rule: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench welfare rule last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchWelfareRuleSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchWelfareRule(row)
}

func (r *assetWorkbenchRepo) FindActiveWelfareRules(ctx context.Context, workerType, jobGrade string, asOf time.Time) ([]*domain.AssetWorkbenchWelfareRule, error) {
	date := asOf.Format("2006-01-02")
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchWelfareRuleSelect()+`
		WHERE worker_type IN (?, ?)
		  AND job_grade IN (?, ?)
		  AND enabled = 1
		  AND effective_from <= ?
		  AND (effective_to IS NULL OR effective_to >= ?)
		ORDER BY (worker_type = ?) DESC, (job_grade = ?) DESC, effective_from DESC, id DESC`,
		workerType, domain.AssetWorkbenchWorkerTypeAll,
		jobGrade, domain.AssetWorkbenchWorkerTypeAll,
		date,
		date,
		workerType,
		jobGrade,
	)
	if err != nil {
		return nil, fmt.Errorf("find active asset workbench welfare rules: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchWelfareRule{}
	for rows.Next() {
		item, err := scanAssetWorkbenchWelfareRule(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) ListPromoCoupons(ctx context.Context, filter repo.AssetWorkbenchPromoCouponFilter) ([]*domain.AssetWorkbenchPromoCoupon, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.WorkerType); v != "" {
		where = append(where, "worker_type = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.JobGrade); v != "" {
		where = append(where, "job_grade = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.DifficultyClass); v != "" {
		where = append(where, "difficulty_class = ?")
		args = append(args, v)
	}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_promo_coupons WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench promo coupons: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchPromoCouponSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY priority ASC, effective_from DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench promo coupons: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchPromoCoupon{}
	for rows.Next() {
		item, err := scanAssetWorkbenchPromoCoupon(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) CreatePromoCoupon(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchPromoCoupon) (*domain.AssetWorkbenchPromoCoupon, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_promo_coupons (
			coupon_code, coupon_name, mode, amount, percent, priority, worker_type, job_grade,
			difficulty_class, eligible_user_ids_json, eligible_codes_json, effective_from, effective_to,
			enabled, stack_policy, created_by, remark
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.CouponCode,
		item.CouponName,
		item.Mode,
		toNullFloat64(item.Amount),
		toNullFloat64(item.Percent),
		item.Priority,
		item.WorkerType,
		item.JobGrade,
		item.DifficultyClass,
		nullableJSON(item.EligibleUserIDs),
		nullableJSON(item.EligibleCodes),
		item.EffectiveFrom,
		toNullTime(item.EffectiveTo),
		item.Enabled,
		item.StackPolicy,
		item.CreatedBy,
		item.Remark,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench promo coupon: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench promo coupon last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchPromoCouponSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchPromoCoupon(row)
}

func (r *assetWorkbenchRepo) ListActivePromoCoupons(ctx context.Context, workerType, jobGrade, difficultyClass string, asOf time.Time) ([]*domain.AssetWorkbenchPromoCoupon, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchPromoCouponSelect()+`
		WHERE worker_type IN (?, ?)
		  AND job_grade IN (?, ?)
		  AND difficulty_class IN (?, ?)
		  AND enabled = 1
		  AND effective_from <= ?
		  AND (effective_to IS NULL OR effective_to >= ?)
		ORDER BY priority ASC, id DESC`,
		workerType, domain.AssetWorkbenchWorkerTypeAll,
		jobGrade, domain.AssetWorkbenchWorkerTypeAll,
		difficultyClass, domain.AssetWorkbenchWorkerTypeAll,
		asOf,
		asOf,
	)
	if err != nil {
		return nil, fmt.Errorf("list active asset workbench promo coupons: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchPromoCoupon{}
	for rows.Next() {
		item, err := scanAssetWorkbenchPromoCoupon(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) CreateUploadSession(ctx context.Context, tx repo.Tx, session *domain.AssetWorkbenchUploadSession) (*domain.AssetWorkbenchUploadSession, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_upload_sessions (
			session_id, owner_user_id, status, object_key, original_filename, file_size, mime_type,
			file_hash, upload_id, multipart_plan_json, expires_at, uploaded_at, cancelled_at, submitted_item_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.SessionID,
		session.OwnerUserID,
		session.Status,
		session.ObjectKey,
		session.OriginalFilename,
		session.FileSize,
		session.MimeType,
		session.FileHash,
		session.UploadID,
		nullableJSON(session.MultipartPlan),
		session.ExpiresAt,
		toNullTime(session.UploadedAt),
		toNullTime(session.CancelledAt),
		toNullInt64(session.SubmittedItemID),
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench upload session: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench upload session last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchUploadSessionSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchUploadSession(row)
}

func (r *assetWorkbenchRepo) GetUploadSession(ctx context.Context, sessionID string) (*domain.AssetWorkbenchUploadSession, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchUploadSessionSelect()+` WHERE session_id = ?`, sessionID)
	return scanAssetWorkbenchUploadSession(row)
}

func (r *assetWorkbenchRepo) UpdateUploadSessionStatus(ctx context.Context, tx repo.Tx, sessionID, status string, uploadedAt *time.Time, cancelledAt *time.Time, submittedItemID *int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_upload_sessions
		SET status = ?,
		    uploaded_at = COALESCE(?, uploaded_at),
		    cancelled_at = COALESCE(?, cancelled_at),
		    submitted_item_id = COALESCE(?, submitted_item_id),
		    updated_at = CURRENT_TIMESTAMP
		WHERE session_id = ?`,
		status,
		toNullTime(uploadedAt),
		toNullTime(cancelledAt),
		toNullInt64(submittedItemID),
		sessionID,
	)
	if err != nil {
		return fmt.Errorf("update asset workbench upload session status: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) ListExpiredUploadSessions(ctx context.Context, now time.Time, limit int) ([]*domain.AssetWorkbenchUploadSession, error) {
	if limit <= 0 || limit > 500 {
		limit = 100
	}
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchUploadSessionSelect()+`
		WHERE expires_at <= ?
		  AND status IN (?, ?)
		ORDER BY expires_at ASC, id ASC
		LIMIT ?`,
		now,
		domain.AssetWorkbenchUploadStatusCreated,
		domain.AssetWorkbenchUploadStatusUploading,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list expired asset workbench upload sessions: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchUploadSession{}
	for rows.Next() {
		item, err := scanAssetWorkbenchUploadSession(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) CreateSubmission(ctx context.Context, tx repo.Tx, submission *domain.AssetWorkbenchSubmission) (*domain.AssetWorkbenchSubmission, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_submissions (
			submission_no, submitter_user_id, business_month, submitted_at, status, item_count,
			file_count, page_count, gross_total, notes
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		submission.SubmissionNo,
		submission.SubmitterUserID,
		submission.BusinessMonth,
		submission.SubmittedAt,
		submission.Status,
		submission.ItemCount,
		submission.FileCount,
		submission.PageCount,
		submission.GrossTotal,
		submission.Notes,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench submission: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench submission last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchSubmission(row)
}

func (r *assetWorkbenchRepo) GetSubmission(ctx context.Context, submissionID int64) (*domain.AssetWorkbenchSubmission, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchSubmissionSelect()+` WHERE id = ?`, submissionID)
	return scanAssetWorkbenchSubmission(row)
}

func (r *assetWorkbenchRepo) CreateSubmissionItem(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_submission_items (
			submission_id, payee_user_id, order_no, difficulty_class, finalized, page_count, item_count,
			business_month, submitted_at, worker_type_snapshot, job_grade_snapshot, base_price_rule_id,
			base_unit_price, promo_coupon_id, promo_snapshot_json, pricing_snapshot_json, gross_amount,
			pricing_status, qc_status, settlement_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.SubmissionID,
		item.PayeeUserID,
		item.OrderNo,
		item.DifficultyClass,
		item.Finalized,
		item.PageCount,
		item.ItemCount,
		item.BusinessMonth,
		item.SubmittedAt,
		item.WorkerTypeSnapshot,
		item.JobGradeSnapshot,
		toNullInt64(item.BasePriceRuleID),
		toNullFloat64(item.BaseUnitPrice),
		toNullInt64(item.PromoCouponID),
		nullableJSON(item.PromoSnapshot),
		nullableJSON(item.PricingSnapshot),
		item.GrossAmount,
		item.PricingStatus,
		item.QCStatus,
		item.SettlementStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench submission item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench submission item last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionItemSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchSubmissionItem(row)
}

func (r *assetWorkbenchRepo) GetSubmissionItem(ctx context.Context, itemID int64) (*domain.AssetWorkbenchSubmissionItem, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchSubmissionItemSelect()+` WHERE id = ?`, itemID)
	return scanAssetWorkbenchSubmissionItem(row)
}

func (r *assetWorkbenchRepo) UpdateSubmissionItemQCStatus(ctx context.Context, tx repo.Tx, itemID int64, qcStatus string) (*domain.AssetWorkbenchSubmissionItem, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_items
		SET qc_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		  AND qc_status <> ?`,
		qcStatus, itemID, domain.AssetWorkbenchSettlementStatusUnsettled, domain.AssetWorkbenchSubmissionStatusVoided)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench submission item qc status: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("asset workbench submission item qc rows affected: %w", err)
	}
	if affected != 1 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission item cannot be changed after settlement or void.", map[string]interface{}{"item_id": itemID})
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionItemSelect()+` WHERE id = ?`, itemID)
	return scanAssetWorkbenchSubmissionItem(row)
}

func (r *assetWorkbenchRepo) VoidSubmissionItem(ctx context.Context, tx repo.Tx, itemID int64, actorID int64, reason string, at time.Time) (*domain.AssetWorkbenchSubmissionItem, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_items
		SET qc_status = ?, voided_at = ?, voided_by = ?, void_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		  AND qc_status <> ?`,
		domain.AssetWorkbenchSubmissionStatusVoided, at, actorID, reason, itemID,
		domain.AssetWorkbenchSettlementStatusUnsettled, domain.AssetWorkbenchSubmissionStatusVoided)
	if err != nil {
		return nil, fmt.Errorf("void asset workbench submission item: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("asset workbench void item rows affected: %w", err)
	}
	if affected != 1 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission item cannot be voided after settlement or when already voided.", map[string]interface{}{"item_id": itemID})
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionItemSelect()+` WHERE id = ?`, itemID)
	return scanAssetWorkbenchSubmissionItem(row)
}

func (r *assetWorkbenchRepo) UpdateSubmissionItemPricing(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	if item == nil {
		return nil, sql.ErrNoRows
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_items
		SET base_price_rule_id = ?, base_unit_price = ?, promo_coupon_id = ?,
		    promo_snapshot_json = ?, pricing_snapshot_json = ?, gross_amount = ?,
		    pricing_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		  AND qc_status <> ?`,
		toNullInt64(item.BasePriceRuleID),
		toNullFloat64(item.BaseUnitPrice),
		toNullInt64(item.PromoCouponID),
		nullableJSON(item.PromoSnapshot),
		nullableJSON(item.PricingSnapshot),
		item.GrossAmount,
		item.PricingStatus,
		item.ID,
		domain.AssetWorkbenchSettlementStatusUnsettled,
		domain.AssetWorkbenchSubmissionStatusVoided,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench submission item pricing: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("asset workbench pricing rows affected: %w", err)
	}
	if affected != 1 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission item cannot be repriced after settlement or void.", map[string]interface{}{"item_id": item.ID})
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionItemSelect()+` WHERE id = ?`, item.ID)
	return scanAssetWorkbenchSubmissionItem(row)
}

func (r *assetWorkbenchRepo) CreateSubmissionFile(ctx context.Context, tx repo.Tx, file *domain.AssetWorkbenchSubmissionFile) (*domain.AssetWorkbenchSubmissionFile, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_submission_files (
			submission_id, submission_item_id, upload_session_id, owner_user_id, object_key, preview_key,
			preview_status, original_filename, file_ext, file_type, mime_type, file_size, file_hash, sort_order
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		file.SubmissionID,
		file.SubmissionItemID,
		toNullInt64(file.UploadSessionID),
		file.OwnerUserID,
		file.ObjectKey,
		file.PreviewKey,
		file.PreviewStatus,
		file.OriginalFilename,
		file.FileExt,
		file.FileType,
		file.MimeType,
		file.FileSize,
		file.FileHash,
		file.SortOrder,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench submission file: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench submission file last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionFileSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchSubmissionFile(row)
}

func (r *assetWorkbenchRepo) RefreshSubmissionTotals(ctx context.Context, tx repo.Tx, submissionID int64) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submissions s
		SET item_count = (SELECT COUNT(*) FROM asset_workbench_submission_items i WHERE i.submission_id = s.id),
		    file_count = (SELECT COUNT(*) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id),
		    page_count = COALESCE((SELECT SUM(i.page_count) FROM asset_workbench_submission_items i WHERE i.submission_id = s.id), 0),
		    gross_total = COALESCE((SELECT SUM(i.gross_amount) FROM asset_workbench_submission_items i WHERE i.submission_id = s.id), 0),
		    updated_at = CURRENT_TIMESTAMP
		WHERE s.id = ?`, submissionID)
	if err != nil {
		return fmt.Errorf("refresh asset workbench submission totals: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) ListSubmissions(ctx context.Context, filter repo.AssetWorkbenchSubmissionFilter) ([]*domain.AssetWorkbenchSubmission, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.SubmitterUserID != nil {
		where = append(where, "submitter_user_id = ?")
		args = append(args, *filter.SubmitterUserID)
	}
	if filter.PayeeUserID != nil {
		where = append(where, `EXISTS (
			SELECT 1 FROM asset_workbench_submission_items i
			WHERE i.submission_id = asset_workbench_submissions.id AND i.payee_user_id = ?
		)`)
		args = append(args, *filter.PayeeUserID)
	}
	if v := strings.TrimSpace(filter.BusinessMonth); v != "" {
		where = append(where, "business_month = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Status); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.SettlementStatus); v != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM asset_workbench_submission_items i
			WHERE i.submission_id = asset_workbench_submissions.id AND i.settlement_status = ?
		)`)
		args = append(args, v)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_submissions WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench submissions: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSubmissionSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY submitted_at DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench submissions: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSubmission{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSubmission(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) ListSubmissionItems(ctx context.Context, submissionID int64) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSubmissionItemSelect()+` WHERE submission_id = ? ORDER BY id ASC`, submissionID)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench submission items: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSubmissionItem{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSubmissionItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) ListSubmissionItemsByMonth(ctx context.Context, businessMonth string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSubmissionItemSelect()+`
		WHERE business_month = ?
		ORDER BY order_no ASC, payee_user_id ASC, id ASC`, businessMonth)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench submission items by month: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSubmissionItem{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSubmissionItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) ListSubmissionFiles(ctx context.Context, submissionItemID int64) ([]*domain.AssetWorkbenchSubmissionFile, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSubmissionFileSelect()+` WHERE submission_item_id = ? ORDER BY sort_order ASC, id ASC`, submissionItemID)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench submission files: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSubmissionFile{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSubmissionFile(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) GetSubmissionFile(ctx context.Context, fileID int64) (*domain.AssetWorkbenchSubmissionFile, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchSubmissionFileSelect()+` WHERE id = ?`, fileID)
	return scanAssetWorkbenchSubmissionFile(row)
}

func (r *assetWorkbenchRepo) ClaimPendingPreviewFiles(ctx context.Context, claim repo.AssetWorkbenchPreviewClaim) ([]*domain.AssetWorkbenchSubmissionFile, error) {
	if claim.Limit <= 0 || claim.Limit > 100 {
		claim.Limit = 8
	}
	if claim.Now.IsZero() {
		claim.Now = time.Now().UTC()
	}
	if claim.LeaseTTL <= 0 {
		claim.LeaseTTL = 5 * time.Minute
	}
	workerID := strings.TrimSpace(claim.WorkerID)
	if workerID == "" {
		workerID = "asset-workbench-preview-worker"
	}
	sqlTx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin asset workbench preview claim tx: %w", err)
	}
	defer rollback(sqlTx)

	rows, err := sqlTx.QueryContext(ctx, assetWorkbenchSubmissionFileSelect()+`
		WHERE (preview_status = ? OR (preview_status = ? AND preview_lease_expires_at <= ?))
		  AND (preview_next_retry_at IS NULL OR preview_next_retry_at <= ?)
		  AND (preview_lease_expires_at IS NULL OR preview_lease_expires_at <= ?)
		ORDER BY id ASC
		LIMIT ?
		FOR UPDATE SKIP LOCKED`,
		domain.AssetWorkbenchPreviewStatusPending,
		domain.AssetWorkbenchPreviewStatusProcessing,
		claim.Now,
		claim.Now,
		claim.Now,
		claim.Limit,
	)
	if err != nil {
		return nil, fmt.Errorf("claim asset workbench preview files: %w", err)
	}
	items := []*domain.AssetWorkbenchSubmissionFile{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSubmissionFile(rows)
		if err != nil {
			_ = rows.Close()
			return nil, err
		}
		items = append(items, item)
	}
	if err := rows.Close(); err != nil {
		return nil, err
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	leaseUntil := claim.Now.Add(claim.LeaseTTL)
	for _, item := range items {
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE asset_workbench_submission_files
			SET preview_status = ?, preview_worker_id = ?, preview_lease_expires_at = ?, updated_at = CURRENT_TIMESTAMP
			WHERE id = ?`,
			domain.AssetWorkbenchPreviewStatusProcessing,
			workerID,
			leaseUntil,
			item.ID,
		); err != nil {
			return nil, fmt.Errorf("mark asset workbench preview claimed: %w", err)
		}
		item.PreviewStatus = domain.AssetWorkbenchPreviewStatusProcessing
		item.PreviewWorkerID = workerID
		item.PreviewLeaseExpiresAt = &leaseUntil
	}
	if err := sqlTx.Commit(); err != nil {
		return nil, fmt.Errorf("commit asset workbench preview claim tx: %w", err)
	}
	return items, nil
}

func (r *assetWorkbenchRepo) MarkPreviewReady(ctx context.Context, tx repo.Tx, fileID int64, previewKey string) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_files
		SET preview_status = ?, preview_key = ?, preview_error = '', preview_worker_id = '',
		    preview_lease_expires_at = NULL, preview_next_retry_at = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, domain.AssetWorkbenchPreviewStatusReady, previewKey, fileID)
	if err != nil {
		return fmt.Errorf("mark asset workbench preview ready: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) MarkPreviewFailed(ctx context.Context, tx repo.Tx, fileID int64, attempts int, message string, nextRetryAt *time.Time) error {
	status := domain.AssetWorkbenchPreviewStatusPending
	if nextRetryAt == nil {
		status = domain.AssetWorkbenchPreviewStatusFailed
	}
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_files
		SET preview_status = ?, preview_attempts = ?, preview_error = ?, preview_worker_id = '',
		    preview_lease_expires_at = NULL, preview_next_retry_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`, status, attempts, message, toNullTime(nextRetryAt), fileID)
	if err != nil {
		return fmt.Errorf("mark asset workbench preview failed: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) CreateErrorImportBatch(ctx context.Context, tx repo.Tx, batch *domain.AssetWorkbenchErrorImportBatch) (*domain.AssetWorkbenchErrorImportBatch, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_error_import_batches (
			import_no, business_month, uploaded_by, original_filename, status, total_rows,
			matched_rows, unmatched_rows, ambiguous_rows, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		batch.ImportNo,
		batch.BusinessMonth,
		batch.UploadedBy,
		batch.OriginalFilename,
		batch.Status,
		batch.TotalRows,
		batch.MatchedRows,
		batch.UnmatchedRows,
		batch.AmbiguousRows,
		batch.ErrorMessage,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench error import batch: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench error import batch last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchErrorImportBatchSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchErrorImportBatch(row)
}

func (r *assetWorkbenchRepo) CreateErrorRecord(ctx context.Context, tx repo.Tx, record *domain.AssetWorkbenchErrorRecord) (*domain.AssetWorkbenchErrorRecord, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_error_records (
			import_batch_id, business_month, payee_user_id, order_no, error_count,
			raw_payload_json, match_status, submission_item_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ImportBatchID,
		record.BusinessMonth,
		toNullInt64(record.PayeeUserID),
		record.OrderNo,
		record.ErrorCount,
		nullableJSON(record.RawPayload),
		record.MatchStatus,
		toNullInt64(record.SubmissionItemID),
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench error record: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench error record last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchErrorRecordSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchErrorRecord(row)
}

func (r *assetWorkbenchRepo) ListErrorRecordsByMonth(ctx context.Context, businessMonth string) ([]*domain.AssetWorkbenchErrorRecord, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchErrorRecordSelect()+`
		WHERE business_month = ?
		ORDER BY id ASC`, businessMonth)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench error records: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchErrorRecord{}
	for rows.Next() {
		item, err := scanAssetWorkbenchErrorRecord(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) FindActiveDeductionRule(ctx context.Context, workerType, jobGrade, difficultyClass string, asOf time.Time) (*domain.AssetWorkbenchDeductionRule, error) {
	date := asOf.Format("2006-01-02")
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchDeductionRuleSelect()+`
		WHERE worker_type IN (?, ?)
		  AND job_grade IN (?, ?)
		  AND difficulty_class = ?
		  AND enabled = 1
		  AND effective_from <= ?
		  AND (effective_to IS NULL OR effective_to >= ?)
		ORDER BY (worker_type = ?) DESC, (job_grade = ?) DESC, effective_from DESC, id DESC
		LIMIT 1`,
		workerType, domain.AssetWorkbenchWorkerTypeAll,
		jobGrade, domain.AssetWorkbenchWorkerTypeAll,
		difficultyClass,
		date,
		date,
		workerType,
		jobGrade,
	)
	return scanAssetWorkbenchDeductionRule(row)
}

func (r *assetWorkbenchRepo) LockSettleableItems(ctx context.Context, tx repo.Tx, businessMonth string) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, assetWorkbenchSubmissionItemSelect()+`
		WHERE business_month = ?
		  AND pricing_status = ?
		  AND qc_status IN (?, ?)
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		ORDER BY payee_user_id ASC, id ASC
		FOR UPDATE`,
		businessMonth,
		domain.AssetWorkbenchPricingStatusPriced,
		domain.AssetWorkbenchSubmissionStatusSubmitted,
		domain.AssetWorkbenchSubmissionStatusChecked,
		domain.AssetWorkbenchSettlementStatusUnsettled,
	)
	if err != nil {
		return nil, fmt.Errorf("lock asset workbench settleable items: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSubmissionItem{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSubmissionItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) LockSettleableSupplements(ctx context.Context, tx repo.Tx, businessMonth string) ([]*domain.AssetWorkbenchSettlementSupplement, error) {
	rows, err := Unwrap(tx).QueryContext(ctx, assetWorkbenchSettlementSupplementSelect()+`
		WHERE business_month = ?
		  AND status = ?
		  AND linked_batch_id IS NULL
		ORDER BY payee_user_id ASC, id ASC
		FOR UPDATE`, businessMonth, domain.AssetWorkbenchSupplementStatusApproved)
	if err != nil {
		return nil, fmt.Errorf("lock asset workbench settleable supplements: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSettlementSupplement{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSettlementSupplement(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) CreateSettlementBatch(ctx context.Context, tx repo.Tx, batch *domain.AssetWorkbenchSettlementBatch) (*domain.AssetWorkbenchSettlementBatch, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_settlement_batches (
			batch_no, business_month, status, generated_by, item_count, gross_amount,
			deduction_amount, welfare_amount, supplement_amount, adjustment_amount, net_amount
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		batch.BatchNo,
		batch.BusinessMonth,
		batch.Status,
		batch.GeneratedBy,
		batch.ItemCount,
		batch.GrossAmount,
		batch.DeductionAmount,
		batch.WelfareAmount,
		batch.SupplementAmount,
		batch.AdjustmentAmount,
		batch.NetAmount,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench settlement batch: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench settlement batch last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSettlementBatchSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchSettlementBatch(row)
}

func (r *assetWorkbenchRepo) CreateSettlementItem(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSettlementItem) (*domain.AssetWorkbenchSettlementItem, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_settlement_items (
			batch_id, item_type, submission_item_id, payee_user_id, business_month, amount,
			quantity, unit_price, direction, source_ref_type, source_ref_id, snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.BatchID,
		item.ItemType,
		toNullInt64(item.SubmissionItemID),
		item.PayeeUserID,
		item.BusinessMonth,
		item.Amount,
		item.Quantity,
		toNullFloat64(item.UnitPrice),
		item.Direction,
		item.SourceRefType,
		toNullInt64(item.SourceRefID),
		nullableJSON(item.Snapshot),
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench settlement item: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench settlement item last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSettlementItemSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchSettlementItem(row)
}

func (r *assetWorkbenchRepo) AttachItemsToSettlementBatch(ctx context.Context, tx repo.Tx, batchID int64, itemIDs []int64) error {
	if len(itemIDs) == 0 {
		return nil
	}
	query, args := inClause(`UPDATE asset_workbench_submission_items
		SET settlement_status = ?, current_settlement_batch_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE settlement_status = ? AND current_settlement_batch_id IS NULL AND id IN (`, `)`,
		append([]interface{}{domain.AssetWorkbenchSettlementStatusInBatch, batchID, domain.AssetWorkbenchSettlementStatusUnsettled}, int64SliceToInterfaces(itemIDs)...)...)
	res, err := Unwrap(tx).ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("attach asset workbench items to settlement batch: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != int64(len(itemIDs)) {
		return domain.NewAppError(domain.ErrCodeConflict, "Some items were already settled or attached to another batch.", map[string]int{"expected": len(itemIDs), "affected": int(affected)})
	}
	return nil
}

func (r *assetWorkbenchRepo) AttachSupplementsToSettlementBatch(ctx context.Context, tx repo.Tx, batchID int64, supplementIDs []int64) error {
	if len(supplementIDs) == 0 {
		return nil
	}
	query, args := inClause(`UPDATE asset_workbench_settlement_supplements
		SET status = ?, linked_batch_id = ?, updated_at = CURRENT_TIMESTAMP
		WHERE status = ? AND linked_batch_id IS NULL AND id IN (`, `)`,
		append([]interface{}{domain.AssetWorkbenchSupplementStatusInBatch, batchID, domain.AssetWorkbenchSupplementStatusApproved}, int64SliceToInterfaces(supplementIDs)...)...)
	res, err := Unwrap(tx).ExecContext(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("attach asset workbench supplements to settlement batch: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != int64(len(supplementIDs)) {
		return domain.NewAppError(domain.ErrCodeConflict, "Some supplements were already settled or attached to another batch.", map[string]int{"expected": len(supplementIDs), "affected": int(affected)})
	}
	return nil
}

func (r *assetWorkbenchRepo) ConfirmSettlementBatch(ctx context.Context, tx repo.Tx, batchID int64, actorID int64, at time.Time) error {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_settlement_batches
		SET status = ?, confirmed_by = ?, confirmed_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = ?`,
		domain.AssetWorkbenchBatchStatusConfirmed, actorID, at, batchID, domain.AssetWorkbenchBatchStatusGenerated)
	if err != nil {
		return fmt.Errorf("confirm asset workbench settlement batch: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return domain.NewAppError(domain.ErrCodeConflict, "Settlement batch is not confirmable.", nil)
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_items
		SET settlement_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE current_settlement_batch_id = ? AND settlement_status = ?`,
		domain.AssetWorkbenchSettlementStatusSettled, batchID, domain.AssetWorkbenchSettlementStatusInBatch); err != nil {
		return fmt.Errorf("mark asset workbench items settled: %w", err)
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_settlement_supplements
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE linked_batch_id = ? AND status = ?`,
		domain.AssetWorkbenchSupplementStatusSettled, batchID, domain.AssetWorkbenchSupplementStatusInBatch); err != nil {
		return fmt.Errorf("mark asset workbench supplements settled: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) CancelGeneratedSettlementBatch(ctx context.Context, tx repo.Tx, batchID int64, actorID int64, reason string, at time.Time) error {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_settlement_batches
		SET status = ?, cancelled_by = ?, cancelled_at = ?, cancel_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = ?`,
		domain.AssetWorkbenchBatchStatusCancelled, actorID, at, reason, batchID, domain.AssetWorkbenchBatchStatusGenerated)
	if err != nil {
		return fmt.Errorf("cancel asset workbench settlement batch: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return domain.NewAppError(domain.ErrCodeConflict, "Settlement batch is not cancellable.", nil)
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_items
		SET settlement_status = ?, current_settlement_batch_id = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE current_settlement_batch_id = ? AND settlement_status = ?`,
		domain.AssetWorkbenchSettlementStatusUnsettled, batchID, domain.AssetWorkbenchSettlementStatusInBatch); err != nil {
		return fmt.Errorf("release asset workbench settlement items: %w", err)
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_settlement_supplements
		SET status = ?, linked_batch_id = NULL, updated_at = CURRENT_TIMESTAMP
		WHERE linked_batch_id = ? AND status = ?`,
		domain.AssetWorkbenchSupplementStatusApproved, batchID, domain.AssetWorkbenchSupplementStatusInBatch); err != nil {
		return fmt.Errorf("release asset workbench settlement supplements: %w", err)
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM asset_workbench_settlement_items WHERE batch_id = ?`, batchID); err != nil {
		return fmt.Errorf("delete cancelled asset workbench settlement items: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) LockSettlementBatch(ctx context.Context, tx repo.Tx, batchID int64) (*domain.AssetWorkbenchSettlementBatch, error) {
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSettlementBatchSelect()+` WHERE id = ? FOR UPDATE`, batchID)
	return scanAssetWorkbenchSettlementBatch(row)
}

func (r *assetWorkbenchRepo) GetSettlementBatch(ctx context.Context, batchID int64) (*domain.AssetWorkbenchSettlementBatch, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchSettlementBatchSelect()+` WHERE id = ?`, batchID)
	return scanAssetWorkbenchSettlementBatch(row)
}

func (r *assetWorkbenchRepo) ListSettlementBatches(ctx context.Context, filter repo.AssetWorkbenchSettlementBatchFilter) ([]*domain.AssetWorkbenchSettlementBatch, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.BusinessMonth); v != "" {
		where = append(where, "business_month = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Status); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_settlement_batches WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench settlement batches: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSettlementBatchSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY generated_at DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench settlement batches: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSettlementBatch{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSettlementBatch(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) ListSettlementItemsByBatch(ctx context.Context, batchID int64) ([]*domain.AssetWorkbenchSettlementItem, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSettlementItemSelect()+` WHERE batch_id = ? ORDER BY payee_user_id ASC, id ASC`, batchID)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench settlement items: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSettlementItem{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSettlementItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) HasConfirmedSettlementForPayeeMonth(ctx context.Context, payeeUserID int64, businessMonth string) (bool, error) {
	var exists int
	err := r.db.db.QueryRowContext(ctx, `
		SELECT 1
		FROM asset_workbench_settlement_items si
		JOIN asset_workbench_settlement_batches sb ON sb.id = si.batch_id
		WHERE si.payee_user_id = ?
		  AND si.business_month = ?
		  AND sb.status = ?
		LIMIT 1`,
		payeeUserID,
		businessMonth,
		domain.AssetWorkbenchBatchStatusConfirmed,
	).Scan(&exists)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return false, nil
		}
		return false, fmt.Errorf("check confirmed settlement payee month: %w", err)
	}
	return true, nil
}

func (r *assetWorkbenchRepo) ListConfirmedSettlementMonthsByPayee(ctx context.Context, payeeUserID int64) ([]string, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT DISTINCT si.business_month
		FROM asset_workbench_settlement_items si
		JOIN asset_workbench_settlement_batches sb ON sb.id = si.batch_id
		WHERE si.payee_user_id = ?
		  AND sb.status = ?
		ORDER BY si.business_month DESC`,
		payeeUserID,
		domain.AssetWorkbenchBatchStatusConfirmed,
	)
	if err != nil {
		return nil, fmt.Errorf("list confirmed settlement months by payee: %w", err)
	}
	defer rows.Close()
	months := []string{}
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err != nil {
			return nil, err
		}
		months = append(months, month)
	}
	return months, rows.Err()
}

func (r *assetWorkbenchRepo) CreateSettlementAdjustment(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSettlementAdjustment) (*domain.AssetWorkbenchSettlementAdjustment, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_settlement_adjustments (
			batch_id, payee_user_id, business_month, adjustment_type, amount, reason, status, payload_json, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		toNullInt64(item.BatchID),
		item.PayeeUserID,
		item.BusinessMonth,
		item.AdjustmentType,
		item.Amount,
		item.Reason,
		item.Status,
		nullableJSON(item.Payload),
		item.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench settlement adjustment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench settlement adjustment last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSettlementAdjustmentSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchSettlementAdjustment(row)
}

func (r *assetWorkbenchRepo) ApplySettlementBatchAdjustment(ctx context.Context, tx repo.Tx, batchID int64, signedAmount float64) error {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_settlement_batches
		SET adjustment_amount = adjustment_amount + ?,
		    net_amount = net_amount + ?,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		signedAmount,
		signedAmount,
		batchID,
	)
	if err != nil {
		return fmt.Errorf("apply asset workbench settlement batch adjustment: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("asset workbench settlement batch adjustment rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *assetWorkbenchRepo) ListSettlementSupplements(ctx context.Context, filter repo.AssetWorkbenchSettlementSupplementFilter) ([]*domain.AssetWorkbenchSettlementSupplement, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.PayeeUserID != nil {
		where = append(where, "payee_user_id = ?")
		args = append(args, *filter.PayeeUserID)
	}
	if v := strings.TrimSpace(filter.BusinessMonth); v != "" {
		where = append(where, "business_month = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.OrderNo); v != "" {
		where = append(where, "order_no = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Status); v != "" {
		where = append(where, "status = ?")
		args = append(args, v)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_settlement_supplements WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench settlement supplements: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSettlementSupplementSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY business_month DESC, payee_user_id ASC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench settlement supplements: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSettlementSupplement{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSettlementSupplement(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) CreateSettlementSupplement(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSettlementSupplement) (*domain.AssetWorkbenchSettlementSupplement, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_settlement_supplements (
			payee_user_id, business_month, linked_batch_id, status, order_no, difficulty_class,
			finalized, page_count, gross_amount, duplicate_hint_json, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.PayeeUserID,
		item.BusinessMonth,
		toNullInt64(item.LinkedBatchID),
		item.Status,
		item.OrderNo,
		item.DifficultyClass,
		item.Finalized,
		item.PageCount,
		item.GrossAmount,
		nullableJSON(item.DuplicateHint),
		item.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench settlement supplement: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench settlement supplement last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSettlementSupplementSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchSettlementSupplement(row)
}

func (r *assetWorkbenchRepo) GetSupplementPermission(ctx context.Context, payeeUserID int64, businessMonth string) (*domain.AssetWorkbenchSupplementPermission, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchSupplementPermissionSelect()+` WHERE payee_user_id = ? AND business_month = ?`, payeeUserID, businessMonth)
	return scanAssetWorkbenchSupplementPermission(row)
}

func (r *assetWorkbenchRepo) ListSupplementPermissions(ctx context.Context, filter repo.AssetWorkbenchSupplementPermissionFilter) ([]*domain.AssetWorkbenchSupplementPermission, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.PayeeUserID != nil {
		where = append(where, "payee_user_id = ?")
		args = append(args, *filter.PayeeUserID)
	}
	if v := strings.TrimSpace(filter.BusinessMonth); v != "" {
		where = append(where, "business_month = ?")
		args = append(args, v)
	}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_supplement_permissions WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench supplement permissions: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSupplementPermissionSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY business_month DESC, payee_user_id ASC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench supplement permissions: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSupplementPermission{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSupplementPermission(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) UpsertSupplementPermission(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSupplementPermission) (*domain.AssetWorkbenchSupplementPermission, error) {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_supplement_permissions (
			payee_user_id, business_month, enabled, reason, granted_by, revoked_by, revoked_at
		) VALUES (?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			enabled = VALUES(enabled),
			reason = VALUES(reason),
			granted_by = VALUES(granted_by),
			revoked_by = VALUES(revoked_by),
			granted_at = IF(VALUES(enabled) = 1, CURRENT_TIMESTAMP, granted_at),
			revoked_at = VALUES(revoked_at),
			updated_at = CURRENT_TIMESTAMP`,
		item.PayeeUserID,
		item.BusinessMonth,
		item.Enabled,
		item.Reason,
		item.GrantedBy,
		toNullInt64(item.RevokedBy),
		toNullTime(item.RevokedAt),
	)
	if err != nil {
		return nil, fmt.Errorf("upsert asset workbench supplement permission: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSupplementPermissionSelect()+` WHERE payee_user_id = ? AND business_month = ?`, item.PayeeUserID, item.BusinessMonth)
	return scanAssetWorkbenchSupplementPermission(row)
}

func (r *assetWorkbenchRepo) AppendEvent(ctx context.Context, tx repo.Tx, event *domain.AssetWorkbenchEvent) (*domain.AssetWorkbenchEvent, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_events (
			actor_user_id, event_type, entity_type, entity_id, before_json, after_json, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		toNullInt64(event.ActorUserID),
		event.EventType,
		event.EntityType,
		toNullInt64(event.EntityID),
		nullableJSON(event.Before),
		nullableJSON(event.After),
		event.Reason,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench event last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchEventSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchEvent(row)
}

func (r *assetWorkbenchRepo) ListEvents(ctx context.Context, filter repo.AssetWorkbenchEventFilter) ([]*domain.AssetWorkbenchEvent, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.EventType); v != "" {
		where = append(where, "event_type = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.EntityType); v != "" {
		where = append(where, "entity_type = ?")
		args = append(args, v)
	}
	if filter.EntityID != nil {
		where = append(where, "entity_id = ?")
		args = append(args, *filter.EntityID)
	}
	if filter.ActorID != nil {
		where = append(where, "actor_user_id = ?")
		args = append(args, *filter.ActorID)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_events WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench events: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchEventSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench events: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchEvent{}
	for rows.Next() {
		item, err := scanAssetWorkbenchEvent(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) ListSavedViews(ctx context.Context, filter repo.AssetWorkbenchSavedViewFilter) ([]*domain.AssetWorkbenchSavedView, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSavedViewSelect()+`
		WHERE user_id = ? AND view_type = ?
		ORDER BY is_default DESC, updated_at DESC, id DESC`,
		filter.UserID,
		strings.TrimSpace(filter.ViewType),
	)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench saved views: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSavedView{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSavedView(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) UpsertSavedView(ctx context.Context, tx repo.Tx, view *domain.AssetWorkbenchSavedView) (*domain.AssetWorkbenchSavedView, error) {
	sqlTx := Unwrap(tx)
	if view.IsDefault {
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE asset_workbench_saved_views
			SET is_default = 0
			WHERE user_id = ? AND view_type = ?`,
			view.UserID,
			view.ViewType,
		); err != nil {
			return nil, fmt.Errorf("clear asset workbench default saved views: %w", err)
		}
	}
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO asset_workbench_saved_views (
			user_id, view_type, view_name, config_json, is_default
		) VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			config_json = VALUES(config_json),
			is_default = VALUES(is_default),
			updated_at = CURRENT_TIMESTAMP`,
		view.UserID,
		view.ViewType,
		view.ViewName,
		nullableJSON(view.Config),
		view.IsDefault,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert asset workbench saved view: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench saved view last insert id: %w", err)
	}
	if id > 0 {
		row := sqlTx.QueryRowContext(ctx, assetWorkbenchSavedViewSelect()+` WHERE id = ?`, id)
		return scanAssetWorkbenchSavedView(row)
	}
	row := sqlTx.QueryRowContext(ctx, assetWorkbenchSavedViewSelect()+` WHERE user_id = ? AND view_type = ? AND view_name = ?`, view.UserID, view.ViewType, view.ViewName)
	return scanAssetWorkbenchSavedView(row)
}

func (r *assetWorkbenchRepo) DeleteSavedView(ctx context.Context, tx repo.Tx, userID, viewID int64) error {
	res, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM asset_workbench_saved_views WHERE id = ? AND user_id = ?`, viewID, userID)
	if err != nil {
		return fmt.Errorf("delete asset workbench saved view: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("asset workbench saved view delete rows affected: %w", err)
	}
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

func assetWorkbenchProfileSelect() string {
	return `SELECT id, user_id, worker_type, job_grade, real_name, phone, province, city, id_card, gender,
		alipay_account, onboarded_at, grade_hidden, status, pii_completed, created_by, updated_by, created_at, updated_at
		FROM asset_workbench_profiles`
}

func assetWorkbenchGradePeriodSelect() string {
	return `SELECT id, profile_id, user_id, worker_type, job_grade, effective_from, effective_to, changed_by, reason, created_at
		FROM asset_workbench_profile_grade_periods`
}

func assetWorkbenchPriceMatrixSelect() string {
	return `SELECT id, worker_type, job_grade, difficulty_class, unit_price, effective_from, effective_to, enabled,
		revision_no, created_by, remark, created_at, updated_at
		FROM asset_workbench_price_matrix`
}

func assetWorkbenchUploadSessionSelect() string {
	return `SELECT id, session_id, owner_user_id, status, object_key, original_filename, file_size, mime_type,
		file_hash, upload_id, multipart_plan_json, expires_at, uploaded_at, cancelled_at, submitted_item_id, created_at, updated_at
		FROM asset_workbench_upload_sessions`
}

func assetWorkbenchSubmissionSelect() string {
	return `SELECT id, submission_no, submitter_user_id, business_month, submitted_at, status, item_count,
		file_count, page_count, gross_total, notes, created_at, updated_at
		FROM asset_workbench_submissions`
}

func assetWorkbenchSubmissionItemSelect() string {
	return `SELECT id, submission_id, payee_user_id, order_no, difficulty_class, finalized, page_count, item_count,
		business_month, submitted_at, worker_type_snapshot, job_grade_snapshot, base_price_rule_id, base_unit_price,
		promo_coupon_id, promo_snapshot_json, pricing_snapshot_json, gross_amount, pricing_status, qc_status,
		settlement_status, current_settlement_batch_id, voided_at, voided_by, void_reason, created_at, updated_at
		FROM asset_workbench_submission_items`
}

func assetWorkbenchSubmissionFileSelect() string {
	return `SELECT id, submission_id, submission_item_id, upload_session_id, owner_user_id, object_key, preview_key,
		preview_status, preview_attempts, preview_error, preview_next_retry_at, preview_worker_id, preview_lease_expires_at,
		original_filename, file_ext, file_type, mime_type, file_size, file_hash, sort_order, created_at, updated_at
		FROM asset_workbench_submission_files`
}

func assetWorkbenchDeductionRuleSelect() string {
	return `SELECT id, worker_type, job_grade, difficulty_class, deduction_amount, effective_from, effective_to,
		enabled, revision_no, created_by, remark, created_at, updated_at
		FROM asset_workbench_deduction_rules`
}

func assetWorkbenchWelfareRuleSelect() string {
	return `SELECT id, rule_name, worker_type, job_grade, rule_type, amount, config_json, effective_from, effective_to,
		enabled, created_by, remark, created_at, updated_at
		FROM asset_workbench_welfare_rules`
}

func assetWorkbenchPromoCouponSelect() string {
	return `SELECT id, coupon_code, coupon_name, mode, amount, percent, priority, worker_type, job_grade,
		difficulty_class, eligible_user_ids_json, eligible_codes_json, effective_from, effective_to,
		enabled, stack_policy, created_by, remark, created_at, updated_at
		FROM asset_workbench_promo_coupons`
}

func assetWorkbenchErrorImportBatchSelect() string {
	return `SELECT id, import_no, business_month, uploaded_by, original_filename, status, total_rows,
		matched_rows, unmatched_rows, ambiguous_rows, error_message, created_at, updated_at
		FROM asset_workbench_error_import_batches`
}

func assetWorkbenchErrorRecordSelect() string {
	return `SELECT id, import_batch_id, business_month, payee_user_id, order_no, error_count,
		raw_payload_json, match_status, submission_item_id, created_at, updated_at
		FROM asset_workbench_error_records`
}

func assetWorkbenchSettlementBatchSelect() string {
	return `SELECT id, batch_no, business_month, status, generated_by, confirmed_by, cancelled_by,
		generated_at, confirmed_at, cancelled_at, cancel_reason, item_count, gross_amount,
		deduction_amount, welfare_amount, supplement_amount, adjustment_amount, net_amount, created_at, updated_at
		FROM asset_workbench_settlement_batches`
}

func assetWorkbenchSettlementItemSelect() string {
	return `SELECT id, batch_id, item_type, submission_item_id, payee_user_id, business_month,
		amount, quantity, unit_price, direction, source_ref_type, source_ref_id, snapshot_json, created_at
		FROM asset_workbench_settlement_items`
}

func assetWorkbenchSettlementAdjustmentSelect() string {
	return `SELECT id, batch_id, payee_user_id, business_month, adjustment_type, amount,
		reason, status, payload_json, created_by, created_at, updated_at
		FROM asset_workbench_settlement_adjustments`
}

func assetWorkbenchSettlementSupplementSelect() string {
	return `SELECT id, payee_user_id, business_month, linked_batch_id, status, order_no, difficulty_class,
		finalized, page_count, gross_amount, duplicate_hint_json, created_by, created_at, updated_at
		FROM asset_workbench_settlement_supplements`
}

func assetWorkbenchSupplementPermissionSelect() string {
	return `SELECT id, payee_user_id, business_month, enabled, reason, granted_by, revoked_by, granted_at, revoked_at, updated_at
		FROM asset_workbench_supplement_permissions`
}

func assetWorkbenchEventSelect() string {
	return `SELECT id, actor_user_id, event_type, entity_type, entity_id, before_json, after_json, reason, created_at
		FROM asset_workbench_events`
}

func assetWorkbenchSavedViewSelect() string {
	return `SELECT id, user_id, view_type, view_name, config_json, is_default, created_at, updated_at
		FROM asset_workbench_saved_views`
}

func scanAssetWorkbenchProfile(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchProfile, error) {
	var item domain.AssetWorkbenchProfile
	var phone, idCard sql.NullString
	var onboardedAt sql.NullTime
	var createdBy, updatedBy sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.UserID, &item.WorkerType, &item.JobGrade, &item.RealName, &phone,
		&item.Province, &item.City, &idCard, &item.Gender, &item.AlipayAccount, &onboardedAt,
		&item.GradeHidden, &item.Status, &item.PIICompleted, &createdBy, &updatedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Phone = fromNullString(phone)
	item.IDCard = fromNullString(idCard)
	item.OnboardedAt = fromNullTime(onboardedAt)
	item.CreatedBy = fromNullInt64(createdBy)
	item.UpdatedBy = fromNullInt64(updatedBy)
	return &item, nil
}

func scanAssetWorkbenchGradePeriod(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchGradePeriod, error) {
	var item domain.AssetWorkbenchGradePeriod
	var effectiveTo sql.NullTime
	var changedBy sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.ProfileID, &item.UserID, &item.WorkerType, &item.JobGrade,
		&item.EffectiveFrom, &effectiveTo, &changedBy, &item.Reason, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.EffectiveTo = fromNullTime(effectiveTo)
	item.ChangedBy = fromNullInt64(changedBy)
	return &item, nil
}

func scanAssetWorkbenchPriceMatrix(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchPriceMatrix, error) {
	var item domain.AssetWorkbenchPriceMatrix
	var effectiveTo sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.WorkerType, &item.JobGrade, &item.DifficultyClass, &item.UnitPrice,
		&item.EffectiveFrom, &effectiveTo, &item.Enabled, &item.RevisionNo, &item.CreatedBy,
		&item.Remark, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.EffectiveTo = fromNullTime(effectiveTo)
	return &item, nil
}

func scanAssetWorkbenchUploadSession(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchUploadSession, error) {
	var item domain.AssetWorkbenchUploadSession
	var rawPlan sql.NullString
	var uploadedAt, cancelledAt sql.NullTime
	var submittedItemID sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.SessionID, &item.OwnerUserID, &item.Status, &item.ObjectKey, &item.OriginalFilename,
		&item.FileSize, &item.MimeType, &item.FileHash, &item.UploadID, &rawPlan, &item.ExpiresAt,
		&uploadedAt, &cancelledAt, &submittedItemID, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.MultipartPlan = cloneValidJSON(rawPlan)
	item.UploadedAt = fromNullTime(uploadedAt)
	item.CancelledAt = fromNullTime(cancelledAt)
	item.SubmittedItemID = fromNullInt64(submittedItemID)
	return &item, nil
}

func scanAssetWorkbenchSubmission(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSubmission, error) {
	var item domain.AssetWorkbenchSubmission
	if err := scanner.Scan(
		&item.ID, &item.SubmissionNo, &item.SubmitterUserID, &item.BusinessMonth, &item.SubmittedAt,
		&item.Status, &item.ItemCount, &item.FileCount, &item.PageCount, &item.GrossTotal,
		&item.Notes, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchSubmissionItem(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSubmissionItem, error) {
	var item domain.AssetWorkbenchSubmissionItem
	var basePriceRuleID, promoCouponID, currentBatchID, voidedBy sql.NullInt64
	var baseUnitPrice sql.NullFloat64
	var promoSnapshot, pricingSnapshot sql.NullString
	var voidedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.SubmissionID, &item.PayeeUserID, &item.OrderNo, &item.DifficultyClass,
		&item.Finalized, &item.PageCount, &item.ItemCount, &item.BusinessMonth, &item.SubmittedAt,
		&item.WorkerTypeSnapshot, &item.JobGradeSnapshot, &basePriceRuleID, &baseUnitPrice,
		&promoCouponID, &promoSnapshot, &pricingSnapshot, &item.GrossAmount, &item.PricingStatus,
		&item.QCStatus, &item.SettlementStatus, &currentBatchID, &voidedAt, &voidedBy,
		&item.VoidReason, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.BasePriceRuleID = fromNullInt64(basePriceRuleID)
	item.BaseUnitPrice = fromNullFloat64(baseUnitPrice)
	item.PromoCouponID = fromNullInt64(promoCouponID)
	item.PromoSnapshot = cloneValidJSON(promoSnapshot)
	item.PricingSnapshot = cloneValidJSON(pricingSnapshot)
	item.CurrentSettlementBatchID = fromNullInt64(currentBatchID)
	item.VoidedAt = fromNullTime(voidedAt)
	item.VoidedBy = fromNullInt64(voidedBy)
	return &item, nil
}

func scanAssetWorkbenchSubmissionFile(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSubmissionFile, error) {
	var item domain.AssetWorkbenchSubmissionFile
	var uploadSessionID sql.NullInt64
	var previewNextRetryAt, previewLeaseExpiresAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.SubmissionID, &item.SubmissionItemID, &uploadSessionID, &item.OwnerUserID,
		&item.ObjectKey, &item.PreviewKey, &item.PreviewStatus, &item.PreviewAttempts,
		&item.PreviewError, &previewNextRetryAt, &item.PreviewWorkerID, &previewLeaseExpiresAt,
		&item.OriginalFilename, &item.FileExt, &item.FileType, &item.MimeType, &item.FileSize,
		&item.FileHash, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.UploadSessionID = fromNullInt64(uploadSessionID)
	item.PreviewNextRetryAt = fromNullTime(previewNextRetryAt)
	item.PreviewLeaseExpiresAt = fromNullTime(previewLeaseExpiresAt)
	return &item, nil
}

func scanAssetWorkbenchDeductionRule(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchDeductionRule, error) {
	var item domain.AssetWorkbenchDeductionRule
	var effectiveTo sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.WorkerType, &item.JobGrade, &item.DifficultyClass, &item.DeductionAmount,
		&item.EffectiveFrom, &effectiveTo, &item.Enabled, &item.RevisionNo, &item.CreatedBy,
		&item.Remark, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.EffectiveTo = fromNullTime(effectiveTo)
	return &item, nil
}

func scanAssetWorkbenchWelfareRule(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchWelfareRule, error) {
	var item domain.AssetWorkbenchWelfareRule
	var config sql.NullString
	var effectiveTo sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.RuleName, &item.WorkerType, &item.JobGrade, &item.RuleType, &item.Amount,
		&config, &item.EffectiveFrom, &effectiveTo, &item.Enabled, &item.CreatedBy,
		&item.Remark, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Config = cloneValidJSON(config)
	item.EffectiveTo = fromNullTime(effectiveTo)
	return &item, nil
}

func scanAssetWorkbenchPromoCoupon(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchPromoCoupon, error) {
	var item domain.AssetWorkbenchPromoCoupon
	var amount, percent sql.NullFloat64
	var usersJSON, codesJSON sql.NullString
	var effectiveTo sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.CouponCode, &item.CouponName, &item.Mode, &amount, &percent, &item.Priority,
		&item.WorkerType, &item.JobGrade, &item.DifficultyClass, &usersJSON, &codesJSON,
		&item.EffectiveFrom, &effectiveTo, &item.Enabled, &item.StackPolicy, &item.CreatedBy,
		&item.Remark, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Amount = fromNullFloat64(amount)
	item.Percent = fromNullFloat64(percent)
	item.EligibleUserIDs = cloneValidJSON(usersJSON)
	item.EligibleCodes = cloneValidJSON(codesJSON)
	item.EffectiveTo = fromNullTime(effectiveTo)
	return &item, nil
}

func scanAssetWorkbenchErrorImportBatch(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchErrorImportBatch, error) {
	var item domain.AssetWorkbenchErrorImportBatch
	if err := scanner.Scan(
		&item.ID, &item.ImportNo, &item.BusinessMonth, &item.UploadedBy, &item.OriginalFilename,
		&item.Status, &item.TotalRows, &item.MatchedRows, &item.UnmatchedRows, &item.AmbiguousRows,
		&item.ErrorMessage, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchErrorRecord(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchErrorRecord, error) {
	var item domain.AssetWorkbenchErrorRecord
	var payeeUserID, submissionItemID sql.NullInt64
	var rawPayload sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.ImportBatchID, &item.BusinessMonth, &payeeUserID, &item.OrderNo,
		&item.ErrorCount, &rawPayload, &item.MatchStatus, &submissionItemID, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.PayeeUserID = fromNullInt64(payeeUserID)
	item.RawPayload = cloneValidJSON(rawPayload)
	item.SubmissionItemID = fromNullInt64(submissionItemID)
	return &item, nil
}

func scanAssetWorkbenchSettlementBatch(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSettlementBatch, error) {
	var item domain.AssetWorkbenchSettlementBatch
	var confirmedBy, cancelledBy sql.NullInt64
	var confirmedAt, cancelledAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.BatchNo, &item.BusinessMonth, &item.Status, &item.GeneratedBy,
		&confirmedBy, &cancelledBy, &item.GeneratedAt, &confirmedAt, &cancelledAt,
		&item.CancelReason, &item.ItemCount, &item.GrossAmount, &item.DeductionAmount,
		&item.WelfareAmount, &item.SupplementAmount, &item.AdjustmentAmount, &item.NetAmount,
		&item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.ConfirmedBy = fromNullInt64(confirmedBy)
	item.CancelledBy = fromNullInt64(cancelledBy)
	item.ConfirmedAt = fromNullTime(confirmedAt)
	item.CancelledAt = fromNullTime(cancelledAt)
	return &item, nil
}

func scanAssetWorkbenchSettlementItem(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSettlementItem, error) {
	var item domain.AssetWorkbenchSettlementItem
	var submissionItemID, sourceRefID sql.NullInt64
	var unitPrice sql.NullFloat64
	var snapshot sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.BatchID, &item.ItemType, &submissionItemID, &item.PayeeUserID,
		&item.BusinessMonth, &item.Amount, &item.Quantity, &unitPrice, &item.Direction,
		&item.SourceRefType, &sourceRefID, &snapshot, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.SubmissionItemID = fromNullInt64(submissionItemID)
	item.UnitPrice = fromNullFloat64(unitPrice)
	item.SourceRefID = fromNullInt64(sourceRefID)
	item.Snapshot = cloneValidJSON(snapshot)
	return &item, nil
}

func scanAssetWorkbenchSettlementAdjustment(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSettlementAdjustment, error) {
	var item domain.AssetWorkbenchSettlementAdjustment
	var batchID sql.NullInt64
	var payload sql.NullString
	if err := scanner.Scan(
		&item.ID, &batchID, &item.PayeeUserID, &item.BusinessMonth, &item.AdjustmentType,
		&item.Amount, &item.Reason, &item.Status, &payload, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.BatchID = fromNullInt64(batchID)
	item.Payload = cloneValidJSON(payload)
	return &item, nil
}

func scanAssetWorkbenchSettlementSupplement(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSettlementSupplement, error) {
	var item domain.AssetWorkbenchSettlementSupplement
	var linkedBatchID sql.NullInt64
	var duplicateHint sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.PayeeUserID, &item.BusinessMonth, &linkedBatchID, &item.Status,
		&item.OrderNo, &item.DifficultyClass, &item.Finalized, &item.PageCount, &item.GrossAmount,
		&duplicateHint, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.LinkedBatchID = fromNullInt64(linkedBatchID)
	item.DuplicateHint = cloneValidJSON(duplicateHint)
	return &item, nil
}

func scanAssetWorkbenchSupplementPermission(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSupplementPermission, error) {
	var item domain.AssetWorkbenchSupplementPermission
	var revokedBy sql.NullInt64
	var revokedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.PayeeUserID, &item.BusinessMonth, &item.Enabled, &item.Reason,
		&item.GrantedBy, &revokedBy, &item.GrantedAt, &revokedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.RevokedBy = fromNullInt64(revokedBy)
	item.RevokedAt = fromNullTime(revokedAt)
	return &item, nil
}

func scanAssetWorkbenchEvent(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchEvent, error) {
	var item domain.AssetWorkbenchEvent
	var actorUserID, entityID sql.NullInt64
	var beforeJSON, afterJSON sql.NullString
	if err := scanner.Scan(
		&item.ID, &actorUserID, &item.EventType, &item.EntityType, &entityID,
		&beforeJSON, &afterJSON, &item.Reason, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.ActorUserID = fromNullInt64(actorUserID)
	item.EntityID = fromNullInt64(entityID)
	item.Before = cloneValidJSON(beforeJSON)
	item.After = cloneValidJSON(afterJSON)
	return &item, nil
}

func scanAssetWorkbenchSavedView(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSavedView, error) {
	var item domain.AssetWorkbenchSavedView
	var config sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.UserID, &item.ViewType, &item.ViewName, &config,
		&item.IsDefault, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Config = cloneValidJSON(config)
	if len(item.Config) == 0 {
		item.Config = json.RawMessage(`{}`)
	}
	return &item, nil
}

func nullableJSON(raw json.RawMessage) sql.NullString {
	if len(raw) == 0 || !json.Valid(raw) {
		return sql.NullString{}
	}
	return sql.NullString{String: string(raw), Valid: true}
}

func int64SliceToInterfaces(values []int64) []interface{} {
	out := make([]interface{}, 0, len(values))
	for _, value := range values {
		out = append(out, value)
	}
	return out
}

func inClause(prefix, suffix string, args ...interface{}) (string, []interface{}) {
	if len(args) <= 3 {
		return prefix + "NULL" + suffix, args
	}
	placeholders := make([]string, 0, len(args)-3)
	for range args[3:] {
		placeholders = append(placeholders, "?")
	}
	return prefix + strings.Join(placeholders, ",") + suffix, args
}

func cloneValidJSON(raw sql.NullString) json.RawMessage {
	if !raw.Valid || raw.String == "" || !json.Valid([]byte(raw.String)) {
		return nil
	}
	return cloneJSON([]byte(raw.String))
}
