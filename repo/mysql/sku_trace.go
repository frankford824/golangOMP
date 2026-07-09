package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type skuTraceRepo struct {
	db *DB
}

func NewSKUTraceRepo(db *DB) repo.SKUTraceRepo {
	return &skuTraceRepo{db: db}
}

func NewSKUComboRepo(db *DB) repo.SKUComboRepo {
	return &skuTraceRepo{db: db}
}

func (r *skuTraceRepo) UpsertSKURecord(ctx context.Context, tx repo.Tx, record *domain.OMPSKURecord) error {
	if record == nil || strings.TrimSpace(record.SKUCode) == "" {
		return nil
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO omp_sku_records (
		  sku_code, sku_kind, first_task_id, last_task_id, first_task_sku_item_id, last_task_sku_item_id,
		  source_mode, task_type, product_name, product_i_id, category_code, category_name,
		  cost_price, estimated_cost, cost_rule_id, cost_rule_name, cost_rule_source,
		  manual_cost_override, requires_manual_review, last_erp_sync_status, last_erp_call_log_id,
		  created_by, last_operator_id, first_seen_at, last_seen_at, trace_version
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, UTC_TIMESTAMP()), COALESCE(?, UTC_TIMESTAMP()), 1)
		ON DUPLICATE KEY UPDATE
		  sku_kind = IF(VALUES(sku_kind) <> '', VALUES(sku_kind), sku_kind),
		  last_task_id = COALESCE(VALUES(last_task_id), last_task_id),
		  last_task_sku_item_id = COALESCE(VALUES(last_task_sku_item_id), last_task_sku_item_id),
		  source_mode = IF(VALUES(source_mode) <> '', VALUES(source_mode), source_mode),
		  task_type = IF(VALUES(task_type) <> '', VALUES(task_type), task_type),
		  product_name = IF(VALUES(product_name) <> '', VALUES(product_name), product_name),
		  product_i_id = IF(VALUES(product_i_id) <> '', VALUES(product_i_id), product_i_id),
		  category_code = IF(VALUES(category_code) <> '', VALUES(category_code), category_code),
		  category_name = IF(VALUES(category_name) <> '', VALUES(category_name), category_name),
		  cost_price = COALESCE(VALUES(cost_price), cost_price),
		  estimated_cost = COALESCE(VALUES(estimated_cost), estimated_cost),
		  cost_rule_id = COALESCE(VALUES(cost_rule_id), cost_rule_id),
		  cost_rule_name = IF(VALUES(cost_rule_name) <> '', VALUES(cost_rule_name), cost_rule_name),
		  cost_rule_source = IF(VALUES(cost_rule_source) <> '', VALUES(cost_rule_source), cost_rule_source),
		  manual_cost_override = VALUES(manual_cost_override),
		  requires_manual_review = VALUES(requires_manual_review),
		  last_erp_sync_status = IF(VALUES(last_erp_sync_status) <> '', VALUES(last_erp_sync_status), last_erp_sync_status),
		  last_erp_call_log_id = COALESCE(VALUES(last_erp_call_log_id), last_erp_call_log_id),
		  created_by = COALESCE(created_by, VALUES(created_by)),
		  last_operator_id = COALESCE(VALUES(last_operator_id), last_operator_id),
		  last_seen_at = COALESCE(VALUES(last_seen_at), UTC_TIMESTAMP()),
		  trace_version = trace_version + 1`,
		strings.TrimSpace(record.SKUCode),
		string(record.SKUKind),
		toNullInt64(record.FirstTaskID),
		toNullInt64(record.LastTaskID),
		toNullInt64(record.FirstTaskSKUItemID),
		toNullInt64(record.LastTaskSKUItemID),
		strings.TrimSpace(record.SourceMode),
		strings.TrimSpace(record.TaskType),
		strings.TrimSpace(record.ProductName),
		strings.TrimSpace(record.ProductIID),
		strings.TrimSpace(record.CategoryCode),
		strings.TrimSpace(record.CategoryName),
		toNullFloat64(record.CostPrice),
		toNullFloat64(record.EstimatedCost),
		toNullInt64(record.CostRuleID),
		strings.TrimSpace(record.CostRuleName),
		strings.TrimSpace(record.CostRuleSource),
		record.ManualCostOverride,
		record.RequiresManualReview,
		strings.TrimSpace(record.LastERPSyncStatus),
		toNullInt64(record.LastERPCallLogID),
		toNullInt64(record.CreatedBy),
		toNullInt64(record.LastOperatorID),
		toNullTime(nonZeroTimePtr(record.FirstSeenAt)),
		toNullTime(nonZeroTimePtr(record.LastSeenAt)),
	)
	if err != nil {
		return fmt.Errorf("upsert omp sku record: %w", err)
	}
	return nil
}

func (r *skuTraceRepo) AppendCostSnapshot(ctx context.Context, tx repo.Tx, snapshot *domain.OMPSKUCostSnapshot) (int64, error) {
	if snapshot == nil || strings.TrimSpace(snapshot.SKUCode) == "" {
		return 0, nil
	}
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO omp_sku_cost_snapshots (
		  sku_code, sku_kind, task_id, task_sku_item_id, event_source, event_reason, operator_id,
		  cost_price, cost_price_present, estimated_cost, estimated_cost_present,
		  cost_rule_id, cost_rule_name, cost_rule_source, matched_rule_version, prefill_source,
		  requires_manual_review, manual_cost_override, manual_cost_override_reason,
		  input_snapshot_json, calculation_snapshot_json
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(snapshot.SKUCode),
		string(snapshot.SKUKind),
		toNullInt64(snapshot.TaskID),
		toNullInt64(snapshot.TaskSKUItemID),
		strings.TrimSpace(snapshot.EventSource),
		strings.TrimSpace(snapshot.EventReason),
		toNullInt64(snapshot.OperatorID),
		toNullFloat64(snapshot.CostPrice),
		snapshot.CostPricePresent,
		toNullFloat64(snapshot.EstimatedCost),
		snapshot.EstimatedCostPresent,
		toNullInt64(snapshot.CostRuleID),
		strings.TrimSpace(snapshot.CostRuleName),
		strings.TrimSpace(snapshot.CostRuleSource),
		toNullInt(snapshot.MatchedRuleVersion),
		strings.TrimSpace(snapshot.PrefillSource),
		snapshot.RequiresManualReview,
		snapshot.ManualCostOverride,
		strings.TrimSpace(snapshot.ManualCostOverrideReason),
		toNullJSONString(snapshot.InputSnapshotJSON),
		toNullJSONString(snapshot.CalculationSnapshotJSON),
	)
	if err != nil {
		return 0, fmt.Errorf("append omp sku cost snapshot: %w", err)
	}
	id, _ := res.LastInsertId()
	if id > 0 {
		if err := r.syncLatestCostSnapshotToProductManagement(ctx, sqlTx, id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (r *skuTraceRepo) AppendERPTraceLog(ctx context.Context, tx repo.Tx, log *domain.OMPSKUERPTraceLog) (int64, error) {
	if log == nil || strings.TrimSpace(log.SKUCode) == "" {
		return 0, nil
	}
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		INSERT INTO omp_sku_erp_trace_logs (
		  sku_code, sku_kind, task_id, task_sku_item_id, call_log_id,
		  connector_key, operation_key, direction, status,
		  request_cost_price, request_cost_price_present, response_cost_price, response_cost_price_present,
		  request_payload_hash, request_payload_json, response_payload_json, error_message
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		strings.TrimSpace(log.SKUCode),
		string(log.SKUKind),
		toNullInt64(log.TaskID),
		toNullInt64(log.TaskSKUItemID),
		toNullInt64(log.CallLogID),
		strings.TrimSpace(log.ConnectorKey),
		strings.TrimSpace(log.OperationKey),
		strings.TrimSpace(log.Direction),
		strings.TrimSpace(log.Status),
		toNullFloat64(log.RequestCostPrice),
		log.RequestCostPricePresent,
		toNullFloat64(log.ResponseCostPrice),
		log.ResponseCostPricePresent,
		strings.TrimSpace(log.RequestPayloadHash),
		toNullJSONString(log.RequestPayloadJSON),
		toNullJSONString(log.ResponsePayloadJSON),
		toNullString(nonEmptyStringPtr(log.ErrorMessage)),
	)
	if err != nil {
		return 0, fmt.Errorf("append omp sku erp trace log: %w", err)
	}
	id, _ := res.LastInsertId()
	if id > 0 {
		if err := r.syncLatestERPTraceToProductManagement(ctx, sqlTx, id); err != nil {
			return 0, err
		}
	}
	return id, nil
}

func (r *skuTraceRepo) UpsertComboRelation(ctx context.Context, tx repo.Tx, relation *domain.OMPSKUComboRelation) error {
	if relation == nil || strings.TrimSpace(relation.ComboSKUCode) == "" || strings.TrimSpace(relation.ChildSKUCode) == "" {
		return nil
	}
	quantity := relation.Quantity
	if quantity <= 0 {
		quantity = 1
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO omp_sku_combo_relations (
		  combo_sku_code, child_sku_code, quantity, source, source_call_log_id, raw_payload_json, first_seen_at, last_seen_at
		) VALUES (?, ?, ?, ?, ?, ?, COALESCE(?, UTC_TIMESTAMP()), COALESCE(?, UTC_TIMESTAMP()))
		ON DUPLICATE KEY UPDATE
		  quantity = VALUES(quantity),
		  source_call_log_id = COALESCE(VALUES(source_call_log_id), source_call_log_id),
		  raw_payload_json = COALESCE(VALUES(raw_payload_json), raw_payload_json),
		  last_seen_at = COALESCE(VALUES(last_seen_at), UTC_TIMESTAMP())`,
		strings.TrimSpace(relation.ComboSKUCode),
		strings.TrimSpace(relation.ChildSKUCode),
		quantity,
		strings.TrimSpace(relation.Source),
		toNullInt64(relation.SourceCallLogID),
		toNullJSONString(relation.RawPayloadJSON),
		toNullTime(nonZeroTimePtr(relation.FirstSeenAt)),
		toNullTime(nonZeroTimePtr(relation.LastSeenAt)),
	)
	if err != nil {
		return fmt.Errorf("upsert omp sku combo relation: %w", err)
	}
	return r.refreshProductManagementComboSearchTextByChildSKUs(ctx, sqlTx, []string{relation.ChildSKUCode})
}

