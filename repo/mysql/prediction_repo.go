package mysqlrepo

import (
	"context"
	"database/sql"
	"fmt"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

type predictionRepo struct{ db *DB }

func NewPredictionRepo(db *DB) repo.PredictionRepo {
	return &predictionRepo{db: db}
}

func (r *predictionRepo) SearchSuggestions(ctx context.Context, actor domain.RequestActor, q, scope string, limit int) ([]domain.PredictionSuggestion, error) {
	limit = normalizePredictionLimit(limit)
	q = strings.TrimSpace(q)
	scope = strings.TrimSpace(scope)
	if scope == "" {
		scope = "all"
	}
	if q == "" {
		return r.recentActorSuggestions(ctx, actor, limit)
	}
	out := make([]domain.PredictionSuggestion, 0, limit)
	remaining := func() int { return limit - len(out) }
	if scope == "all" || scope == "tasks" {
		items, err := r.searchTaskSuggestions(ctx, q, remaining())
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	if remaining() > 0 && (scope == "all" || scope == "assets") {
		items, err := r.searchAssetSuggestions(ctx, q, remaining())
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	if remaining() > 0 && (scope == "all" || scope == "products") {
		items, err := r.searchProductSuggestions(ctx, q, remaining())
		if err != nil {
			return nil, err
		}
		out = append(out, items...)
	}
	return out, nil
}

func (r *predictionRepo) TaskCreateSuggestions(ctx context.Context, actor domain.RequestActor, keyword, taskType string, limit int) ([]domain.PredictionSuggestion, error) {
	limit = normalizePredictionLimit(limit)
	keyword = strings.TrimSpace(keyword)
	taskType = strings.TrimSpace(taskType)
	where := []string{"1=1"}
	args := make([]interface{}, 0, 10)
	if keyword != "" {
		like := "%" + keyword + "%"
		where = append(where, `(COALESCE(t.product_name_snapshot, '') LIKE ?
			OR COALESCE(td.category_name, '') LIKE ?
			OR COALESCE(td.category, '') LIKE ?
			OR COALESCE(td.material, '') LIKE ?
			OR COALESCE(td.spec_text, '') LIKE ?
			OR COALESCE(td.size_text, '') LIKE ?
			OR COALESCE(td.process, '') LIKE ?
			OR COALESCE(td.demand_text, '') LIKE ?
			OR COALESCE(td.copy_text, '') LIKE ?)`)
		args = append(args, repeatInterface(like, 9)...)
	} else {
		where = append(where, "t.created_at >= DATE_SUB(NOW(), INTERVAL 120 DAY)")
	}
	if taskType != "" && taskType != "all" {
		where = append(where, "t.task_type = ?")
		args = append(args, taskType)
	}
	args = append(args, limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT
		  COALESCE(NULLIF(td.category_name, ''), NULLIF(td.category, ''), '未分类') AS category_name,
		  COALESCE(td.category_code, '') AS category_code,
		  COALESCE(td.material, '') AS material,
		  COALESCE(td.spec_text, '') AS spec_text,
		  COALESCE(td.size_text, '') AS size_text,
		  COALESCE(td.process, '') AS process_text,
		  COALESCE(t.task_type, '') AS task_type,
		  COUNT(*) AS use_count,
		  MAX(t.created_at) AS last_used_at
		FROM tasks t
		LEFT JOIN task_details td ON td.task_id = t.id
		WHERE `+strings.Join(where, " AND ")+`
		GROUP BY category_name, category_code, material, spec_text, size_text, process_text, task_type
		ORDER BY use_count DESC, last_used_at DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("query task create prediction suggestions: %w", err)
	}
	defer rows.Close()

	out := make([]domain.PredictionSuggestion, 0, limit)
	for rows.Next() {
		var categoryName, categoryCode, material, specText, sizeText, processText, rowTaskType string
		var count int
		var lastUsed sql.NullTime
		if err := rows.Scan(&categoryName, &categoryCode, &material, &specText, &sizeText, &processText, &rowTaskType, &count, &lastUsed); err != nil {
			return nil, fmt.Errorf("scan task create prediction suggestion: %w", err)
		}
		titleParts := compactStrings(categoryName, material, sizeText, processText)
		if len(titleParts) == 0 {
			titleParts = []string{"近期常用创建参数"}
		}
		detail := "近期相似任务使用 " + strconv.Itoa(count) + " 次"
		if lastUsed.Valid {
			detail += "，最近一次 " + lastUsed.Time.Format("2006-01-02")
		}
		out = append(out, domain.PredictionSuggestion{
			ID:          predictionID("task_create", categoryCode, categoryName, material, specText, processText, rowTaskType),
			Type:        "task_create",
			Title:       strings.Join(titleParts, " / "),
			Detail:      detail,
			ActionLabel: "作为填写参考",
			ActionType:  "prefill_hint",
			TargetType:  "task_template",
			Confidence:  predictionConfidence(count),
			Source:      "历史任务",
			Metadata: compactMetadata(map[string]string{
				"category_name": categoryName,
				"category_code": categoryCode,
				"material":      material,
				"spec_text":     specText,
				"size_text":     sizeText,
				"process":       processText,
				"task_type":     rowTaskType,
			}),
		})
	}
	return out, rows.Err()
}

func (r *predictionRepo) TaskNextActionSuggestions(ctx context.Context, actor domain.RequestActor, taskID int64, limit int) ([]domain.PredictionSuggestion, error) {
	limit = normalizePredictionLimit(limit)
	if taskID <= 0 {
		return []domain.PredictionSuggestion{}, nil
	}
	var taskNo, productName, taskStatus, taskType, filingStatus string
	var erpRequired bool
	var costPrice sql.NullString
	err := r.db.db.QueryRowContext(ctx, `
		SELECT t.task_no, COALESCE(t.product_name_snapshot, ''), COALESCE(t.task_status, ''),
		       COALESCE(t.task_type, ''), COALESCE(td.filing_status, ''), COALESCE(td.erp_sync_required, 0),
		       CAST(td.cost_price AS CHAR)
		FROM tasks t
		LEFT JOIN task_details td ON td.task_id = t.id
		WHERE t.id = ?
		LIMIT 1`, taskID).Scan(&taskNo, &productName, &taskStatus, &taskType, &filingStatus, &erpRequired, &costPrice)
	if err == sql.ErrNoRows {
		return []domain.PredictionSuggestion{}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query task next action task: %w", err)
	}
	assetSummary, err := r.taskAssetSummary(ctx, taskID)
	if err != nil {
		return nil, err
	}
	modules, err := r.taskModuleStates(ctx, taskID)
	if err != nil {
		return nil, err
	}
	out := make([]domain.PredictionSuggestion, 0, limit)
	add := func(item domain.PredictionSuggestion) {
		if len(out) < limit {
			if item.TargetType == "" {
				item.TargetType = "task"
			}
			if item.TargetID == "" {
				item.TargetID = strconv.FormatInt(taskID, 10)
			}
			if item.Confidence == "" {
				item.Confidence = "medium"
			}
			item.Metadata = compactMetadata(item.Metadata)
			out = append(out, item)
		}
	}
	if assetSummary.approvedDelivery > 0 {
		add(domain.PredictionSuggestion{
			ID:          predictionID("task_next", taskNo, "asset_ready"),
			Type:        "task_next_action",
			Title:       "已有可直接使用的最终成品图",
			Detail:      fmt.Sprintf("%s 当前有 %d 个已审核成品文件，仓库可从资产中心取图。", taskNo, assetSummary.approvedDelivery),
			ActionLabel: "查看资产",
			ActionType:  "open_task_assets",
			Source:      "资产状态",
		})
	} else if assetSummary.pendingDelivery > 0 {
		add(domain.PredictionSuggestion{
			ID:          predictionID("task_next", taskNo, "asset_pending"),
			Type:        "task_next_action",
			Title:       "成品图已提交，等待审核确认",
			Detail:      fmt.Sprintf("检测到 %d 个待审核成品文件，建议审核人员优先处理。", assetSummary.pendingDelivery),
			ActionLabel: "进入审核",
			ActionType:  "open_task",
			Source:      "资产状态",
		})
	} else {
		add(domain.PredictionSuggestion{
			ID:          predictionID("task_next", taskNo, "no_delivery"),
			Type:        "task_next_action",
			Title:       "暂未发现最终成品图",
			Detail:      "任务还没有可供仓库使用的成品文件，建议设计环节先提交最终文件。",
			ActionLabel: "进入任务",
			ActionType:  "open_task",
			Source:      "资产状态",
		})
	}
	if assetSummary.rejectedDelivery > 0 {
		add(domain.PredictionSuggestion{
			ID:          predictionID("task_next", taskNo, "asset_rejected"),
			Type:        "task_next_action",
			Title:       "存在审核未通过的成品版本",
			Detail:      fmt.Sprintf("检测到 %d 个被打回版本，建议先查看打回原因再继续。", assetSummary.rejectedDelivery),
			ActionLabel: "查看版本",
			ActionType:  "open_task_assets",
			Source:      "审核结果",
		})
	}
	if erpRequired || filingStatus == "filing_failed" || filingStatus == "pending_filing" {
		detail := "ERP 同步仍有待处理项"
		if strings.TrimSpace(costPrice.String) != "" {
			detail += "，当前成本记录为 " + strings.TrimSpace(costPrice.String)
		}
		add(domain.PredictionSuggestion{
			ID:          predictionID("task_next", taskNo, "erp_sync"),
			Type:        "task_next_action",
			Title:       "需要关注 ERP 同步状态",
			Detail:      detail,
			ActionLabel: "查看同步",
			ActionType:  "open_task_erp",
			Source:      "ERP 建档",
		})
	}
	if strings.EqualFold(taskStatus, "PendingWarehouseReceive") || strings.EqualFold(taskStatus, "PendingClose") {
		add(domain.PredictionSuggestion{
			ID:          predictionID("task_next", taskNo, "warehouse"),
			Type:        "task_next_action",
			Title:       "仓库环节可优先处理",
			Detail:      "任务已到仓库/结单阶段，确认可用资产后可推进收货或结单。",
			ActionLabel: "进入任务",
			ActionType:  "open_task",
			Source:      "任务状态",
		})
	}
	if state := modules["design"]; state != "" {
		add(domain.PredictionSuggestion{
			ID:          predictionID("task_next", taskNo, "design_state", state),
			Type:        "task_next_action",
			Title:       "设计环节状态：" + state,
			Detail:      compactSentence(productName, taskType),
			ActionLabel: "查看设计",
			ActionType:  "open_task",
			Source:      "流程模块",
			Confidence:  "low",
		})
	}
	return out, nil
}

func (r *predictionRepo) AssetSuggestions(ctx context.Context, actor domain.RequestActor, q string, limit int) ([]domain.PredictionSuggestion, error) {
	return r.searchAssetSuggestions(ctx, strings.TrimSpace(q), normalizePredictionLimit(limit))
}

func (r *predictionRepo) ManagementSuggestions(ctx context.Context, actor domain.RequestActor, from, to time.Time, limit int) ([]domain.PredictionSuggestion, error) {
	limit = normalizePredictionLimit(limit)
	if from.IsZero() {
		from = time.Now().AddDate(0, 0, -7)
	}
	if to.IsZero() {
		to = time.Now()
	}
	counts := map[string]int64{}
	queries := map[string]string{
		"created": `SELECT COUNT(*) FROM tasks WHERE created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)`,
		"stale":   `SELECT COUNT(*) FROM tasks WHERE task_status NOT IN ('Completed', 'Cancelled') AND updated_at < DATE_SUB(NOW(), INTERVAL 48 HOUR)`,
		"rejected_assets": `SELECT COUNT(*) FROM task_assets
			WHERE flow_review_status = 'rejected'
			  AND created_at >= ? AND created_at < DATE_ADD(?, INTERVAL 1 DAY)`,
		"erp_pending": `SELECT COUNT(*) FROM task_details
			WHERE erp_sync_required = 1
			  AND updated_at >= ? AND updated_at < DATE_ADD(?, INTERVAL 1 DAY)`,
	}
	for key, query := range queries {
		var n int64
		var err error
		if key == "stale" {
			err = r.db.db.QueryRowContext(ctx, query).Scan(&n)
		} else {
			err = r.db.db.QueryRowContext(ctx, query, from, to).Scan(&n)
		}
		if err != nil {
			return nil, fmt.Errorf("query management prediction %s: %w", key, err)
		}
		counts[key] = n
	}
	out := make([]domain.PredictionSuggestion, 0, limit)
	add := func(item domain.PredictionSuggestion) {
		if len(out) < limit {
			item.Metadata = compactMetadata(item.Metadata)
			out = append(out, item)
		}
	}
	if counts["stale"] > 0 {
		add(domain.PredictionSuggestion{
			ID:          predictionID("management", "stale", strconv.FormatInt(counts["stale"], 10)),
			Type:        "management",
			Title:       "存在 48 小时未推进任务",
			Detail:      fmt.Sprintf("当前有 %d 个进行中任务超过 48 小时未更新，建议按负责人筛选追踪。", counts["stale"]),
			ActionLabel: "查看任务中心",
			ActionType:  "open_task_center",
			TargetType:  "task_center",
			Confidence:  "high",
			Source:      "任务更新",
		})
	}
	if counts["rejected_assets"] > 0 {
		add(domain.PredictionSuggestion{
			ID:          predictionID("management", "rejected_assets", strconv.FormatInt(counts["rejected_assets"], 10)),
			Type:        "management",
			Title:       "近期存在成品图打回",
			Detail:      fmt.Sprintf("所选周期内有 %d 个成品文件被标记为审核未通过，可结合设计人员打回率查看原因。", counts["rejected_assets"]),
			ActionLabel: "查看绩效看板",
			ActionType:  "open_kpi",
			TargetType:  "data_center",
			Confidence:  "medium",
			Source:      "审核结果",
		})
	}
	if counts["erp_pending"] > 0 {
		add(domain.PredictionSuggestion{
			ID:          predictionID("management", "erp_pending", strconv.FormatInt(counts["erp_pending"], 10)),
			Type:        "management",
			Title:       "ERP 同步待处理",
			Detail:      fmt.Sprintf("所选周期内仍有 %d 条任务明细需要 ERP 同步或重试。", counts["erp_pending"]),
			ActionLabel: "查看同步记录",
			ActionType:  "open_integration_logs",
			TargetType:  "logs",
			Confidence:  "medium",
			Source:      "ERP 建档",
		})
	}
	add(domain.PredictionSuggestion{
		ID:          predictionID("management", "created", strconv.FormatInt(counts["created"], 10), from.Format("2006-01-02"), to.Format("2006-01-02")),
		Type:        "management",
		Title:       "本周期任务创建量：" + strconv.FormatInt(counts["created"], 10),
		Detail:      "可结合运营人员表查看任务发起量和平均发起间隔。",
		ActionLabel: "查看运营 KPI",
		ActionType:  "open_kpi_ops",
		TargetType:  "data_center",
		Confidence:  "medium",
		Source:      "任务统计",
	})
	return out, nil
}

func (r *predictionRepo) recentActorSuggestions(ctx context.Context, actor domain.RequestActor, limit int) ([]domain.PredictionSuggestion, error) {
	if actor.ID <= 0 {
		return []domain.PredictionSuggestion{}, nil
	}
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT COALESCE(task_id, 0), COALESCE(asset_id, 0), COALESCE(resource_type, ''), COALESCE(resource_id, ''),
		       COALESCE(page_name, ''), COALESCE(action, ''), occurred_at
		FROM workflow_trace_events
		WHERE actor_id = ?
		  AND event_source = 'frontend'
		  AND (task_id IS NOT NULL OR asset_id IS NOT NULL OR NULLIF(resource_id, '') IS NOT NULL)
		ORDER BY occurred_at DESC, id DESC
		LIMIT ?`, actor.ID, limit)
	if err != nil {
		return nil, fmt.Errorf("query recent actor prediction suggestions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.PredictionSuggestion, 0, limit)
	seen := map[string]struct{}{}
	for rows.Next() {
		var taskID, assetID int64
		var resourceType, resourceID, pageName, action string
		var occurredAt time.Time
		if err := rows.Scan(&taskID, &assetID, &resourceType, &resourceID, &pageName, &action, &occurredAt); err != nil {
			return nil, fmt.Errorf("scan recent actor prediction suggestion: %w", err)
		}
		targetType, targetID := "resource", resourceID
		if taskID > 0 {
			targetType, targetID = "task", strconv.FormatInt(taskID, 10)
		} else if assetID > 0 {
			targetType, targetID = "asset", strconv.FormatInt(assetID, 10)
		}
		key := targetType + ":" + targetID
		if targetID == "" {
			key = action + ":" + pageName
		}
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		title := firstNonEmptyString(pageName, action, "最近访问")
		out = append(out, domain.PredictionSuggestion{
			ID:          predictionID("recent", key),
			Type:        "recent",
			Title:       title,
			Detail:      "你最近在 " + occurredAt.Format("2006-01-02 15:04") + " 访问过",
			ActionLabel: "继续查看",
			ActionType:  "open",
			TargetType:  targetType,
			TargetID:    targetID,
			Confidence:  "medium",
			Source:      "个人行为",
			Metadata: compactMetadata(map[string]string{
				"resource_type": resourceType,
				"resource_id":   resourceID,
				"action":        action,
			}),
		})
	}
	return out, rows.Err()
}

func (r *predictionRepo) searchTaskSuggestions(ctx context.Context, q string, limit int) ([]domain.PredictionSuggestion, error) {
	if limit <= 0 {
		return []domain.PredictionSuggestion{}, nil
	}
	like := "%" + strings.TrimSpace(q) + "%"
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT id, task_no, COALESCE(product_name_snapshot, ''), COALESCE(task_status, ''),
		       COALESCE(task_type, ''), COALESCE(sku_code, ''), COALESCE(primary_sku_code, ''), updated_at
		FROM tasks
		WHERE task_no LIKE ?
		   OR COALESCE(product_name_snapshot, '') LIKE ?
		   OR COALESCE(sku_code, '') LIKE ?
		   OR COALESCE(primary_sku_code, '') LIKE ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`, like, like, like, like, limit)
	if err != nil {
		return nil, fmt.Errorf("query task prediction suggestions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.PredictionSuggestion, 0, limit)
	for rows.Next() {
		var id int64
		var taskNo, productName, status, taskType, sku, primarySKU string
		var updatedAt time.Time
		if err := rows.Scan(&id, &taskNo, &productName, &status, &taskType, &sku, &primarySKU, &updatedAt); err != nil {
			return nil, fmt.Errorf("scan task prediction suggestion: %w", err)
		}
		out = append(out, domain.PredictionSuggestion{
			ID:          predictionID("search_task", strconv.FormatInt(id, 10)),
			Type:        "search",
			Title:       firstNonEmptyString(productName, taskNo),
			Detail:      compactSentence(taskNo, status, firstNonEmptyString(sku, primarySKU)),
			ActionLabel: "打开任务",
			ActionType:  "open_task",
			TargetType:  "task",
			TargetID:    strconv.FormatInt(id, 10),
			Confidence:  "high",
			Source:      "任务",
			Metadata: compactMetadata(map[string]string{
				"task_no":    taskNo,
				"task_type":  taskType,
				"sku_code":   sku,
				"updated_at": updatedAt.Format(time.RFC3339),
			}),
		})
	}
	return out, rows.Err()
}

func (r *predictionRepo) searchAssetSuggestions(ctx context.Context, q string, limit int) ([]domain.PredictionSuggestion, error) {
	if limit <= 0 {
		return []domain.PredictionSuggestion{}, nil
	}
	activeAssetWhere := taskAssetsActiveSQL(ctx, r.db.db, "ta")
	args := make([]interface{}, 0, 12)
	where := []string{activeAssetWhere}
	if q != "" {
		like := "%" + strings.TrimSpace(q) + "%"
		where = append(where, `(COALESCE(ta.file_name, '') LIKE ?
			OR COALESCE(ta.original_filename, '') LIKE ?
			OR COALESCE(t.task_no, '') LIKE ?
			OR COALESCE(t.product_name_snapshot, '') LIKE ?
			OR COALESCE(t.sku_code, '') LIKE ?
			OR COALESCE(t.primary_sku_code, '') LIKE ?)`)
		args = append(args, repeatInterface(like, 6)...)
	}
	args = append(args, limit)
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT ta.id AS task_asset_id, ta.asset_id AS asset_id,
		       COALESCE(ta.file_name, ''), COALESCE(ta.original_filename, ''),
		       COALESCE(ta.asset_type, ''), COALESCE(ta.flow_review_status, ''),
		       COALESCE(ta.task_id, 0), COALESCE(t.task_no, ''), COALESCE(t.product_name_snapshot, ''),
		       COALESCE(ta.created_at, NOW())
		FROM task_assets ta
		LEFT JOIN design_assets da ON da.id = ta.asset_id
		LEFT JOIN tasks t ON t.id = ta.task_id
		WHERE `+strings.Join(where, " AND ")+`
		  AND ta.asset_id IS NOT NULL
		  AND ta.id = COALESCE(da.current_version_id, (
		      SELECT ta2.id FROM task_assets ta2 WHERE ta2.asset_id = da.id ORDER BY ta2.asset_version_no DESC, ta2.id DESC LIMIT 1
		    ))
		ORDER BY CASE COALESCE(ta.flow_review_status, '')
		           WHEN 'approved' THEN 0
		           WHEN 'pending_review' THEN 1
		           WHEN 'not_applicable' THEN 2
		           ELSE 3
		         END,
		         ta.created_at DESC, ta.id DESC
		LIMIT ?`, args...)
	if err != nil {
		return nil, fmt.Errorf("query asset prediction suggestions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.PredictionSuggestion, 0, limit)
	for rows.Next() {
		var taskAssetID, assetID, taskID int64
		var fileName, originalName, assetType, flowStatus, taskNo, productName string
		var createdAt time.Time
		if err := rows.Scan(&taskAssetID, &assetID, &fileName, &originalName, &assetType, &flowStatus, &taskID, &taskNo, &productName, &createdAt); err != nil {
			return nil, fmt.Errorf("scan asset prediction suggestion: %w", err)
		}
		stateLabel := predictionAssetStateLabel(flowStatus)
		out = append(out, domain.PredictionSuggestion{
			ID:          predictionID("asset", strconv.FormatInt(assetID, 10)),
			Type:        "asset",
			Title:       firstNonEmptyString(originalName, fileName),
			Detail:      compactSentence(stateLabel, taskNo, productName),
			ActionLabel: "打开资产",
			ActionType:  "open_asset",
			TargetType:  "asset",
			TargetID:    strconv.FormatInt(assetID, 10),
			Confidence:  predictionAssetConfidence(flowStatus),
			Source:      "资产中心",
			Metadata: compactMetadata(map[string]string{
				"task_asset_id":      int64String(taskAssetID),
				"asset_id":           int64String(assetID),
				"task_id":            int64String(taskID),
				"task_no":            taskNo,
				"asset_type":         assetType,
				"flow_review_status": flowStatus,
				"created_at":         createdAt.Format(time.RFC3339),
			}),
		})
	}
	return out, rows.Err()
}

func (r *predictionRepo) searchProductSuggestions(ctx context.Context, q string, limit int) ([]domain.PredictionSuggestion, error) {
	if limit <= 0 || !r.tableExists(ctx, "products") {
		return []domain.PredictionSuggestion{}, nil
	}
	like := "%" + strings.TrimSpace(q) + "%"
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT COALESCE(sku_code, ''), COALESCE(product_name, ''), COALESCE(category, '')
		FROM products
		WHERE COALESCE(sku_code, '') LIKE ?
		   OR COALESCE(product_name, '') LIKE ?
		   OR COALESCE(category, '') LIKE ?
		ORDER BY updated_at DESC, id DESC
		LIMIT ?`, like, like, like, limit)
	if err != nil {
		return nil, fmt.Errorf("query product prediction suggestions: %w", err)
	}
	defer rows.Close()
	out := make([]domain.PredictionSuggestion, 0, limit)
	for rows.Next() {
		var sku, productName, category string
		if err := rows.Scan(&sku, &productName, &category); err != nil {
			return nil, fmt.Errorf("scan product prediction suggestion: %w", err)
		}
		target := firstNonEmptyString(sku, productName)
		out = append(out, domain.PredictionSuggestion{
			ID:          predictionID("product", target),
			Type:        "product",
			Title:       firstNonEmptyString(productName, sku),
			Detail:      compactSentence(sku, category),
			ActionLabel: "作为商品参考",
			ActionType:  "use_product_hint",
			TargetType:  "product",
			TargetID:    target,
			Confidence:  "medium",
			Source:      "ERP 商品",
			Metadata: compactMetadata(map[string]string{
				"sku_code": sku,
				"category": category,
			}),
		})
	}
	return out, rows.Err()
}

type predictionTaskAssetSummary struct {
	approvedDelivery int
	pendingDelivery  int
	rejectedDelivery int
}

func (r *predictionRepo) taskAssetSummary(ctx context.Context, taskID int64) (predictionTaskAssetSummary, error) {
	activeAssetWhere := taskAssetsActiveSQL(ctx, r.db.db, "ta")
	var out predictionTaskAssetSummary
	err := r.db.db.QueryRowContext(ctx, `
		SELECT
		  COALESCE(SUM(CASE WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return') AND ta.flow_review_status = 'approved' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return') AND ta.flow_review_status = 'pending_review' THEN 1 ELSE 0 END), 0),
		  COALESCE(SUM(CASE WHEN ta.asset_type IN ('delivery', 'draft', 'revised', 'final', 'outsource_return') AND ta.flow_review_status = 'rejected' THEN 1 ELSE 0 END), 0)
		FROM task_assets ta
		WHERE ta.task_id = ?
		  AND `+activeAssetWhere, taskID).Scan(&out.approvedDelivery, &out.pendingDelivery, &out.rejectedDelivery)
	if err != nil {
		return out, fmt.Errorf("query task asset prediction summary: %w", err)
	}
	return out, nil
}

func (r *predictionRepo) taskModuleStates(ctx context.Context, taskID int64) (map[string]string, error) {
	rows, err := r.db.db.QueryContext(ctx, `
		SELECT module_key, state
		FROM task_modules
		WHERE task_id = ?`, taskID)
	if err != nil {
		return nil, fmt.Errorf("query task module prediction states: %w", err)
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var key, state string
		if err := rows.Scan(&key, &state); err != nil {
			return nil, fmt.Errorf("scan task module prediction state: %w", err)
		}
		out[key] = state
	}
	return out, rows.Err()
}

func normalizePredictionLimit(limit int) int {
	if limit <= 0 {
		return 8
	}
	if limit > 20 {
		return 20
	}
	return limit
}

func repeatInterface(value interface{}, count int) []interface{} {
	out := make([]interface{}, 0, count)
	for i := 0; i < count; i++ {
		out = append(out, value)
	}
	return out
}

func compactStrings(values ...string) []string {
	out := make([]string, 0, len(values))
	seen := map[string]struct{}{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	return out
}

func compactSentence(values ...string) string {
	return strings.Join(compactStrings(values...), " · ")
}

func compactMetadata(values map[string]string) map[string]string {
	out := map[string]string{}
	for key, value := range values {
		key = strings.TrimSpace(key)
		value = strings.TrimSpace(value)
		if key == "" || value == "" {
			continue
		}
		out[key] = value
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func predictionID(parts ...string) string {
	normalized := compactStrings(parts...)
	if len(normalized) == 0 {
		return "prediction"
	}
	return strings.ToLower(strings.ReplaceAll(strings.Join(normalized, "-"), " ", "_"))
}

func predictionConfidence(count int) string {
	switch {
	case count >= 10:
		return "high"
	case count >= 3:
		return "medium"
	default:
		return "low"
	}
}

func predictionAssetStateLabel(status string) string {
	switch strings.TrimSpace(status) {
	case "approved":
		return "可直接使用"
	case "pending_review":
		return "待审核"
	case "rejected":
		return "审核未通过"
	case "superseded":
		return "历史版本"
	case "cleaned":
		return "已清理"
	default:
		return "不进入审核流"
	}
}

func predictionAssetConfidence(status string) string {
	switch strings.TrimSpace(status) {
	case "approved":
		return "high"
	case "pending_review":
		return "medium"
	default:
		return "low"
	}
}

func int64String(value int64) string {
	if value <= 0 {
		return ""
	}
	return strconv.FormatInt(value, 10)
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

func (r *predictionRepo) tableExists(ctx context.Context, table string) bool {
	table = strings.TrimSpace(table)
	if table == "" {
		return false
	}
	var found string
	err := r.db.db.QueryRowContext(ctx, `
		SELECT table_name
		  FROM information_schema.tables
		 WHERE table_schema = DATABASE()
		   AND table_name = ?
		 LIMIT 1`, table).Scan(&found)
	return err == nil
}
