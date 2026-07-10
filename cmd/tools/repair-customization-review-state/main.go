package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/domain"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

type runConfig struct {
	Apply  bool
	Limit  int
	TaskNo string
	Mode   string
}

const (
	repairModeReviewState           = "review-state"
	repairModeForcedCloseCorrection = "forced-close-correction"
)

type repairCandidate struct {
	TaskID       int64             `json:"task_id"`
	TaskNo       string            `json:"task_no"`
	TaskStatus   domain.TaskStatus `json:"task_status"`
	ReviewerID   int64             `json:"reviewer_id"`
	ReviewedAt   time.Time         `json:"reviewed_at"`
	VersionCount int64             `json:"version_count"`
}

type repairResult struct {
	Apply      bool              `json:"apply"`
	Discovered int               `json:"discovered"`
	Repaired   int               `json:"repaired"`
	Candidates []repairCandidate `json:"candidates"`
}

type customizationAssetFlowRepo interface {
	MarkCurrentDeliveryVersionsApprovedForTask(ctx context.Context, tx repo.Tx, taskID, actorID int64, approvedAt time.Time) (int64, error)
}

type moduleStateRepo interface {
	Enter(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, poolTeamCode *string, data json.RawMessage) (*domain.TaskModule, error)
	UpdateState(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string, state domain.ModuleState, terminal bool, data json.RawMessage) error
}

type moduleEventWriter interface {
	Insert(ctx context.Context, tx repo.Tx, event *domain.TaskModuleEvent) (int64, error)
}

type lockedRepairState struct {
	TaskNo       string
	TaskStatus   domain.TaskStatus
	ReviewerID   int64
	ReviewedAt   time.Time
	VersionCount int64
}