func (r *skuTraceRepo) DeleteStaleComboRelations(ctx context.Context, tx repo.Tx, comboSKUCode string, source string, currentChildSKUs []string) error {
	comboSKUCode = strings.TrimSpace(comboSKUCode)
	if comboSKUCode == "" {
		return nil
	}
	source = firstNonEmptySQLString(source, "jst_openweb_combine_sku_query")
	sqlTx := Unwrap(tx)
	existingChildSKUs, err := r.comboRelationChildSKUs(ctx, sqlTx, comboSKUCode, source)
	if err != nil {
		return err
	}
	normalized := make([]string, 0, len(currentChildSKUs))
	seen := map[string]struct{}{}
	for _, sku := range currentChildSKUs {
		sku = strings.TrimSpace(sku)
		if sku == "" {
			continue
		}
		key := strings.ToUpper(sku)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, sku)
	}
	if len(normalized) == 0 {
		if _, err := sqlTx.ExecContext(ctx, `
			DELETE FROM omp_sku_combo_relations
			 WHERE combo_sku_code = ? AND source = ?`,
			comboSKUCode, source,
		); err != nil {
			return fmt.Errorf("delete stale omp sku combo relations: %w", err)
		}
		return r.refreshProductManagementComboSearchTextByChildSKUs(ctx, sqlTx, existingChildSKUs)
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(normalized)), ",")
	args := make([]interface{}, 0, 2+len(normalized))
	args = append(args, comboSKUCode, source)
	for _, sku := range normalized {
		args = append(args, sku)
	}
	if _, err := sqlTx.ExecContext(ctx, `
		DELETE FROM omp_sku_combo_relations
		 WHERE combo_sku_code = ? AND source = ?
		   AND child_sku_code NOT IN (`+placeholders+`)`, args...); err != nil {
		return fmt.Errorf("delete stale omp sku combo relations except current children: %w", err)
	}
	return r.refreshProductManagementComboSearchTextByChildSKUs(ctx, sqlTx, append(existingChildSKUs, normalized...))
}

