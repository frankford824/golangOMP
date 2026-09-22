// Explicit operator-only live verification. Default is read-only. --apply
// requires a named SKU and a new recovery file; all temporary prices are
// restored unless a concurrent, unrelated operator change is detected.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	mysql "github.com/go-sql-driver/mysql"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"
	"workflow/config"
	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
)

type backup struct {
	SKU            string
	ID, TaskID     int64
	Cost           float64
	Manual, Review bool
	Reason, Actor  string
	OverrideAt     *time.Time
	ERP            *domain.ERPProduct
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "VERIFY_FAILED:", err)
		os.Exit(1)
	}
}
func run() (runErr error) {
	sku := flag.String("sku", "", "existing eligible real SKU")
	apply := flag.Bool("apply", false, "authorize temporary real price writes and restoration")
	recovery := flag.String("recovery", "", "new absolute recovery JSON path")
	flag.Parse()
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	dsn, err := mysql.ParseDSN(cfg.MySQL.DSN)
	if err != nil {
		return err
	}
	dsn.ParseTime = true
	db, err := sql.Open("mysql", dsn.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	remote, err := service.NewRemoteERPBridgeClient(service.ERPRemoteClientConfig{BaseURL: cfg.ERPRemote.BaseURL, AuthMode: cfg.ERPRemote.AuthMode, AppKey: cfg.ERPRemote.AppKey, AppSecret: cfg.ERPRemote.AppSecret, AccessToken: cfg.ERPRemote.AccessToken, UpsertPath: cfg.ERPRemote.UpsertPath, SkuQueryPath: cfg.ERPRemote.SkuQueryPath, Timeout: 15 * time.Second, RetryMax: 1, RetryBackoff: time.Second, OpenWebVersion: cfg.ERPRemote.OpenWebVersion, OpenWebCharset: cfg.ERPRemote.OpenWebCharset})
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 6*time.Minute)
	defer cancel()
	if *sku == "" {
		if *apply {
			return fmt.Errorf("--apply requires an explicit --sku")
		}
		rows, err := db.QueryContext(ctx, `SELECT s.sku_code,s.cost_price FROM task_sku_items s JOIN tasks t ON t.id=s.task_id JOIN task_details d ON d.task_id=t.id JOIN jst_inventory j ON j.sku_id=s.sku_code WHERE t.task_status='Completed' AND t.batch_item_count=1 AND d.cost_price=s.cost_price AND d.manual_cost_override=0 AND s.manual_cost_override=0 AND s.requires_manual_review=0 AND s.cost_price>0 AND s.cost_price<100 AND s.cost_price=j.cost_price AND COALESCE(j.inventory_qty,0)=0 AND s.updated_at<UTC_TIMESTAMP()-INTERVAL 1 DAY AND NOT EXISTS(SELECT 1 FROM task_erp_outbox o WHERE o.task_id=s.task_id AND o.status<>'succeeded') ORDER BY s.updated_at LIMIT 5`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var id string
			var cost float64
			if err := rows.Scan(&id, &cost); err != nil {
				return err
			}
			fmt.Printf("CANDIDATE sku=%s cost=%.4f\n", id, cost)
		}
		return rows.Err()
	}
	var b backup
	b.SKU = *sku
	if err = db.QueryRowContext(ctx, `SELECT s.id,s.task_id,s.cost_price,s.manual_cost_override,s.requires_manual_review,s.manual_cost_override_reason,s.override_actor,s.override_at FROM task_sku_items s JOIN tasks t ON t.id=s.task_id JOIN task_details d ON d.task_id=t.id JOIN jst_inventory j ON j.sku_id=s.sku_code WHERE s.sku_code=? AND t.task_status='Completed' AND t.batch_item_count=1 AND d.cost_price=s.cost_price AND d.manual_cost_override=0 AND s.manual_cost_override=0 AND s.requires_manual_review=0 AND COALESCE(j.inventory_qty,0)=0 AND s.cost_price>0 AND s.cost_price<100 AND NOT EXISTS(SELECT 1 FROM task_erp_outbox o WHERE o.task_id=s.task_id AND o.status<>'succeeded')`, *sku).Scan(&b.ID, &b.TaskID, &b.Cost, &b.Manual, &b.Review, &b.Reason, &b.Actor, &b.OverrideAt); err != nil {
		return fmt.Errorf("SKU is not eligible for low-impact verification: %w", err)
	}
	b.ERP, err = remote.GetProductByID(ctx, *sku)
	if err != nil {
		return err
	}
	if b.ERP == nil || !domain.EqualCost(b.ERP.CostPrice, &b.Cost) {
		return fmt.Errorf("local and live ERP prices are not initially equal")
	}
	fmt.Printf("BASELINE sku=%s cost=%.4f\n", *sku, b.Cost)
	if !*apply {
		return nil
	}
	if !filepath.IsAbs(*recovery) {
		return fmt.Errorf("--recovery must be absolute")
	}
	if err = os.MkdirAll(filepath.Dir(*recovery), 0700); err != nil {
		return err
	}
	f, err := os.OpenFile(*recovery, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	err = json.NewEncoder(f).Encode(b)
	f.Close()
	if err != nil {
		return err
	}
	store := mysqlrepo.NewCostSyncRepo(mysqlrepo.New(db))
	if err = store.Observe(ctx, *sku, b.ERP.CostPrice); err != nil {
		return err
	}
	delta1 := math.Round((b.Cost+0.0001)*10000) / 10000
	delta2 := math.Round((b.Cost+0.0002)*10000) / 10000
	wait := func(ctx context.Context, predicate func(*domain.CostSyncState) bool) error {
		deadline := time.Now().Add(100 * time.Second)
		for time.Now().Before(deadline) {
			s, err := store.Get(ctx, *sku)
			if err != nil {
				return err
			}
			if s != nil && predicate(s) {
				return nil
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Second):
			}
		}
		s, _ := store.Get(ctx, *sku)
		return fmt.Errorf("synchronization did not finish: %+v", s)
	}
	writeERP := func(ctx context.Context, cost float64) error {
		_, err := remote.UpsertProduct(ctx, domain.ERPProductUpsertPayload{ProductID: *sku, SKUID: *sku, SKUCode: *sku, CostPrice: &cost, Operation: "cost_sync"})
		return err
	}
	ingest := func() error {
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, "http://127.0.0.1:8082/jst/sync/inc", nil)
		response, err := (&http.Client{Timeout: 45 * time.Second}).Do(req)
		if err != nil {
			return err
		}
		defer response.Body.Close()
		if response.StatusCode != 200 {
			return fmt.Errorf("incremental collection HTTP %d", response.StatusCode)
		}
		return nil
	}
	// Recovery does not overwrite a concurrent human price. No audit entries
	// are deleted, and the immutable recovery JSON remains available on failure.
	defer func() {
		restoreCtx, stop := context.WithTimeout(context.Background(), 150*time.Second)
		defer stop()
		recoverErr := func() error {
			current, err := remote.GetProductByID(restoreCtx, *sku)
			if err != nil || current == nil {
				return fmt.Errorf("cannot safely read ERP for restore: %v", err)
			}
			if !domain.EqualCost(current.CostPrice, &b.Cost) && !domain.EqualCost(current.CostPrice, &delta1) && !domain.EqualCost(current.CostPrice, &delta2) {
				return fmt.Errorf("ERP has a concurrent unrelated price; automatic restore refused")
			}
			if !domain.EqualCost(current.CostPrice, &b.Cost) {
				if err := writeERP(restoreCtx, b.Cost); err != nil {
					return err
				}
			}
			var price float64
			var actor string
			if err := db.QueryRowContext(restoreCtx, `SELECT cost_price,override_actor FROM task_sku_items WHERE id=?`, b.ID).Scan(&price, &actor); err != nil {
				return err
			}
			if (!domain.EqualCost(&price, &b.Cost) && !domain.EqualCost(&price, &delta1) && !domain.EqualCost(&price, &delta2)) || (actor != "codex_verification" && actor != "erp_cost_sync" && actor != b.Actor) {
				return fmt.Errorf("local SKU has a concurrent unrelated edit; restore refused")
			}
			result, err := db.ExecContext(restoreCtx, `UPDATE task_sku_items SET cost_price=?,manual_cost_override=?,requires_manual_review=?,manual_cost_override_reason=?,override_actor=?,override_at=?,updated_at=? WHERE id=? AND cost_price=? AND override_actor=?`, b.Cost, b.Manual, b.Review, b.Reason, b.Actor, b.OverrideAt, time.Now().UTC(), b.ID, price, actor)
			if err != nil {
				return err
			}
			n, _ := result.RowsAffected()
			if n == 0 && (!domain.EqualCost(&price, &b.Cost) || actor != b.Actor) {
				return fmt.Errorf("local edit raced with restoration")
			}
			// Explicitly request a fresh read even if a failed probe made no local change.
			if _, err = db.ExecContext(restoreCtx, `UPDATE sku_cost_sync_states SET needs_check=1,next_check_at=UTC_TIMESTAMP(3) WHERE sku_code=?`, *sku); err != nil {
				return err
			}
			if err = wait(restoreCtx, func(s *domain.CostSyncState) bool {
				return s.Status == "synced" && s.Revision == s.AckRevision && s.ProjectedRevision == s.Revision && domain.EqualCost(s.LocalCost, &b.Cost) && domain.EqualCost(s.ERPCost, &b.Cost) && s.ManualLock == b.Manual
			}); err != nil {
				return err
			}
			current, err = remote.GetProductByID(restoreCtx, *sku)
			if err != nil {
				return err
			}
			if current == nil || !domain.EqualCost(current.CostPrice, &b.Cost) || current.Name != b.ERP.Name || current.IID != b.ERP.IID || !domain.EqualCost(current.Price, b.ERP.Price) {
				return fmt.Errorf("ERP restore or unchanged identity verification failed")
			}
			var bad int
			if err = db.QueryRowContext(restoreCtx, `SELECT COUNT(*) FROM (SELECT cost_price FROM task_sku_items WHERE id=? UNION ALL SELECT cost_price FROM task_details WHERE task_id=? UNION ALL SELECT cost_price FROM omp_sku_records WHERE sku_code=? UNION ALL SELECT cost_price FROM erp_product_sync_records WHERE sku_code=?) p WHERE NOT(cost_price <=> ?)`, b.ID, b.TaskID, *sku, *sku, b.Cost).Scan(&bad); err != nil || bad != 0 {
				return fmt.Errorf("local mirror restore mismatch count=%d err=%v", bad, err)
			}
			_, err = db.ExecContext(restoreCtx, `INSERT INTO sku_cost_sync_audit(sku_code,action,local_cost,erp_cost,revision,reason,created_at) SELECT sku_code,'verification_restored',local_cost,erp_cost,revision,'用户授权真实SKU闭环验证完成，原价及人工标记已恢复',UTC_TIMESTAMP(3) FROM sku_cost_sync_states WHERE sku_code=?`, *sku)
			return err
		}()
		if recoverErr != nil {
			runErr = errors.Join(runErr, fmt.Errorf("RESTORE_REQUIRES_ATTENTION recovery=%s: %w", *recovery, recoverErr))
		} else {
			fmt.Printf("RESTORED sku=%s cost=%.4f identity_unchanged=true\n", *sku, b.Cost)
		}
	}()
	outboundStart := time.Now()
	result, err := db.ExecContext(ctx, `UPDATE task_sku_items SET cost_price=?,manual_cost_override=1,requires_manual_review=0,manual_cost_override_reason='用户授权闭环验证，稍后恢复',override_actor='codex_verification',override_at=?,updated_at=? WHERE id=? AND cost_price=? AND manual_cost_override=0`, delta1, time.Now().UTC(), time.Now().UTC(), b.ID, b.Cost)
	if err != nil {
		return err
	}
	n, _ := result.RowsAffected()
	if n != 1 {
		return fmt.Errorf("baseline changed before test")
	}
	if err = wait(ctx, func(s *domain.CostSyncState) bool {
		return s.Status == "synced" && s.ProjectedRevision == s.Revision && domain.EqualCost(s.ERPCost, &delta1)
	}); err != nil {
		return err
	}
	fmt.Printf("OUTBOUND_VERIFIED %.4f elapsed_ms=%d\n", delta1, time.Since(outboundStart).Milliseconds())
	inboundStart := time.Now()
	if err = writeERP(ctx, delta2); err != nil {
		return err
	}
	if err = ingest(); err != nil {
		return err
	}
	if err = wait(ctx, func(s *domain.CostSyncState) bool {
		return s.Status == "conflict" && domain.EqualCost(s.LocalCost, &delta1) && domain.EqualCost(s.ERPCost, &delta2)
	}); err != nil {
		return err
	}
	fmt.Printf("CONFLICT_PRESERVED_LOCAL=true forced_collection_elapsed_ms=%d\n", time.Since(inboundStart).Milliseconds())
	s, err := store.Get(ctx, *sku)
	if err != nil {
		return err
	}
	if err = store.Resolve(ctx, domain.CostSyncResolution{SKUCode: *sku, Revision: s.Revision, ERPRevision: s.ERPRevision, Choice: "erp", Reason: "用户授权真实SKU验证：确认临时ERP测试价，随后恢复"}); err != nil {
		return err
	}
	if err = writeERP(ctx, b.Cost); err != nil {
		return err
	}
	if err = ingest(); err != nil {
		return err
	}
	if err = wait(ctx, func(s *domain.CostSyncState) bool {
		return s.Status == "synced" && domain.EqualCost(s.LocalCost, &b.Cost) && s.ManualOrigin == "erp"
	}); err != nil {
		return err
	}
	fmt.Println("INBOUND_AND_NO_ECHO_VERIFIED=true")
	return nil
}