type repairStateLocker interface {
	LockAndRevalidate(ctx context.Context, tx repo.Tx, candidate repairCandidate) (*lockedRepairState, error)
	LockModule(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (*domain.TaskModule, error)
}

type forcedCloseCorrectionLocker interface {
	LockForcedCloseCorrection(ctx context.Context, tx repo.Tx, candidate repairCandidate) (*domain.TaskModule, error)
}

type mysqlRepairStateLocker struct{}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	var cfg runConfig
	flag.BoolVar(&cfg.Apply, "apply", false, "apply repairs; default is read-only dry-run")
	flag.IntVar(&cfg.Limit, "limit", 10, "maximum tasks to inspect in this run")
	flag.StringVar(&cfg.TaskNo, "task-no", "", "exact task number (required with --apply)")
	flag.StringVar(&cfg.Mode, "mode", repairModeReviewState, "repair mode: review-state or forced-close-correction")
	flag.Parse()
	cfg.TaskNo = strings.TrimSpace(cfg.TaskNo)
	cfg.Mode = strings.TrimSpace(cfg.Mode)
	if err := validateRunConfig(cfg); err != nil {
		return err
	}

	dsn := strings.TrimSpace(os.Getenv("MYSQL_DSN"))
	if dsn == "" {
		return errors.New("MYSQL_DSN is required")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("open mysql: %w", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("ping mysql: %w", err)
	}

	candidates, err := discoverCandidatesForMode(ctx, db, cfg.Mode, cfg.TaskNo, cfg.Limit)
	if err != nil {
		return err
	}
	result := repairResult{Apply: cfg.Apply, Discovered: len(candidates), Candidates: candidates}
	if cfg.Apply {
		// validateRunConfig requires an exact task number for every mutating run.
		if len(candidates) > 1 {
			return fmt.Errorf("exact task query returned %d candidates", len(candidates))
		}
		mdb := mysqlrepo.New(db)
		moduleRepo := mysqlrepo.NewTaskModuleRepo(mdb)
		moduleEventRepo := mysqlrepo.NewTaskModuleEventRepo(mdb)
		locker := mysqlRepairStateLocker{}
		for _, candidate := range candidates {
			var repairErr error
			if cfg.Mode == repairModeForcedCloseCorrection {
				repairErr = correctForcedCloseOne(ctx, mdb, locker, moduleRepo, moduleEventRepo, candidate)
			} else {
				assetRepo := mysqlrepo.NewTaskAssetRepo(mdb)
				flowRepo, ok := assetRepo.(customizationAssetFlowRepo)
				if !ok {
					return errors.New("task asset repository does not support review-state repair")
				}
				repairErr = repairOne(ctx, mdb, locker, flowRepo, moduleRepo, moduleEventRepo, candidate)
			}
			if repairErr != nil {
				return fmt.Errorf("repair task %s: %w", candidate.TaskNo, repairErr)
			}
			result.Repaired++
		}
	}

	encoded, err := json.MarshalIndent(result, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(encoded))
	return nil
}

func validateRunConfig(cfg runConfig) error {
	if cfg.Limit <= 0 || cfg.Limit > 100 {
		return errors.New("--limit must be between 1 and 100")
	}
	if cfg.Apply && strings.TrimSpace(cfg.TaskNo) == "" {
		return errors.New("--apply requires an exact --task-no")
	}
	if cfg.Mode != repairModeReviewState && cfg.Mode != repairModeForcedCloseCorrection {
		return fmt.Errorf("--mode must be %q or %q", repairModeReviewState, repairModeForcedCloseCorrection)
	}
	return nil
}

func discoverCandidatesForMode(ctx context.Context, db *sql.DB, mode, taskNo string, limit int) ([]repairCandidate, error) {
	if mode == repairModeForcedCloseCorrection {
		return discoverForcedCloseCandidates(ctx, db, taskNo, limit)
	}
	return discoverCandidates(ctx, db, taskNo, limit)
}

func discoverCandidates(ctx context.Context, db *sql.DB, taskNo string, limit int) ([]repairCandidate, error) {
	query := candidateDiscoveryQuery(taskNo != "")
	args := make([]interface{}, 0, 2)
	if taskNo != "" {
		args = append(args, taskNo)
	}
	args = append(args, limit)

	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("discover repair candidates: %w", err)
	}
	defer rows.Close()
	out := make([]repairCandidate, 0)
	for rows.Next() {
		var candidate repairCandidate
		if err := rows.Scan(&candidate.TaskID, &candidate.TaskNo, &candidate.TaskStatus, &candidate.ReviewerID, &candidate.ReviewedAt, &candidate.VersionCount); err != nil {
			return nil, fmt.Errorf("scan repair candidate: %w", err)
		}
		if candidate.ReviewerID <= 0 {
			return nil, fmt.Errorf("candidate %s has no reviewer actor", candidate.TaskNo)
		}
		out = append(out, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate repair candidates: %w", err)
	}
	return out, nil
}

func candidateDiscoveryQuery(filterTaskNo bool) string {
	query := `
SELECT t.id,
       t.task_no,
       t.task_status,
       tel.operator_id,
       tel.created_at,
       COUNT(*) AS version_count
  FROM tasks t
  JOIN customization_jobs cj
    ON cj.id = (
       SELECT cj2.id
         FROM customization_jobs cj2
        WHERE cj2.task_id = t.id
        ORDER BY cj2.id DESC
        LIMIT 1
   )
  JOIN task_event_logs tel
    ON tel.id = (
       SELECT tel2.id
         FROM task_event_logs tel2
        WHERE tel2.task_id = t.id
          AND tel2.event_type = 'task.customization.reviewed'
        ORDER BY tel2.sequence DESC
        LIMIT 1
   )
  JOIN task_assets ta
    ON ta.task_id = t.id
   AND ta.asset_type = 'delivery'
	   AND ta.flow_review_status = 'pending_review'
	   AND COALESCE(ta.is_archived, 0) = 0
   AND ta.deleted_at IS NULL
   AND ta.cleaned_at IS NULL
   AND ta.created_at <= tel.created_at
  JOIN design_assets da
    ON da.id = ta.asset_id
   AND da.current_version_id = ta.id
 WHERE t.customization_required = 1
   AND t.task_status IN ('PendingWarehouseReceive', 'PendingProductionTransfer', 'PendingClose', 'Completed')
   AND cj.customization_review_decision IN ('approved', 'reviewer_fixed')
   AND JSON_UNQUOTE(JSON_EXTRACT(tel.payload, '$.customization_review_decision')) IN ('approved', 'reviewer_fixed')
   AND tel.operator_id IS NOT NULL
   AND tel.operator_id > 0`
	if filterTaskNo {
		query += " AND t.task_no = ?"
	}
	query += `
 GROUP BY t.id, t.task_no, t.task_status, tel.operator_id, tel.created_at
 ORDER BY t.id ASC
 LIMIT ?`
	return query
}

func discoverForcedCloseCandidates(ctx context.Context, db *sql.DB, taskNo string, limit int) ([]repairCandidate, error) {
	query := forcedCloseCandidateDiscoveryQuery(taskNo != "")
	args := make([]interface{}, 0, 2)
	if taskNo != "" {
		args = append(args, taskNo)
	}
	args = append(args, limit)
	rows, err := db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("discover forced-close correction candidates: %w", err)
	}
	defer rows.Close()
	out := make([]repairCandidate, 0)
	for rows.Next() {
		var candidate repairCandidate
		if err := rows.Scan(&candidate.TaskID, &candidate.TaskNo, &candidate.TaskStatus, &candidate.ReviewerID, &candidate.ReviewedAt, &candidate.VersionCount); err != nil {
			return nil, fmt.Errorf("scan forced-close correction candidate: %w", err)
		}
		out = append(out, candidate)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate forced-close correction candidates: %w", err)
	}
	return out, nil
}