func (r *skuTraceRepo) UpsertComboRecord(ctx context.Context, tx repo.Tx, record *domain.OMPSKUComboRecord) error {
	if record == nil || strings.TrimSpace(record.ComboSKUCode) == "" {
		return nil
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		INSERT INTO omp_sku_combo_records (
		  combo_sku_code, name, short_name, erp_i_id, enabled, cost_price, sale_price,
		  modified_at, source, raw_payload_json, last_synced_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, COALESCE(?, UTC_TIMESTAMP()))
		ON DUPLICATE KEY UPDATE
		  name = IF(VALUES(name) <> '', VALUES(name), name),
		  short_name = IF(VALUES(short_name) <> '', VALUES(short_name), short_name),
		  erp_i_id = IF(VALUES(erp_i_id) <> '', VALUES(erp_i_id), erp_i_id),
		  enabled = COALESCE(VALUES(enabled), enabled),
		  cost_price = COALESCE(VALUES(cost_price), cost_price),
		  sale_price = COALESCE(VALUES(sale_price), sale_price),
		  modified_at = COALESCE(VALUES(modified_at), modified_at),
		  source = IF(VALUES(source) <> '', VALUES(source), source),
		  raw_payload_json = COALESCE(VALUES(raw_payload_json), raw_payload_json),
		  last_synced_at = COALESCE(VALUES(last_synced_at), UTC_TIMESTAMP())`,
		strings.TrimSpace(record.ComboSKUCode),
		strings.TrimSpace(record.Name),
		strings.TrimSpace(record.ShortName),
		strings.TrimSpace(record.ERPIID),
		toNullBool(record.Enabled),
		toNullFloat64(record.CostPrice),
		toNullFloat64(record.SalePrice),
		toNullTime(record.ModifiedAt),
		firstNonEmptySQLString(record.Source, "jst_openweb_combine_sku_query"),
		toNullJSONString(record.RawPayloadJSON),
		toNullTime(nonZeroTimePtr(record.LastSyncedAt)),
	)
	if err != nil {
		return fmt.Errorf("upsert omp sku combo record: %w", err)
	}
	return r.refreshProductManagementComboSearchTextByComboSKU(ctx, sqlTx, record.ComboSKUCode)
}

func (r *skuTraceRepo) syncLatestCostSnapshotToProductManagement(ctx context.Context, tx *sql.Tx, snapshotID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE erp_product_sync_records pm
		JOIN omp_sku_cost_snapshots cost_snapshot ON cost_snapshot.id = ?
		LEFT JOIN omp_sku_cost_snapshots current_snapshot ON current_snapshot.id = pm.latest_cost_snapshot_id
		LEFT JOIN task_details pm_td ON pm_td.task_id = pm.task_id
		LEFT JOIN task_sku_items pm_tsi ON pm.task_sku_item_id IS NOT NULL AND pm_tsi.id = pm.task_sku_item_id
		   SET pm.latest_cost_snapshot_id = cost_snapshot.id,
		       pm.cost_legacy_alias_fallback = CASE
		         WHEN JSON_VALID(cost_snapshot.calculation_snapshot_json)
		          AND JSON_UNQUOTE(JSON_EXTRACT(cost_snapshot.calculation_snapshot_json, '$.legacy_alias_fallback')) = 'true'
		         THEN 1 ELSE 0 END,
		       pm.cost_area_spec_abnormal = CASE
		         WHEN pm.cost_price IS NOT NULL AND pm.cost_price > 0
		          AND COALESCE(pm_td.area, 0) <= 0
		          AND (COALESCE(pm_td.width, 0) <= 0 OR COALESCE(pm_td.height, 0) <= 0)
		          AND (
		            pm.task_sku_item_id IS NULL
		            OR NOT JSON_VALID(pm_tsi.variant_json)
		            OR (
		                 COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.area')) AS DECIMAL(12,4)), 0) <= 0
		             AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.width')) AS DECIMAL(12,4)), 0) <= 0
		             AND COALESCE(CAST(JSON_UNQUOTE(JSON_EXTRACT(pm_tsi.variant_json, '$.height')) AS DECIMAL(12,4)), 0) <= 0
		            )
		          )
		         THEN 1 ELSE 0 END
		 WHERE pm.sku_code = cost_snapshot.sku_code
		   AND (
		     (cost_snapshot.task_sku_item_id IS NOT NULL AND pm.task_sku_item_id = cost_snapshot.task_sku_item_id)
		     OR (cost_snapshot.task_sku_item_id IS NULL AND cost_snapshot.task_id IS NOT NULL AND pm.task_id = cost_snapshot.task_id AND pm.task_sku_item_id IS NULL)
		     OR (cost_snapshot.task_id IS NOT NULL AND pm.task_id = cost_snapshot.task_id)
		     OR cost_snapshot.task_id IS NULL
		   )
		   AND (
		     pm.latest_cost_snapshot_id IS NULL
		     OR current_snapshot.id IS NULL
		     OR (
		       CASE
		         WHEN pm.task_sku_item_id IS NOT NULL AND cost_snapshot.task_sku_item_id = pm.task_sku_item_id THEN 0
		         WHEN pm.task_sku_item_id IS NULL AND cost_snapshot.task_id = pm.task_id AND cost_snapshot.task_sku_item_id IS NULL THEN 1
		         WHEN cost_snapshot.task_id = pm.task_id THEN 2
		         ELSE 3
		       END
		     ) < (
		       CASE
		         WHEN pm.task_sku_item_id IS NOT NULL AND current_snapshot.task_sku_item_id = pm.task_sku_item_id THEN 0
		         WHEN pm.task_sku_item_id IS NULL AND current_snapshot.task_id = pm.task_id AND current_snapshot.task_sku_item_id IS NULL THEN 1
		         WHEN current_snapshot.task_id = pm.task_id THEN 2
		         ELSE 3
		       END
		     )
		     OR (
		       (
		         CASE
		           WHEN pm.task_sku_item_id IS NOT NULL AND cost_snapshot.task_sku_item_id = pm.task_sku_item_id THEN 0
		           WHEN pm.task_sku_item_id IS NULL AND cost_snapshot.task_id = pm.task_id AND cost_snapshot.task_sku_item_id IS NULL THEN 1
		           WHEN cost_snapshot.task_id = pm.task_id THEN 2
		           ELSE 3
		         END
		       ) = (
		         CASE
		           WHEN pm.task_sku_item_id IS NOT NULL AND current_snapshot.task_sku_item_id = pm.task_sku_item_id THEN 0
		           WHEN pm.task_sku_item_id IS NULL AND current_snapshot.task_id = pm.task_id AND current_snapshot.task_sku_item_id IS NULL THEN 1
		           WHEN current_snapshot.task_id = pm.task_id THEN 2
		           ELSE 3
		         END
		       )
		       AND (
		         cost_snapshot.created_at > current_snapshot.created_at
		         OR (cost_snapshot.created_at = current_snapshot.created_at AND cost_snapshot.id > current_snapshot.id)
		       )
		     )
		   )`, snapshotID)
	if err != nil {
		return fmt.Errorf("sync product management latest cost snapshot: %w", err)
	}
	return nil
}

