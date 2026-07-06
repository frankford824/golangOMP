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
	if filter.UserID > 0 {
		where = append(where, "user_id = ?")
		args = append(args, filter.UserID)
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

func (r *assetWorkbenchRepo) ListDifficultyClasses(ctx context.Context, filter repo.AssetWorkbenchDifficultyClassFilter) ([]*domain.AssetWorkbenchDifficultyClass, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchDifficultyClassSelect()+` WHERE `+strings.Join(where, " AND ")+` ORDER BY sort_order ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench difficulty classes: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchDifficultyClass{}
	for rows.Next() {
		item, err := scanAssetWorkbenchDifficultyClass(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) GetDifficultyClass(ctx context.Context, code string) (*domain.AssetWorkbenchDifficultyClass, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchDifficultyClassSelect()+` WHERE code = ?`, strings.TrimSpace(code))
	return scanAssetWorkbenchDifficultyClass(row)
}

func (r *assetWorkbenchRepo) CreateDifficultyClass(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchDifficultyClass) (*domain.AssetWorkbenchDifficultyClass, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_difficulty_classes (
			code, name, description, enabled, sort_order, created_by, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		item.Code,
		item.Name,
		item.Description,
		item.Enabled,
		item.SortOrder,
		toNullInt64(item.CreatedBy),
		toNullInt64(item.UpdatedBy),
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench difficulty class: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench difficulty class last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchDifficultyClassSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchDifficultyClass(row)
}

func (r *assetWorkbenchRepo) UpdateDifficultyClass(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchDifficultyClass) (*domain.AssetWorkbenchDifficultyClass, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_difficulty_classes
		SET name = ?, description = ?, enabled = ?, sort_order = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE code = ?`,
		item.Name,
		item.Description,
		item.Enabled,
		item.SortOrder,
		toNullInt64(item.UpdatedBy),
		item.Code,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench difficulty class: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench difficulty class rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchDifficultyClassSelect()+` WHERE code = ?`, item.Code)
	return scanAssetWorkbenchDifficultyClass(row)
}

func (r *assetWorkbenchRepo) ListMembers(ctx context.Context, filter repo.AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, error) {
	return r.listMembers(ctx, filter, false)
}

func (r *assetWorkbenchRepo) SearchPeople(ctx context.Context, filter repo.AssetWorkbenchMemberFilter) ([]*domain.AssetWorkbenchMember, int64, error) {
	return r.listMembers(ctx, filter, true)
}

func (r *assetWorkbenchRepo) listMembers(ctx context.Context, filter repo.AssetWorkbenchMemberFilter, lookup bool) ([]*domain.AssetWorkbenchMember, int64, error) {
	allUsers := lookup && strings.TrimSpace(filter.Scope) == "all_users"
	where := []string{}
	args := []interface{}{}
	if !allUsers {
		where = append(where, "am.app_code = ?")
		args = append(args, domain.AssetWorkbenchAppCode)
	}
	if filter.UserID > 0 {
		where = append(where, "u.id = ?")
		args = append(args, filter.UserID)
	}
	if v := strings.TrimSpace(filter.Keyword); v != "" {
		like := "%" + v + "%"
		where = append(where, "(u.username LIKE ? OR u.display_name LIKE ? OR u.mobile LIKE ? OR p.real_name LIKE ? OR p.phone LIKE ?)")
		args = append(args, like, like, like, like, like)
	}
	if v := strings.TrimSpace(filter.Status); v != "" && !allUsers {
		where = append(where, "am.status = ?")
		args = append(args, v)
	} else if lookup && !allUsers {
		where = append(where, "am.status = ?")
		args = append(args, domain.AppMembershipStatusActive)
	}
	if v := strings.TrimSpace(filter.Identity); v != "" {
		switch v {
		case "admin":
			where = append(where, assetWorkbenchAdminExistsSQL())
		case "normal":
			where = append(where, "NOT "+assetWorkbenchAdminExistsSQL())
		}
	}
	if len(where) == 0 {
		where = append(where, "1=1")
	}
	var total int64
	countArgs := append([]interface{}{}, args...)
	if err := r.db.db.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM users u
		LEFT JOIN app_memberships am ON am.user_id = u.id AND am.app_code = 'asset_workbench'
		LEFT JOIN asset_workbench_profiles p ON p.user_id = u.id
		WHERE `+strings.Join(where, " AND "), countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench members: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	if lookup && (filter.PageSize <= 0 || filter.PageSize > 50) {
		pageSize = 20
	}
	queryArgs := append([]interface{}{}, args...)
	queryArgs = append(queryArgs, pageSize, (page-1)*pageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchMemberSelect()+`
		WHERE `+strings.Join(where, " AND ")+`
		GROUP BY u.id, am.status, p.id
		ORDER BY
			CASE WHEN p.real_name IS NULL OR p.real_name = '' THEN 1 ELSE 0 END ASC,
			p.real_name ASC,
			u.display_name ASC,
			u.id DESC
		LIMIT ? OFFSET ?`, queryArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench members: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchMember{}
	for rows.Next() {
		item, err := scanAssetWorkbenchMember(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) GetMembership(ctx context.Context, appCode string, userID int64) (*domain.AppMembership, error) {
	row := r.db.db.QueryRowContext(ctx, appMembershipSelect()+` WHERE app_code = ? AND user_id = ?`, appCode, userID)
	return scanAppMembership(row)
}

func (r *assetWorkbenchRepo) LockMembership(ctx context.Context, tx repo.Tx, appCode string, userID int64) (*domain.AppMembership, error) {
	row := Unwrap(tx).QueryRowContext(ctx, appMembershipSelect()+` WHERE app_code = ? AND user_id = ? FOR UPDATE`, appCode, userID)
	return scanAppMembership(row)
}

func (r *assetWorkbenchRepo) UpsertMembership(ctx context.Context, tx repo.Tx, membership *domain.AppMembership) (*domain.AppMembership, error) {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO app_memberships (
			app_code, user_id, status, identity_type, source, last_asset_roles_json, opened_by, disabled_by, disabled_reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE
			status = VALUES(status),
			identity_type = VALUES(identity_type),
			source = CASE WHEN VALUES(source) = '' THEN source ELSE VALUES(source) END,
			last_asset_roles_json = VALUES(last_asset_roles_json),
			opened_by = VALUES(opened_by),
			disabled_by = VALUES(disabled_by),
			disabled_reason = VALUES(disabled_reason),
			updated_at = CURRENT_TIMESTAMP`,
		membership.AppCode,
		membership.UserID,
		membership.Status,
		membership.IdentityType,
		membership.Source,
		nullableJSON(membership.LastAssetRolesJSON),
		toNullInt64(membership.OpenedBy),
		toNullInt64(membership.DisabledBy),
		membership.DisabledReason,
	)
	if err != nil {
		return nil, fmt.Errorf("upsert app membership: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, appMembershipSelect()+` WHERE app_code = ? AND user_id = ?`, membership.AppCode, membership.UserID)
	return scanAppMembership(row)
}

func (r *assetWorkbenchRepo) RequestMembership(ctx context.Context, tx repo.Tx, appCode string, userID int64, identityType string) (*domain.AppMembership, error) {
	_, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO app_memberships (
			app_code, user_id, status, identity_type, source
		) VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE updated_at = updated_at`,
		appCode, userID, domain.AppMembershipStatusPending, identityType, domain.AppMembershipSourceRequestApproved,
	)
	if err != nil {
		return nil, fmt.Errorf("request app membership: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, appMembershipSelect()+` WHERE app_code = ? AND user_id = ?`, appCode, userID)
	return scanAppMembership(row)
}

func (r *assetWorkbenchRepo) OpenMembership(ctx context.Context, tx repo.Tx, params repo.AssetWorkbenchAccessOpenParams) (*domain.AppMembership, error) {
	openedBy := &params.OpenedBy
	membership := &domain.AppMembership{
		AppCode:      domain.AssetWorkbenchAppCode,
		UserID:       params.UserID,
		Status:       params.Status,
		IdentityType: params.IdentityType,
		Source:       params.Source,
		OpenedBy:     openedBy,
	}
	return r.UpsertMembership(ctx, tx, membership)
}

func (r *assetWorkbenchRepo) DisableMembership(ctx context.Context, tx repo.Tx, appCode string, userID int64, disabledBy int64, reason string, lastRoles []domain.Role) (*domain.AppMembership, error) {
	raw, _ := json.Marshal(lastRoles)
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE app_memberships
		SET status = ?, disabled_by = ?, disabled_reason = ?, last_asset_roles_json = ?, updated_at = CURRENT_TIMESTAMP
		WHERE app_code = ? AND user_id = ? AND status = ?`,
		domain.AppMembershipStatusDisabled,
		disabledBy,
		reason,
		raw,
		appCode,
		userID,
		domain.AppMembershipStatusActive,
	)
	if err != nil {
		return nil, fmt.Errorf("disable app membership: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Membership is not active.", nil)
	}
	row := Unwrap(tx).QueryRowContext(ctx, appMembershipSelect()+` WHERE app_code = ? AND user_id = ?`, appCode, userID)
	return scanAppMembership(row)
}

func (r *assetWorkbenchRepo) MarkMembershipMerged(ctx context.Context, tx repo.Tx, appCode string, sourceUserID int64) error {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE app_memberships
		SET status = ?, source = ?, updated_at = CURRENT_TIMESTAMP
		WHERE app_code = ? AND user_id = ?`,
		domain.AppMembershipStatusMerged,
		domain.AppMembershipSourceMerged,
		appCode,
		sourceUserID,
	)
	if err != nil {
		return fmt.Errorf("mark app membership merged: %w", err)
	}
	affected, _ := res.RowsAffected()
	if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *assetWorkbenchRepo) CreateAppIdentityEvent(ctx context.Context, tx repo.Tx, event *domain.AppIdentityEvent) (*domain.AppIdentityEvent, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO app_identity_events (
			actor_user_id, target_user_id, source_app, target_app, action, before_json, after_json, reason
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		toNullInt64(event.ActorUserID),
		toNullInt64(event.TargetUserID),
		event.SourceApp,
		event.TargetApp,
		event.Action,
		nullableJSON(event.Before),
		nullableJSON(event.After),
		event.Reason,
	)
	if err != nil {
		return nil, fmt.Errorf("insert app identity event: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("app identity event last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, appIdentityEventSelect()+` WHERE id = ?`, id)
	return scanAppIdentityEvent(row)
}

func (r *assetWorkbenchRepo) CreateAccountLink(ctx context.Context, tx repo.Tx, link *domain.AssetWorkbenchAccountLink) (*domain.AssetWorkbenchAccountLink, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_account_links (
			source_user_id, canonical_user_id, status, created_by
		) VALUES (?, ?, ?, ?)`,
		link.SourceUserID,
		link.CanonicalUserID,
		"merged",
		link.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench account link: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench account link last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchAccountLinkSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchAccountLink(row)
}

func (r *assetWorkbenchRepo) GetAccountLinkBySource(ctx context.Context, sourceUserID int64) (*domain.AssetWorkbenchAccountLink, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchAccountLinkSelect()+` WHERE source_user_id = ?`, sourceUserID)
	return scanAssetWorkbenchAccountLink(row)
}

func (r *assetWorkbenchRepo) GetAccountLinkByCanonical(ctx context.Context, canonicalUserID int64) (*domain.AssetWorkbenchAccountLink, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchAccountLinkSelect()+` WHERE canonical_user_id = ? LIMIT 1`, canonicalUserID)
	return scanAssetWorkbenchAccountLink(row)
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

func (r *assetWorkbenchRepo) GetPriceMatrixForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.AssetWorkbenchPriceMatrix, error) {
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchPriceMatrixSelect()+` WHERE id = ? FOR UPDATE`, id)
	return scanAssetWorkbenchPriceMatrix(row)
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

func (r *assetWorkbenchRepo) SetPriceMatrixEnabled(ctx context.Context, tx repo.Tx, id int64, enabled bool) (*domain.AssetWorkbenchPriceMatrix, error) {
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_price_matrix
		SET enabled = ?, updated_at = NOW()
		WHERE id = ?`, enabled, id); err != nil {
		return nil, fmt.Errorf("set asset workbench price matrix enabled: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchPriceMatrixSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchPriceMatrix(row)
}

func (r *assetWorkbenchRepo) SetPriceMatrixEffectiveTo(ctx context.Context, tx repo.Tx, id int64, effectiveTo *time.Time) (*domain.AssetWorkbenchPriceMatrix, error) {
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_price_matrix
		SET effective_to = ?, updated_at = NOW()
		WHERE id = ?`, toNullTime(effectiveTo), id); err != nil {
		return nil, fmt.Errorf("set asset workbench price matrix effective_to: %w", err)
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

func (r *assetWorkbenchRepo) GetDeductionRuleForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.AssetWorkbenchDeductionRule, error) {
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchDeductionRuleSelect()+` WHERE id = ? FOR UPDATE`, id)
	return scanAssetWorkbenchDeductionRule(row)
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

func (r *assetWorkbenchRepo) SetDeductionRuleEnabled(ctx context.Context, tx repo.Tx, id int64, enabled bool) (*domain.AssetWorkbenchDeductionRule, error) {
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_deduction_rules
		SET enabled = ?, updated_at = NOW()
		WHERE id = ?`, enabled, id); err != nil {
		return nil, fmt.Errorf("set asset workbench deduction rule enabled: %w", err)
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

func (r *assetWorkbenchRepo) GetWelfareRuleForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.AssetWorkbenchWelfareRule, error) {
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchWelfareRuleSelect()+` WHERE id = ? FOR UPDATE`, id)
	return scanAssetWorkbenchWelfareRule(row)
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

func (r *assetWorkbenchRepo) SetWelfareRuleEnabled(ctx context.Context, tx repo.Tx, id int64, enabled bool) (*domain.AssetWorkbenchWelfareRule, error) {
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_welfare_rules
		SET enabled = ?, updated_at = NOW()
		WHERE id = ?`, enabled, id); err != nil {
		return nil, fmt.Errorf("set asset workbench welfare rule enabled: %w", err)
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

func (r *assetWorkbenchRepo) GetPromoCouponForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.AssetWorkbenchPromoCoupon, error) {
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchPromoCouponSelect()+` WHERE id = ? FOR UPDATE`, id)
	return scanAssetWorkbenchPromoCoupon(row)
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

