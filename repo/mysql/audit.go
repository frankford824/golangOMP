package mysqlrepo

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
)

// ErrDuplicateAuditStage is returned when a different action_id tries to audit
// the same (asset_ver_id, stage) combination — the stage has already been decided.
var ErrDuplicateAuditStage = errors.New("audit stage already decided for this asset version")

// AuditRepoImpl implements repo.AuditRepo.
type AuditRepoImpl struct{ db *sql.DB }

func NewAuditRepo(db *DB) repo.AuditRepo { return &AuditRepoImpl{db: db.db} }

const auditActionSelectCols = `
	id, action_id, asset_ver_id, stage, decision, whole_hash, auditor_id, reason, created_at`

func scanAuditAction(s interface {
	Scan(...interface{}) error
}) (*domain.AuditAction, error) {
	aa := &domain.AuditAction{}
	var reason sql.NullString
	if err := s.Scan(
		&aa.ID,
		&aa.ActionID,
		&aa.AssetVerID,
		&aa.Stage,
		&aa.Decision,
		&aa.WholeHash,
		&aa.AuditorID,
		&reason,
		&aa.CreatedAt,
	); err != nil {
		return nil, err
	}
	aa.Reason = fromNullString(reason)
	return aa, nil
}

// InsertIdempotent attempts to insert a new audit_action inside the caller's transaction.
//
// Idempotency contract (spec §7.3):
//   - Same action_id again → return (existing, false, nil)   — idempotent replay
//   - Different action_id, same (asset_ver_id, stage) → return (nil, false, ErrDuplicateAuditStage)
//   - First insert → return (new, true, nil)
func (r *AuditRepoImpl) InsertIdempotent(
	ctx context.Context,
	tx repo.Tx,
	action *domain.AuditAction,
) (*domain.AuditAction, bool, error) {
	sqlTx := Unwrap(tx)
	now := time.Now()

	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO audit_actions
			(action_id, asset_ver_id, stage, decision, whole_hash, auditor_id, reason, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		action.ActionID,
		action.AssetVerID,
		action.Stage,
		action.Decision,
		action.WholeHash,
		action.AuditorID,
		toNullString(action.Reason),
		now,
	)
	if err == nil {
		// Success: load the row back to get the generated id.
		row := sqlTx.QueryRowContext(ctx,
			`SELECT`+auditActionSelectCols+` FROM audit_actions WHERE action_id = ?`,
			action.ActionID,
		)
		inserted, scanErr := scanAuditAction(row)
		if scanErr != nil {
			return nil, false, fmt.Errorf("fetch inserted audit_action: %w", scanErr)
		}
		return inserted, true, nil
	}

	// Inspect the MySQL error to distinguish which unique constraint fired.
	var mysqlErr *mysql.MySQLError
	if !errors.As(err, &mysqlErr) || mysqlErr.Number != 1062 {
		return nil, false, fmt.Errorf("insert audit_action: %w", err)
	}

	if strings.Contains(mysqlErr.Message, "uk_action_id") {
		// Same action_id submitted again — idempotent return.
		existing, fetchErr := r.GetByActionID(ctx, action.ActionID)
		if fetchErr != nil {
			return nil, false, fetchErr
		}
		return existing, false, nil
	}

	// uk_asset_stage fired: a different action already decided this stage.
	return nil, false, ErrDuplicateAuditStage
}

func (r *AuditRepoImpl) GetByActionID(ctx context.Context, actionID string) (*domain.AuditAction, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT`+auditActionSelectCols+` FROM audit_actions WHERE action_id = ?`,
		actionID,
	)
	aa, err := scanAuditAction(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return aa, err
}
