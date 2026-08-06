package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"

	"workflow/config"
)

type candidate struct {
	TaskID              int64  `json:"task_id"`
	TaskNo              string `json:"task_no"`
	TaskType            string `json:"task_type"`
	TaskStatus          string `json:"task_status"`
	IsBatchTask         bool   `json:"is_batch_task,omitempty"`
	TaskSKUCode         string `json:"task_sku_code,omitempty"`
	TaskProductName     string `json:"task_product_name,omitempty"`
	DetailCategory      string `json:"detail_category,omitempty"`
	DetailCategoryName  string `json:"detail_category_name,omitempty"`
	ItemID              int64  `json:"task_sku_item_id"`
	SKUCode             string `json:"sku_code"`
	ItemProductName     string `json:"item_product_name,omitempty"`
	CurrentIID          string `json:"current_i_id,omitempty"`
	VariantIID          string `json:"variant_i_id,omitempty"`
	ProductIID          string `json:"product_i_id,omitempty"`
	SyncRecordIID       string `json:"sync_record_i_id,omitempty"`
	CategoryCode        string `json:"category_code,omitempty"`
	CategoryExactIID    string `json:"category_exact_i_id,omitempty"`
	PlanningRevisionID  int64  `json:"planning_revision_id,omitempty"`
	PlanningIID         string `json:"planning_i_id,omitempty"`
	PlanningName        string `json:"planning_name,omitempty"`
	PlanningDescription string `json:"planning_description,omitempty"`
	ImageRefID          string `json:"image_ref_id,omitempty"`
	HasSyncedRecord     bool   `json:"has_synced_record,omitempty"`
	ResolvedIID         string `json:"resolved_i_id,omitempty"`
	ResolutionSource    string `json:"resolution_source,omitempty"`
	BlockReason         string `json:"block_reason,omitempty"`
}

type taskPlan struct {
	TaskID         int64        `json:"task_id"`
	TaskNo         string       `json:"task_no"`
	TaskType       string       `json:"task_type"`
	RecoveryAction string       `json:"recovery_action,omitempty"`
	Items          []*candidate `json:"items"`
	Blocked        bool         `json:"blocked"`
}

type report struct {
	RunID                  string      `json:"run_id"`
	Database               string      `json:"database"`
	DryRun                 bool        `json:"dry_run"`
	RecoverableTasks       int         `json:"recoverable_tasks"`
	RecoverableItems       int         `json:"recoverable_items"`
	TerminalExcludedTasks  int         `json:"terminal_excluded_tasks"`
	TerminalExcludedItems  int         `json:"terminal_excluded_items"`
	BlockedTasks           int         `json:"blocked_tasks"`
	BlockedItems           int         `json:"blocked_items"`
	UpdatedItemIIDs        int64       `json:"updated_item_iids"`
	ReconciledTasks        int64       `json:"reconciled_tasks"`
	ReconciledItems        int64       `json:"reconciled_items"`
	QueuedTaskFilingJobs   int64       `json:"queued_task_filing_jobs"`
	QueuedPlanningSKUJobs  int64       `json:"queued_planning_sku_jobs"`
	Blocked                []*taskPlan `json:"blocked"`
	GeneratedAtUTC         time.Time   `json:"generated_at_utc"`
	ProductionSafetyNotice string      `json:"production_safety_notice"`
}