func forcedCloseCandidateDiscoveryQuery(filterTaskNo bool) string {
	query := `
SELECT t.id,
       t.task_no,
       t.task_status,
       COALESCE(tme.actor_id, CAST(JSON_UNQUOTE(JSON_EXTRACT(tme.payload, '$.historical_reviewer_id')) AS UNSIGNED), 0) AS reviewer_id,
       tme.created_at AS reviewed_at,
       0 AS version_count
  FROM tasks t
  JOIN task_modules tm
    ON tm.task_id = t.id
   AND tm.module_key = 'design'
   AND tm.state = 'forcibly_closed'
  JOIN task_module_events tme
    ON tme.id = (
       SELECT tme2.id
         FROM task_module_events tme2
         JOIN task_modules tm2 ON tm2.id = tme2.task_module_id
        WHERE tm2.task_id = t.id
          AND tm2.module_key = 'design'
          AND tme2.event_type = 'forcibly_closed'
          AND tme2.to_state = 'forcibly_closed'
          AND JSON_UNQUOTE(JSON_EXTRACT(tme2.payload, '$.source')) = 'repair_customization_review_state'
        ORDER BY tme2.id DESC
        LIMIT 1
   )
 WHERE t.customization_required = 1
   AND t.task_status IN ('PendingWarehouseReceive', 'PendingProductionTransfer', 'PendingClose', 'Completed')`
	if filterTaskNo {
		query += " AND t.task_no = ?"
	}
	query += `
 ORDER BY t.id ASC
 LIMIT ?`
	return query
}

