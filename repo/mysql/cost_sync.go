package mysqlrepo

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"
	"workflow/domain"
	"workflow/repo"
)

type costSyncRepo struct{ db *DB }

func NewCostSyncRepo(db *DB) repo.CostSyncRepo { return &costSyncRepo{db} }
func (r *costSyncRepo) Coverage(ctx context.Context) (*domain.CostSyncCoverage, error) {
	var c domain.CostSyncCoverage
	err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*),COUNT(c.sku_code),COALESCE(SUM(c.status='synced'),0),COALESCE(SUM(c.status='conflict'),0),COALESCE(SUM(c.status='no_price'),0) FROM task_sku_items s LEFT JOIN sku_cost_sync_states c ON c.sku_code=s.sku_code`).Scan(&c.Total, &c.Tracked, &c.Verified, &c.Conflicts, &c.Unpriced)
	return &c, err
}

const syncColumns = `sku_code,local_cost,erp_cost,revision,ack_revision,erp_revision,manual_lock,manual_origin,local_confirmed,projected_revision,status,needs_check,reason,checked_at`

type costRow interface{ Scan(...interface{}) error }

func scanCostState(row costRow) (*domain.CostSyncState, error) {
	var s domain.CostSyncState
	err := row.Scan(&s.SKUCode, &s.LocalCost, &s.ERPCost, &s.Revision, &s.AckRevision, &s.ERPRevision, &s.ManualLock, &s.ManualOrigin, &s.LocalConfirmed, &s.ProjectedRevision, &s.Status, &s.NeedsCheck, &s.Reason, &s.CheckedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	// This table deliberately stores UTC wall time. The shared DB driver may
	// use Asia/Shanghai for legacy tables, so do not apply its offset here.
	if s.CheckedAt != nil {
		v := *s.CheckedAt
		utc := time.Date(v.Year(), v.Month(), v.Day(), v.Hour(), v.Minute(), v.Second(), v.Nanosecond(), time.UTC)
		s.CheckedAt = &utc
	}
	return &s, err
}
func (r *costSyncRepo) Get(ctx context.Context, sku string) (*domain.CostSyncState, error) {
	return scanCostState(r.db.db.QueryRowContext(ctx, `SELECT `+syncColumns+` FROM sku_cost_sync_states WHERE sku_code=?`, sku))
}
func (r *costSyncRepo) List(ctx context.Context, status, q string, page, size int) ([]*domain.CostSyncState, int64, error) {
	where := `1=1`
	args := []interface{}{}
	if status != "" {
		where += ` AND status=?`
		args = append(args, status)
	}
	if q != "" {
		where += ` AND sku_code LIKE ?`
		args = append(args, "%"+q+"%")
	}
	var total int64
	if err := r.db.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM sku_cost_sync_states WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, err
	}
	rows, err := r.db.db.QueryContext(ctx, `SELECT `+syncColumns+` FROM sku_cost_sync_states WHERE `+where+` ORDER BY updated_at DESC,sku_code LIMIT ? OFFSET ?`, append(args, size, (page-1)*size)...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	result := []*domain.CostSyncState{}
	for rows.Next() {
		s, err := scanCostState(rows)
		if err != nil {
			return nil, 0, err
		}
		result = append(result, s)
	}
	return result, total, rows.Err()
}
func (r *costSyncRepo) Lock(ctx context.Context, sku string) (func(), error) {
	conn, err := r.db.db.Conn(ctx)
	if err != nil {
		return nil, err
	}
	key := fmt.Sprintf("cost:%x", sha256.Sum256([]byte(sku)))[:60]
	var got int
	if err = conn.QueryRowContext(ctx, `SELECT GET_LOCK(?,1)`, key).Scan(&got); err != nil || got != 1 {
		conn.Close()
		return nil, fmt.Errorf("SKU成本正在同步，请稍后重试")
	}
	return func() {
		release, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		conn.ExecContext(release, `DO RELEASE_LOCK(?)`, key)
		conn.Close()
	}, nil
}

// Lock canonical SKU first: all ordinary writers acquire this row before the
// trigger touches the sync state, avoiding the state->SKU lock inversion.
func (r *costSyncRepo) withRow(ctx context.Context, sku string, fn func(*sql.Tx, *domain.CostSyncState, int64, int64) error) error {
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id, taskID int64
	if err = tx.QueryRowContext(ctx, `SELECT id,task_id FROM task_sku_items WHERE sku_code=? FOR UPDATE`, sku).Scan(&id, &taskID); errors.Is(err, sql.ErrNoRows) {
		return nil
	} else if err != nil {
		return err
	}
	s, err := scanCostState(tx.QueryRowContext(ctx, `SELECT `+syncColumns+` FROM sku_cost_sync_states WHERE sku_code=? FOR UPDATE`, sku))
	if err != nil {
		return err
	}
	if s == nil {
		return fmt.Errorf("cost state missing")
	}
	if err = fn(tx, s, id, taskID); err != nil {
		return err
	}
	return tx.Commit()
}
func auditCost(ctx context.Context, tx *sql.Tx, s *domain.CostSyncState, action, reason string, actor interface{}) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO sku_cost_sync_audit(sku_code,action,local_cost,erp_cost,revision,actor_id,reason,created_at) VALUES(?,?,?,?,?,?,?,UTC_TIMESTAMP(3))`, s.SKUCode, action, s.LocalCost, s.ERPCost, s.Revision, actor, reason)
	return err
}
func persistCostState(ctx context.Context, tx *sql.Tx, s *domain.CostSyncState) error {
	_, err := tx.ExecContext(ctx, `UPDATE sku_cost_sync_states SET erp_cost=?,ack_revision=?,erp_revision=?,projected_revision=?,status=?,needs_check=?,reason=?,checked_at=UTC_TIMESTAMP(3),updated_at=UTC_TIMESTAMP(3),attempts=0,next_check_at=IF(?,UTC_TIMESTAMP(3),next_check_at) WHERE sku_code=?`, s.ERPCost, s.AckRevision, s.ERPRevision, s.ProjectedRevision, s.Status, s.NeedsCheck, s.Reason, s.NeedsCheck, s.SKUCode)
	return err
}
func (r *costSyncRepo) Observe(ctx context.Context, sku string, value *float64) error {
	return r.withRow(ctx, sku, func(tx *sql.Tx, s *domain.CostSyncState, id, taskID int64) error {
		action := domain.DecideCostObservation(s, value)
		changed := !domain.EqualCost(s.ERPCost, value) || s.Status != "synced" || s.AckRevision != s.Revision
		if !domain.EqualCost(s.ERPCost, value) {
			s.ERPRevision++
		}
		s.ERPCost = value
		switch action {
		case "agree":
			if err := r.projectCanonical(ctx, tx, s, id, taskID, nil, "实时核对一致", "agree"); err != nil {
				return err
			}
			s.AckRevision = s.Revision
			s.Status = "synced"
			if value == nil {
				s.Status = "no_price"
			}
			s.NeedsCheck = false
			s.Reason = ""
		case "unchanged":
			s.NeedsCheck = s.Status == "pending" || s.Status == "retry"
		case "conflict":
			s.Status = "conflict"
			s.NeedsCheck = false
			s.Reason = "本地价格与ERP价格冲突，请确认保留哪一侧；不会自动覆盖人工价格"
			if s.LocalCost != nil && !s.LocalConfirmed {
				s.Reason = "本地报价尚未核定，请核对价格后确认；相同金额不代表报价已审核"
			} else if (s.LocalCost != nil && !domain.ValidObservedCost(s.LocalCost)) || (value != nil && !domain.ValidObservedCost(value)) {
				s.Reason = "存在无效成本（负数或超出范围），请核对后再同步"
			} else if value == nil {
				s.Reason = "ERP未提供可核对的成本，请确认商品建档和报价"
			} else if *value == 0 && !domain.EqualCost(s.LocalCost, value) {
				s.Reason = "ERP为零价，需确认是有效报价还是尚未填写；未自动覆盖系统成本"
			}
		case "accept_erp":
			if err := r.importERP(ctx, tx, s, id, taskID, nil, "ERP外部价格变更，按人工价保护", "erp"); err != nil {
				return err
			}
		}
		if err := persistCostState(ctx, tx, s); err != nil {
			return err
		}
		if changed && action != "unchanged" {
			return auditCost(ctx, tx, s, action, s.Reason, nil)
		}
		return nil
	})
}
func (r *costSyncRepo) importERP(ctx context.Context, tx *sql.Tx, s *domain.CostSyncState, id, taskID int64, actor interface{}, reason, origin string) error {
	if !domain.ValidObservedCost(s.ERPCost) {
		return fmt.Errorf("ERP成本无效，不能用于替换")
	}
	overrideActor := "erp_cost_sync"
	if origin == "local" {
		overrideActor = fmt.Sprint(actor)
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE task_sku_items SET cost_price=?,manual_cost_override=1,requires_manual_review=0,manual_cost_override_reason=?,override_actor=?,override_at=?,updated_at=? WHERE id=?`, s.ERPCost, reason, overrideActor, now, now, id); err != nil {
		return err
	}
	if err := tx.QueryRowContext(ctx, `SELECT revision FROM sku_cost_sync_states WHERE sku_code=?`, s.SKUCode).Scan(&s.Revision); err != nil {
		return err
	}
	s.LocalCost = s.ERPCost
	s.ManualLock = true
	s.ManualOrigin = origin
	s.LocalConfirmed = true
	s.AckRevision = s.Revision
	s.Status = "synced"
	s.NeedsCheck = false
	s.Reason = ""
	return r.projectCanonical(ctx, tx, s, id, taskID, actor, reason, origin)
}
func (r *costSyncRepo) projectCanonical(ctx context.Context, tx *sql.Tx, s *domain.CostSyncState, id, taskID int64, actor interface{}, reason, origin string) error {
	if s.ProjectedRevision == s.Revision {
		return nil
	}
	var changed bool
	if err := tx.QueryRowContext(ctx, `SELECT NOT EXISTS(SELECT 1 FROM omp_sku_records WHERE sku_code=? AND cost_price <=> ? AND manual_cost_override=?)`, s.SKUCode, s.LocalCost, s.ManualLock).Scan(&changed); err != nil {
		return err
	}
	now := time.Now().UTC()
	if _, err := tx.ExecContext(ctx, `UPDATE task_details d JOIN tasks t ON t.id=d.task_id JOIN task_sku_items s ON s.id=? SET d.cost_price=s.cost_price,d.estimated_cost=s.estimated_cost,d.manual_cost_override=s.manual_cost_override,d.requires_manual_review=s.requires_manual_review,d.manual_cost_override_reason=s.manual_cost_override_reason,d.override_actor=s.override_actor,d.override_at=s.override_at,d.updated_at=IF(?,?,d.updated_at) WHERE d.task_id=? AND (t.primary_sku_code=? OR t.sku_code=? OR t.batch_item_count=1)`, id, changed, now, taskID, s.SKUCode, s.SKUCode); err != nil {
		return err
	}
	result, err := tx.ExecContext(ctx, `INSERT INTO omp_sku_cost_snapshots(sku_code,task_id,task_sku_item_id,event_source,event_reason,operator_id,cost_price,cost_price_present,estimated_cost,estimated_cost_present,cost_rule_id,cost_rule_name,cost_rule_source,matched_rule_version,prefill_source,requires_manual_review,manual_cost_override,manual_cost_override_reason,input_snapshot_json,calculation_snapshot_json,created_at)
 SELECT sku_code,task_id,id,?, ?, ?,cost_price,cost_price IS NOT NULL,estimated_cost,estimated_cost IS NOT NULL,cost_rule_id,cost_rule_name,cost_rule_source,matched_rule_version,prefill_source,requires_manual_review,manual_cost_override,manual_cost_override_reason,JSON_OBJECT('source',?),JSON_OBJECT('cost_sync_revision',?,'erp_revision',?),? FROM task_sku_items WHERE id=?`, "cost_sync_"+origin, reason, actor, origin, s.Revision, s.ERPRevision, now, id)
	if err != nil {
		return err
	}
	snapshot, _ := result.LastInsertId()
	if _, err = tx.ExecContext(ctx, `INSERT INTO omp_sku_records(sku_code,last_task_id,last_task_sku_item_id,product_name,product_i_id,cost_price,estimated_cost,manual_cost_override,requires_manual_review,last_operator_id,created_at,updated_at)
 SELECT sku_code,task_id,id,product_name_snapshot,product_i_id,cost_price,estimated_cost,manual_cost_override,requires_manual_review,?,?,? FROM task_sku_items WHERE id=?
 ON DUPLICATE KEY UPDATE cost_price=VALUES(cost_price),estimated_cost=VALUES(estimated_cost),manual_cost_override=VALUES(manual_cost_override),requires_manual_review=VALUES(requires_manual_review),last_operator_id=VALUES(last_operator_id),trace_version=trace_version+1,updated_at=IF(?,VALUES(updated_at),omp_sku_records.updated_at)`, actor, now, now, id, changed); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE erp_product_sync_records SET cost_price=?,latest_cost_snapshot_id=?,updated_at=IF(?,?,updated_at) WHERE sku_code=?`, s.LocalCost, snapshot, changed, now, s.SKUCode); err != nil {
		return err
	}
	s.ProjectedRevision = s.Revision
	if !changed {
		return nil
	}
	_, err = NewTaskEventRepo(r.db).Append(ctx, &MySQLTx{tx: tx}, taskID, domain.TaskEventCostUpdated, nil, map[string]interface{}{"sku_code": s.SKUCode, "cost_price": s.LocalCost, "source": "cost_sync_" + origin, "revision": s.Revision, "manual_cost_override": s.ManualLock})
	return err
}
func (r *costSyncRepo) Acknowledge(ctx context.Context, sku string, revision int64, value *float64) error {
	return r.withRow(ctx, sku, func(tx *sql.Tx, s *domain.CostSyncState, id, taskID int64) error {
		action, reason := "outbound_ack", "ERP回读一致"
		if !domain.EqualCost(s.ERPCost, value) {
			s.ERPRevision++
		}
		s.ERPCost = value
		if s.Revision == revision && domain.EqualCost(s.LocalCost, value) {
			if err := r.projectCanonical(ctx, tx, s, id, taskID, nil, "ERP回读一致", "ack"); err != nil {
				return err
			}
			s.AckRevision = revision
			s.Status = "synced"
			s.NeedsCheck = false
			s.Reason = ""
		} else {
			action = "stale_ack"
			reason = fmt.Sprintf("回读版本%d；当前版本%d仍待同步", revision, s.Revision)
			s.NeedsCheck = true
			if s.Status != "conflict" {
				s.Status = "pending"
			}
		}
		if err := persistCostState(ctx, tx, s); err != nil {
			return err
		}
		return auditCost(ctx, tx, s, action, reason, nil)
	})
}
func (r *costSyncRepo) Fail(ctx context.Context, sku string, revision int64, message string) error {
	if len([]rune(message)) > 500 {
		message = string([]rune(message)[:500])
	}
	_, err := r.db.db.ExecContext(ctx, `UPDATE sku_cost_sync_states SET status=IF(status='conflict','conflict','retry'),needs_check=IF(status='conflict',0,1),reason=?,attempts=attempts+1,next_check_at=DATE_ADD(UTC_TIMESTAMP(3),INTERVAL LEAST(300,5*POW(2,LEAST(attempts,6))) SECOND),updated_at=UTC_TIMESTAMP(3) WHERE sku_code=? AND revision=?`, message, sku, revision)
	return err
}
func (r *costSyncRepo) Due(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.db.QueryContext(ctx, `SELECT sku_code FROM sku_cost_sync_states WHERE needs_check=1 AND status<>'baseline' AND next_check_at<=UTC_TIMESTAMP(3) ORDER BY next_check_at,sku_code LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var sku string
		if err := rows.Scan(&sku); err != nil {
			return nil, err
		}
		out = append(out, sku)
	}
	return out, rows.Err()
}
func (r *costSyncRepo) Collect(ctx context.Context) error {
	tx, err := r.db.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var cursor int64
	if err = tx.QueryRowContext(ctx, `SELECT event_id FROM sku_cost_sync_cursor WHERE name='erp' FOR UPDATE`).Scan(&cursor); err != nil {
		return err
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,sku_id,new_cost_price FROM jst_cost_changes WHERE id>? ORDER BY id LIMIT 500 FOR SHARE`, cursor)
	if err != nil {
		return err
	}
	type event struct {
		id   int64
		sku  string
		cost *float64
	}
	events := []event{}
	for rows.Next() {
		var e event
		if err = rows.Scan(&e.id, &e.sku, &e.cost); err != nil {
			rows.Close()
			return err
		}
		events = append(events, e)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, e := range events {
		if _, err = tx.ExecContext(ctx, `UPDATE sku_cost_sync_states SET needs_check=1,status=IF(status='baseline','observe',status),next_check_at=UTC_TIMESTAMP(3) WHERE sku_code=? AND NOT(erp_cost <=> ?)`, e.sku, e.cost); err != nil {
			return err
		}
		cursor = e.id
	}
	if _, err = tx.ExecContext(ctx, `UPDATE sku_cost_sync_cursor SET event_id=? WHERE name='erp'`, cursor); err != nil {
		return err
	}
	return tx.Commit()
}
func (r *costSyncRepo) BaselineDue(ctx context.Context, limit int) ([]string, error) {
	rows, err := r.db.db.QueryContext(ctx, `SELECT sku_code FROM sku_cost_sync_states WHERE needs_check=1 AND status='baseline' AND next_check_at<=UTC_TIMESTAMP(3) ORDER BY sku_code LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []string{}
	for rows.Next() {
		var sku string
		if err := rows.Scan(&sku); err != nil {
			return nil, err
		}
		out = append(out, sku)
	}
	return out, rows.Err()
}
func (r *costSyncRepo) Resolve(ctx context.Context, p domain.CostSyncResolution) error {
	var actor interface{}
	if p.ActorID > 0 {
		actor = p.ActorID
	}
	return r.withRow(ctx, p.SKUCode, func(tx *sql.Tx, s *domain.CostSyncState, id, taskID int64) error {
		if s.Revision != p.Revision || s.ERPRevision != p.ERPRevision {
			return domain.NewAppError(domain.ErrCodeConflict, "价格已变化，请刷新后重新确认", nil)
		}
		if p.Choice == "erp" {
			if err := r.importERP(ctx, tx, s, id, taskID, actor, p.Reason, "erp"); err != nil {
				return err
			}
		} else if p.Choice == "local" {
			if !domain.ValidObservedCost(s.LocalCost) {
				return fmt.Errorf("本地成本尚未确认")
			}
			observedERP := s.ERPCost
			s.ERPCost = s.LocalCost
			if err := r.importERP(ctx, tx, s, id, taskID, actor, p.Reason, "local"); err != nil {
				return err
			}
			s.ERPCost = observedERP
			s.AckRevision = s.Revision - 1
			s.Status = "pending"
			s.ManualLock = true
			s.NeedsCheck = true
			s.Reason = "已确认保留本地价，等待ERP回读"
		} else {
			return fmt.Errorf("请选择本地或ERP价格")
		}
		if s.Revision == p.Revision {
			if _, err := tx.ExecContext(ctx, `UPDATE sku_cost_sync_states SET revision=revision+1 WHERE sku_code=?`, s.SKUCode); err != nil {
				return err
			}
			s.Revision++
			if err := r.projectCanonical(ctx, tx, s, id, taskID, actor, p.Reason, "resolution"); err != nil {
				return err
			}
		}
		if p.Choice == "erp" {
			s.AckRevision = s.Revision
		} else {
			s.AckRevision = s.Revision - 1
		}
		if err := persistCostState(ctx, tx, s); err != nil {
			return err
		}
		return auditCost(ctx, tx, s, "resolve_"+p.Choice, strings.TrimSpace(p.Reason), actor)
	})
}
