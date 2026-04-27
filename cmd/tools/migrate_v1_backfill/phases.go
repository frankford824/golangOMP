package main

import (
	"context"
	"database/sql"
	"fmt"
	"math"
	"strings"
	"time"

	"workflow/cmd/tools/internal/v1migrate"
	"workflow/domain"
)

func runBackfill(ctx context.Context, db *sql.DB, opts BackfillOptions) (BackfillResult, error) {
	if opts.BatchSize <= 0 {
		opts.BatchSize = 1000
	}
	start := time.Now()
	logf("start dry_run=%t batch_size=%d", opts.DryRun, opts.BatchSize)

	phases := []func(context.Context, *sql.DB, BackfillOptions) (PhaseStat, error){
		phaseA,
		phaseB,
		phaseC,
		phaseD,
		phaseE,
	}
	result := BackfillResult{}
	for _, phase := range phases {
		stat, err := phase(ctx, db, opts)
		result.Stats = append(result.Stats, stat)
		if err != nil {
			return result, err
		}
		if stat.Duration > 60*time.Second {
			return result, v1migrate.NewHardAbort(1, "%s exceeded 60s hard upper bound: %s", stat.Name, stat.Duration)
		}
	}
	result.TotalDuration = time.Since(start)
	logf("total_duration=%s", result.TotalDuration.Round(time.Millisecond))
	if result.TotalDuration > 10*time.Second {
		logf("warning: production performance threshold exceeded: %s > 10s", result.TotalDuration.Round(time.Millisecond))
	}
	return result, nil
}

func phaseA(ctx context.Context, db *sql.DB, opts BackfillOptions) (PhaseStat, error) {
	start := time.Now()
	stat := PhaseStat{Name: "Phase A"}
	tasks, err := loadTasks(ctx, db)
	if err != nil {
		return finishPhase(stat, start), err
	}
	for _, task := range tasks {
		stat.Processed++
		for _, spec := range taskModules(task) {
			moduleID, inserted, err := ensureModule(ctx, db, opts.DryRun, spec)
			if err != nil {
				stat.Errors++
				return finishPhase(stat, start), err
			}
			if inserted {
				stat.Generated++
				if err := insertEvent(ctx, db, opts.DryRun, moduleID, "migrated_from_v0_9", "", spec.State, map[string]any{
					"task_status": task.TaskStatus,
					"task_type":   task.TaskType,
				}); err != nil {
					stat.Errors++
					return finishPhase(stat, start), err
				}
			}
		}
	}
	return finishPhase(stat, start), nil
}

