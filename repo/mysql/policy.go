package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// PolicyRepoImpl implements repo.PolicyRepo.
type PolicyRepoImpl struct{ db *sql.DB }

func NewPolicyRepo(db *DB) repo.PolicyRepo { return &PolicyRepoImpl{db: db.db} }

const policySelectCols = `id, ` + "`key`" + `, value, version, updated_by, updated_at`

func scanPolicy(s interface {
	Scan(...interface{}) error
}) (*domain.SystemPolicy, error) {
	p := &domain.SystemPolicy{}
	if err := s.Scan(&p.ID, &p.Key, &p.Value, &p.Version, &p.UpdatedBy, &p.UpdatedAt); err != nil {
		return nil, err
	}
	return p, nil
}

func (r *PolicyRepoImpl) GetByID(ctx context.Context, id int64) (*domain.SystemPolicy, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+policySelectCols+" FROM system_policies WHERE id = ?",
		id,
	)
	p, err := scanPolicy(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PolicyRepoImpl) GetByKey(ctx context.Context, key string) (*domain.SystemPolicy, error) {
	row := r.db.QueryRowContext(ctx,
		"SELECT "+policySelectCols+" FROM system_policies WHERE `key` = ?",
		key,
	)
	p, err := scanPolicy(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return p, err
}

func (r *PolicyRepoImpl) ListAll(ctx context.Context) ([]*domain.SystemPolicy, error) {
	rows, err := r.db.QueryContext(ctx,
		"SELECT "+policySelectCols+" FROM system_policies ORDER BY `key` ASC",
	)
	if err != nil {
		return nil, fmt.Errorf("list policies: %w", err)
	}
	defer rows.Close()

	var policies []*domain.SystemPolicy
	for rows.Next() {
		p, err := scanPolicy(rows)
		if err != nil {
			return nil, fmt.Errorf("scan policy: %w", err)
		}
		policies = append(policies, p)
	}
	return policies, rows.Err()
}

// Upsert inserts or updates a policy. On duplicate key, value and version are bumped.
// The caller (service layer) is responsible for writing the audit log and event_log.
func (r *PolicyRepoImpl) Upsert(ctx context.Context, policy *domain.SystemPolicy) error {
	now := time.Now()
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO system_policies (`+"`key`"+`, value, version, updated_by, updated_at)
		VALUES (?, ?, 1, ?, ?)
		ON DUPLICATE KEY UPDATE
			value      = VALUES(value),
			version    = version + 1,
			updated_by = VALUES(updated_by),
			updated_at = VALUES(updated_at)`,
		policy.Key,
		policy.Value,
		policy.UpdatedBy,
		now,
	)
	return wrapErr(err, "upsert policy")
}