func (mysqlRepairStateLocker) LockAndRevalidate(ctx context.Context, tx repo.Tx, candidate repairCandidate) (*lockedRepairState, error) {
	sqlTx := mysqlrepo.Unwrap(tx)
	var state lockedRepairState
	var customizationRequired bool
	if err := sqlTx.QueryRowContext(ctx, `
SELECT task_no, task_status, customization_required
  FROM tasks
 WHERE id = ?
 FOR UPDATE`, candidate.TaskID).Scan(&state.TaskNo, &state.TaskStatus, &customizationRequired); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task %d no longer exists", candidate.TaskID)
		}
		return nil, fmt.Errorf("lock repair task: %w", err)
	}
	if state.TaskNo != candidate.TaskNo {
		return nil, fmt.Errorf("task number changed from %q to %q", candidate.TaskNo, state.TaskNo)
	}
	if !customizationRequired {
		return nil, errors.New("task is no longer customization_required")
	}
	if !repairableTaskStatus(state.TaskStatus) {
		return nil, fmt.Errorf("task status %s is no longer repairable", state.TaskStatus)
	}
	if state.TaskStatus != candidate.TaskStatus {
		return nil, fmt.Errorf("task status changed from %s to %s after discovery", candidate.TaskStatus, state.TaskStatus)
	}

	var jobDecision string
	if err := sqlTx.QueryRowContext(ctx, `
SELECT customization_review_decision
  FROM customization_jobs
 WHERE task_id = ?
 ORDER BY id DESC
 LIMIT 1
 FOR UPDATE`, candidate.TaskID).Scan(&jobDecision); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("latest customization job is missing")
		}
		return nil, fmt.Errorf("lock latest customization job: %w", err)
	}
	if !approvedCustomizationDecision(jobDecision) {
		return nil, fmt.Errorf("latest customization job decision is %q, not approved", jobDecision)
	}

	var eventDecision sql.NullString
	var reviewerID sql.NullInt64
	if err := sqlTx.QueryRowContext(ctx, `
SELECT JSON_UNQUOTE(JSON_EXTRACT(payload, '$.customization_review_decision')),
       operator_id,
       created_at
  FROM task_event_logs
 WHERE task_id = ?
   AND event_type = 'task.customization.reviewed'
 ORDER BY sequence DESC
 LIMIT 1
 FOR UPDATE`, candidate.TaskID).Scan(&eventDecision, &reviewerID, &state.ReviewedAt); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("latest customization review event is missing")
		}
		return nil, fmt.Errorf("lock latest customization review event: %w", err)
	}
	if !eventDecision.Valid || !approvedCustomizationDecision(eventDecision.String) {
		return nil, fmt.Errorf("latest customization review event decision is %q, not approved", eventDecision.String)
	}
	if !reviewerID.Valid || reviewerID.Int64 <= 0 {
		return nil, errors.New("latest customization review event has no reviewer actor")
	}
	state.ReviewerID = reviewerID.Int64
	if state.ReviewerID != candidate.ReviewerID || !state.ReviewedAt.Equal(candidate.ReviewedAt) {
		return nil, errors.New("latest customization review event changed after discovery")
	}

	rows, err := sqlTx.QueryContext(ctx, `
SELECT ta.id, ta.flow_review_status, ta.created_at
  FROM task_assets ta
  JOIN design_assets da
    ON da.id = ta.asset_id
   AND da.current_version_id = ta.id
 WHERE ta.task_id = ?
	   AND ta.asset_type = 'delivery'
	   AND COALESCE(ta.is_archived, 0) = 0
   AND ta.deleted_at IS NULL
   AND ta.cleaned_at IS NULL
 ORDER BY ta.id
 FOR UPDATE`, candidate.TaskID)
	if err != nil {
		return nil, fmt.Errorf("lock current delivery versions: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var versionID int64
		var reviewStatus string
		var createdAt time.Time
		if err := rows.Scan(&versionID, &reviewStatus, &createdAt); err != nil {
			return nil, fmt.Errorf("scan locked current delivery version: %w", err)
		}
		if reviewStatus != string(domain.TaskAssetFlowReviewStatusPendingReview) {
			return nil, fmt.Errorf("current delivery version %d has status %q, want pending_review", versionID, reviewStatus)
		}
		if createdAt.After(state.ReviewedAt) {
			return nil, fmt.Errorf("current delivery version %d was created after the approved review", versionID)
		}
		state.VersionCount++
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate locked current delivery versions: %w", err)
	}
	if state.VersionCount == 0 {
		return nil, errors.New("no current delivery version remains eligible for repair")
	}
	if state.VersionCount != candidate.VersionCount {
		return nil, fmt.Errorf("current delivery version count changed from %d to %d after discovery", candidate.VersionCount, state.VersionCount)
	}
	return &state, nil
}

func (mysqlRepairStateLocker) LockModule(ctx context.Context, tx repo.Tx, taskID int64, moduleKey string) (*domain.TaskModule, error) {
	var module domain.TaskModule
	err := mysqlrepo.Unwrap(tx).QueryRowContext(ctx, `
SELECT id, task_id, module_key, state
  FROM task_modules
 WHERE task_id = ?
   AND module_key = ?
 FOR UPDATE`, taskID, moduleKey).Scan(&module.ID, &module.TaskID, &module.ModuleKey, &module.State)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("lock %s task module: %w", moduleKey, err)
	}
	return &module, nil
}

