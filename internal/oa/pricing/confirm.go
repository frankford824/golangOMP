package pricing

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"workflow/internal/oa/platform"
)

type selection struct {
	SKU   string `json:"sku_code"`
	Token string `json:"token"`
}
type confirmation struct {
	RequestID string      `json:"request_id"`
	Reason    string      `json:"reason"`
	Items     []selection `json:"items"`
}
type batch struct {
	ID   int64 `json:"batch_id"`
	Rows []row `json:"rows"`
}

func (a app) confirm(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !CanConfirm(actor) {
		platform.Error(w, http.StatusForbidden, "批量核价需要商品管理及 ERP 管理权限")
		return
	}
	var request confirmation
	if !decode(w, r, &request) {
		return
	}
	request.Reason = strings.TrimSpace(request.Reason)
	if !requestIDPattern.MatchString(request.RequestID) || request.Reason == "" || utf8.RuneCountInString(request.Reason) > 200 {
		platform.Error(w, 400, "请求编号无效或核价说明不在 1 到 200 字范围内")
		return
	}
	skus := make([]string, 0, len(request.Items))
	for _, entry := range request.Items {
		skus = append(skus, entry.SKU)
	}
	skus, ok := normalize(skus)
	if !ok || len(skus) != len(request.Items) {
		platform.Error(w, 400, "请选择 1 到 30 个不同的商品编码")
		return
	}
	for i, sku := range skus {
		request.Items[i].SKU = sku
		if !validToken(request.Items[i].Token) {
			platform.Error(w, 400, "核价依据凭据无效，请重新试算")
			return
		}
	}
	sort.Slice(request.Items, func(i, j int) bool { return request.Items[i].SKU < request.Items[j].SKU })
	for i, item := range request.Items {
		skus[i] = item.SKU
	}
	hash := digest(request)
	respond := func(verifyScope bool) bool {
		done, previous, err := a.readBatch(r.Context(), actor.ID, request.RequestID)
		switch {
		case errors.Is(err, sql.ErrNoRows):
			return false
		case err != nil:
			platform.Error(w, 503, "暂时无法核实批次状态，请保留原请求重试")
		case previous != hash:
			platform.Error(w, 409, "该请求编号已用于另一批核价内容")
		default:
			status := http.StatusOK
			if verifyScope {
				status = a.receiptScopeStatus(r, done)
			}
			switch status {
			case http.StatusForbidden:
				platform.Error(w, status, "当前账号无法读取该核价批次关联的任务")
			case http.StatusOK:
				platform.JSON(w, status, done)
			default:
				platform.Error(w, status, "暂时无法核实已确认批次的任务权限，请保留原请求重试")
			}
		}
		return true
	}
	if respond(true) {
		return
	}
	resolved := a.resolve(r, skus)
	for i, item := range resolved {
		if item.State != "ready" || item.Token != request.Items[i].Token || item.Suggested == nil {
			if !respond(true) {
				platform.Error(w, 409, "编码 "+item.SKU+" 的核价依据已变化或无法核实，请重新试算")
			}
			return
		}
	}
	if err := a.saveBatch(r.Context(), actor, request, hash, resolved); err != nil {
		if !respond(true) {
			var sqlError *mysql.MySQLError
			if errors.As(err, &sqlError) && (sqlError.Number == 1062 || sqlError.Number == 1213) {
				platform.Error(w, 409, "所选编码已有核价记录，请重新试算")
			} else {
				platform.Error(w, 503, "核价提交状态暂不可用，请保留原请求重试")
			}
		}
		return
	}
	if !respond(false) {
		platform.Error(w, 503, "暂时无法读取确认批次，请保留原请求重试")
	}
}

func (a app) receiptScopeStatus(r *http.Request, receipt batch) int {
	ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
	defer cancel()
	r = r.WithContext(ctx)
	tasks := make(chan int64, len(receipt.Rows))
	seen := make(map[int64]bool)
	for _, item := range receipt.Rows {
		if item.TaskID <= 0 {
			return http.StatusServiceUnavailable
		}
		if !seen[item.TaskID] {
			seen[item.TaskID] = true
			tasks <- item.TaskID
		}
	}
	close(tasks)
	var workers sync.WaitGroup
	var outcome atomic.Int32
	for range min(4, len(seen)) {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for id := range tasks {
				status := a.readCore(r, http.MethodGet, "/v1/tasks/"+strconv.FormatInt(id, 10), nil, nil)
				if status != http.StatusOK {
					if status == http.StatusNotFound || status == http.StatusForbidden {
						outcome.CompareAndSwap(0, http.StatusForbidden)
					} else {
						outcome.CompareAndSwap(0, http.StatusServiceUnavailable)
					}
					cancel()
					return
				}
			}
		}()
	}
	workers.Wait()
	if status := outcome.Load(); status != 0 {
		return int(status)
	}
	if ctx.Err() != nil {
		return http.StatusServiceUnavailable
	}
	return http.StatusOK
}

