package orders

import (
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"

	"workflow/internal/oa/platform"
)

var costAmount = regexp.MustCompile(`^(0|[1-9][0-9]{0,9})\.[0-9]{2}$`)

const skuCostSchema = `CREATE TABLE IF NOT EXISTS oa_sku_cost_audit (
 id BIGINT AUTO_INCREMENT PRIMARY KEY, sku_item_id BIGINT NOT NULL, actor_id BIGINT NOT NULL,
 before_cost DECIMAL(14,4) NULL, after_cost DECIMAL(14,4) NOT NULL,
 reason VARCHAR(255) NOT NULL, created_at TIMESTAMP(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
 KEY sku_revision (sku_item_id,id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4`

func (s *server) skuCost(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if actor.ID <= 0 || (!actor.Actions["catalog.manage"] || !actor.Actions["erp.manage"]) {
		platform.Error(w, 403, "需要成本维护和 ERP 管理权限")
		return
	}
	if s.db == nil {
		platform.Error(w, 503, "沙箱成本服务暂时不可用")
		return
	}
	ctx := r.Context()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		platform.Error(w, 503, "沙箱成本服务暂时不可用")
		return
	}
	defer tx.Rollback()
	var database string
	if err = tx.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil || database != "yongbo_oa" {
		platform.Error(w, 503, "成本维护仅适用于 OA 验证库")
		return
	}
	var itemID, taskID int64
	var before sql.NullString
	var modified string
	err = tx.QueryRowContext(ctx, `SELECT s.id,t.id,CAST(s.cost_price AS CHAR),CAST(s.updated_at AS CHAR)
  FROM task_sku_items s INNER JOIN tasks t ON t.id=s.task_id WHERE t.id=? AND s.sku_code=? AND BINARY s.sku_code=BINARY ?
  AND (?=1 OR (t.creator_id=? AND t.owner_department_id=? AND (t.owner_team_id IS NULL OR t.owner_team_id=?))) FOR UPDATE`,
		r.PathValue("task_id"), r.PathValue("sku"), r.PathValue("sku"), actor.SuperAdmin, actor.ID, actor.DepartmentID, actor.TeamID).Scan(&itemID, &taskID, &before, &modified)
	if errors.Is(err, sql.ErrNoRows) {
		platform.Error(w, 404, "未找到可维护的 SKU")
		return
	}
	if err != nil {
		platform.Error(w, 503, "沙箱成本暂时不可用")
		return
	}
	var latest int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(id),0) FROM oa_sku_cost_audit WHERE sku_item_id=?`, itemID).Scan(&latest); err != nil {
		platform.Error(w, 503, "成本版本暂时不可用")
		return
	}
	version := costVersion(itemID, latest, before, modified)
	var amount any
	if before.Valid {
		amount = displayUnitCost(before.String)
	}
	if r.Method == http.MethodPatch {
		var request struct {
			Version string `json:"version"`
			Cost    string `json:"cost_price"`
			Reason  string `json:"reason"`
		}
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF || !costAmount.MatchString(request.Cost) || strings.TrimSpace(request.Reason) == "" || len([]rune(request.Reason)) > 255 {
			platform.Error(w, 400, "请填写两位小数单价和维护原因")
			return
		}
		if request.Version != version {
			platform.Error(w, 409, "成本版本已变化，请刷新后重试")
			return
		}
		if _, err = tx.ExecContext(ctx, `UPDATE task_sku_items SET cost_price=?,manual_cost_override=1,manual_cost_override_reason=?,override_actor=?,override_at=CURRENT_TIMESTAMP,updated_at=CURRENT_TIMESTAMP WHERE id=?`, request.Cost, request.Reason, actor.ID, itemID); err != nil {
			platform.Error(w, 503, "成本暂时无法维护")
			return
		}
		if _, err = tx.ExecContext(ctx, `UPDATE task_details d INNER JOIN tasks t ON t.id=d.task_id INNER JOIN task_sku_items s ON s.id=? SET d.cost_price=s.cost_price,d.manual_cost_override=s.manual_cost_override,d.manual_cost_override_reason=s.manual_cost_override_reason,d.override_actor=s.override_actor,d.override_at=s.override_at,d.updated_at=CURRENT_TIMESTAMP WHERE t.id=? AND (t.primary_sku_code=s.sku_code OR t.sku_code=s.sku_code OR t.batch_item_count=1)`, itemID, taskID); err != nil {
			platform.Error(w, 503, "任务成本暂时无法维护")
			return
		}
		audit, err := tx.ExecContext(ctx, `INSERT INTO oa_sku_cost_audit(sku_item_id,actor_id,before_cost,after_cost,reason) VALUES(?,?,?,?,?)`, itemID, actor.ID, before, request.Cost, request.Reason)
		if err != nil {
			platform.Error(w, 503, "成本审计暂时无法保存")
			return
		}
		latest, err = audit.LastInsertId()
		if err != nil {
			platform.Error(w, 503, "成本审计暂时无法保存")
			return
		}
		if err = tx.QueryRowContext(ctx, `SELECT CAST(cost_price AS CHAR),CAST(updated_at AS CHAR) FROM task_sku_items WHERE id=?`, itemID).Scan(&before, &modified); err != nil {
			platform.Error(w, 503, "成本版本暂时不可用")
			return
		}
		version = costVersion(itemID, latest, before, modified)
		amount = displayUnitCost(before.String)
	}
	if err = tx.Commit(); err != nil {
		platform.Error(w, 503, "成本事务暂时无法完成")
		return
	}
	platform.JSON(w, 200, map[string]any{"task_id": taskID, "sku": r.PathValue("sku"), "cost_price": amount, "version": version, "erp_observation": nil, "erp_status": "not_checked"})
}

func costVersion(item, latest int64, price sql.NullString, modified string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(fmt.Sprintf("%d:%d:%t:%s:%s", item, latest, price.Valid, price.String, modified))))
}

func displayUnitCost(stored string) string {
	dot := strings.IndexByte(stored, '.')
	if dot >= 0 {
		for len(stored)-dot > 3 && strings.HasSuffix(stored, "0") {
			stored = stored[:len(stored)-1]
		}
	}
	return stored
}