func (locker mysqlRepairStateLocker) LockForcedCloseCorrection(ctx context.Context, tx repo.Tx, candidate repairCandidate) (*domain.TaskModule, error) {
	sqlTx := mysqlrepo.Unwrap(tx)
	var taskNo string
	var taskStatus domain.TaskStatus
	var customizationRequired bool
	if err := sqlTx.QueryRowContext(ctx, `
SELECT task_no, task_status, customization_required
  FROM tasks
 WHERE id = ?
 FOR UPDATE`, candidate.TaskID).Scan(&taskNo, &taskStatus, &customizationRequired); err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task %d no longer exists", candidate.TaskID)
		}
		return nil, fmt.Errorf("lock forced-close correction task: %w", err)
	}
	if taskNo != candidate.TaskNo || taskStatus != candidate.TaskStatus || !customizationRequired || !repairableTaskStatus(taskStatus) {
		return nil, errors.New("forced-close correction candidate changed after discovery")
	}
	module, err := locker.LockModule(ctx, tx, candidate.TaskID, domain.ModuleKeyDesign)
	if err != nil {
		return nil, err
	}
	if module == nil {
		return nil, errors.New("design task module is missing")
	}
	if module.State != domain.ModuleStateForciblyClosed {
		return nil, fmt.Errorf("design module state is %s, want forcibly_closed", module.State)
	}
	var provenanceEventID int64
	if err := sqlTx.QueryRowContext(ctx, `
SELECT tme.id
  FROM task_module_events tme
  JOIN task_modules tm ON tm.id = tme.task_module_id
 WHERE tm.task_id = ?
   AND tm.module_key = 'design'
   AND tme.event_type = 'forcibly_closed'
   AND tme.to_state = 'forcibly_closed'
   AND JSON_UNQUOTE(JSON_EXTRACT(tme.payload, '$.source')) = 'repair_customization_review_state'
 ORDER BY tme.id DESC
 LIMIT 1
 FOR UPDATE`, candidate.TaskID).Scan(&provenanceEventID); err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("design forcibly_closed state has no matching repair-tool provenance event")
		}
		return nil, fmt.Errorf("lock forced-close provenance event: %w", err)
	}
	return module, nil
}

func approvedCustomizationDecision(decision string) bool {
	switch domain.CustomizationReviewDecision(strings.TrimSpace(decision)) {
	case domain.CustomizationReviewDecisionApproved, domain.CustomizationReviewDecisionReviewerFixed:
		return true
	default:
		return false
	}
}

func repairableTaskStatus(status domain.TaskStatus) bool {
	switch status {
	case domain.TaskStatusPendingWarehouseReceive,
		domain.TaskStatusPendingProductionTransfer,
		domain.TaskStatusPendingClose,
		domain.TaskStatusCompleted:
		return true
	default:
		return false
	}
}