func (r *skuTraceRepo) syncLatestERPTraceToProductManagement(ctx context.Context, tx *sql.Tx, traceID int64) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE erp_product_sync_records pm
		JOIN omp_sku_erp_trace_logs latest_erp_trace ON latest_erp_trace.id = ?
		   SET pm.latest_erp_trace_id = latest_erp_trace.id
		 WHERE pm.sku_code = latest_erp_trace.sku_code
		   AND (
		     (latest_erp_trace.task_sku_item_id IS NOT NULL AND pm.task_sku_item_id = latest_erp_trace.task_sku_item_id)
		     OR (latest_erp_trace.task_sku_item_id IS NULL AND latest_erp_trace.task_id IS NOT NULL AND pm.task_id = latest_erp_trace.task_id AND pm.task_sku_item_id IS NULL)
		     OR (latest_erp_trace.task_id IS NOT NULL AND pm.task_id = latest_erp_trace.task_id)
		     OR latest_erp_trace.task_id IS NULL
		   )`, traceID)
	if err != nil {
		return fmt.Errorf("sync product management latest erp trace: %w", err)
	}
	return nil
}

func (r *skuTraceRepo) refreshProductManagementComboSearchTextByComboSKU(ctx context.Context, tx *sql.Tx, comboSKUCode string) error {
	childSKUs, err := r.comboRelationChildSKUs(ctx, tx, comboSKUCode, "")
	if err != nil {
		return err
	}
	return r.refreshProductManagementComboSearchTextByChildSKUs(ctx, tx, childSKUs)
}

func (r *skuTraceRepo) comboRelationChildSKUs(ctx context.Context, tx *sql.Tx, comboSKUCode string, source string) ([]string, error) {
	comboSKUCode = strings.TrimSpace(comboSKUCode)
	if comboSKUCode == "" {
		return nil, nil
	}
	clauses := []string{"combo_sku_code = ?"}
	args := []interface{}{comboSKUCode}
	if source = strings.TrimSpace(source); source != "" {
		clauses = append(clauses, "source = ?")
		args = append(args, source)
	}
	rows, err := tx.QueryContext(ctx, `SELECT DISTINCT child_sku_code FROM omp_sku_combo_relations WHERE `+strings.Join(clauses, " AND "), args...)
	if err != nil {
		return nil, fmt.Errorf("list combo relation child skus: %w", err)
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var sku string
		if err := rows.Scan(&sku); err != nil {
			return nil, fmt.Errorf("scan combo relation child sku: %w", err)
		}
		out = append(out, sku)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate combo relation child skus: %w", err)
	}
	return out, nil
}

func (r *skuTraceRepo) refreshProductManagementComboSearchTextByChildSKUs(ctx context.Context, tx *sql.Tx, childSKUs []string) error {
	normalized := make([]string, 0, len(childSKUs))
	seen := map[string]struct{}{}
	for _, sku := range childSKUs {
		sku = strings.TrimSpace(sku)
		if sku == "" {
			continue
		}
		key := strings.ToUpper(sku)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, sku)
	}
	if len(normalized) == 0 {
		return nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(normalized)), ",")
	args := make([]interface{}, 0, len(normalized)*2)
	for _, sku := range normalized {
		args = append(args, sku)
	}
	for _, sku := range normalized {
		args = append(args, sku)
	}
	_, err := tx.ExecContext(ctx, `
		UPDATE erp_product_sync_records pm
		LEFT JOIN (
		    SELECT
		      limited.child_sku_code,
		      GROUP_CONCAT(DISTINCT limited.search_token ORDER BY limited.search_token SEPARATOR ' ') AS combo_search_text
		      FROM (
		        SELECT ranked.child_sku_code, ranked.search_token
		          FROM (
		            SELECT
		              rel.child_sku_code,
		              LEFT(CONCAT_WS(' ', rel.combo_sku_code, rec.erp_i_id, rec.name, rec.short_name), 256) AS search_token,
		              ROW_NUMBER() OVER (PARTITION BY rel.child_sku_code ORDER BY rel.combo_sku_code, COALESCE(rec.id, 0)) AS rn
		              FROM omp_sku_combo_relations rel
		              LEFT JOIN omp_sku_combo_records rec ON rec.combo_sku_code = rel.combo_sku_code
		             WHERE rel.child_sku_code IN (`+placeholders+`)
		          ) ranked
		         WHERE ranked.rn <= 200
		      ) limited
		     GROUP BY limited.child_sku_code
		) combo ON combo.child_sku_code = pm.sku_code
		   SET pm.combo_search_text = COALESCE(combo.combo_search_text, '')
		 WHERE pm.sku_code IN (`+placeholders+`)`, args...)
	if err != nil {
		return fmt.Errorf("refresh product management combo search text: %w", err)
	}
	return nil
}

func (r *skuTraceRepo) ListRelationsByChildSKUs(ctx context.Context, childSKUs []string) ([]*domain.OMPSKUComboRelationWithRecord, error) {
	normalized := make([]string, 0, len(childSKUs))
	seen := map[string]struct{}{}
	for _, sku := range childSKUs {
		sku = strings.TrimSpace(sku)
		if sku == "" {
			continue
		}
		key := strings.ToUpper(sku)
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		normalized = append(normalized, sku)
	}
	if len(normalized) == 0 {
		return []*domain.OMPSKUComboRelationWithRecord{}, nil
	}
	placeholders := strings.TrimRight(strings.Repeat("?,", len(normalized)), ",")
	args := make([]interface{}, 0, len(normalized))
	for _, sku := range normalized {
		args = append(args, sku)
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT
		  rel.id, rel.combo_sku_code, rel.child_sku_code, rel.quantity, rel.source,
		  rel.source_call_log_id, rel.raw_payload_json, rel.first_seen_at, rel.last_seen_at,
		  rel.created_at, rel.updated_at,
		  rec.combo_sku_code, rec.name, rec.short_name, rec.erp_i_id, rec.enabled,
		  rec.cost_price, rec.sale_price, rec.modified_at, rec.source, rec.raw_payload_json,
		  rec.last_synced_at, rec.created_at, rec.updated_at
		FROM omp_sku_combo_relations rel
		LEFT JOIN omp_sku_combo_records rec ON rec.combo_sku_code = rel.combo_sku_code
		WHERE rel.child_sku_code IN (`+placeholders+`)
		ORDER BY rel.combo_sku_code ASC, rel.child_sku_code ASC`, args...)
	if err != nil {
		return nil, fmt.Errorf("list sku combo relations by child skus: %w", err)
	}
	defer rows.Close()
	var out []*domain.OMPSKUComboRelationWithRecord
	for rows.Next() {
		item, err := scanComboRelationWithRecord(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate sku combo relations: %w", err)
	}
	return out, nil
}

