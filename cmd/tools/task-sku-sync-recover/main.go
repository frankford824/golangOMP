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
	ItemID              int64  `json:"task_sku_item_id"`
	SKUCode             string `json:"sku_code"`
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
	ResolvedIID         string `json:"resolved_i_id,omitempty"`
	ResolutionSource    string `json:"resolution_source,omitempty"`
}

type taskPlan struct {
	TaskID   int64        `json:"task_id"`
	TaskNo   string       `json:"task_no"`
	TaskType string       `json:"task_type"`
	Items    []*candidate `json:"items"`
	Blocked  bool         `json:"blocked"`
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
	flag.BoolVar(&apply, "apply", false, "repair deterministic i_id values and enqueue durable ERP jobs")
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
		ProductionSafetyNotice: "Cancelled and Archived tasks are excluded. Any task with one unresolved required SKU is blocked atomically. No ERP API is called by this tool; only durable outbox work is queued.",
	}
	for _, plan := range plans {
		if plan.Blocked {
			out.BlockedTasks++
			for _, item := range plan.Items {
				if item.ResolvedIID == "" {
					out.BlockedItems++
				}
			}
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
		       tsi.id, tsi.sku_code, COALESCE(tsi.product_i_id, ''),
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
		       ), '')
		  FROM task_sku_items tsi
		  JOIN tasks t ON t.id = tsi.task_id
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
			&item.ItemID, &item.SKUCode, &item.CurrentIID, &item.VariantIID,
			&item.ProductIID, &item.SyncRecordIID, &item.CategoryCode, &item.CategoryExactIID,
			&item.PlanningRevisionID, &item.PlanningIID, &item.PlanningName, &item.PlanningDescription, &item.ImageRefID,
		); err != nil {
			return nil, err
		}
		if item.TaskType == "sku_planning" && strings.TrimSpace(item.PlanningName) == "" {
			item.PlanningName = truncateRunes(strings.TrimSpace(item.PlanningDescription), 255)
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
		}
	}
	plans := make([]*taskPlan, 0, len(byTask))
	for _, plan := range byTask {
		plans = append(plans, plan)
	}
	sort.Slice(plans, func(i, j int) bool { return plans[i].TaskID < plans[j].TaskID })
	return plans, len(excluded), excludedItems
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