func repairOne(ctx context.Context, txRunner repo.TxRunner, locker repairStateLocker, flowRepo customizationAssetFlowRepo, moduleRepo moduleStateRepo, moduleEventRepo moduleEventWriter, candidate repairCandidate) error {
	return txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		locked, err := locker.LockAndRevalidate(ctx, tx, candidate)
		if err != nil {
			return err
		}
		candidate.TaskNo = locked.TaskNo
		candidate.TaskStatus = locked.TaskStatus
		candidate.ReviewerID = locked.ReviewerID
		candidate.ReviewedAt = locked.ReviewedAt.UTC()
		candidate.VersionCount = locked.VersionCount

		updated, err := flowRepo.MarkCurrentDeliveryVersionsApprovedForTask(ctx, tx, candidate.TaskID, candidate.ReviewerID, candidate.ReviewedAt)
		if err != nil {
			return err
		}
		if updated != candidate.VersionCount {
			return fmt.Errorf("approved current delivery version count %d does not match locked candidate count %d", updated, candidate.VersionCount)
		}

		payload, err := json.Marshal(map[string]interface{}{
			"source":                 "repair_customization_review_state",
			"repair_actor":           "system_tool",
			"task_no":                candidate.TaskNo,
			"task_status":            candidate.TaskStatus,
			"historical_reviewer_id": candidate.ReviewerID,
			"historical_reviewed_at": candidate.ReviewedAt.Format(time.RFC3339),
		})
		if err != nil {
			return fmt.Errorf("encode repair module event payload: %w", err)
		}
		if err := transitionModule(ctx, tx, locker, moduleRepo, moduleEventRepo, candidate, domain.ModuleKeyDesign, domain.ModuleStateClosed, domain.ModuleEventClosed, payload); err != nil {
			return err
		}
		if err := transitionModule(ctx, tx, locker, moduleRepo, moduleEventRepo, candidate, domain.ModuleKeyCustomization, domain.ModuleStateClosed, domain.ModuleEventClosed, payload); err != nil {
			return err
		}
		if err := transitionModule(ctx, tx, locker, moduleRepo, moduleEventRepo, candidate, domain.ModuleKeyAudit, domain.ModuleStateClosed, domain.ModuleEventApproved, payload); err != nil {
			return err
		}

		warehouseState, eventType := warehouseModuleTarget(candidate.TaskStatus)
		if warehouseState == domain.ModuleStatePending {
			return syncWarehousePending(ctx, tx, locker, moduleRepo, moduleEventRepo, candidate, payload)
		}
		return transitionModule(ctx, tx, locker, moduleRepo, moduleEventRepo, candidate, domain.ModuleKeyWarehouse, warehouseState, eventType, payload)
	})
}

func correctForcedCloseOne(ctx context.Context, txRunner repo.TxRunner, locker forcedCloseCorrectionLocker, moduleRepo moduleStateRepo, eventRepo moduleEventWriter, candidate repairCandidate) error {
	return txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		module, err := locker.LockForcedCloseCorrection(ctx, tx, candidate)
		if err != nil {
			return err
		}
		from := module.State
		to := domain.ModuleStateClosed
		if err := moduleRepo.UpdateState(ctx, tx, candidate.TaskID, domain.ModuleKeyDesign, to, true, nil); err != nil {
			return err
		}
		payload, err := json.Marshal(map[string]interface{}{
			"source":      "repair_customization_review_forced_close_correction",
			"task_no":     candidate.TaskNo,
			"task_status": candidate.TaskStatus,
			"reason":      "forcibly_closed is reserved for cancelled or blocked workflows",
		})
		if err != nil {
			return fmt.Errorf("encode forced-close correction payload: %w", err)
		}
		_, err = eventRepo.Insert(ctx, tx, &domain.TaskModuleEvent{
			TaskID:       candidate.TaskID,
			TaskModuleID: module.ID,
			ModuleKey:    domain.ModuleKeyDesign,
			EventType:    domain.ModuleEventClosed,
			FromState:    &from,
			ToState:      &to,
			ActorID:      nil,
			Payload:      payload,
		})
		return err
	})
}

func warehouseModuleTarget(status domain.TaskStatus) (domain.ModuleState, domain.ModuleEventType) {
	switch status {
	case domain.TaskStatusPendingWarehouseReceive:
		return domain.ModuleStatePending, domain.ModuleEventEntered
	case domain.TaskStatusPendingProductionTransfer:
		return domain.ModuleStateReceived, domain.ModuleEventReceived
	case domain.TaskStatusPendingClose, domain.TaskStatusCompleted:
		return domain.ModuleStateCompleted, domain.ModuleEventCompleted
	default:
		return domain.ModuleStatePending, domain.ModuleEventEntered
	}
}

func transitionModule(ctx context.Context, tx repo.Tx, locker repairStateLocker, moduleRepo moduleStateRepo, eventRepo moduleEventWriter, candidate repairCandidate, moduleKey string, next domain.ModuleState, eventType domain.ModuleEventType, payload json.RawMessage) error {
	module, err := locker.LockModule(ctx, tx, candidate.TaskID, moduleKey)
	if err != nil {
		return err
	}
	if module == nil {
		return fmt.Errorf("required %s task module is missing", moduleKey)
	}
	return transitionLockedModule(ctx, tx, moduleRepo, eventRepo, candidate, module, next, eventType, payload)
}