func (r *assetWorkbenchRepo) SetPromoCouponEnabled(ctx context.Context, tx repo.Tx, id int64, enabled bool) (*domain.AssetWorkbenchPromoCoupon, error) {
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_promo_coupons
		SET enabled = ?, updated_at = NOW()
		WHERE id = ?`, enabled, id); err != nil {
		return nil, fmt.Errorf("set asset workbench promo coupon enabled: %w", err)
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

func (r *assetWorkbenchRepo) ListGroups(ctx context.Context, filter repo.AssetWorkbenchGroupFilter) ([]*domain.AssetWorkbenchGroup, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.Keyword); v != "" {
		like := "%" + v + "%"
		where = append(where, "(name LIKE ? OR description LIKE ?)")
		args = append(args, like, like)
	}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_groups WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench groups: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchGroupSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY enabled DESC, updated_at DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench groups: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchGroup{}
	for rows.Next() {
		item, err := scanAssetWorkbenchGroup(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) CreateGroup(ctx context.Context, tx repo.Tx, group *domain.AssetWorkbenchGroup) (*domain.AssetWorkbenchGroup, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_groups (name, description, enabled, created_by)
		VALUES (?, ?, ?, ?)`,
		group.Name, group.Description, group.Enabled, group.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench group: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench group last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchGroupSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchGroup(row)
}

func (r *assetWorkbenchRepo) UpdateGroup(ctx context.Context, tx repo.Tx, group *domain.AssetWorkbenchGroup) (*domain.AssetWorkbenchGroup, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_groups
		SET name = ?, description = ?, enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		group.Name, group.Description, group.Enabled, group.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench group: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench group rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchGroupSelect()+` WHERE id = ?`, group.ID)
	return scanAssetWorkbenchGroup(row)
}

func (r *assetWorkbenchRepo) SetGroupEnabled(ctx context.Context, tx repo.Tx, groupID int64, enabled bool) (*domain.AssetWorkbenchGroup, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_groups
		SET enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		enabled, groupID,
	)
	if err != nil {
		return nil, fmt.Errorf("set asset workbench group enabled: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench group enabled rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchGroupSelect()+` WHERE id = ?`, groupID)
	return scanAssetWorkbenchGroup(row)
}

func (r *assetWorkbenchRepo) AddGroupMembers(ctx context.Context, tx repo.Tx, groupID int64, userIDs []int64) error {
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		if _, err := Unwrap(tx).ExecContext(ctx, `
			INSERT IGNORE INTO asset_workbench_group_members (group_id, user_id)
			VALUES (?, ?)`, groupID, userID); err != nil {
			return fmt.Errorf("insert asset workbench group member: %w", err)
		}
	}
	return nil
}