func (r *skuTraceRepo) GetLatestSyncState(ctx context.Context) (*domain.OMPSKUComboSyncState, error) {
	row := r.db.db.QueryRowContext(ctx, `
		SELECT id, window_begin, window_end, page_index, page_size, status,
		       last_success_at, next_retry_at, last_error, processed_items, created_at, updated_at
		FROM omp_sku_combo_sync_state
		ORDER BY updated_at DESC, id DESC
		LIMIT 1`)
	state, err := scanComboSyncState(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get latest sku combo sync state: %w", err)
	}
	return state, nil
}

func (r *skuTraceRepo) EnsureNextSyncWindow(ctx context.Context, now time.Time, windowSize time.Duration) (*domain.OMPSKUComboSyncState, error) {
	if windowSize <= 0 {
		windowSize = 7 * 24 * time.Hour
	}
	if now.IsZero() {
		now = time.Now()
	}
	current, err := r.GetLatestSyncState(ctx)
	if err != nil {
		return nil, err
	}
	if current != nil && strings.TrimSpace(current.Status) != "done" {
		return current, nil
	}
	var begin time.Time
	if current == nil {
		begin = now.Add(-windowSize)
		if earliest, err := r.earliestProductManagementTaskCreatedAt(ctx); err != nil {
			return nil, err
		} else if earliest != nil && !earliest.IsZero() {
			candidate := earliest.Add(-windowSize)
			if candidate.Before(begin) {
				begin = candidate
			}
		}
	} else {
		begin = current.WindowEnd
		if !begin.Before(now) {
			begin = now.Add(-windowSize)
		}
	}
	end := begin.Add(windowSize)
	if end.After(now) {
		end = now
	}
	if !begin.Before(end) {
		begin = now.Add(-windowSize)
		end = now
	}
	_, err = r.db.db.ExecContext(ctx, `
		INSERT INTO omp_sku_combo_sync_state (window_begin, window_end, page_index, page_size, status)
		VALUES (?, ?, 1, 50, 'pending')
		ON DUPLICATE KEY UPDATE
		  status = IF(status = 'done', 'pending', status),
		  next_retry_at = NULL`,
		begin, end,
	)
	if err != nil {
		return nil, fmt.Errorf("ensure sku combo sync window: %w", err)
	}
	return r.GetLatestSyncState(ctx)
}