func phaseB(ctx context.Context, db *sql.DB, opts BackfillOptions) (PhaseStat, error) {
	start := time.Now()
	stat := PhaseStat{Name: "Phase B"}
	if err := validateAssetTypes(ctx, db); err != nil {
		stat.Errors++
		return finishPhase(stat, start), err
	}
	assets, err := loadAssets(ctx, db)
	if err != nil {
		return finishPhase(stat, start), err
	}
	for _, asset := range assets {
		stat.Processed++
		moduleKey, ok := inferAssetModule(asset.AssetType, asset.TaskType, asset.CustomizationRequired)
		if !ok {
			stat.Errors++
			return finishPhase(stat, start), v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "unknown asset_type: %s", asset.AssetType)
		}
		moduleID, exists, err := findModuleID(ctx, db, asset.TaskID, moduleKey)
		if err != nil {
			stat.Errors++
			return finishPhase(stat, start), err
		}
		if !exists {
			moduleID, exists, err = ensureModule(ctx, db, opts.DryRun, moduleInstance{
				TaskID:    asset.TaskID,
				ModuleKey: moduleKey,
				State:     "closed",
				TerminalAt: sql.NullTime{
					Time:  time.Now(),
					Valid: true,
				},
				Data: `{"backfill_placeholder":true}`,
			})
			if err != nil {
				stat.Errors++
				return finishPhase(stat, start), err
			}
			if exists {
				stat.Generated++
				if err := insertEvent(ctx, db, opts.DryRun, moduleID, "backfill_placeholder", "", "closed", map[string]any{
					"asset_id":          asset.ID,
					"source_module_key": moduleKey,
				}); err != nil {
					stat.Errors++
					return finishPhase(stat, start), err
				}
			}
		}
		if opts.DryRun {
			continue
		}
		res, err := db.ExecContext(ctx, `
			UPDATE task_assets
			   SET source_module_key=?, source_task_module_id=?
			 WHERE id=?
			   AND (source_module_key<>? OR source_task_module_id IS NULL OR source_task_module_id<>?)`,
			moduleKey, moduleID, asset.ID, moduleKey, moduleID)
		if err != nil {
			stat.Errors++
			return finishPhase(stat, start), err
		}
		affected, _ := res.RowsAffected()
		stat.Generated += affected
	}
	var nullCount int64
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM task_assets WHERE source_module_key IS NULL OR source_module_key=''`).Scan(&nullCount); err != nil {
		return finishPhase(stat, start), err
	}
	if nullCount != 0 {
		stat.Errors++
		return finishPhase(stat, start), consistencyError("task_assets.source_module_key empty count=%d", nullCount)
	}
	return finishPhase(stat, start), nil
}

func validateAssetTypes(ctx context.Context, db *sql.DB) error {
	allowed := map[string]bool{
		"reference":    true,
		"source":       true,
		"delivery":     true,
		"design_thumb": true,
		"preview":      true,
	}
	legacy := map[string]bool{
		"original": true, "draft": true, "revised": true, "final": true, "outsource_return": true,
	}
	rows, err := db.QueryContext(ctx, `SELECT COALESCE(asset_type,''), COUNT(*) FROM task_assets GROUP BY asset_type`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var value string
		var count int64
		if err := rows.Scan(&value, &count); err != nil {
			return err
		}
		if allowed[value] {
			continue
		}
		if legacy[value] {
			return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "legacy asset_type remains from before 036: %s, count=%d; run 036 first", value, count)
		}
		return v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "unknown asset_type: %s, count=%d", value, count)
	}
	return rows.Err()
}

func phaseC(ctx context.Context, db *sql.DB, opts BackfillOptions) (PhaseStat, error) {
	start := time.Now()
	stat := PhaseStat{Name: "Phase C"}
	expected := map[string]struct{}{}
	if err := flattenTaskDetailRefs(ctx, db, opts, &stat, expected); err != nil {
		return finishPhase(stat, start), err
	}
	if err := flattenSKURefs(ctx, db, opts, &stat, expected); err != nil {
		return finishPhase(stat, start), err
	}
	if !opts.DryRun {
		var actual int64
		if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_file_refs`).Scan(&actual); err != nil {
			return finishPhase(stat, start), err
		}
		min := int64(math.Floor(float64(len(expected)) * 0.995))
		if actual < min {
			stat.Errors++
			return finishPhase(stat, start), consistencyError("reference_file_refs count=%d below expected_min=%d expected_unique=%d", actual, min, len(expected))
		}
	}
	return finishPhase(stat, start), nil
}