func (r *assetWorkbenchRepo) RemoveGroupMembers(ctx context.Context, tx repo.Tx, groupID int64, userIDs []int64) error {
	if groupID <= 0 || len(userIDs) == 0 {
		return nil
	}
	placeholders := make([]string, 0, len(userIDs))
	args := []interface{}{groupID}
	for _, userID := range userIDs {
		if userID <= 0 {
			continue
		}
		placeholders = append(placeholders, "?")
		args = append(args, userID)
	}
	if len(placeholders) == 0 {
		return nil
	}
	_, err := Unwrap(tx).ExecContext(ctx, `
		DELETE FROM asset_workbench_group_members
		WHERE group_id = ? AND user_id IN (`+strings.Join(placeholders, ",")+`)`, args...)
	if err != nil {
		return fmt.Errorf("delete asset workbench group members: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) ListGroupMembers(ctx context.Context, groupID int64) ([]*domain.AssetWorkbenchGroupMember, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT gm.group_id, gm.user_id, gm.created_at,
		       COALESCE(u.username, ''),
		       COALESCE(u.display_name, ''),
		       COALESCE(NULLIF(p.real_name, ''), u.display_name, u.username, ''),
		       COALESCE(p.worker_type, ''),
		       COALESCE(p.job_grade, ''),
		       CASE WHEN `+assetWorkbenchAdminExistsSQL()+` THEN 'admin' ELSE 'normal' END,
		       COALESCE(p.pii_completed, 0)
		FROM asset_workbench_group_members gm
		LEFT JOIN users u ON u.id = gm.user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = gm.user_id
		WHERE gm.group_id = ?
		ORDER BY real_name ASC, u.display_name ASC, gm.user_id ASC`, groupID)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench group members: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchGroupMember{}
	for rows.Next() {
		item, err := scanAssetWorkbenchGroupMemberDetail(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) ListTemplates(ctx context.Context, filter repo.AssetWorkbenchTemplateFilter) ([]*domain.AssetWorkbenchTemplate, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if v := strings.TrimSpace(filter.Keyword); v != "" {
		like := "%" + v + "%"
		where = append(where, "(name LIKE ? OR category LIKE ? OR difficulty_class LIKE ?)")
		args = append(args, like, like, like)
	}
	if v := strings.TrimSpace(filter.Category); v != "" {
		where = append(where, "category = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.DifficultyClass); v != "" {
		where = append(where, "difficulty_class = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.WorkerType); v != "" {
		where = append(where, "worker_type = ?")
		args = append(args, v)
	}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_templates WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench templates: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchTemplateSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY enabled DESC, sort_order ASC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench templates: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchTemplate{}
	for rows.Next() {
		item, err := scanAssetWorkbenchTemplate(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) GetTemplate(ctx context.Context, templateID int64) (*domain.AssetWorkbenchTemplate, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchTemplateSelect()+` WHERE id = ?`, templateID)
	return scanAssetWorkbenchTemplate(row)
}

func (r *assetWorkbenchRepo) CreateTemplate(ctx context.Context, tx repo.Tx, template *domain.AssetWorkbenchTemplate) (*domain.AssetWorkbenchTemplate, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_templates (
			name, category, difficulty_class, worker_type, enabled, sort_order, created_by
		) VALUES (?, ?, ?, ?, ?, ?, ?)`,
		template.Name,
		template.Category,
		template.DifficultyClass,
		template.WorkerType,
		template.Enabled,
		template.SortOrder,
		template.CreatedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench template: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench template last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchTemplateSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchTemplate(row)
}

func (r *assetWorkbenchRepo) UpdateTemplate(ctx context.Context, tx repo.Tx, template *domain.AssetWorkbenchTemplate) (*domain.AssetWorkbenchTemplate, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_templates
		SET name = ?, category = ?, difficulty_class = ?, worker_type = ?, enabled = ?, sort_order = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		template.Name,
		template.Category,
		template.DifficultyClass,
		template.WorkerType,
		template.Enabled,
		template.SortOrder,
		template.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench template: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench template rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchTemplateSelect()+` WHERE id = ?`, template.ID)
	return scanAssetWorkbenchTemplate(row)
}

func (r *assetWorkbenchRepo) SetTemplateEnabled(ctx context.Context, tx repo.Tx, templateID int64, enabled bool) (*domain.AssetWorkbenchTemplate, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_templates
		SET enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		enabled, templateID,
	)
	if err != nil {
		return nil, fmt.Errorf("set asset workbench template enabled: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench template enabled rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchTemplateSelect()+` WHERE id = ?`, templateID)
	return scanAssetWorkbenchTemplate(row)
}

func (r *assetWorkbenchRepo) ListTemplatesForUser(ctx context.Context, userID int64) ([]*domain.AssetWorkbenchTemplate, error) {
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchTemplateSelect()+`
		WHERE enabled = 1
		  AND id IN (
		    SELECT template_id
		    FROM asset_workbench_template_assignments
		    WHERE enabled = 1
		      AND (
		        (target_type = ? AND target_id = ?)
		        OR (
		          target_type = ?
		          AND target_id IN (SELECT group_id FROM asset_workbench_group_members WHERE user_id = ?)
		        )
		      )
		  )
		ORDER BY sort_order ASC, id ASC`,
		domain.AssetWorkbenchAssignmentTargetUser,
		userID,
		domain.AssetWorkbenchAssignmentTargetGroup,
		userID,
	)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench templates for user: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchTemplate{}
	for rows.Next() {
		item, err := scanAssetWorkbenchTemplate(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) IsTemplateAssignedToUser(ctx context.Context, userID, templateID int64) (bool, error) {
	var exists int
	err := r.db.db.QueryRowContext(ctx, `
		SELECT 1
		FROM asset_workbench_template_assignments a
		JOIN asset_workbench_templates t ON t.id = a.template_id AND t.enabled = 1
		WHERE a.template_id = ?
		  AND a.enabled = 1
		  AND (
		    (a.target_type = ? AND a.target_id = ?)
		    OR (
		      a.target_type = ?
		      AND a.target_id IN (SELECT group_id FROM asset_workbench_group_members WHERE user_id = ?)
		    )
		  )
		LIMIT 1`,
		templateID,
		domain.AssetWorkbenchAssignmentTargetUser,
		userID,
		domain.AssetWorkbenchAssignmentTargetGroup,
		userID,
	).Scan(&exists)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check asset workbench template assignment: %w", err)
	}
	return true, nil
}

func (r *assetWorkbenchRepo) ListTemplateAssignments(ctx context.Context, filter repo.AssetWorkbenchTemplateAssignmentFilter) ([]*domain.AssetWorkbenchTemplateAssignment, int64, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.TemplateID != nil {
		where = append(where, "a.template_id = ?")
		args = append(args, *filter.TemplateID)
	}
	if v := strings.TrimSpace(filter.TargetType); v != "" {
		where = append(where, "a.target_type = ?")
		args = append(args, v)
	}
	if filter.TargetID != nil {
		where = append(where, "a.target_id = ?")
		args = append(args, *filter.TargetID)
	}
	if filter.Enabled != nil {
		where = append(where, "a.enabled = ?")
		args = append(args, *filter.Enabled)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_template_assignments a WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench template assignments: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchTemplateAssignmentDetailSelect()+`
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY a.updated_at DESC, a.id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench template assignments: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchTemplateAssignment{}
	for rows.Next() {
		item, err := scanAssetWorkbenchTemplateAssignmentDetail(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) CreateTemplateAssignment(ctx context.Context, tx repo.Tx, assignment *domain.AssetWorkbenchTemplateAssignment) (*domain.AssetWorkbenchTemplateAssignment, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_template_assignments (
			template_id, target_type, target_id, enabled, assigned_by
		) VALUES (?, ?, ?, ?, ?)
		ON DUPLICATE KEY UPDATE enabled = VALUES(enabled), assigned_by = VALUES(assigned_by), updated_at = CURRENT_TIMESTAMP`,
		assignment.TemplateID,
		assignment.TargetType,
		assignment.TargetID,
		assignment.Enabled,
		assignment.AssignedBy,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench template assignment: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench template assignment last insert id: %w", err)
	}
	if id == 0 {
		row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchTemplateAssignmentSelect()+`
			WHERE template_id = ? AND target_type = ? AND target_id = ?`,
			assignment.TemplateID, assignment.TargetType, assignment.TargetID)
		return scanAssetWorkbenchTemplateAssignment(row)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchTemplateAssignmentSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchTemplateAssignment(row)
}

func (r *assetWorkbenchRepo) SetTemplateAssignmentEnabled(ctx context.Context, tx repo.Tx, assignmentID int64, enabled bool) (*domain.AssetWorkbenchTemplateAssignment, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_template_assignments
		SET enabled = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		enabled, assignmentID,
	)
	if err != nil {
		return nil, fmt.Errorf("set asset workbench template assignment enabled: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench template assignment rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchTemplateAssignmentSelect()+` WHERE id = ?`, assignmentID)
	return scanAssetWorkbenchTemplateAssignment(row)
}

func (r *assetWorkbenchRepo) ListUploadDirectories(ctx context.Context, filter repo.AssetWorkbenchUploadDirectoryFilter) ([]*domain.AssetWorkbenchUploadDirectory, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchUploadDirectorySelect()+` WHERE `+strings.Join(where, " AND ")+` ORDER BY sort_order ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench upload directories: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchUploadDirectory{}
	for rows.Next() {
		item, err := scanAssetWorkbenchUploadDirectory(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) GetUploadDirectory(ctx context.Context, directoryID int64) (*domain.AssetWorkbenchUploadDirectory, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchUploadDirectorySelect()+` WHERE id = ?`, directoryID)
	return scanAssetWorkbenchUploadDirectory(row)
}

func (r *assetWorkbenchRepo) CreateUploadDirectory(ctx context.Context, tx repo.Tx, directory *domain.AssetWorkbenchUploadDirectory) (*domain.AssetWorkbenchUploadDirectory, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_upload_directories (
			name, oss_prefix, description, difficulty_class, allowed_file_types_json, enabled, sort_order, created_by, updated_by
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		directory.Name,
		directory.OSSPrefix,
		directory.Description,
		directory.DifficultyClass,
		jsonArrayOrNull(directory.AllowedFileTypes),
		directory.Enabled,
		directory.SortOrder,
		directory.CreatedBy,
		toNullInt64(directory.UpdatedBy),
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench upload directory: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench upload directory last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchUploadDirectorySelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchUploadDirectory(row)
}

func (r *assetWorkbenchRepo) UpdateUploadDirectory(ctx context.Context, tx repo.Tx, directory *domain.AssetWorkbenchUploadDirectory) (*domain.AssetWorkbenchUploadDirectory, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_upload_directories
		SET name = ?, oss_prefix = ?, description = ?, difficulty_class = ?, allowed_file_types_json = ?, enabled = ?, sort_order = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		directory.Name,
		directory.OSSPrefix,
		directory.Description,
		directory.DifficultyClass,
		jsonArrayOrNull(directory.AllowedFileTypes),
		directory.Enabled,
		directory.SortOrder,
		toNullInt64(directory.UpdatedBy),
		directory.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench upload directory: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench upload directory rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchUploadDirectorySelect()+` WHERE id = ?`, directory.ID)
	return scanAssetWorkbenchUploadDirectory(row)
}

func (r *assetWorkbenchRepo) ListClientMaterials(ctx context.Context, filter repo.AssetWorkbenchClientMaterialFilter) ([]*domain.AssetWorkbenchClientMaterial, error) {
	where := []string{"1=1"}
	args := []interface{}{}
	if filter.Enabled != nil {
		where = append(where, "enabled = ?")
		args = append(args, *filter.Enabled)
	}
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchClientMaterialSelect()+` WHERE `+strings.Join(where, " AND ")+` ORDER BY sort_order ASC, id ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench client materials: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchClientMaterial{}
	for rows.Next() {
		item, err := scanAssetWorkbenchClientMaterial(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (r *assetWorkbenchRepo) GetClientMaterial(ctx context.Context, materialID int64) (*domain.AssetWorkbenchClientMaterial, error) {
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchClientMaterialSelect()+` WHERE id = ?`, materialID)
	return scanAssetWorkbenchClientMaterial(row)
}

func (r *assetWorkbenchRepo) CreateClientMaterial(ctx context.Context, tx repo.Tx, material *domain.AssetWorkbenchClientMaterial) (*domain.AssetWorkbenchClientMaterial, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_client_materials (
			asset_id, source_type, source_ref, title, description, filename_snapshot, mime_type_snapshot, file_size_snapshot,
			enabled, sort_order, published_by, updated_by, published_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		material.AssetID,
		material.SourceType,
		material.SourceRef,
		material.Title,
		material.Description,
		material.FilenameSnapshot,
		material.MimeTypeSnapshot,
		material.FileSizeSnapshot,
		material.Enabled,
		material.SortOrder,
		material.PublishedBy,
		toNullInt64(material.UpdatedBy),
		material.PublishedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("insert asset workbench client material: %w", err)
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("asset workbench client material last insert id: %w", err)
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchClientMaterialSelect()+` WHERE id = ?`, id)
	return scanAssetWorkbenchClientMaterial(row)
}

func (r *assetWorkbenchRepo) UpdateClientMaterial(ctx context.Context, tx repo.Tx, material *domain.AssetWorkbenchClientMaterial) (*domain.AssetWorkbenchClientMaterial, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_client_materials
		SET asset_id = ?, source_type = ?, source_ref = ?, title = ?, description = ?, filename_snapshot = ?, mime_type_snapshot = ?,
		    file_size_snapshot = ?, enabled = ?, sort_order = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		material.AssetID,
		material.SourceType,
		material.SourceRef,
		material.Title,
		material.Description,
		material.FilenameSnapshot,
		material.MimeTypeSnapshot,
		material.FileSizeSnapshot,
		material.Enabled,
		material.SortOrder,
		toNullInt64(material.UpdatedBy),
		material.ID,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench client material: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return nil, fmt.Errorf("asset workbench client material rows affected: %w", err)
	} else if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchClientMaterialSelect()+` WHERE id = ?`, material.ID)
	return scanAssetWorkbenchClientMaterial(row)
}

func (r *assetWorkbenchRepo) DeleteClientMaterial(ctx context.Context, tx repo.Tx, materialID int64) error {
	res, err := Unwrap(tx).ExecContext(ctx, `DELETE FROM asset_workbench_client_materials WHERE id = ?`, materialID)
	if err != nil {
		return fmt.Errorf("delete asset workbench client material: %w", err)
	}
	if affected, err := res.RowsAffected(); err != nil {
		return fmt.Errorf("asset workbench client material delete rows affected: %w", err)
	} else if affected != 1 {
		return sql.ErrNoRows
	}
	return nil
}

func (r *assetWorkbenchRepo) CreateUploadSession(ctx context.Context, tx repo.Tx, session *domain.AssetWorkbenchUploadSession) (*domain.AssetWorkbenchUploadSession, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_upload_sessions (
			session_id, owner_user_id, upload_directory_id, upload_directory_name, upload_directory_prefix, upload_directory_difficulty_class,
			upload_batch_id, relative_path, is_folder_upload, expected_business_month,
			status, object_key, original_filename, file_size, mime_type,
			file_hash, upload_id, multipart_plan_json, expires_at, uploaded_at, cancelled_at, submitted_item_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		session.SessionID,
		session.OwnerUserID,
		toNullInt64(session.UploadDirectoryID),
		session.UploadDirectoryName,
		session.UploadDirectoryPrefix,
		session.UploadDirectoryDifficultyClass,
		session.UploadBatchID,
		session.RelativePath,
		session.IsFolderUpload,
		session.ExpectedBusinessMonth,
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

func (r *assetWorkbenchRepo) GetSubmissionForUpdate(ctx context.Context, tx repo.Tx, submissionID int64) (*domain.AssetWorkbenchSubmission, error) {
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionSelect()+` WHERE id = ? FOR UPDATE`, submissionID)
	return scanAssetWorkbenchSubmission(row)
}

func (r *assetWorkbenchRepo) VoidSubmission(ctx context.Context, tx repo.Tx, submissionID int64, actorID int64, reason string, at time.Time) (*domain.AssetWorkbenchSubmission, error) {
	var blocked int
	if err := Unwrap(tx).QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM asset_workbench_submission_items
		WHERE submission_id = ?
		  AND (settlement_status <> ? OR current_settlement_batch_id IS NOT NULL)`,
		submissionID,
		domain.AssetWorkbenchSettlementStatusUnsettled,
	).Scan(&blocked); err != nil {
		return nil, fmt.Errorf("check asset workbench submission voidability: %w", err)
	}
	if blocked > 0 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission contains settled or in-batch items and cannot be voided.", map[string]interface{}{"submission_id": submissionID})
	}
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_items
		SET qc_status = ?, voided_at = ?, voided_by = ?, void_reason = ?, updated_at = CURRENT_TIMESTAMP
		WHERE submission_id = ?
		  AND qc_status <> ?`,
		domain.AssetWorkbenchSubmissionStatusVoided,
		at,
		actorID,
		reason,
		submissionID,
		domain.AssetWorkbenchSubmissionStatusVoided,
	); err != nil {
		return nil, fmt.Errorf("void asset workbench submission items: %w", err)
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submissions
		SET status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status <> ?`,
		domain.AssetWorkbenchSubmissionStatusVoided,
		submissionID,
		domain.AssetWorkbenchSubmissionStatusVoided,
	)
	if err != nil {
		return nil, fmt.Errorf("void asset workbench submission: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("asset workbench void submission rows affected: %w", err)
	}
	if affected != 1 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission is already voided.", map[string]interface{}{"submission_id": submissionID})
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionSelect()+` WHERE id = ?`, submissionID)
	return scanAssetWorkbenchSubmission(row)
}

func (r *assetWorkbenchRepo) CreateSubmissionItem(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO asset_workbench_submission_items (
			submission_id, payee_user_id, order_no, template_id, template_name_snapshot, category_snapshot,
			difficulty_class, finalized, page_count, item_count,
			business_month, submitted_at, worker_type_snapshot, job_grade_snapshot, base_price_rule_id,
			base_unit_price, promo_coupon_id, promo_snapshot_json, pricing_snapshot_json, gross_amount,
			pricing_status, qc_status, settlement_status
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		item.SubmissionID,
		item.PayeeUserID,
		item.OrderNo,
		toNullInt64(item.TemplateID),
		item.TemplateNameSnapshot,
		item.CategorySnapshot,
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

func (r *assetWorkbenchRepo) UpdateSubmissionItemEditableFields(ctx context.Context, tx repo.Tx, item *domain.AssetWorkbenchSubmissionItem) (*domain.AssetWorkbenchSubmissionItem, error) {
	if item == nil {
		return nil, sql.ErrNoRows
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_items
		SET order_no = ?, difficulty_class = ?, finalized = ?, page_count = ?, item_count = ?,
		    worker_type_snapshot = ?, job_grade_snapshot = ?,
		    base_price_rule_id = ?, base_unit_price = ?, promo_coupon_id = ?,
		    promo_snapshot_json = ?, pricing_snapshot_json = ?, gross_amount = ?,
		    pricing_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		  AND qc_status <> ?`,
		item.OrderNo,
		item.DifficultyClass,
		item.Finalized,
		item.PageCount,
		item.ItemCount,
		item.WorkerTypeSnapshot,
		item.JobGradeSnapshot,
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
		return nil, fmt.Errorf("update asset workbench submission item editable fields: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("asset workbench editable item rows affected: %w", err)
	}
	if affected != 1 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission item cannot be edited after settlement or void.", map[string]interface{}{"item_id": item.ID})
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionItemSelect()+` WHERE id = ?`, item.ID)
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
		SET worker_type_snapshot = ?, job_grade_snapshot = ?,
		    base_price_rule_id = ?, base_unit_price = ?, promo_coupon_id = ?,
		    promo_snapshot_json = ?, pricing_snapshot_json = ?, gross_amount = ?,
		    pricing_status = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		  AND qc_status <> ?`,
		item.WorkerTypeSnapshot,
		item.JobGradeSnapshot,
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
			submission_id, submission_item_id, upload_session_id, owner_user_id,
			upload_directory_id, upload_directory_name, upload_directory_prefix, upload_directory_difficulty_class,
			upload_batch_id, relative_path, display_name, is_folder_upload,
			object_key, preview_key,
			preview_status, original_filename, file_ext, file_type, mime_type, file_size, file_hash, sort_order,
			created_at, updated_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, UTC_TIMESTAMP()), COALESCE(?, UTC_TIMESTAMP()))`,
		file.SubmissionID,
		file.SubmissionItemID,
		toNullInt64(file.UploadSessionID),
		file.OwnerUserID,
		toNullInt64(file.UploadDirectoryID),
		file.UploadDirectoryName,
		file.UploadDirectoryPrefix,
		file.UploadDirectoryDifficultyClass,
		file.UploadBatchID,
		file.RelativePath,
		firstNonEmpty(file.DisplayName, file.OriginalFilename),
		file.IsFolderUpload,
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
		toNullTime(nonZeroTimePtr(file.CreatedAt)),
		toNullTime(nonZeroTimePtr(file.UpdatedAt)),
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
		SET item_count = (SELECT COUNT(*) FROM asset_workbench_submission_items i WHERE i.submission_id = s.id AND i.voided_at IS NULL),
		    file_count = (SELECT COUNT(*) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id AND f.deleted_at IS NULL),
		    page_count = COALESCE((SELECT SUM(i.page_count) FROM asset_workbench_submission_items i WHERE i.submission_id = s.id AND i.voided_at IS NULL), 0),
		    gross_total = COALESCE((SELECT SUM(i.gross_amount) FROM asset_workbench_submission_items i WHERE i.submission_id = s.id AND i.voided_at IS NULL), 0),
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
		where = append(where, "s.submitter_user_id = ?")
		args = append(args, *filter.SubmitterUserID)
	}
	if filter.PayeeUserID != nil {
		where = append(where, `EXISTS (
			SELECT 1 FROM asset_workbench_submission_items i
			WHERE i.submission_id = s.id AND i.payee_user_id = ?
		)`)
		args = append(args, *filter.PayeeUserID)
	}
	if v := strings.TrimSpace(filter.BusinessMonth); v != "" {
		where = append(where, "s.business_month = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.Status); v != "" {
		where = append(where, "s.status = ?")
		args = append(args, v)
	}
	if v := strings.TrimSpace(filter.SettlementStatus); v != "" {
		where = append(where, `EXISTS (
			SELECT 1 FROM asset_workbench_submission_items i
			WHERE i.submission_id = s.id AND i.settlement_status = ?
		)`)
		args = append(args, v)
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM asset_workbench_submissions s WHERE `+strings.Join(where, " AND "), args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count asset workbench submissions: %w", err)
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSubmissionListSelect()+` WHERE `+strings.Join(where, " AND ")+`
		ORDER BY `+assetWorkbenchSubmissionOrderBy(filter.OrderBy, filter.OrderDir)+`
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("list asset workbench submissions: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchSubmission{}
	for rows.Next() {
		item, err := scanAssetWorkbenchSubmissionList(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func (r *assetWorkbenchRepo) SearchOverviewRows(ctx context.Context, filter repo.AssetWorkbenchOverviewSearchFilter) ([]*domain.AssetWorkbenchOverviewRow, int64, error) {
	query, args := buildAssetWorkbenchOverviewQuery(filter)
	var total int64
	usedFallback := false
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+query+`) aw_overview_count`, args...).Scan(&total); err != nil {
		if strings.TrimSpace(filter.Keyword) == "" || !isMySQLFullTextIndexMissing(err) {
			return nil, 0, fmt.Errorf("count asset workbench overview rows: %w", err)
		}
		query, args = buildAssetWorkbenchOverviewQueryWithMode(filter, false)
		usedFallback = true
		if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+query+`) aw_overview_count`, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count asset workbench overview rows fallback: %w", err)
		}
	}
	if !usedFallback && total == 0 && strings.TrimSpace(filter.Keyword) != "" && externalAssetBooleanQuery(filter.Keyword) != "" {
		query, args = buildAssetWorkbenchOverviewQueryWithMode(filter, false)
		if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (`+query+`) aw_overview_count`, args...).Scan(&total); err != nil {
			return nil, 0, fmt.Errorf("count asset workbench overview rows fallback: %w", err)
		}
	}
	page, pageSize := normalizePage(filter.Page, filter.PageSize)
	rows, err := r.db.db.QueryContext(ctx, `SELECT source, id, title, primary_code, secondary_code, order_no,
		creator_user_id, creator_name, business_month, status, page_count, amount,
		created_at, updated_at, route_path, meta_json
		FROM (`+query+`) aw_overview
		ORDER BY created_at DESC, id DESC
		LIMIT ? OFFSET ?`, append(args, pageSize, (page-1)*pageSize)...)
	if err != nil {
		return nil, 0, fmt.Errorf("search asset workbench overview rows: %w", err)
	}
	defer rows.Close()
	items := []*domain.AssetWorkbenchOverviewRow{}
	for rows.Next() {
		item, err := scanAssetWorkbenchOverviewRow(rows)
		if err != nil {
			return nil, 0, err
		}
		items = append(items, item)
	}
	return items, total, rows.Err()
}

func buildAssetWorkbenchOverviewQuery(filter repo.AssetWorkbenchOverviewSearchFilter) (string, []interface{}) {
	return buildAssetWorkbenchOverviewQueryWithMode(filter, true)
}

func buildAssetWorkbenchOverviewQueryWithMode(filter repo.AssetWorkbenchOverviewSearchFilter, preferFullText bool) (string, []interface{}) {
	includeSubmissions := filter.Submissions
	includeItems := filter.Items
	includeFiles := filter.Files
	if !includeSubmissions && !includeItems && !includeFiles {
		includeSubmissions = true
		includeItems = true
		includeFiles = true
	}
	queries := []string{}
	args := []interface{}{}
	if includeSubmissions {
		submissionWhere, submissionArgs := buildAssetWorkbenchOverviewSubmissionWhere(filter, preferFullText)
		queries = append(queries, assetWorkbenchOverviewSubmissionSelect()+` WHERE `+strings.Join(submissionWhere, " AND "))
		args = append(args, submissionArgs...)
	}
	if includeItems {
		itemWhere, itemArgs := buildAssetWorkbenchOverviewItemWhere(filter, preferFullText)
		queries = append(queries, assetWorkbenchOverviewItemSelect()+` WHERE `+strings.Join(itemWhere, " AND "))
		args = append(args, itemArgs...)
	}
	if includeFiles {
		fileWhere, fileArgs := buildAssetWorkbenchOverviewFileWhere(filter, preferFullText)
		queries = append(queries, assetWorkbenchOverviewFileSelect()+` WHERE `+strings.Join(fileWhere, " AND "))
		args = append(args, fileArgs...)
	}
	if len(queries) == 0 {
		return `SELECT * FROM (` + assetWorkbenchOverviewSubmissionSelect() + ` WHERE 1=0) empty_overview`, nil
	}
	return strings.Join(queries, `
		UNION ALL
		`), args
}

func buildAssetWorkbenchOverviewSubmissionWhere(filter repo.AssetWorkbenchOverviewSearchFilter, preferFullText bool) ([]string, []interface{}) {
	where := []string{"1=1"}
	args := []interface{}{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		if fullText := externalAssetBooleanQuery(keyword); preferFullText && fullText != "" {
			like := "%" + keyword + "%"
			where = append(where, `(s.id IN (
			SELECT s2.id FROM asset_workbench_submissions s2 WHERE `+assetWorkbenchSubmissionFullTextMatchFor("s2")+`
		) OR `+assetWorkbenchOverviewCreatorName("s.submitter_user_id")+` LIKE ? OR s.id IN (
			SELECT i2.submission_id FROM asset_workbench_submission_items i2 WHERE `+assetWorkbenchItemFullTextMatchFor("i2")+`
		) OR s.id IN (
			SELECT f2.submission_id
			FROM asset_workbench_submission_files f2
			JOIN asset_workbench_submission_items i3 ON i3.id = f2.submission_item_id AND i3.voided_at IS NULL
			WHERE f2.deleted_at IS NULL AND `+assetWorkbenchFileFullTextMatchFor("f2")+`
		))`)
			args = append(args, fullText, like, fullText, fullText)
		} else {
			like := "%" + keyword + "%"
			where = append(where, `(s.submission_no LIKE ? OR s.notes LIKE ? OR `+assetWorkbenchOverviewCreatorName("s.submitter_user_id")+` LIKE ? OR EXISTS (
			SELECT 1 FROM asset_workbench_submission_items i WHERE i.submission_id = s.id AND i.order_no LIKE ?
		) OR EXISTS (
			SELECT 1 FROM asset_workbench_submission_files f WHERE f.submission_id = s.id AND f.deleted_at IS NULL AND (f.original_filename LIKE ? OR f.display_name LIKE ? OR f.relative_path LIKE ? OR f.file_type LIKE ?)
		))`)
			args = append(args, like, like, like, like, like, like, like, like)
		}
	}
	if creator := strings.TrimSpace(filter.Creator); creator != "" {
		like := "%" + creator + "%"
		where = append(where, assetWorkbenchOverviewCreatorName("s.submitter_user_id")+` LIKE ?`)
		args = append(args, like)
	}
	if filter.OwnerUserID != nil {
		where = append(where, "s.submitter_user_id = ?")
		args = append(args, *filter.OwnerUserID)
	}
	if filter.CreatedFrom != nil {
		where = append(where, "s.submitted_at >= ?")
		args = append(args, *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		where = append(where, "s.submitted_at <= ?")
		args = append(args, *filter.CreatedTo)
	}
	return where, args
}

func buildAssetWorkbenchOverviewItemWhere(filter repo.AssetWorkbenchOverviewSearchFilter, preferFullText bool) ([]string, []interface{}) {
	where := []string{"1=1"}
	args := []interface{}{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		if fullText := externalAssetBooleanQuery(keyword); preferFullText && fullText != "" {
			like := "%" + keyword + "%"
			where = append(where, `(i.id IN (
			SELECT i2.id FROM asset_workbench_submission_items i2 WHERE `+assetWorkbenchItemFullTextMatchFor("i2")+`
		) OR i.submission_id IN (
			SELECT s2.id FROM asset_workbench_submissions s2 WHERE `+assetWorkbenchSubmissionFullTextMatchFor("s2")+`
		) OR `+assetWorkbenchOverviewCreatorName("i.payee_user_id")+` LIKE ? OR i.id IN (
			SELECT f2.submission_item_id FROM asset_workbench_submission_files f2 WHERE f2.deleted_at IS NULL AND `+assetWorkbenchFileFullTextMatchFor("f2")+`
		))`)
			args = append(args, fullText, fullText, like, fullText)
		} else {
			like := "%" + keyword + "%"
			where = append(where, `(i.order_no LIKE ? OR i.template_name_snapshot LIKE ? OR i.category_snapshot LIKE ? OR i.difficulty_class LIKE ? OR s.submission_no LIKE ? OR `+assetWorkbenchOverviewCreatorName("i.payee_user_id")+` LIKE ? OR EXISTS (
			SELECT 1 FROM asset_workbench_submission_files f WHERE f.submission_item_id = i.id AND f.deleted_at IS NULL AND (f.original_filename LIKE ? OR f.display_name LIKE ? OR f.relative_path LIKE ? OR f.file_type LIKE ?)
		))`)
			args = append(args, like, like, like, like, like, like, like, like, like, like)
		}
	}
	if creator := strings.TrimSpace(filter.Creator); creator != "" {
		like := "%" + creator + "%"
		where = append(where, assetWorkbenchOverviewCreatorName("i.payee_user_id")+` LIKE ?`)
		args = append(args, like)
	}
	if filter.OwnerUserID != nil {
		where = append(where, "i.payee_user_id = ?")
		args = append(args, *filter.OwnerUserID)
	}
	if filter.CreatedFrom != nil {
		where = append(where, "i.submitted_at >= ?")
		args = append(args, *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		where = append(where, "i.submitted_at <= ?")
		args = append(args, *filter.CreatedTo)
	}
	return where, args
}

func buildAssetWorkbenchOverviewFileWhere(filter repo.AssetWorkbenchOverviewSearchFilter, preferFullText bool) ([]string, []interface{}) {
	where := []string{"f.deleted_at IS NULL", "i.voided_at IS NULL"}
	args := []interface{}{}
	if keyword := strings.TrimSpace(filter.Keyword); keyword != "" {
		if fullText := externalAssetBooleanQuery(keyword); preferFullText && fullText != "" {
			like := "%" + keyword + "%"
			where = append(where, `(f.id IN (
			SELECT f2.id FROM asset_workbench_submission_files f2 WHERE f2.deleted_at IS NULL AND `+assetWorkbenchFileFullTextMatchFor("f2")+`
		) OR f.submission_item_id IN (
			SELECT i2.id FROM asset_workbench_submission_items i2 WHERE `+assetWorkbenchItemFullTextMatchFor("i2")+`
		) OR f.submission_id IN (
			SELECT s2.id FROM asset_workbench_submissions s2 WHERE `+assetWorkbenchSubmissionFullTextMatchFor("s2")+`
		) OR `+assetWorkbenchOverviewCreatorName("f.owner_user_id")+` LIKE ?)`)
			args = append(args, fullText, fullText, fullText, like)
		} else {
			like := "%" + keyword + "%"
			where = append(where, `(f.original_filename LIKE ? OR f.display_name LIKE ? OR f.relative_path LIKE ? OR f.file_type LIKE ? OR f.mime_type LIKE ? OR i.order_no LIKE ? OR s.submission_no LIKE ? OR COALESCE(f.upload_directory_name, '') LIKE ? OR `+assetWorkbenchOverviewCreatorName("f.owner_user_id")+` LIKE ?)`)
			args = append(args, like, like, like, like, like, like, like, like, like)
		}
	}
	if creator := strings.TrimSpace(filter.Creator); creator != "" {
		like := "%" + creator + "%"
		where = append(where, assetWorkbenchOverviewCreatorName("f.owner_user_id")+` LIKE ?`)
		args = append(args, like)
	}
	if filter.OwnerUserID != nil {
		where = append(where, "f.owner_user_id = ?")
		args = append(args, *filter.OwnerUserID)
	}
	if filter.CreatedFrom != nil {
		where = append(where, "f.created_at >= ?")
		args = append(args, *filter.CreatedFrom)
	}
	if filter.CreatedTo != nil {
		where = append(where, "f.created_at <= ?")
		args = append(args, *filter.CreatedTo)
	}
	return where, args
}

func assetWorkbenchOverviewText(expr string) string {
	return "CAST((" + expr + ") AS CHAR CHARACTER SET utf8mb4) COLLATE utf8mb4_0900_ai_ci"
}

func assetWorkbenchOverviewNullIfBlank(expr string) string {
	return "NULLIF(" + assetWorkbenchOverviewText(expr) + ", " + assetWorkbenchOverviewText("''") + ")"
}

func assetWorkbenchOverviewCreatorName(userIDExpr string) string {
	return "COALESCE(" +
		assetWorkbenchOverviewNullIfBlank("p.real_name") + ", " +
		assetWorkbenchOverviewNullIfBlank("u.display_name") + ", " +
		assetWorkbenchOverviewNullIfBlank("u.username") + ", " +
		assetWorkbenchOverviewText("CONCAT('用户 ', "+userIDExpr+")") +
		")"
}

func assetWorkbenchSubmissionOrderBy(orderBy, orderDir string) string {
	dir := "DESC"
	if strings.EqualFold(strings.TrimSpace(orderDir), "asc") {
		dir = "ASC"
	}
	idDir := dir
	if dir == "ASC" {
		idDir = "ASC"
	}
	switch strings.ToLower(strings.TrimSpace(orderBy)) {
	case "created_at":
		return "s.created_at " + dir + ", s.id " + idDir
	case "submitted_at", "time":
		return "s.submitted_at " + dir + ", s.id " + idDir
	case "file_type":
		return "(SELECT MIN(LOWER(f.file_type)) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id AND f.deleted_at IS NULL) " + dir + ", s.submitted_at DESC, s.id DESC"
	case "file_name", "filename":
		return "(SELECT MIN(LOWER(COALESCE(NULLIF(f.display_name, ''), f.original_filename))) FROM asset_workbench_submission_files f WHERE f.submission_id = s.id AND f.deleted_at IS NULL) " + dir + ", s.submitted_at DESC, s.id DESC"
	default:
		return "s.submitted_at DESC, s.id DESC"
	}
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

func (r *assetWorkbenchRepo) ListPendingGradeSubmissionItemsForPayee(ctx context.Context, tx repo.Tx, payeeUserID int64, limit int) ([]*domain.AssetWorkbenchSubmissionItem, error) {
	if payeeUserID <= 0 {
		return []*domain.AssetWorkbenchSubmissionItem{}, nil
	}
	if limit <= 0 || limit > 500 {
		limit = 500
	}
	rows, err := Unwrap(tx).QueryContext(ctx, assetWorkbenchSubmissionItemSelect()+`
		WHERE payee_user_id = ?
		  AND pricing_status = ?
		  AND settlement_status = ?
		  AND current_settlement_batch_id IS NULL
		  AND qc_status <> ?
		ORDER BY submitted_at ASC, id ASC
		LIMIT ? FOR UPDATE`,
		payeeUserID,
		domain.AssetWorkbenchPricingStatusPendingGrade,
		domain.AssetWorkbenchSettlementStatusUnsettled,
		domain.AssetWorkbenchSubmissionStatusVoided,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench pending-grade submission items: %w", err)
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
	rows, err := r.db.db.QueryContext(ctx, assetWorkbenchSubmissionFileSelect()+` WHERE submission_item_id = ? AND deleted_at IS NULL ORDER BY sort_order ASC, id ASC`, submissionItemID)
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
	row := r.db.db.QueryRowContext(ctx, assetWorkbenchSubmissionFileSelect()+` WHERE id = ? AND deleted_at IS NULL`, fileID)
	return scanAssetWorkbenchSubmissionFile(row)
}

func (r *assetWorkbenchRepo) ListSubmissionFilesByIDs(ctx context.Context, fileIDs []int64) ([]*domain.AssetWorkbenchSubmissionFile, error) {
	if len(fileIDs) == 0 {
		return []*domain.AssetWorkbenchSubmissionFile{}, nil
	}
	query, args := simpleInClause(assetWorkbenchSubmissionFileSelect()+` WHERE id IN (`, `) AND deleted_at IS NULL ORDER BY id ASC`, int64SliceToInterfaces(fileIDs)...)
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list asset workbench submission files by ids: %w", err)
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

func (r *assetWorkbenchRepo) UpdateSubmissionFileDisplayName(ctx context.Context, tx repo.Tx, fileID int64, displayName string) (*domain.AssetWorkbenchSubmissionFile, error) {
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_files
		SET display_name = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND deleted_at IS NULL`,
		displayName,
		fileID,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench submission file display name: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("asset workbench file display name rows affected: %w", err)
	}
	if affected != 1 {
		return nil, sql.ErrNoRows
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionFileSelect()+` WHERE id = ?`, fileID)
	return scanAssetWorkbenchSubmissionFile(row)
}

func (r *assetWorkbenchRepo) UpdateSubmissionFileLocation(ctx context.Context, tx repo.Tx, file *domain.AssetWorkbenchSubmissionFile) (*domain.AssetWorkbenchSubmissionFile, error) {
	if file == nil {
		return nil, sql.ErrNoRows
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_files f
		JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id
		SET f.upload_directory_id = ?, f.upload_directory_name = ?, f.upload_directory_prefix = ?, f.upload_directory_difficulty_class = ?,
		    f.object_key = ?, f.preview_key = ?, f.updated_at = CURRENT_TIMESTAMP
		WHERE f.id = ?
		  AND i.settlement_status = ?
		  AND i.current_settlement_batch_id IS NULL
		  AND i.qc_status <> ?`,
		toNullInt64(file.UploadDirectoryID),
		file.UploadDirectoryName,
		file.UploadDirectoryPrefix,
		file.UploadDirectoryDifficultyClass,
		file.ObjectKey,
		file.PreviewKey,
		file.ID,
		domain.AssetWorkbenchSettlementStatusUnsettled,
		domain.AssetWorkbenchSubmissionStatusVoided,
	)
	if err != nil {
		return nil, fmt.Errorf("update asset workbench submission file location: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("asset workbench file location rows affected: %w", err)
	}
	if affected != 1 {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "Submission file cannot be moved after settlement or void.", map[string]interface{}{"file_id": file.ID})
	}
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSubmissionFileSelect()+` WHERE id = ?`, file.ID)
	return scanAssetWorkbenchSubmissionFile(row)
}

func (r *assetWorkbenchRepo) DeleteSubmissionFile(ctx context.Context, tx repo.Tx, fileID int64, actorID int64, reason string, at time.Time) error {
	if at.IsZero() {
		at = time.Now().UTC()
	}
	res, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_submission_files f
		JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id
		SET f.deleted_at = ?, f.deleted_by = ?, f.delete_reason = ?, f.updated_at = CURRENT_TIMESTAMP
		WHERE f.id = ?
		  AND f.deleted_at IS NULL
		  AND i.settlement_status = ?
		  AND i.current_settlement_batch_id IS NULL
		  AND i.qc_status <> ?`,
		at,
		actorID,
		strings.TrimSpace(reason),
		fileID,
		domain.AssetWorkbenchSettlementStatusUnsettled,
		domain.AssetWorkbenchSubmissionStatusVoided,
	)
	if err != nil {
		return fmt.Errorf("delete asset workbench submission file: %w", err)
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("asset workbench delete file rows affected: %w", err)
	}
	if affected != 1 {
		return domain.NewAppError(domain.ErrCodeConflict, "Submission file cannot be deleted after settlement or void.", map[string]interface{}{"file_id": fileID})
	}
	return nil
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
		  AND deleted_at IS NULL
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
			import_batch_id, business_month, payee_user_id, order_no, difficulty_class, occurred_date, error_count,
			raw_payload_json, match_status, submission_item_id
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		record.ImportBatchID,
		record.BusinessMonth,
		toNullInt64(record.PayeeUserID),
		record.OrderNo,
		record.DifficultyClass,
		toNullTime(record.OccurredDate),
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
			quantity, unit_price, direction, source_ref_type, source_ref_id, snapshot_json, paid_to_user_id, payout_snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
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
		toNullInt64(item.PaidToUserID),
		nullableJSON(item.PayoutSnapshot),
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

func (r *assetWorkbenchRepo) FreezeSettlementPayouts(ctx context.Context, tx repo.Tx, batchID int64, at time.Time, snapshots map[int64]json.RawMessage) error {
	if len(snapshots) == 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	for payeeUserID, snapshot := range snapshots {
		res, err := sqlTx.ExecContext(ctx, `
			UPDATE asset_workbench_settlement_items
			SET paid_to_user_id = payee_user_id,
			    payout_snapshot_json = ?
			WHERE batch_id = ?
			  AND payee_user_id = ?
			  AND paid_to_user_id IS NULL`,
			nullableJSON(snapshot),
			batchID,
			payeeUserID,
		)
		if err != nil {
			return fmt.Errorf("freeze asset workbench settlement payout: %w", err)
		}
		if affected, _ := res.RowsAffected(); affected == 0 {
			continue
		}
	}
	var missing int
	if err := sqlTx.QueryRowContext(ctx, `
		SELECT COUNT(*)
		FROM asset_workbench_settlement_items
		WHERE batch_id = ?
		  AND paid_to_user_id IS NULL`,
		batchID,
	).Scan(&missing); err != nil {
		return fmt.Errorf("count missing asset workbench payout snapshots: %w", err)
	}
	if missing > 0 {
		return domain.NewAppError(domain.ErrCodeConflict, "Some settlement items are missing payout snapshots.", map[string]int{"missing": missing})
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

func (r *assetWorkbenchRepo) ListConfirmedSettlementItemsByPayee(ctx context.Context, payeeUserID int64) ([]*domain.AssetWorkbenchSettlementItem, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT si.id, si.batch_id, si.item_type, si.submission_item_id, si.payee_user_id, si.paid_to_user_id, si.business_month,
			si.amount, si.quantity, si.unit_price, si.direction, si.source_ref_type, si.source_ref_id, si.snapshot_json, si.payout_snapshot_json, si.created_at
		FROM asset_workbench_settlement_items si
		JOIN asset_workbench_settlement_batches sb ON sb.id = si.batch_id
		WHERE si.payee_user_id = ?
		  AND sb.status = ?
		ORDER BY si.business_month DESC, si.id ASC`,
		payeeUserID,
		domain.AssetWorkbenchBatchStatusConfirmed,
	)
	if err != nil {
		return nil, fmt.Errorf("list confirmed asset workbench settlement items by payee: %w", err)
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

func (r *assetWorkbenchRepo) GetSettlementSupplementForUpdate(ctx context.Context, tx repo.Tx, id int64) (*domain.AssetWorkbenchSettlementSupplement, error) {
	row := Unwrap(tx).QueryRowContext(ctx, assetWorkbenchSettlementSupplementSelect()+` WHERE id = ? FOR UPDATE`, id)
	return scanAssetWorkbenchSettlementSupplement(row)
}

func (r *assetWorkbenchRepo) VoidSettlementSupplement(ctx context.Context, tx repo.Tx, id int64) (*domain.AssetWorkbenchSettlementSupplement, error) {
	if _, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE asset_workbench_settlement_supplements
		SET status = ?, updated_at = NOW()
		WHERE id = ?`, domain.AssetWorkbenchSupplementStatusVoided, id); err != nil {
		return nil, fmt.Errorf("void asset workbench settlement supplement: %w", err)
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

func (r *assetWorkbenchRepo) MergeProfiles(ctx context.Context, tx repo.Tx, sourceUserID, canonicalUserID int64, fieldChoices map[string]string, actorID int64) error {
	sqlTx := Unwrap(tx)
	source, err := scanAssetWorkbenchProfile(sqlTx.QueryRowContext(ctx, assetWorkbenchProfileSelect()+` WHERE user_id = ? FOR UPDATE`, sourceUserID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil
		}
		return fmt.Errorf("lock source asset workbench profile: %w", err)
	}
	canonical, err := scanAssetWorkbenchProfile(sqlTx.QueryRowContext(ctx, assetWorkbenchProfileSelect()+` WHERE user_id = ? FOR UPDATE`, canonicalUserID))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("lock canonical asset workbench profile: %w", err)
	}
	if canonical == nil {
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE asset_workbench_profiles
			SET user_id = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
			WHERE user_id = ?`,
			canonicalUserID, actorID, sourceUserID,
		); err != nil {
			return fmt.Errorf("move source asset workbench profile: %w", err)
		}
		if _, err := sqlTx.ExecContext(ctx, `
			UPDATE asset_workbench_profile_grade_periods
			SET user_id = ?
			WHERE user_id = ?`,
			canonicalUserID, sourceUserID,
		); err != nil {
			return fmt.Errorf("move source asset workbench grade periods user: %w", err)
		}
		return nil
	}

	merged := *canonical
	mergeString := func(field, sourceValue, canonicalValue string) string {
		if strings.TrimSpace(canonicalValue) == "" {
			return sourceValue
		}
		if strings.TrimSpace(sourceValue) == "" {
			return canonicalValue
		}
		if strings.EqualFold(strings.TrimSpace(fieldChoices[field]), "source") {
			return sourceValue
		}
		return canonicalValue
	}
	mergePtr := func(field string, sourceValue, canonicalValue *string) *string {
		if canonicalValue == nil || strings.TrimSpace(*canonicalValue) == "" {
			return sourceValue
		}
		if sourceValue == nil || strings.TrimSpace(*sourceValue) == "" {
			return canonicalValue
		}
		if strings.EqualFold(strings.TrimSpace(fieldChoices[field]), "source") {
			return sourceValue
		}
		return canonicalValue
	}
	merged.WorkerType = mergeString("worker_type", source.WorkerType, canonical.WorkerType)
	merged.JobGrade = mergeString("job_grade", source.JobGrade, canonical.JobGrade)
	merged.RealName = mergeString("real_name", source.RealName, canonical.RealName)
	merged.Phone = mergePtr("phone", source.Phone, canonical.Phone)
	merged.Province = mergeString("province", source.Province, canonical.Province)
	merged.City = mergeString("city", source.City, canonical.City)
	merged.IDCard = mergePtr("id_card", source.IDCard, canonical.IDCard)
	merged.Gender = mergeString("gender", source.Gender, canonical.Gender)
	merged.AlipayAccount = mergeString("alipay_account", source.AlipayAccount, canonical.AlipayAccount)
	if merged.OnboardedAt == nil {
		merged.OnboardedAt = source.OnboardedAt
	}
	merged.Status = mergeString("status", source.Status, canonical.Status)
	merged.PIICompleted = merged.RealName != "" && merged.Phone != nil && *merged.Phone != "" && merged.IDCard != nil && *merged.IDCard != "" && merged.AlipayAccount != ""

	if _, err := sqlTx.ExecContext(ctx, `
		UPDATE asset_workbench_profiles
		SET worker_type = ?, job_grade = ?, real_name = ?, phone = ?, province = ?, city = ?,
		    id_card = ?, gender = ?, alipay_account = ?, onboarded_at = ?, status = ?,
		    pii_completed = ?, updated_by = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?`,
		merged.WorkerType,
		merged.JobGrade,
		merged.RealName,
		toNullStringPtr(merged.Phone),
		merged.Province,
		merged.City,
		toNullStringPtr(merged.IDCard),
		merged.Gender,
		merged.AlipayAccount,
		toNullTime(merged.OnboardedAt),
		merged.Status,
		merged.PIICompleted,
		actorID,
		canonical.ID,
	); err != nil {
		return fmt.Errorf("merge canonical asset workbench profile: %w", err)
	}

	if _, err := sqlTx.ExecContext(ctx, `
		INSERT INTO asset_workbench_profile_grade_periods (
			profile_id, user_id, worker_type, job_grade, effective_from, effective_to, changed_by, reason
		)
		SELECT ?, ?, sgp.worker_type, sgp.job_grade, sgp.effective_from, sgp.effective_to, ?, CONCAT('merged from user ', ?)
		FROM asset_workbench_profile_grade_periods sgp
		WHERE sgp.user_id = ?
		  AND NOT EXISTS (
			SELECT 1
			FROM asset_workbench_profile_grade_periods cgp
			WHERE cgp.user_id = ?
			  AND cgp.effective_from <= COALESCE(sgp.effective_to, '9999-12-31')
			  AND COALESCE(cgp.effective_to, '9999-12-31') >= sgp.effective_from
		  )`,
		canonical.ID,
		canonicalUserID,
		actorID,
		sourceUserID,
		sourceUserID,
		canonicalUserID,
	); err != nil {
		return fmt.Errorf("merge asset workbench grade periods: %w", err)
	}
	if _, err := sqlTx.ExecContext(ctx, `DELETE FROM asset_workbench_profile_grade_periods WHERE user_id = ?`, sourceUserID); err != nil {
		return fmt.Errorf("delete source asset workbench grade periods: %w", err)
	}
	if _, err := sqlTx.ExecContext(ctx, `DELETE FROM asset_workbench_profiles WHERE id = ?`, source.ID); err != nil {
		return fmt.Errorf("delete source asset workbench profile: %w", err)
	}
	return nil
}

func (r *assetWorkbenchRepo) CountAccountMergeImpact(ctx context.Context, sourceUserID, canonicalUserID int64) (repo.AssetWorkbenchMergeRewriteCounts, error) {
	var counts repo.AssetWorkbenchMergeRewriteCounts
	count := func(target *int64, query string, args ...interface{}) error {
		if err := r.db.db.QueryRowContext(ctx, query, args...).Scan(target); err != nil {
			return err
		}
		return nil
	}
	if err := count(&counts.Submissions, `SELECT COUNT(*) FROM asset_workbench_submissions WHERE submitter_user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge submissions impact: %w", err)
	}
	if err := count(&counts.SubmissionItems, `SELECT COUNT(*) FROM asset_workbench_submission_items WHERE payee_user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge submission items impact: %w", err)
	}
	if err := count(&counts.UploadSessions, `SELECT COUNT(*) FROM asset_workbench_upload_sessions WHERE owner_user_id = ? AND status IN ('created', 'uploading', 'uploaded', 'submitted')`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge upload sessions impact: %w", err)
	}
	if err := count(&counts.SubmissionFiles, `SELECT COUNT(*) FROM asset_workbench_submission_files WHERE owner_user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge submission files impact: %w", err)
	}
	if err := count(&counts.ErrorRecords, `SELECT COUNT(*) FROM asset_workbench_error_records WHERE payee_user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge error records impact: %w", err)
	}
	if err := count(&counts.SettlementSupplements, `SELECT COUNT(*) FROM asset_workbench_settlement_supplements WHERE payee_user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge settlement supplements impact: %w", err)
	}
	if err := count(&counts.SettlementItems, `SELECT COUNT(*) FROM asset_workbench_settlement_items WHERE payee_user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge settlement items impact: %w", err)
	}
	if err := count(&counts.SettlementItemsDeduped, `
		SELECT COALESCE(SUM(duplicate_count), 0)
		FROM (
			SELECT COUNT(*) - 1 AS duplicate_count
			FROM asset_workbench_settlement_items si
			JOIN asset_workbench_settlement_batches sb ON sb.id = si.batch_id
			WHERE si.payee_user_id IN (?, ?)
			  AND sb.status = ?
			GROUP BY si.batch_id, si.item_type, si.business_month, si.direction,
				CASE WHEN si.submission_item_id IS NULL THEN 0 ELSE si.submission_item_id END,
				CASE WHEN si.submission_item_id IS NULL THEN IFNULL(si.source_ref_type, '') ELSE '' END,
				CASE WHEN si.submission_item_id IS NULL THEN IFNULL(si.source_ref_id, 0) ELSE 0 END
			HAVING COUNT(*) > 1
		) d`,
		sourceUserID,
		canonicalUserID,
		domain.AssetWorkbenchBatchStatusGenerated,
	); err != nil {
		return counts, fmt.Errorf("count merge settlement item duplicate impact: %w", err)
	}
	if err := count(&counts.GroupMembers, `SELECT COUNT(*) FROM asset_workbench_group_members WHERE user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge group members impact: %w", err)
	}
	if err := count(&counts.TemplateAssignments, `SELECT COUNT(*) FROM asset_workbench_template_assignments WHERE target_type = 'user' AND target_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge template assignments impact: %w", err)
	}
	if err := count(&counts.SavedViews, `SELECT COUNT(*) FROM asset_workbench_saved_views WHERE user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge saved views impact: %w", err)
	}
	if err := count(&counts.GradePeriods, `SELECT COUNT(*) FROM asset_workbench_profile_grade_periods WHERE user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge grade periods impact: %w", err)
	}
	if err := count(&counts.SupplementPermissions, `SELECT COUNT(*) FROM asset_workbench_supplement_permissions WHERE payee_user_id = ?`, sourceUserID); err != nil {
		return counts, fmt.Errorf("count merge supplement permissions impact: %w", err)
	}
	return counts, nil
}

func (r *assetWorkbenchRepo) RewriteAccountOwnership(ctx context.Context, tx repo.Tx, sourceUserID, canonicalUserID int64) (repo.AssetWorkbenchMergeRewriteCounts, error) {
	sqlTx := Unwrap(tx)
	var counts repo.AssetWorkbenchMergeRewriteCounts
	execCount := func(target *int64, query string, args ...interface{}) error {
		res, err := sqlTx.ExecContext(ctx, query, args...)
		if err != nil {
			return err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return err
		}
		*target = affected
		return nil
	}
	if err := execCount(&counts.Submissions, `UPDATE asset_workbench_submissions SET submitter_user_id = ?, updated_at = CURRENT_TIMESTAMP WHERE submitter_user_id = ?`, canonicalUserID, sourceUserID); err != nil {
		return counts, fmt.Errorf("rewrite asset workbench submissions owner: %w", err)
	}
	if err := execCount(&counts.SubmissionItems, `UPDATE asset_workbench_submission_items SET payee_user_id = ?, updated_at = CURRENT_TIMESTAMP WHERE payee_user_id = ?`, canonicalUserID, sourceUserID); err != nil {
		return counts, fmt.Errorf("rewrite asset workbench submission items payee: %w", err)
	}
	if err := execCount(&counts.UploadSessions, `UPDATE asset_workbench_upload_sessions SET owner_user_id = ?, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = ? AND status IN ('created', 'uploading', 'uploaded', 'submitted')`, canonicalUserID, sourceUserID); err != nil {
		return counts, fmt.Errorf("rewrite asset workbench upload sessions owner: %w", err)
	}
	if err := execCount(&counts.SubmissionFiles, `UPDATE asset_workbench_submission_files SET owner_user_id = ?, updated_at = CURRENT_TIMESTAMP WHERE owner_user_id = ?`, canonicalUserID, sourceUserID); err != nil {
		return counts, fmt.Errorf("rewrite asset workbench submission files owner: %w", err)
	}
	if err := execCount(&counts.ErrorRecords, `UPDATE asset_workbench_error_records SET payee_user_id = ?, updated_at = CURRENT_TIMESTAMP WHERE payee_user_id = ?`, canonicalUserID, sourceUserID); err != nil {
		return counts, fmt.Errorf("rewrite asset workbench error record payee: %w", err)
	}
	if err := execCount(&counts.SettlementSupplements, `UPDATE asset_workbench_settlement_supplements SET payee_user_id = ?, updated_at = CURRENT_TIMESTAMP WHERE payee_user_id = ?`, canonicalUserID, sourceUserID); err != nil {
		return counts, fmt.Errorf("rewrite asset workbench settlement supplement payee: %w", err)
	}
	if err := execCount(&counts.SettlementItems, `UPDATE asset_workbench_settlement_items SET payee_user_id = ? WHERE payee_user_id = ?`, canonicalUserID, sourceUserID); err != nil {
		return counts, fmt.Errorf("rewrite asset workbench settlement item payee: %w", err)
	}
	deduped, err := mergeGeneratedSettlementItemDuplicates(ctx, sqlTx, canonicalUserID)
	if err != nil {
		return counts, fmt.Errorf("dedupe generated asset workbench settlement items: %w", err)
	}
	counts.SettlementItemsDeduped = deduped

	if err := mergeUniqueUserRows(ctx, sqlTx, "asset_workbench_group_members", "user_id", "group_id", sourceUserID, canonicalUserID, &counts.GroupMembers); err != nil {
		return counts, fmt.Errorf("merge asset workbench group members: %w", err)
	}
	if err := mergeUniqueUserRows(ctx, sqlTx, "asset_workbench_template_assignments", "target_id", "template_id", sourceUserID, canonicalUserID, &counts.TemplateAssignments, "target_type = 'user'"); err != nil {
		return counts, fmt.Errorf("merge asset workbench template assignments: %w", err)
	}
	if err := mergeUniqueUserRows(ctx, sqlTx, "asset_workbench_saved_views", "user_id", "CONCAT(view_type, ':', view_name)", sourceUserID, canonicalUserID, &counts.SavedViews); err != nil {
		return counts, fmt.Errorf("merge asset workbench saved views: %w", err)
	}
	if err := mergeUniqueUserRows(ctx, sqlTx, "asset_workbench_supplement_permissions", "payee_user_id", "business_month", sourceUserID, canonicalUserID, &counts.SupplementPermissions); err != nil {
		return counts, fmt.Errorf("merge asset workbench supplement permissions: %w", err)
	}
	return counts, nil
}

type generatedSettlementItemMergeRow struct {
	ID               int64
	BatchID          int64
	ItemType         string
	SubmissionItemID sql.NullInt64
	BusinessMonth    string
	Amount           float64
	Quantity         float64
	UnitPrice        sql.NullFloat64
	Direction        string
	SourceRefType    string
	SourceRefID      sql.NullInt64
}

type generatedSettlementItemMergeKey struct {
	BatchID       int64
	ItemType      string
	BusinessMonth string
	Direction     string
	SubmissionKey int64
	SourceRefType string
	SourceRefKey  int64
}

func mergeGeneratedSettlementItemDuplicates(ctx context.Context, tx *sql.Tx, canonicalUserID int64) (int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT si.id, si.batch_id, si.item_type, si.submission_item_id, si.business_month,
			si.amount, si.quantity, si.unit_price, si.direction, si.source_ref_type, si.source_ref_id
		FROM asset_workbench_settlement_items si
		JOIN asset_workbench_settlement_batches sb ON sb.id = si.batch_id
		WHERE si.payee_user_id = ?
		  AND sb.status = ?
		ORDER BY si.batch_id ASC, si.item_type ASC, si.business_month ASC, si.id ASC`,
		canonicalUserID,
		domain.AssetWorkbenchBatchStatusGenerated,
	)
	if err != nil {
		return 0, err
	}
	defer rows.Close()

	groups := map[generatedSettlementItemMergeKey][]generatedSettlementItemMergeRow{}
	for rows.Next() {
		var row generatedSettlementItemMergeRow
		if err := rows.Scan(
			&row.ID,
			&row.BatchID,
			&row.ItemType,
			&row.SubmissionItemID,
			&row.BusinessMonth,
			&row.Amount,
			&row.Quantity,
			&row.UnitPrice,
			&row.Direction,
			&row.SourceRefType,
			&row.SourceRefID,
		); err != nil {
			return 0, err
		}
		groups[row.mergeKey()] = append(groups[row.mergeKey()], row)
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	var deduped int64
	for _, group := range groups {
		if len(group) <= 1 {
			continue
		}
		keep := group[0]
		deleteIDs := make([]int64, 0, len(group)-1)
		totalAmount := 0.0
		totalQuantity := 0.0
		for _, row := range group {
			totalAmount += row.Amount
			totalQuantity += row.Quantity
			if row.ID != keep.ID {
				deleteIDs = append(deleteIDs, row.ID)
			}
		}
		var unitPrice sql.NullFloat64
		if totalQuantity != 0 {
			unitPrice = sql.NullFloat64{Float64: totalAmount / totalQuantity, Valid: true}
		}
		if _, err := tx.ExecContext(ctx, `
			UPDATE asset_workbench_settlement_items
			SET amount = ?, quantity = ?, unit_price = ?
			WHERE id = ?`,
			totalAmount,
			totalQuantity,
			unitPrice,
			keep.ID,
		); err != nil {
			return deduped, err
		}
		query, args := inClause(`DELETE FROM asset_workbench_settlement_items WHERE id IN (`, `)`, int64SliceToInterfaces(deleteIDs)...)
		res, err := tx.ExecContext(ctx, query, args...)
		if err != nil {
			return deduped, err
		}
		affected, err := res.RowsAffected()
		if err != nil {
			return deduped, err
		}
		deduped += affected
	}
	return deduped, nil
}

func (r generatedSettlementItemMergeRow) mergeKey() generatedSettlementItemMergeKey {
	key := generatedSettlementItemMergeKey{
		BatchID:       r.BatchID,
		ItemType:      r.ItemType,
		BusinessMonth: r.BusinessMonth,
		Direction:     r.Direction,
	}
	if r.SubmissionItemID.Valid {
		key.SubmissionKey = r.SubmissionItemID.Int64
		return key
	}
	key.SourceRefType = r.SourceRefType
	if r.SourceRefID.Valid {
		key.SourceRefKey = r.SourceRefID.Int64
	}
	return key
}

func mergeUniqueUserRows(ctx context.Context, tx *sql.Tx, table, userColumn, uniqueExpr string, sourceUserID, canonicalUserID int64, target *int64, extraWhere ...string) error {
	where := fmt.Sprintf("%s = ?", userColumn)
	args := []interface{}{sourceUserID}
	if len(extraWhere) > 0 && strings.TrimSpace(extraWhere[0]) != "" {
		where += " AND " + extraWhere[0]
	}
	updateSQL := fmt.Sprintf(`
		UPDATE %s src
		LEFT JOIN %s dst
		  ON dst.%s = ?
		 AND %s = %s
		SET src.%s = ?
		WHERE src.%s = ?
		  AND dst.%s IS NULL`,
		table, table, userColumn, uniqueExprForAlias("dst", uniqueExpr), uniqueExprForAlias("src", uniqueExpr), userColumn, userColumn, userColumn,
	)
	if len(extraWhere) > 0 && strings.TrimSpace(extraWhere[0]) != "" {
		if strings.Contains(extraWhere[0], "target_type") {
			updateSQL = strings.Replace(updateSQL, "SET src."+userColumn, "AND dst.target_type = src.target_type\n\t\tSET src."+userColumn, 1)
		}
		updateSQL += " AND " + strings.ReplaceAll(extraWhere[0], "target_type", "src.target_type")
	}
	res, err := tx.ExecContext(ctx, updateSQL, canonicalUserID, canonicalUserID, sourceUserID)
	if err != nil {
		return err
	}
	affected, err := res.RowsAffected()
	if err != nil {
		return err
	}
	*target += affected
	deleteSQL := fmt.Sprintf(`DELETE FROM %s WHERE %s`, table, where)
	if _, err := tx.ExecContext(ctx, deleteSQL, args...); err != nil {
		return err
	}
	return nil
}

func uniqueExprForAlias(alias, expr string) string {
	expr = strings.TrimSpace(expr)
	if expr == "CONCAT(view_type, ':', view_name)" {
		return "CONCAT(" + alias + ".view_type, ':', " + alias + ".view_name)"
	}
	return alias + "." + expr
}

func assetWorkbenchProfileSelect() string {
	return `SELECT id, user_id, worker_type, job_grade, real_name, phone, province, city, id_card, gender,
		alipay_account, onboarded_at, grade_hidden, status, pii_completed, created_by, updated_by, created_at, updated_at
		FROM asset_workbench_profiles`
}

func appMembershipSelect() string {
	return `SELECT id, app_code, user_id, status, identity_type, source, last_asset_roles_json,
		opened_by, disabled_by, disabled_reason, created_at, updated_at
		FROM app_memberships`
}

func appIdentityEventSelect() string {
	return `SELECT id, actor_user_id, target_user_id, source_app, target_app, action,
		before_json, after_json, reason, created_at
		FROM app_identity_events`
}

func assetWorkbenchAccountLinkSelect() string {
	return `SELECT id, source_user_id, canonical_user_id, status, created_by, created_at
		FROM asset_workbench_account_links`
}

func assetWorkbenchAdminExistsSQL() string {
	return fmt.Sprintf(`EXISTS (
		SELECT 1
		FROM user_roles aw_admin_roles
		WHERE aw_admin_roles.user_id = u.id
		  AND aw_admin_roles.role IN ('%s', '%s', '%s')
	)`, domain.RoleAssetManager, domain.RoleAssetTemplateAdmin, domain.RoleAssetSettlement)
}

func assetWorkbenchMemberSelect() string {
	return `SELECT u.id,
		       COALESCE(u.username, ''),
		       COALESCE(u.display_name, ''),
		       COALESCE(NULLIF(p.real_name, ''), u.display_name, u.username, ''),
		       COALESCE(p.worker_type, ''),
		       COALESCE(p.job_grade, ''),
		       COALESCE(am.status, ''),
		       COALESCE(p.pii_completed, 0),
		       CASE WHEN ` + assetWorkbenchAdminExistsSQL() + ` THEN 'admin' ELSE 'normal' END,
		       COALESCE(GROUP_CONCAT(DISTINCT ur.role ORDER BY ur.role SEPARATOR ','), ''),
		       COALESCE(p.created_at, u.created_at),
		       COALESCE(p.updated_at, u.updated_at)
		FROM users u
		LEFT JOIN app_memberships am ON am.user_id = u.id AND am.app_code = 'asset_workbench'
		LEFT JOIN asset_workbench_profiles p ON p.user_id = u.id
		LEFT JOIN user_roles ur ON ur.user_id = u.id`
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
	return `SELECT id, session_id, owner_user_id, upload_directory_id, upload_directory_name, upload_directory_prefix, upload_directory_difficulty_class,
		upload_batch_id, relative_path, is_folder_upload, expected_business_month,
		status, object_key, original_filename, file_size, mime_type,
		file_hash, upload_id, multipart_plan_json, expires_at, uploaded_at, cancelled_at, submitted_item_id, created_at, updated_at
		FROM asset_workbench_upload_sessions`
}

func assetWorkbenchSubmissionSelect() string {
	return `SELECT id, submission_no, submitter_user_id, business_month, submitted_at, status, item_count,
		file_count, page_count, gross_total, notes, created_at, updated_at
		FROM asset_workbench_submissions`
}

func assetWorkbenchSubmissionListSelect() string {
	return `SELECT s.id, s.submission_no, s.submitter_user_id,
		COALESCE(NULLIF(p.real_name, ''), NULLIF(u.display_name, ''), NULLIF(u.username, ''), CONCAT('用户 ', s.submitter_user_id)) AS submitter_name,
		COALESCE(u.username, '') AS submitter_username,
		s.business_month, s.submitted_at, s.status, s.item_count,
		s.file_count, s.page_count, s.gross_total, s.notes, s.created_at, s.updated_at
		FROM asset_workbench_submissions s
		LEFT JOIN users u ON u.id = s.submitter_user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = s.submitter_user_id`
}

func assetWorkbenchOverviewSubmissionSelect() string {
	return `SELECT ` + assetWorkbenchOverviewText("'submission'") + ` AS source,
		s.id AS id,
		` + assetWorkbenchOverviewText("s.submission_no") + ` AS title,
		` + assetWorkbenchOverviewText("s.submission_no") + ` AS primary_code,
		` + assetWorkbenchOverviewText("''") + ` AS secondary_code,
		` + assetWorkbenchOverviewText("''") + ` AS order_no,
		s.submitter_user_id AS creator_user_id,
		` + assetWorkbenchOverviewCreatorName("s.submitter_user_id") + ` AS creator_name,
		` + assetWorkbenchOverviewText("s.business_month") + ` AS business_month,
		` + assetWorkbenchOverviewText("s.status") + ` AS status,
		s.page_count AS page_count,
		s.gross_total AS amount,
		s.submitted_at AS created_at,
		s.updated_at AS updated_at,
		` + assetWorkbenchOverviewText("CONCAT('/drive?q=', s.submission_no, '&scope=orders')") + ` AS route_path,
		` + assetWorkbenchOverviewText("JSON_OBJECT('item_count', s.item_count, 'file_count', s.file_count, 'notes', s.notes)") + ` AS meta_json
		FROM asset_workbench_submissions s
		LEFT JOIN users u ON u.id = s.submitter_user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = s.submitter_user_id`
}

func assetWorkbenchOverviewItemSelect() string {
	return `SELECT ` + assetWorkbenchOverviewText("'piecework_item'") + ` AS source,
		i.id AS id,
		COALESCE(` + assetWorkbenchOverviewNullIfBlank("i.order_no") + `, ` + assetWorkbenchOverviewText("CONCAT('计件明细 ', i.id)") + `) AS title,
		` + assetWorkbenchOverviewText("i.order_no") + ` AS primary_code,
		COALESCE(` + assetWorkbenchOverviewNullIfBlank("i.template_name_snapshot") + `, ` + assetWorkbenchOverviewNullIfBlank("i.category_snapshot") + `, ` + assetWorkbenchOverviewText("i.difficulty_class") + `) AS secondary_code,
		` + assetWorkbenchOverviewText("i.order_no") + ` AS order_no,
		i.payee_user_id AS creator_user_id,
		` + assetWorkbenchOverviewCreatorName("i.payee_user_id") + ` AS creator_name,
		` + assetWorkbenchOverviewText("i.business_month") + ` AS business_month,
		` + assetWorkbenchOverviewText("CONCAT(i.qc_status, '/', i.settlement_status)") + ` AS status,
		i.page_count AS page_count,
		i.gross_amount AS amount,
		i.submitted_at AS created_at,
		i.updated_at AS updated_at,
		` + assetWorkbenchOverviewText("CONCAT('/drive?q=', i.order_no, '&scope=orders&item_id=', i.id)") + ` AS route_path,
		` + assetWorkbenchOverviewText("JSON_OBJECT('submission_id', i.submission_id, 'difficulty_class', i.difficulty_class, 'finalized', i.finalized, 'pricing_status', i.pricing_status, 'template_id', i.template_id)") + ` AS meta_json
		FROM asset_workbench_submission_items i
		JOIN asset_workbench_submissions s ON s.id = i.submission_id
		LEFT JOIN users u ON u.id = i.payee_user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = i.payee_user_id`
}

func assetWorkbenchOverviewFileSelect() string {
	return `SELECT ` + assetWorkbenchOverviewText("'submission_file'") + ` AS source,
		f.id AS id,
		COALESCE(` + assetWorkbenchOverviewNullIfBlank("f.display_name") + `, ` + assetWorkbenchOverviewNullIfBlank("f.original_filename") + `, ` + assetWorkbenchOverviewText("CONCAT('交稿文件 ', f.id)") + `) AS title,
		COALESCE(` + assetWorkbenchOverviewNullIfBlank("f.relative_path") + `, ` + assetWorkbenchOverviewNullIfBlank("f.display_name") + `, ` + assetWorkbenchOverviewText("f.original_filename") + `) AS primary_code,
		COALESCE(` + assetWorkbenchOverviewNullIfBlank("f.upload_directory_name") + `, ` + assetWorkbenchOverviewNullIfBlank("f.file_type") + `, ` + assetWorkbenchOverviewText("f.mime_type") + `) AS secondary_code,
		` + assetWorkbenchOverviewText("i.order_no") + ` AS order_no,
		f.owner_user_id AS creator_user_id,
		` + assetWorkbenchOverviewCreatorName("f.owner_user_id") + ` AS creator_name,
		` + assetWorkbenchOverviewText("i.business_month") + ` AS business_month,
		` + assetWorkbenchOverviewText("f.preview_status") + ` AS status,
		i.page_count AS page_count,
		i.gross_amount AS amount,
		f.created_at AS created_at,
		f.updated_at AS updated_at,
		` + assetWorkbenchOverviewText("CONCAT('/drive?file_id=', f.id)") + ` AS route_path,
		` + assetWorkbenchOverviewText("JSON_OBJECT('file_id', f.id, 'submission_id', f.submission_id, 'submission_item_id', f.submission_item_id, 'upload_directory_id', f.upload_directory_id, 'upload_directory_name', f.upload_directory_name, 'mime_type', f.mime_type, 'file_size', f.file_size, 'preview_status', f.preview_status, 'difficulty_class', i.difficulty_class, 'display_name', f.display_name, 'relative_path', f.relative_path, 'upload_batch_id', f.upload_batch_id)") + ` AS meta_json
		FROM asset_workbench_submission_files f
		JOIN asset_workbench_submission_items i ON i.id = f.submission_item_id
		JOIN asset_workbench_submissions s ON s.id = f.submission_id
		LEFT JOIN users u ON u.id = f.owner_user_id
		LEFT JOIN asset_workbench_profiles p ON p.user_id = f.owner_user_id`
}

func assetWorkbenchSubmissionItemSelect() string {
	return `SELECT id, submission_id, payee_user_id, order_no, template_id, template_name_snapshot, category_snapshot,
		difficulty_class, finalized, page_count, item_count, business_month, submitted_at, worker_type_snapshot,
		job_grade_snapshot, base_price_rule_id, base_unit_price, promo_coupon_id, promo_snapshot_json,
		pricing_snapshot_json, gross_amount, pricing_status, qc_status,
		settlement_status, current_settlement_batch_id, voided_at, voided_by, void_reason, created_at, updated_at
		FROM asset_workbench_submission_items`
}

func assetWorkbenchSubmissionFileSelect() string {
	return `SELECT id, submission_id, submission_item_id, upload_session_id, owner_user_id,
		upload_directory_id, upload_directory_name, upload_directory_prefix, upload_directory_difficulty_class,
		upload_batch_id, relative_path, display_name, is_folder_upload, object_key, preview_key,
		preview_status, preview_attempts, preview_error, preview_next_retry_at, preview_worker_id, preview_lease_expires_at,
		original_filename, file_ext, file_type, mime_type, file_size, file_hash, sort_order, created_at, updated_at,
		deleted_at, deleted_by, delete_reason
		FROM asset_workbench_submission_files`
}

func assetWorkbenchUploadDirectorySelect() string {
	return `SELECT id, name, oss_prefix, description, difficulty_class, allowed_file_types_json, enabled, sort_order, created_by, updated_by, created_at, updated_at
		FROM asset_workbench_upload_directories`
}

func assetWorkbenchDifficultyClassSelect() string {
	return `SELECT id, code, name, description, enabled, sort_order, created_by, updated_by, created_at, updated_at
		FROM asset_workbench_difficulty_classes`
}

func assetWorkbenchClientMaterialSelect() string {
	return `SELECT id, asset_id, source_type, source_ref, title, description, filename_snapshot, mime_type_snapshot, file_size_snapshot,
		enabled, sort_order, published_by, updated_by, published_at, created_at, updated_at
		FROM asset_workbench_client_materials`
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

func assetWorkbenchGroupSelect() string {
	return `SELECT id, name, description, enabled, created_by, created_at, updated_at
		FROM asset_workbench_groups`
}

func assetWorkbenchGroupMemberSelect() string {
	return `SELECT group_id, user_id, created_at
		FROM asset_workbench_group_members`
}

func assetWorkbenchTemplateSelect() string {
	return `SELECT id, name, category, difficulty_class, worker_type, enabled, sort_order, created_by, created_at, updated_at
		FROM asset_workbench_templates`
}

func assetWorkbenchTemplateAssignmentSelect() string {
	return `SELECT id, template_id, target_type, target_id, enabled, assigned_by, created_at, updated_at
		FROM asset_workbench_template_assignments`
}

func assetWorkbenchTemplateAssignmentDetailSelect() string {
	return `SELECT a.id, a.template_id, COALESCE(t.name, ''),
		       a.target_type, a.target_id,
		       CASE
		         WHEN a.target_type = 'group' THEN COALESCE(g.name, CONCAT('分组 ', a.target_id))
		         ELSE COALESCE(NULLIF(p.real_name, ''), u.display_name, u.username, CONCAT('用户 ', a.target_id))
		       END,
		       a.enabled, a.assigned_by, a.created_at, a.updated_at
		FROM asset_workbench_template_assignments a
		LEFT JOIN asset_workbench_templates t ON t.id = a.template_id
		LEFT JOIN asset_workbench_groups g ON a.target_type = 'group' AND g.id = a.target_id
		LEFT JOIN users u ON a.target_type = 'user' AND u.id = a.target_id
		LEFT JOIN asset_workbench_profiles p ON a.target_type = 'user' AND p.user_id = a.target_id`
}

func assetWorkbenchErrorImportBatchSelect() string {
	return `SELECT id, import_no, business_month, uploaded_by, original_filename, status, total_rows,
		matched_rows, unmatched_rows, ambiguous_rows, error_message, created_at, updated_at
		FROM asset_workbench_error_import_batches`
}

func assetWorkbenchErrorRecordSelect() string {
	return `SELECT id, import_batch_id, business_month, payee_user_id, order_no, difficulty_class, occurred_date, error_count,
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
	return `SELECT id, batch_id, item_type, submission_item_id, payee_user_id, paid_to_user_id, business_month,
		amount, quantity, unit_price, direction, source_ref_type, source_ref_id, snapshot_json, payout_snapshot_json, created_at
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

func scanAssetWorkbenchMember(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchMember, error) {
	var item domain.AssetWorkbenchMember
	var rolesCSV string
	if err := scanner.Scan(
		&item.UserID,
		&item.Username,
		&item.DisplayName,
		&item.RealName,
		&item.WorkerType,
		&item.JobGrade,
		&item.Status,
		&item.PIICompleted,
		&item.Identity,
		&rolesCSV,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.Roles = parseRoleCSV(rolesCSV)
	return &item, nil
}

func scanAppMembership(scanner interface{ Scan(...interface{}) error }) (*domain.AppMembership, error) {
	var item domain.AppMembership
	var lastRoles sql.NullString
	var openedBy, disabledBy sql.NullInt64
	if err := scanner.Scan(
		&item.ID,
		&item.AppCode,
		&item.UserID,
		&item.Status,
		&item.IdentityType,
		&item.Source,
		&lastRoles,
		&openedBy,
		&disabledBy,
		&item.DisabledReason,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.LastAssetRolesJSON = cloneValidJSON(lastRoles)
	item.OpenedBy = fromNullInt64(openedBy)
	item.DisabledBy = fromNullInt64(disabledBy)
	return &item, nil
}

func scanAppIdentityEvent(scanner interface{ Scan(...interface{}) error }) (*domain.AppIdentityEvent, error) {
	var item domain.AppIdentityEvent
	var actorID, targetID sql.NullInt64
	var beforeJSON, afterJSON sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&actorID,
		&targetID,
		&item.SourceApp,
		&item.TargetApp,
		&item.Action,
		&beforeJSON,
		&afterJSON,
		&item.Reason,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.ActorUserID = fromNullInt64(actorID)
	item.TargetUserID = fromNullInt64(targetID)
	item.Before = cloneValidJSON(beforeJSON)
	item.After = cloneValidJSON(afterJSON)
	return &item, nil
}

func scanAssetWorkbenchAccountLink(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchAccountLink, error) {
	var item domain.AssetWorkbenchAccountLink
	if err := scanner.Scan(
		&item.ID,
		&item.SourceUserID,
		&item.CanonicalUserID,
		&item.Status,
		&item.CreatedBy,
		&item.CreatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func parseRoleCSV(raw string) []domain.Role {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	return domain.NormalizeRoles(strings.Split(raw, ","))
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
	var uploadDirectoryID sql.NullInt64
	var uploadedAt, cancelledAt sql.NullTime
	var submittedItemID sql.NullInt64
	if err := scanner.Scan(
		&item.ID, &item.SessionID, &item.OwnerUserID, &uploadDirectoryID, &item.UploadDirectoryName,
		&item.UploadDirectoryPrefix, &item.UploadDirectoryDifficultyClass, &item.UploadBatchID,
		&item.RelativePath, &item.IsFolderUpload, &item.ExpectedBusinessMonth,
		&item.Status, &item.ObjectKey, &item.OriginalFilename,
		&item.FileSize, &item.MimeType, &item.FileHash, &item.UploadID, &rawPlan, &item.ExpiresAt,
		&uploadedAt, &cancelledAt, &submittedItemID, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.UploadDirectoryID = fromNullInt64(uploadDirectoryID)
	item.MultipartPlan = cloneValidJSON(rawPlan)
	item.UploadedAt = fromNullTime(uploadedAt)
	item.CancelledAt = fromNullTime(cancelledAt)
	item.SubmittedItemID = fromNullInt64(submittedItemID)
	return &item, nil
}

func scanAssetWorkbenchUploadDirectory(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchUploadDirectory, error) {
	var item domain.AssetWorkbenchUploadDirectory
	var updatedBy sql.NullInt64
	var allowedTypes sql.NullString
	if err := scanner.Scan(
		&item.ID,
		&item.Name,
		&item.OSSPrefix,
		&item.Description,
		&item.DifficultyClass,
		&allowedTypes,
		&item.Enabled,
		&item.SortOrder,
		&item.CreatedBy,
		&updatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.UpdatedBy = fromNullInt64(updatedBy)
	item.AllowedFileTypes = stringSliceFromJSON(allowedTypes.String)
	return &item, nil
}

func scanAssetWorkbenchDifficultyClass(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchDifficultyClass, error) {
	var item domain.AssetWorkbenchDifficultyClass
	var createdBy sql.NullInt64
	var updatedBy sql.NullInt64
	if err := scanner.Scan(
		&item.ID,
		&item.Code,
		&item.Name,
		&item.Description,
		&item.Enabled,
		&item.SortOrder,
		&createdBy,
		&updatedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.CreatedBy = fromNullInt64(createdBy)
	item.UpdatedBy = fromNullInt64(updatedBy)
	return &item, nil
}

func scanAssetWorkbenchClientMaterial(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchClientMaterial, error) {
	var item domain.AssetWorkbenchClientMaterial
	var updatedBy sql.NullInt64
	if err := scanner.Scan(
		&item.ID,
		&item.AssetID,
		&item.SourceType,
		&item.SourceRef,
		&item.Title,
		&item.Description,
		&item.FilenameSnapshot,
		&item.MimeTypeSnapshot,
		&item.FileSizeSnapshot,
		&item.Enabled,
		&item.SortOrder,
		&item.PublishedBy,
		&updatedBy,
		&item.PublishedAt,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.UpdatedBy = fromNullInt64(updatedBy)
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

func scanAssetWorkbenchSubmissionList(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSubmission, error) {
	var item domain.AssetWorkbenchSubmission
	if err := scanner.Scan(
		&item.ID, &item.SubmissionNo, &item.SubmitterUserID, &item.SubmitterName, &item.SubmitterUsername,
		&item.BusinessMonth, &item.SubmittedAt, &item.Status, &item.ItemCount, &item.FileCount,
		&item.PageCount, &item.GrossTotal, &item.Notes, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchSubmissionItem(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchSubmissionItem, error) {
	var item domain.AssetWorkbenchSubmissionItem
	var templateID, basePriceRuleID, promoCouponID, currentBatchID, voidedBy sql.NullInt64
	var baseUnitPrice sql.NullFloat64
	var promoSnapshot, pricingSnapshot sql.NullString
	var voidedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.SubmissionID, &item.PayeeUserID, &item.OrderNo, &templateID,
		&item.TemplateNameSnapshot, &item.CategorySnapshot, &item.DifficultyClass,
		&item.Finalized, &item.PageCount, &item.ItemCount, &item.BusinessMonth, &item.SubmittedAt,
		&item.WorkerTypeSnapshot, &item.JobGradeSnapshot, &basePriceRuleID, &baseUnitPrice,
		&promoCouponID, &promoSnapshot, &pricingSnapshot, &item.GrossAmount, &item.PricingStatus,
		&item.QCStatus, &item.SettlementStatus, &currentBatchID, &voidedAt, &voidedBy,
		&item.VoidReason, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.TemplateID = fromNullInt64(templateID)
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
	var uploadSessionID, uploadDirectoryID, deletedBy sql.NullInt64
	var previewNextRetryAt, previewLeaseExpiresAt, deletedAt sql.NullTime
	if err := scanner.Scan(
		&item.ID, &item.SubmissionID, &item.SubmissionItemID, &uploadSessionID, &item.OwnerUserID,
		&uploadDirectoryID, &item.UploadDirectoryName, &item.UploadDirectoryPrefix, &item.UploadDirectoryDifficultyClass,
		&item.UploadBatchID, &item.RelativePath, &item.DisplayName, &item.IsFolderUpload,
		&item.ObjectKey, &item.PreviewKey, &item.PreviewStatus, &item.PreviewAttempts,
		&item.PreviewError, &previewNextRetryAt, &item.PreviewWorkerID, &previewLeaseExpiresAt,
		&item.OriginalFilename, &item.FileExt, &item.FileType, &item.MimeType, &item.FileSize,
		&item.FileHash, &item.SortOrder, &item.CreatedAt, &item.UpdatedAt,
		&deletedAt, &deletedBy, &item.DeleteReason,
	); err != nil {
		return nil, err
	}
	if strings.TrimSpace(item.DisplayName) == "" {
		item.DisplayName = item.OriginalFilename
	}
	item.UploadSessionID = fromNullInt64(uploadSessionID)
	item.UploadDirectoryID = fromNullInt64(uploadDirectoryID)
	item.PreviewNextRetryAt = fromNullTime(previewNextRetryAt)
	item.PreviewLeaseExpiresAt = fromNullTime(previewLeaseExpiresAt)
	item.DeletedAt = fromNullTime(deletedAt)
	item.DeletedBy = fromNullInt64(deletedBy)
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

func scanAssetWorkbenchGroup(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchGroup, error) {
	var item domain.AssetWorkbenchGroup
	if err := scanner.Scan(
		&item.ID, &item.Name, &item.Description, &item.Enabled, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchGroupMember(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchGroupMember, error) {
	var item domain.AssetWorkbenchGroupMember
	if err := scanner.Scan(&item.GroupID, &item.UserID, &item.CreatedAt); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchGroupMemberDetail(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchGroupMember, error) {
	var item domain.AssetWorkbenchGroupMember
	if err := scanner.Scan(
		&item.GroupID,
		&item.UserID,
		&item.CreatedAt,
		&item.Username,
		&item.DisplayName,
		&item.RealName,
		&item.WorkerType,
		&item.JobGrade,
		&item.Identity,
		&item.PIICompleted,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchTemplate(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchTemplate, error) {
	var item domain.AssetWorkbenchTemplate
	if err := scanner.Scan(
		&item.ID, &item.Name, &item.Category, &item.DifficultyClass, &item.WorkerType,
		&item.Enabled, &item.SortOrder, &item.CreatedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchTemplateAssignment(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchTemplateAssignment, error) {
	var item domain.AssetWorkbenchTemplateAssignment
	if err := scanner.Scan(
		&item.ID, &item.TemplateID, &item.TargetType, &item.TargetID, &item.Enabled,
		&item.AssignedBy, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchTemplateAssignmentDetail(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchTemplateAssignment, error) {
	var item domain.AssetWorkbenchTemplateAssignment
	if err := scanner.Scan(
		&item.ID,
		&item.TemplateID,
		&item.TemplateName,
		&item.TargetType,
		&item.TargetID,
		&item.TargetName,
		&item.Enabled,
		&item.AssignedBy,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	return &item, nil
}

func scanAssetWorkbenchOverviewRow(scanner interface{ Scan(...interface{}) error }) (*domain.AssetWorkbenchOverviewRow, error) {
	var item domain.AssetWorkbenchOverviewRow
	var meta sql.NullString
	if err := scanner.Scan(
		&item.Source,
		&item.ID,
		&item.Title,
		&item.PrimaryCode,
		&item.SecondaryCode,
		&item.OrderNo,
		&item.CreatorUserID,
		&item.CreatorName,
		&item.BusinessMonth,
		&item.Status,
		&item.PageCount,
		&item.Amount,
		&item.CreatedAt,
		&item.UpdatedAt,
		&item.RoutePath,
		&meta,
	); err != nil {
		return nil, err
	}
	item.Meta = cloneValidJSON(meta)
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
	var occurredDate sql.NullTime
	var rawPayload sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.ImportBatchID, &item.BusinessMonth, &payeeUserID, &item.OrderNo,
		&item.DifficultyClass, &occurredDate, &item.ErrorCount, &rawPayload, &item.MatchStatus,
		&submissionItemID, &item.CreatedAt, &item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.PayeeUserID = fromNullInt64(payeeUserID)
	item.OccurredDate = fromNullTime(occurredDate)
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
	var submissionItemID, paidToUserID, sourceRefID sql.NullInt64
	var unitPrice sql.NullFloat64
	var snapshot, payoutSnapshot sql.NullString
	if err := scanner.Scan(
		&item.ID, &item.BatchID, &item.ItemType, &submissionItemID, &item.PayeeUserID, &paidToUserID,
		&item.BusinessMonth, &item.Amount, &item.Quantity, &unitPrice, &item.Direction,
		&item.SourceRefType, &sourceRefID, &snapshot, &payoutSnapshot, &item.CreatedAt,
	); err != nil {
		return nil, err
	}
	item.SubmissionItemID = fromNullInt64(submissionItemID)
	item.PaidToUserID = fromNullInt64(paidToUserID)
	item.UnitPrice = fromNullFloat64(unitPrice)
	item.SourceRefID = fromNullInt64(sourceRefID)
	item.Snapshot = cloneValidJSON(snapshot)
	item.PayoutSnapshot = cloneValidJSON(payoutSnapshot)
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

func simpleInClause(prefix, suffix string, args ...interface{}) (string, []interface{}) {
	if len(args) == 0 {
		return prefix + "NULL" + suffix, args
	}
	placeholders := make([]string, 0, len(args))
	for range args {
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

func stringSliceFromJSON(raw string) []string {
	if strings.TrimSpace(raw) == "" {
		return nil
	}
	var values []string
	if err := json.Unmarshal([]byte(raw), &values); err != nil {
		return nil
	}
	return values
}

func jsonArrayOrNull(values []string) interface{} {
	if len(values) == 0 {
		return nil
	}
	data, err := json.Marshal(values)
	if err != nil {
		return nil
	}
	return string(data)
}