func (r *skuTraceRepo) ClaimSyncState(ctx context.Context, tx repo.Tx, id int64, now time.Time) (bool, error) {
	if id <= 0 {
		return false, nil
	}
	if now.IsZero() {
		now = time.Now()
	}
	staleBefore := now.Add(-15 * time.Minute)
	sqlTx := Unwrap(tx)
	res, err := sqlTx.ExecContext(ctx, `
		UPDATE omp_sku_combo_sync_state
		   SET status = 'running',
		       next_retry_at = NULL,
		       last_error = ''
		 WHERE id = ?
		   AND (
		     status = 'pending'
		     OR (status = 'failed' AND (next_retry_at IS NULL OR next_retry_at <= ?))
		     OR (status = 'running' AND updated_at < ?)
		   )`,
		id, now, staleBefore,
	)
	if err != nil {
		return false, fmt.Errorf("claim sku combo sync state: %w", err)
	}
	affected, _ := res.RowsAffected()
	return affected > 0, nil
}

func (r *skuTraceRepo) MarkSyncStateSuccess(ctx context.Context, tx repo.Tx, id int64, nextPage int, processed int, finished bool, now time.Time) error {
	if id <= 0 {
		return nil
	}
	if now.IsZero() {
		now = time.Now().UTC()
	}
	status := "pending"
	if finished {
		status = "done"
	}
	if nextPage < 1 {
		nextPage = 1
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE omp_sku_combo_sync_state
		   SET status = ?,
		       page_index = ?,
		       processed_items = processed_items + ?,
		       last_success_at = ?,
		       next_retry_at = NULL,
		       last_error = ''
		 WHERE id = ?`,
		status, nextPage, processed, now, id,
	)
	if err != nil {
		return fmt.Errorf("mark sku combo sync state success: %w", err)
	}
	return nil
}

func (r *skuTraceRepo) earliestProductManagementTaskCreatedAt(ctx context.Context) (*time.Time, error) {
	var value sql.NullTime
	err := r.db.db.QueryRowContext(ctx, `
		SELECT MIN(task_created_at)
		  FROM erp_product_sync_records
		 WHERE task_created_at IS NOT NULL`).Scan(&value)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get earliest product management task created at: %w", err)
	}
	if !value.Valid {
		return nil, nil
	}
	return &value.Time, nil
}

func (r *skuTraceRepo) MarkSyncStateFailed(ctx context.Context, tx repo.Tx, id int64, message string, nextRetryAt time.Time) error {
	if id <= 0 {
		return nil
	}
	sqlTx := Unwrap(tx)
	_, err := sqlTx.ExecContext(ctx, `
		UPDATE omp_sku_combo_sync_state
		   SET status = 'failed',
		       next_retry_at = ?,
		       last_error = ?
		 WHERE id = ?`,
		nextRetryAt, truncateSQLString(message, 500), id,
	)
	if err != nil {
		return fmt.Errorf("mark sku combo sync state failed: %w", err)
	}
	return nil
}

type comboRelationScanner interface {
	Scan(dest ...interface{}) error
}

func scanComboRelationWithRecord(scanner comboRelationScanner) (*domain.OMPSKUComboRelationWithRecord, error) {
	var item domain.OMPSKUComboRelationWithRecord
	var relationCallLogID sql.NullInt64
	var relationRaw sql.NullString
	var recCombo, recName, recShortName, recIID, recSource sql.NullString
	var recEnabled sql.NullBool
	var recCost, recSale sql.NullFloat64
	var recModified, recLastSynced, recCreated, recUpdated sql.NullTime
	var recRaw sql.NullString
	if err := scanner.Scan(
		&item.Relation.ID,
		&item.Relation.ComboSKUCode,
		&item.Relation.ChildSKUCode,
		&item.Relation.Quantity,
		&item.Relation.Source,
		&relationCallLogID,
		&relationRaw,
		&item.Relation.FirstSeenAt,
		&item.Relation.LastSeenAt,
		&item.Relation.CreatedAt,
		&item.Relation.UpdatedAt,
		&recCombo,
		&recName,
		&recShortName,
		&recIID,
		&recEnabled,
		&recCost,
		&recSale,
		&recModified,
		&recSource,
		&recRaw,
		&recLastSynced,
		&recCreated,
		&recUpdated,
	); err != nil {
		return nil, fmt.Errorf("scan sku combo relation: %w", err)
	}
	item.Relation.SourceCallLogID = fromNullInt64(relationCallLogID)
	if relationRaw.Valid {
		item.Relation.RawPayloadJSON = []byte(relationRaw.String)
	}
	if recCombo.Valid && strings.TrimSpace(recCombo.String) != "" {
		item.Record = &domain.OMPSKUComboRecord{
			ComboSKUCode:   recCombo.String,
			Name:           recName.String,
			ShortName:      recShortName.String,
			ERPIID:         recIID.String,
			Enabled:        fromNullBool(recEnabled),
			CostPrice:      fromNullFloat64(recCost),
			SalePrice:      fromNullFloat64(recSale),
			ModifiedAt:     fromNullTime(recModified),
			Source:         recSource.String,
			RawPayloadJSON: []byte(recRaw.String),
			LastSyncedAt:   recLastSynced.Time,
			CreatedAt:      recCreated.Time,
			UpdatedAt:      recUpdated.Time,
		}
		domain.HydrateOMPSKUComboRecordDerived(item.Record)
	}
	return &item, nil
}

func scanComboSyncState(scanner comboRelationScanner) (*domain.OMPSKUComboSyncState, error) {
	var item domain.OMPSKUComboSyncState
	var lastSuccess, nextRetry sql.NullTime
	if err := scanner.Scan(
		&item.ID,
		&item.WindowBegin,
		&item.WindowEnd,
		&item.PageIndex,
		&item.PageSize,
		&item.Status,
		&lastSuccess,
		&nextRetry,
		&item.LastError,
		&item.ProcessedItems,
		&item.CreatedAt,
		&item.UpdatedAt,
	); err != nil {
		return nil, err
	}
	item.LastSuccessAt = fromNullTime(lastSuccess)
	item.NextRetryAt = fromNullTime(nextRetry)
	return &item, nil
}

func nonZeroTimePtr(value time.Time) *time.Time {
	if value.IsZero() {
		return nil
	}
	return &value
}

func nonEmptyStringPtr(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func toNullBool(value *bool) sql.NullBool {
	if value == nil {
		return sql.NullBool{}
	}
	return sql.NullBool{Bool: *value, Valid: true}
}

func fromNullBool(value sql.NullBool) *bool {
	if !value.Valid {
		return nil
	}
	return &value.Bool
}

func firstNonEmptySQLString(values ...string) string {
	for _, value := range values {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed
		}
	}
	return ""
}

func truncateSQLString(value string, limit int) string {
	value = strings.TrimSpace(value)
	if limit <= 0 || len(value) <= limit {
		return value
	}
	return value[:limit]
}