func validToken(token string) bool {
	if len(token) != 64 {
		return false
	}
	for _, c := range token {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			return false
		}
	}
	return true
}

func (a app) saveBatch(ctx context.Context, actor platform.Actor, request confirmation, hash string, rows []row) error {
	tx, err := a.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	header, err := tx.ExecContext(ctx, `INSERT INTO oa_sku_price_reviews (operator_id,operator_name,request_id,request_hash,reason) VALUES (?,?,?,?,?)`,
		actor.ID, actor.Name, request.RequestID, hash, request.Reason)
	if err != nil {
		return err
	}
	batchID, err := header.LastInsertId()
	if err != nil {
		return err
	}
	for _, item := range rows {
		var previous int64
		if item.Review != nil {
			previous = item.Review.ID
		}
		item.Review = nil
		evidence, err := json.Marshal(item)
		if err != nil {
			return err
		}
		if _, err := tx.ExecContext(ctx, `INSERT INTO oa_sku_price_reviews (batch_id,sku_code,previous_review_id,evidence_hash,evidence) VALUES (?,?,?,?,?)`, batchID, item.SKU, previous, item.evidenceHash, evidence); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (a app) readBatch(ctx context.Context, actor int64, requestID string) (batch, string, error) {
	var result batch
	var hash string
	var stored review
	err := a.db.QueryRowContext(ctx, `SELECT id,request_hash,operator_id,operator_name,reason,created_at FROM oa_sku_price_reviews WHERE operator_id=? AND request_id=?`, actor, requestID).
		Scan(&result.ID, &hash, &stored.OperatorID, &stored.Operator, &stored.Reason, &stored.ConfirmedAt)
	if err != nil {
		return result, "", err
	}
	stored.BatchID = result.ID
	rows, err := a.db.QueryContext(ctx, `SELECT id,evidence FROM oa_sku_price_reviews WHERE batch_id=? ORDER BY id`, result.ID)
	if err != nil {
		return result, "", err
	}
	defer rows.Close()
	for rows.Next() {
		copy := stored
		if err = rows.Scan(&copy.ID, &copy.Evidence); err != nil {
			return result, "", err
		}
		item, err := reviewedRow(copy)
		if err != nil {
			return result, "", err
		}
		result.Rows = append(result.Rows, item)
	}
	if err = rows.Err(); err != nil {
		return result, "", err
	}
	if len(result.Rows) == 0 {
		return result, "", errors.New("missing confirmed batch evidence")
	}
	return result, hash, nil
}

func (a app) findReview(ctx context.Context, sku string) (review, error) {
	return a.loadReview(ctx, "ORDER BY d.id DESC LIMIT 1", sku)
}

func (a app) findMatchingReview(ctx context.Context, sku, evidenceHash string) (review, bool, error) {
	value, err := a.loadReview(ctx, "AND d.evidence_hash=?", sku, evidenceHash)
	if errors.Is(err, sql.ErrNoRows) {
		return value, false, nil
	}
	return value, err == nil, err
}

func (a app) loadReview(ctx context.Context, suffix string, args ...any) (review, error) {
	var stored review
	err := a.db.QueryRowContext(ctx, `SELECT d.id,h.id,h.operator_id,h.operator_name,h.reason,h.created_at,d.evidence
 FROM oa_sku_price_reviews d JOIN oa_sku_price_reviews h ON h.id=d.batch_id WHERE d.sku_code=? `+suffix, args...).
		Scan(&stored.ID, &stored.BatchID, &stored.OperatorID, &stored.Operator, &stored.Reason, &stored.ConfirmedAt, &stored.Evidence)
	if err != nil {
		return stored, err
	}
	item, err := reviewedRow(stored)
	if err == nil {
		stored = *item.Review
	}
	return stored, err
}

func reviewedRow(stored review) (row, error) {
	var item row
	if err := json.Unmarshal(stored.Evidence, &item); err != nil {
		return item, err
	}
	if item.Suggested == nil {
		return item, errors.New("missing accepted cost evidence")
	}
	stored.AcceptedCost = *item.Suggested
	item.Review = &stored
	item.State, item.Token, item.Reason = "confirmed", "", "编码已有 OA 核价记录"
	return item, nil
}