func main() {
	var dsn, confirmDatabase, runID string
	var apply bool
	var actorID int64
	var timeout time.Duration
	flag.StringVar(&dsn, "dsn", "", "MySQL DSN; defaults to config MySQL DSN")
	flag.BoolVar(&apply, "apply", false, "repair deterministic i_id values, reconcile proven synced projections, and enqueue durable ERP jobs")
	flag.StringVar(&confirmDatabase, "confirm-database", "", "required with --apply; must equal SELECT DATABASE()")
	flag.StringVar(&runID, "run-id", "", "immutable recovery run identifier; defaults to UTC timestamp")
	flag.Int64Var(&actorID, "actor-id", 1, "operator id recorded in recovery task_filing payload")
	flag.DurationVar(&timeout, "timeout", 5*time.Minute, "whole run timeout")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		exitError("load config", err)
	}
	if strings.TrimSpace(dsn) == "" {
		dsn = cfg.MySQL.DSN
	}
	if strings.TrimSpace(dsn) == "" {
		exitError("validate config", fmt.Errorf("mysql dsn is required"))
	}
	if strings.TrimSpace(runID) == "" {
		runID = "sku-sync-recovery-" + time.Now().UTC().Format("20060102T150405Z")
	}
	if apply && (strings.TrimSpace(confirmDatabase) == "" || actorID <= 0) {
		exitError("validate apply guard", fmt.Errorf("--confirm-database and a positive --actor-id are required with --apply"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		exitError("open mysql", err)
	}
	defer db.Close()
	if err := db.PingContext(ctx); err != nil {
		exitError("ping mysql", err)
	}
	var database string
	if err := db.QueryRowContext(ctx, `SELECT DATABASE()`).Scan(&database); err != nil {
		exitError("read database", err)
	}
	if apply && database != strings.TrimSpace(confirmDatabase) {
		exitError("validate database guard", fmt.Errorf("connected database %q does not match confirmation %q", database, confirmDatabase))
	}

	candidates, err := loadCandidates(ctx, db)
	if err != nil {
		exitError("load recovery candidates", err)
	}
	plans, excludedTasks, excludedItems := buildPlans(candidates)
	out := report{
		RunID: runID, Database: database, DryRun: !apply, TerminalExcludedTasks: excludedTasks,
		TerminalExcludedItems: excludedItems, GeneratedAtUTC: time.Now().UTC(),
		ProductionSafetyNotice: "Cancelled and Archived tasks are excluded. Any task with one unresolved required SKU is blocked atomically. No ERP API is called by this tool; exact locally synced ERP records may close their matching projections, while all remaining ERP writes are queued through the durable outbox.",
	}
	for _, plan := range plans {
		if plan.Blocked {
			out.BlockedTasks++
			out.BlockedItems += len(plan.Items)
			out.Blocked = append(out.Blocked, plan)
			continue
		}
		out.RecoverableTasks++
		out.RecoverableItems += len(plan.Items)
	}
	if apply && out.RecoverableTasks > 0 {
		if err := applyPlans(ctx, db, plans, runID, actorID, &out); err != nil {
			exitError("apply recovery", err)
		}
	}
	if err := json.NewEncoder(os.Stdout).Encode(out); err != nil {
		exitError("encode report", err)
	}
}

func loadCandidates(ctx context.Context, db *sql.DB) ([]*candidate, error) {
	rows, err := db.QueryContext(ctx, `
		SELECT t.id, t.task_no, t.task_type, t.task_status,
		       t.is_batch_task, COALESCE(t.sku_code, ''), COALESCE(t.product_name_snapshot, ''),
		       COALESCE(td.category, ''), COALESCE(td.category_name, ''),
		       tsi.id, tsi.sku_code, COALESCE(tsi.product_name_snapshot, ''),
		       COALESCE(tsi.product_i_id, ''),
		       COALESCE(JSON_UNQUOTE(JSON_EXTRACT(tsi.variant_json, '$.product_i_id')), ''),
		       COALESCE((
		         SELECT NULLIF(TRIM(p.i_id_gen), '') FROM products p
		          WHERE p.sku_code = tsi.sku_code AND NULLIF(TRIM(p.i_id_gen), '') IS NOT NULL
		          ORDER BY p.id DESC LIMIT 1
		       ), ''),
		       COALESCE((
		         SELECT COALESCE(NULLIF(TRIM(r.erp_i_id), ''), NULLIF(TRIM(r.product_i_id), ''))
		           FROM erp_product_sync_records r
		          WHERE r.sku_code = tsi.sku_code
		            AND COALESCE(NULLIF(TRIM(r.erp_i_id), ''), NULLIF(TRIM(r.product_i_id), '')) IS NOT NULL
		          ORDER BY r.id DESC LIMIT 1
		       ), ''),
		       COALESCE(tsi.category_code, ''),
		       COALESCE((
		         SELECT p.i_id_gen FROM products p
		          WHERE LOWER(TRIM(p.i_id_gen)) = LOWER(TRIM(tsi.category_code))
		          ORDER BY p.id DESC LIMIT 1
		       ), ''),
		       COALESCE(rev.id, 0), COALESCE(rev.erp_product_i_id, ''),
		       COALESCE(rev.erp_product_name, ''), COALESCE(rev.description_spec, ''),
		       COALESCE((
		         SELECT image.storage_ref_id
		           FROM task_planning_sku_revision_images image
		          WHERE image.revision_id = rev.id
		          LIMIT 1
		       ), ''),
		       EXISTS (
		         SELECT 1
		           FROM erp_product_sync_records synced
		          WHERE synced.task_id = t.id
		            AND synced.task_sku_item_id = tsi.id
		            AND synced.base_sync_status = 'synced'
		       )
		  FROM task_sku_items tsi
		  JOIN tasks t ON t.id = tsi.task_id
		  LEFT JOIN task_details td ON td.task_id = t.id
		  LEFT JOIN task_planning_sku_details pd ON pd.task_sku_item_id = tsi.id
		  LEFT JOIN task_planning_sku_revisions rev ON rev.id = pd.current_revision_id
		 WHERE tsi.erp_sync_required = 1
		   AND (tsi.erp_sync_status <> 'filed' OR tsi.filing_status <> 'filed')
		 ORDER BY t.id, tsi.sequence_no, tsi.id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]*candidate, 0)
	for rows.Next() {
		item := &candidate{}
		if err := rows.Scan(
			&item.TaskID, &item.TaskNo, &item.TaskType, &item.TaskStatus,
			&item.IsBatchTask, &item.TaskSKUCode, &item.TaskProductName,
			&item.DetailCategory, &item.DetailCategoryName,
			&item.ItemID, &item.SKUCode, &item.ItemProductName,
			&item.CurrentIID, &item.VariantIID,
			&item.ProductIID, &item.SyncRecordIID, &item.CategoryCode, &item.CategoryExactIID,
			&item.PlanningRevisionID, &item.PlanningIID, &item.PlanningName, &item.PlanningDescription, &item.ImageRefID,
			&item.HasSyncedRecord,
		); err != nil {
			return nil, err
		}
		if item.TaskType == "sku_planning" {
			item.PlanningName = strings.TrimSpace(item.PlanningName)
			if item.PlanningName == "" {
				item.PlanningName = strings.TrimSpace(item.PlanningDescription)
			}
			// 聚水潭商品名称/简称上限为 40 个 Unicode 字符；完整的冻结
			// planning revision 仍保留在本地，只收敛异步 ERP 载荷。
			item.PlanningName = truncateRunes(item.PlanningName, 40)
		}
		item.ResolvedIID, item.ResolutionSource = resolveIID(item)
		out = append(out, item)
	}
	return out, rows.Err()
}

func truncateRunes(value string, limit int) string {
	runes := []rune(value)
	if limit <= 0 || len(runes) <= limit {
		return value
	}
	return string(runes[:limit])
}

func resolveIID(item *candidate) (string, string) {
	for _, source := range []struct {
		value, name string
	}{
		{item.CurrentIID, "task_sku_item.product_i_id"},
		{item.VariantIID, "task_sku_item.variant_json"},
		{item.PlanningIID, "planning_revision.erp_product_i_id"},
		{item.ProductIID, "products.sku_code_exact"},
		{item.SyncRecordIID, "erp_sync_record.sku_code_exact"},
		{item.CategoryExactIID, "category_code_exact_existing_erp_iid"},
	} {
		if value := strings.TrimSpace(source.value); value != "" {
			return value, source.name
		}
	}
	return "", ""
}

func buildPlans(items []*candidate) ([]*taskPlan, int, int) {
	byTask := make(map[int64]*taskPlan)
	excluded := make(map[int64]struct{})
	excludedItems := 0
	for _, item := range items {
		if item.TaskStatus == "Cancelled" || item.TaskStatus == "Archived" {
			excluded[item.TaskID] = struct{}{}
			excludedItems++
			continue
		}
		plan := byTask[item.TaskID]
		if plan == nil {
			plan = &taskPlan{TaskID: item.TaskID, TaskNo: item.TaskNo, TaskType: item.TaskType}
			byTask[item.TaskID] = plan
		}
		plan.Items = append(plan.Items, item)
		if item.ResolvedIID == "" || (item.TaskType == "sku_planning" && (item.PlanningRevisionID <= 0 || strings.TrimSpace(item.PlanningName) == "")) {
			plan.Blocked = true
			if item.ResolvedIID == "" {
				item.BlockReason = "missing deterministic ERP i_id"
			} else {
				item.BlockReason = "missing planning revision or frozen product name"
			}
		}
	}
	plans := make([]*taskPlan, 0, len(byTask))
	for _, plan := range byTask {
		switch {
		case plan.Blocked:
			plan.RecoveryAction = "blocked"
		case plan.TaskType == "sku_planning":
			plan.RecoveryAction = "planning_sku_resync"
		case everyItemHasSyncedRecord(plan.Items):
			plan.RecoveryAction = "reconcile_synced_projection"
		case taskFilingRecoveryEligible(plan):
			plan.RecoveryAction = "task_filing"
		default:
			plan.Blocked = true
			plan.RecoveryAction = "blocked"
			for _, item := range plan.Items {
				item.BlockReason = "ERP filing payload is incomplete and no exact synced ERP record exists"
			}
		}
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].TaskID < plans[j].TaskID })
	return plans, len(excluded), excludedItems
}

func everyItemHasSyncedRecord(items []*candidate) bool {
	if len(items) == 0 {
		return false
	}
	for _, item := range items {
		if item == nil || !item.HasSyncedRecord {
			return false
		}
	}
	return true
}

func taskFilingRecoveryEligible(plan *taskPlan) bool {
	if plan == nil || plan.TaskType != "new_product_development" || len(plan.Items) == 0 {
		return false
	}
	first := plan.Items[0]
	if first == nil {
		return false
	}
	if first.IsBatchTask {
		if strings.TrimSpace(first.TaskProductName) == "" {
			return false
		}
		for _, item := range plan.Items {
			if item == nil || strings.TrimSpace(item.SKUCode) == "" ||
				strings.TrimSpace(item.ItemProductName) == "" || strings.TrimSpace(item.ResolvedIID) == "" {
				return false
			}
		}
		return true
	}
	return strings.TrimSpace(first.TaskSKUCode) != "" &&
		strings.TrimSpace(first.TaskProductName) != "" &&
		(strings.TrimSpace(first.DetailCategory) != "" || strings.TrimSpace(first.DetailCategoryName) != "")
}

func applyPlans(ctx context.Context, db *sql.DB, plans []*taskPlan, runID string, actorID int64, out *report) error {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		return err
	}
	defer tx.Rollback()
	for _, plan := range plans {
		if plan.Blocked {
			continue
		}
		for _, item := range plan.Items {
			result, err := tx.ExecContext(ctx, `
				UPDATE task_sku_items
				   SET product_i_id = ?
				 WHERE id = ? AND (product_i_id IS NULL OR TRIM(product_i_id) = '')`,
				item.ResolvedIID, item.ItemID)
			if err != nil {
				return fmt.Errorf("repair item %d: %w", item.ItemID, err)
			}
			changed, err := result.RowsAffected()
			if err != nil {
				return err
			}
			out.UpdatedItemIIDs += changed
		}
		if plan.RecoveryAction == "reconcile_synced_projection" {
			for _, item := range plan.Items {
				result, err := tx.ExecContext(ctx, `
					UPDATE task_sku_items tsi
					   SET sku_status = 'filed',
					       filing_status = 'filed',
					       erp_sync_status = 'filed',
					       erp_sync_required = 0,
					       erp_sync_version = erp_sync_version + 1,
					       last_filed_at = COALESCE(last_filed_at, CURRENT_TIMESTAMP),
					       filing_error_message = '',
					       updated_at = CURRENT_TIMESTAMP
					 WHERE tsi.task_id = ? AND tsi.id = ?
					   AND EXISTS (
					     SELECT 1
					       FROM erp_product_sync_records synced
					      WHERE synced.task_id = tsi.task_id
					        AND synced.task_sku_item_id = tsi.id
					        AND synced.base_sync_status = 'synced'
					   )
					   AND (
					     tsi.erp_sync_required <> 0
					     OR tsi.filing_status <> 'filed'
					     OR tsi.erp_sync_status <> 'filed'
					   )`,
					plan.TaskID, item.ItemID)
				if err != nil {
					return fmt.Errorf("reconcile synced item %d: %w", item.ItemID, err)
				}
				changed, _ := result.RowsAffected()
				out.ReconciledItems += changed
			}
			result, err := tx.ExecContext(ctx, `
				UPDATE task_details td
				   SET filing_status = 'filed',
				       erp_sync_required = 0,
				       erp_sync_version = erp_sync_version + 1,
				       last_filed_at = COALESCE(last_filed_at, CURRENT_TIMESTAMP),
				       filed_at = COALESCE(filed_at, CURRENT_TIMESTAMP),
				       filing_error_message = '',
				       updated_at = CURRENT_TIMESTAMP
				 WHERE td.task_id = ?
				   AND NOT EXISTS (
				     SELECT 1
				       FROM task_sku_items tsi
				      WHERE tsi.task_id = td.task_id
				        AND (
				          tsi.erp_sync_required <> 0
				          OR tsi.filing_status <> 'filed'
				          OR tsi.erp_sync_status <> 'filed'
				        )
				   )
				   AND (
				     td.erp_sync_required <> 0
				     OR td.filing_status <> 'filed'
				   )`,
				plan.TaskID)
			if err != nil {
				return fmt.Errorf("reconcile synced task %d: %w", plan.TaskID, err)
			}
			changed, _ := result.RowsAffected()
			out.ReconciledTasks += changed
			continue
		}
		if plan.TaskType == "sku_planning" {
			for _, item := range plan.Items {
				var generation int
				if err := tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(generation), 0) + 1 FROM task_erp_outbox WHERE task_sku_item_id = ?`, item.ItemID).Scan(&generation); err != nil {
					return err
				}
				payload, err := json.Marshal(map[string]interface{}{
					"task_id": plan.TaskID, "task_sku_item_id": item.ItemID, "sku_code": item.SKUCode,
					"revision_id": item.PlanningRevisionID, "erp_product_i_id": item.ResolvedIID,
					"erp_product_name": item.PlanningName, "image_ref_id": item.ImageRefID,
				})
				if err != nil {
					return err
				}
				result, err := tx.ExecContext(ctx, `
					INSERT INTO task_erp_outbox
					  (task_id, task_sku_item_id, job_type, generation, dedupe_key, payload_json)
					VALUES (?, ?, 'planning_sku_resync', ?, ?, ?)
					ON DUPLICATE KEY UPDATE dedupe_key = dedupe_key`,
					plan.TaskID, item.ItemID, generation,
					fmt.Sprintf("planning_sku_resync:recovery:%s:%d:%d", runID, plan.TaskID, item.ItemID), payload)
				if err != nil {
					return fmt.Errorf("enqueue planning item %d: %w", item.ItemID, err)
				}
				changed, _ := result.RowsAffected()
				out.QueuedPlanningSKUJobs += changed
			}
			continue
		}
		payload, err := json.Marshal(map[string]interface{}{
			"task_id": plan.TaskID, "operator_id": actorID, "source": "task_sku_sync_recovery", "run_id": runID,
		})
		if err != nil {
			return err
		}
		result, err := tx.ExecContext(ctx, `
			INSERT INTO task_erp_outbox (task_id, job_type, dedupe_key, payload_json)
			VALUES (?, 'task_filing', ?, ?)
			ON DUPLICATE KEY UPDATE dedupe_key = dedupe_key`,
			plan.TaskID, fmt.Sprintf("task_filing:recovery:%s:%d", runID, plan.TaskID), payload)
		if err != nil {
			return fmt.Errorf("enqueue task %d: %w", plan.TaskID, err)
		}
		changed, _ := result.RowsAffected()
		out.QueuedTaskFilingJobs += changed
	}
	return tx.Commit()
}

func exitError(operation string, err error) {
	_ = json.NewEncoder(os.Stderr).Encode(map[string]string{"error": operation + ": " + err.Error()})
	os.Exit(1)
}
