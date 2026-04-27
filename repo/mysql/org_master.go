package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type orgRepo struct{ db *DB }

func NewOrgRepo(db *DB) repo.OrgRepo { return &orgRepo{db: db} }

func (r *orgRepo) ListDepartments(ctx context.Context, includeDisabled bool) ([]*domain.OrgDepartment, error) {
	query := `
		SELECT id, name, enabled, created_at, updated_at
		FROM org_departments`
	args := []interface{}{}
	if !includeDisabled {
		query += ` WHERE enabled = 1`
	}
	query += ` ORDER BY id ASC`
	rows, err := r.db.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list org_departments: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.OrgDepartment, 0)
	for rows.Next() {
		item, scanErr := scanOrgDepartment(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *orgRepo) ListTeams(ctx context.Context, includeDisabled bool) ([]*domain.OrgTeam, error) {
	query := `
		SELECT t.id, t.department_id, d.name AS department, t.name, t.enabled, t.created_at, t.updated_at
		FROM org_teams t
		INNER JOIN org_departments d ON d.id = t.department_id`
	if !includeDisabled {
		query += ` WHERE t.enabled = 1 AND d.enabled = 1`
	}
	query += ` ORDER BY t.id ASC`
	rows, err := r.db.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("list org_teams: %w", err)
	}
	defer rows.Close()

	out := make([]*domain.OrgTeam, 0)
	for rows.Next() {
		item, scanErr := scanOrgTeam(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, item)
	}
	return out, rows.Err()
}

func (r *orgRepo) GetDepartmentByID(ctx context.Context, id int64) (*domain.OrgDepartment, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, name, enabled, created_at, updated_at
		FROM org_departments
		WHERE id = ?`, id)
	return scanOrgDepartment(row)
}

func (r *orgRepo) GetDepartmentByName(ctx context.Context, name string) (*domain.OrgDepartment, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, name, enabled, created_at, updated_at
		FROM org_departments
		WHERE name = ?`, strings.TrimSpace(name))
	return scanOrgDepartment(row)
}

func (r *orgRepo) GetTeamByID(ctx context.Context, id int64) (*domain.OrgTeam, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT t.id, t.department_id, d.name AS department, t.name, t.enabled, t.created_at, t.updated_at
		FROM org_teams t
		INNER JOIN org_departments d ON d.id = t.department_id
		WHERE t.id = ?`, id)
	return scanOrgTeam(row)
}

func (r *orgRepo) GetTeamByName(ctx context.Context, name string) (*domain.OrgTeam, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT t.id, t.department_id, d.name AS department, t.name, t.enabled, t.created_at, t.updated_at
		FROM org_teams t
		INNER JOIN org_departments d ON d.id = t.department_id
		WHERE t.name = ?`, strings.TrimSpace(name))
	return scanOrgTeam(row)
}

func (r *orgRepo) CreateDepartment(ctx context.Context, tx repo.Tx, department *domain.OrgDepartment) (int64, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO org_departments (name, enabled)
		VALUES (?, ?)`,
		strings.TrimSpace(department.Name),
		department.Enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("insert org_department: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("org_department last insert id: %w", err)
	}
	return id, nil
}

func (r *orgRepo) UpdateDepartment(ctx context.Context, tx repo.Tx, department *domain.OrgDepartment) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE org_departments
		SET enabled = ?
		WHERE id = ?`,
		department.Enabled,
		department.ID,
	)
	if err != nil {
		return fmt.Errorf("update org_department: %w", err)
	}
	return nil
}

func (r *orgRepo) CreateTeam(ctx context.Context, tx repo.Tx, team *domain.OrgTeam) (int64, error) {
	result, err := Unwrap(tx).ExecContext(ctx, `
		INSERT INTO org_teams (department_id, name, enabled)
		VALUES (?, ?, ?)`,
		team.DepartmentID,
		strings.TrimSpace(team.Name),
		team.Enabled,
	)
	if err != nil {
		return 0, fmt.Errorf("insert org_team: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return 0, fmt.Errorf("org_team last insert id: %w", err)
	}
	return id, nil
}

func (r *orgRepo) UpdateTeam(ctx context.Context, tx repo.Tx, team *domain.OrgTeam) error {
	_, err := Unwrap(tx).ExecContext(ctx, `
		UPDATE org_teams
		SET enabled = ?
		WHERE id = ?`,
		team.Enabled,
		team.ID,
	)
	if err != nil {
		return fmt.Errorf("update org_team: %w", err)
	}
	return nil
}

func scanOrgDepartment(scanner interface {
	Scan(dest ...interface{}) error
}) (*domain.OrgDepartment, error) {
	item := &domain.OrgDepartment{}
	if err := scanner.Scan(&item.ID, &item.Name, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan org_department: %w", err)
	}
	return item, nil
}

func scanOrgTeam(scanner interface {
	Scan(dest ...interface{}) error
}) (*domain.OrgTeam, error) {
	item := &domain.OrgTeam{}
	if err := scanner.Scan(&item.ID, &item.DepartmentID, &item.Department, &item.Name, &item.Enabled, &item.CreatedAt, &item.UpdatedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("scan org_team: %w", err)
	}
	return item, nil
}