func transitionLockedModule(ctx context.Context, tx repo.Tx, moduleRepo moduleStateRepo, eventRepo moduleEventWriter, candidate repairCandidate, module *domain.TaskModule, next domain.ModuleState, eventType domain.ModuleEventType, payload json.RawMessage) error {
	if module.State == next {
		return nil
	}
	if module.State.Terminal() {
		return fmt.Errorf("module %s is terminal in state %s; refusing transition to %s", module.ModuleKey, module.State, next)
	}
	if !repairModuleTransitionAllowed(module.ModuleKey, module.State, next) {
		return fmt.Errorf("module %s transition %s -> %s is outside the repair projection", module.ModuleKey, module.State, next)
	}
	from := module.State
	if err := moduleRepo.UpdateState(ctx, tx, candidate.TaskID, module.ModuleKey, next, next.Terminal(), nil); err != nil {
		return err
	}
	_, err := eventRepo.Insert(ctx, tx, &domain.TaskModuleEvent{
		TaskID:       candidate.TaskID,
		TaskModuleID: module.ID,
		ModuleKey:    module.ModuleKey,
		EventType:    eventType,
		FromState:    &from,
		ToState:      &next,
		ActorID:      nil,
		Payload:      payload,
	})
	return err
}

func repairModuleTransitionAllowed(moduleKey string, from, next domain.ModuleState) bool {
	switch moduleKey {
	case domain.ModuleKeyDesign:
		return next == domain.ModuleStateClosed
	case domain.ModuleKeyCustomization:
		return next == domain.ModuleStateClosed &&
			(from == domain.ModuleStatePendingClaim || from == domain.ModuleStateInProgress || from == domain.ModuleStateSubmitted || from == domain.ModuleStateRejected)
	case domain.ModuleKeyAudit:
		return next == domain.ModuleStateClosed &&
			(from == domain.ModuleStatePendingClaim || from == domain.ModuleStateInProgress || from == domain.ModuleStateRejected)
	case domain.ModuleKeyWarehouse:
		switch next {
		case domain.ModuleStatePending:
			return from == domain.ModuleStatePendingClaim || from == domain.ModuleStateRejected
		case domain.ModuleStateReceived:
			return from == domain.ModuleStatePending || from == domain.ModuleStatePendingClaim || from == domain.ModuleStatePreparing
		case domain.ModuleStateCompleted:
			return from == domain.ModuleStatePending || from == domain.ModuleStatePendingClaim || from == domain.ModuleStatePreparing || from == domain.ModuleStateReceived
		}
	}
	return false
}

func syncWarehousePending(ctx context.Context, tx repo.Tx, locker repairStateLocker, moduleRepo moduleStateRepo, eventRepo moduleEventWriter, candidate repairCandidate, payload json.RawMessage) error {
	module, err := locker.LockModule(ctx, tx, candidate.TaskID, domain.ModuleKeyWarehouse)
	if err != nil {
		return err
	}
	if module != nil {
		return transitionLockedModule(ctx, tx, moduleRepo, eventRepo, candidate, module, domain.ModuleStatePending, domain.ModuleEventEntered, payload)
	}

	module, err = moduleRepo.Enter(ctx, tx, candidate.TaskID, domain.ModuleKeyWarehouse, domain.ModuleStatePending, nil, json.RawMessage(`{}`))
	if err != nil {
		return err
	}
	if module == nil {
		return errors.New("enter warehouse task module returned nil")
	}
	to := domain.ModuleStatePending
	_, err = eventRepo.Insert(ctx, tx, &domain.TaskModuleEvent{
		TaskID:       candidate.TaskID,
		TaskModuleID: module.ID,
		ModuleKey:    domain.ModuleKeyWarehouse,
		EventType:    domain.ModuleEventEntered,
		ToState:      &to,
		ActorID:      nil,
		Payload:      payload,
	})
	return err
}
