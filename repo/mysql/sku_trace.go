package mysqlrepo

import (
	"context"
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
	return nil
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