func flattenTaskDetailRefs(ctx context.Context, db *sql.DB, opts BackfillOptions, stat *PhaseStat, expected map[string]struct{}) error {
	rows, err := db.QueryContext(ctx, `
		SELECT td.task_id, td.reference_file_refs_json, t.customization_required
		  FROM task_details td
		  JOIN tasks t ON t.id = td.task_id
		 WHERE td.reference_file_refs_json IS NOT NULL
		   AND td.reference_file_refs_json <> ''
		   AND td.reference_file_refs_json <> '[]'
		 ORDER BY td.task_id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID int64
		var raw string
		var custom int
		if err := rows.Scan(&taskID, &raw, &custom); err != nil {
			return err
		}
		stat.Processed++
		refs, ok := parseRefs(raw)
		if !ok {
			stat.Errors++
			if err := writeTaskEventByKey(ctx, db, opts.DryRun, taskID, "basic_info", "backfill_error", map[string]any{"phase": "C", "reason": "invalid_task_details_reference_json"}); err != nil {
				return err
			}
			continue
		}
		for _, ref := range refs {
			if err := insertFlattenedRef(ctx, db, opts, stat, expected, taskID, sql.NullInt64{}, ref, custom == 1, false); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}

func flattenSKURefs(ctx context.Context, db *sql.DB, opts BackfillOptions, stat *PhaseStat, expected map[string]struct{}) error {
	rows, err := db.QueryContext(ctx, `
		SELECT tsi.id, tsi.task_id, tsi.reference_file_refs_json, t.customization_required
		  FROM task_sku_items tsi
		  JOIN tasks t ON t.id = tsi.task_id
		 WHERE tsi.reference_file_refs_json IS NOT NULL
		   AND tsi.reference_file_refs_json <> ''
		   AND tsi.reference_file_refs_json <> '[]'
		 ORDER BY tsi.id`)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var skuID, taskID int64
		var raw string
		var custom int
		if err := rows.Scan(&skuID, &taskID, &raw, &custom); err != nil {
			return err
		}
		stat.Processed++
		refs, ok := parseRefs(raw)
		if !ok {
			stat.Errors++
			if err := writeTaskEventByKey(ctx, db, opts.DryRun, taskID, "basic_info", "backfill_error", map[string]any{"phase": "C", "reason": "invalid_task_sku_reference_json", "sku_item_id": skuID}); err != nil {
				return err
			}
			continue
		}
		for _, ref := range refs {
			if err := insertFlattenedRef(ctx, db, opts, stat, expected, taskID, sql.NullInt64{Int64: skuID, Valid: true}, ref, custom == 1, true); err != nil {
				return err
			}
		}
	}
	return rows.Err()
}

func parseRefs(raw string) ([]domain.ReferenceFileRef, bool) {
	refs := domain.ParseReferenceFileRefsJSON(raw)
	if refs == nil && strings.TrimSpace(raw) != "" && strings.TrimSpace(raw) != "[]" {
		return nil, false
	}
	return refs, true
}

func insertFlattenedRef(ctx context.Context, db *sql.DB, opts BackfillOptions, stat *PhaseStat, expected map[string]struct{}, taskID int64, skuID sql.NullInt64, ref domain.ReferenceFileRef, customizationRequired bool, skuLevel bool) error {
	refID := strings.TrimSpace(ref.CanonicalID())
	if refID == "" {
		stat.Warnings++
		return nil
	}
	var ownerType string
	err := db.QueryRowContext(ctx, `SELECT owner_type FROM asset_storage_refs WHERE ref_id=?`, refID).Scan(&ownerType)
	if err == sql.ErrNoRows {
		stat.Warnings++
		return writeTaskEventByKey(ctx, db, opts.DryRun, taskID, "basic_info", "backfill_warning", map[string]any{"phase": "C", "reason": "missing_asset_storage_ref", "ref_id": refID})
	}
	if err != nil {
		return err
	}
	ownerModule, fallback := mapOwnerModule(ownerType, customizationRequired, skuLevel)
	if fallback {
		stat.Warnings++
		if err := writeTaskEventByKey(ctx, db, opts.DryRun, taskID, "basic_info", "backfill_warning", map[string]any{"phase": "C", "reason": "owner_type_fallback", "owner_type": ownerType, "owner_module_key": ownerModule}); err != nil {
			return err
		}
	}
	key := fmt.Sprintf("%d/%s/%d", taskID, refID, skuID.Int64)
	expected[key] = struct{}{}
	if opts.DryRun {
		return nil
	}
	var exists int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM reference_file_refs WHERE task_id=? AND ref_id=? AND sku_item_id <=> ?`, taskID, refID, nullIntValue(skuID)).Scan(&exists); err != nil {
		return err
	}
	if exists > 0 {
		return nil
	}
	res, err := db.ExecContext(ctx, `
		INSERT INTO reference_file_refs (task_id, sku_item_id, ref_id, owner_module_key, context, attached_at)
		VALUES (?, ?, ?, ?, NULLIF(?, ''), NOW())`,
		taskID, nullIntValue(skuID), refID, ownerModule, ref.Source)
	if err != nil {
		return err
	}
	affected, _ := res.RowsAffected()
	stat.Generated += affected
	return nil
}

func phaseD(ctx context.Context, db *sql.DB, opts BackfillOptions) (PhaseStat, error) {
	start := time.Now()
	stat := PhaseStat{Name: "Phase D"}
	rows, err := db.QueryContext(ctx, `SELECT id, priority FROM tasks WHERE priority NOT IN ('low','normal','high','critical')`)
	if err != nil {
		return finishPhase(stat, start), err
	}
	defer rows.Close()
	var invalid []struct {
		taskID   int64
		priority string
	}
	for rows.Next() {
		var row struct {
			taskID   int64
			priority string
		}
		if err := rows.Scan(&row.taskID, &row.priority); err != nil {
			return finishPhase(stat, start), err
		}
		invalid = append(invalid, row)
	}
	if err := rows.Err(); err != nil {
		return finishPhase(stat, start), err
	}
	stat.Processed = int64(len(invalid))
	for _, row := range invalid {
		stat.Errors++
		if err := writeTaskEventByKey(ctx, db, opts.DryRun, row.taskID, "basic_info", "backfill_priority_out_of_range", map[string]any{"priority": row.priority}); err != nil {
			return finishPhase(stat, start), err
		}
		logf("priority out of range: task_id=%d priority=%s", row.taskID, row.priority)
	}
	if len(invalid) > 0 {
		return finishPhase(stat, start), v1migrate.NewHardAbort(v1migrate.ExitCodeHardAbort, "priority out of range count=%d", len(invalid))
	}
	return finishPhase(stat, start), nil
}

func phaseE(ctx context.Context, db *sql.DB, opts BackfillOptions) (PhaseStat, error) {
	start := time.Now()
	stat := PhaseStat{Name: "Phase E"}
	rows, err := db.QueryContext(ctx, `
		SELECT id
		  FROM tasks
		 WHERE customization_required=1
		   AND task_type='customer_customization'
		 ORDER BY id`)
	if err != nil {
		return finishPhase(stat, start), err
	}
	defer rows.Close()
	for rows.Next() {
		var taskID int64
		if err := rows.Scan(&taskID); err != nil {
			return finishPhase(stat, start), err
		}
		stat.Processed++
		if opts.DryRun {
			continue
		}
		res, err := db.ExecContext(ctx, `
			INSERT IGNORE INTO task_customization_orders
			  (task_id, online_order_no, requirement_note, ordered_at, erp_product_code)
			VALUES (?, '', '', NULL, '')`, taskID)
		if err != nil {
			stat.Errors++
			return finishPhase(stat, start), err
		}
		affected, _ := res.RowsAffected()
		stat.Generated += affected
	}
	if err := rows.Err(); err != nil {
		return finishPhase(stat, start), err
	}
	return finishPhase(stat, start), nil
}

func writeTaskEventByKey(ctx context.Context, db *sql.DB, dryRun bool, taskID int64, moduleKey, eventType string, payload any) error {
	moduleID, ok, err := findModuleID(ctx, db, taskID, moduleKey)
	if err != nil {
		return err
	}
	if !ok {
		moduleID, _, err = ensureModule(ctx, db, dryRun, moduleInstance{
			TaskID:    taskID,
			ModuleKey: moduleKey,
			State:     "active",
			Data:      "{}",
		})
		if err != nil {
			return err
		}
	}
	return insertEvent(ctx, db, dryRun, moduleID, eventType, "", "", payload)
}
