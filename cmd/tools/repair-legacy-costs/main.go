// Audited recovery of recent, initially unpriced SKUs. Dry-run by default.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"time"

	mysql "github.com/go-sql-driver/mysql"
	"workflow/config"
	"workflow/domain"
	mysqlrepo "workflow/repo/mysql"
	"workflow/service"
)

type item struct {
	Preview *service.LegacyCostRepairPreview `json:"preview"`
	ERP     *float64                         `json:"observed_erp"`
}
type report struct {
	Database  string    `json:"database"`
	CreatedAt time.Time `json:"created_at"`
	Items     []item    `json:"items"`
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	apply := flag.Bool("apply", false, "apply eligible rows from reviewed report, queue cost-only synchronization")
	input := flag.String("input", "", "preview report to apply")
	output := flag.String("output", "", "new exclusive report path")
	expected := flag.String("expected-plan", "", "reviewed report SHA256")
	confirm := flag.String("confirm-database", "", "required with apply")
	actor := flag.Int64("actor-id", 0, "operator identity; required with apply")
	since := flag.String("since", "2026-09-18", "earliest affected task creation date")
	flag.Parse()
	if *output == "" {
		return fmt.Errorf("--output is required")
	}
	if _, err := time.Parse("2006-01-02", *since); err != nil {
		return err
	}
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		c := mysql.NewConfig()
		c.User = os.Getenv("DB_USER")
		c.Passwd = os.Getenv("DB_PASS")
		c.DBName = os.Getenv("DB_NAME")
		c.Net = "tcp"
		port := os.Getenv("DB_PORT")
		if port == "" {
			port = "3306"
		}
		c.Addr = os.Getenv("DB_HOST") + ":" + port
		c.ParseTime = true
		dsn = c.FormatDSN()
		os.Setenv("MYSQL_DSN", dsn)
	}
	c, err := mysql.ParseDSN(dsn)
	if err != nil {
		return err
	}
	c.ParseTime = true
	db, err := sql.Open("mysql", c.FormatDSN())
	if err != nil {
		return err
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()
	var database string
	if err = db.QueryRowContext(ctx, "SELECT DATABASE()").Scan(&database); err != nil {
		return err
	}
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	remote, err := service.NewRemoteERPBridgeClient(service.ERPRemoteClientConfig{BaseURL: cfg.ERPRemote.BaseURL, AuthMode: cfg.ERPRemote.AuthMode, AppKey: cfg.ERPRemote.AppKey, AppSecret: cfg.ERPRemote.AppSecret, AccessToken: cfg.ERPRemote.AccessToken, SkuQueryPath: cfg.ERPRemote.SkuQueryPath, Timeout: 20 * time.Second, RetryMax: 1, RetryBackoff: time.Second, OpenWebVersion: cfg.ERPRemote.OpenWebVersion, OpenWebCharset: cfg.ERPRemote.OpenWebCharset})
	if err != nil {
		return err
	}
	w := mysqlrepo.New(db)
	store := mysqlrepo.NewCostSyncRepo(w)
	svc := service.NewLegacyCostRepairService(mysqlrepo.NewTaskRepo(w), mysqlrepo.NewCostRuleRepo(w), mysqlrepo.NewCostRuleBindingRepo(w), mysqlrepo.NewSKUTraceRepo(w), w, store)
	if *apply {
		if *actor <= 0 || *input == "" || *confirm != database {
			return fmt.Errorf("apply requires --input, --actor-id and matching --confirm-database")
		}
		b, err := os.ReadFile(*input)
		if err != nil {
			return err
		}
		var p report
		if err = json.Unmarshal(b, &p); err != nil {
			return err
		}
		if service.LegacyCostPlanDigest(p) != *expected || p.Database != database {
			return fmt.Errorf("report digest/database mismatch")
		}
		var validActor bool
		if err = db.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM users WHERE id=?)", *actor).Scan(&validActor); err != nil || !validActor {
			return fmt.Errorf("operator does not exist")
		}
		f, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
		if err != nil {
			return err
		}
		defer f.Close()
		enc := json.NewEncoder(f)
		applied, skipped, quarantined := 0, 0, 0
		for _, it := range p.Items {
			if it.Preview == nil || (it.Preview.SkipReason != "" && !it.Preview.NeedsQuarantine()) {
				continue
			}
			row := it.Preview
			r, err := remote.GetProductByID(ctx, row.SKU)
			out := map[string]interface{}{"sku": row.SKU, "old_cost": row.OldCost, "new_cost": row.NewCost, "revision": row.Revision}
			if err != nil || r == nil || r.SKUID != row.SKU || !domain.EqualCost(r.CostPrice, it.ERP) {
				out["status"] = "skipped_erp_changed_or_unavailable"
				skipped++
			} else {
				// Immutable prepared record is synced to disk before any write.
				out["status"] = "prepared"
				if err = enc.Encode(out); err != nil {
					return err
				}
				if err = f.Sync(); err != nil {
					return err
				}
				if row.NeedsQuarantine() {
					err = svc.Quarantine(ctx, row, *actor, *expected)
				} else {
					err = svc.Apply(ctx, row, r.CostPrice, *actor, *expected)
				}
				if err != nil {
					out["status"] = "skipped_changed_or_failed"
					out["error"] = err.Error()
					skipped++
				} else if row.NeedsQuarantine() {
					out["status"] = "quarantined"
					quarantined++
				} else {
					out["status"] = "queued"
					applied++
				}
			}
			if err = enc.Encode(out); err != nil {
				return err
			}
			if err = f.Sync(); err != nil {
				return err
			}
			time.Sleep(150 * time.Millisecond)
		}
		fmt.Printf("APPLIED queued=%d quarantined=%d skipped=%d journal=%s\n", applied, quarantined, skipped, *output)
		return nil
	}
	rows, err := db.QueryContext(ctx, `SELECT s.sku_code FROM task_sku_items s JOIN tasks t ON t.id=s.task_id
 WHERE t.created_at>=? AND t.task_type='new_product_development' AND t.task_status NOT IN ('Cancelled','Archived')
 AND EXISTS(SELECT 1 FROM omp_sku_cost_snapshots p WHERE p.sku_code=s.sku_code AND p.event_source='task_create' AND p.cost_price IS NULL)
 AND NOT EXISTS(SELECT 1 FROM sku_cost_sync_audit a WHERE a.sku_code=s.sku_code AND a.actor_id IS NOT NULL AND a.action IN ('manual_saved','resolve_local','resolve_erp'))
 AND NOT EXISTS(SELECT 1 FROM jst_cost_changes j WHERE j.sku_id=s.sku_code AND j.old_cost_price IS NOT NULL AND NOT(j.old_cost_price <=> j.new_cost_price))
 ORDER BY s.id`, *since)
	if err != nil {
		return err
	}
	ids := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()
	p := report{Database: database, CreatedAt: time.Now().UTC()}
	for _, sku := range ids {
		r, err := svc.Preview(ctx, sku)
		if err != nil {
			return err
		}
		p.Items = append(p.Items, item{Preview: r})
	}
	// Batch reads are observation only. No Observe/TaskView(refresh) call may
	// certify or mutate any price while preparing the report.
	eligible := []int{}
	for i, it := range p.Items {
		if it.Preview.SkipReason == "" || it.Preview.NeedsQuarantine() {
			eligible = append(eligible, i)
		}
	}
	for start := 0; start < len(eligible); start += 50 {
		end := start + 50
		if end > len(eligible) {
			end = len(eligible)
		}
		codes := []string{}
		for _, idx := range eligible[start:end] {
			codes = append(codes, p.Items[idx].Preview.SKU)
		}
		batchReader, ok := remote.(interface {
			BatchCostProducts(context.Context, []string) ([]*domain.ERPProduct, error)
		})
		if !ok {
			return fmt.Errorf("ERP batch reader unavailable")
		}
		products, err := batchReader.BatchCostProducts(ctx, codes)
		if err != nil {
			return err
		}
		bySKU := map[string]*domain.ERPProduct{}
		for _, product := range products {
			bySKU[product.SKUID] = product
		}
		for _, idx := range eligible[start:end] {
			it := &p.Items[idx]
			product := bySKU[it.Preview.SKU]
			if product == nil {
				it.Preview.SkipReason = "erp_sku_not_found"
				continue
			}
			it.ERP = product.CostPrice
			if it.Preview.OldCost != nil && !domain.EqualCost(it.Preview.OldCost, it.ERP) {
				it.Preview.SkipReason = "erp_price_differs_from_local_baseline"
			}
		}
		time.Sleep(200 * time.Millisecond)
	}
	f, err := os.OpenFile(*output, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	if err = json.NewEncoder(f).Encode(p); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	reasons := map[string]int{}
	for _, it := range p.Items {
		reasons[it.Preview.SkipReason]++
	}
	b, _ := json.Marshal(reasons)
	fmt.Printf("PREVIEW count=%d sha256=%s reasons=%s report=%s\n", len(p.Items), service.LegacyCostPlanDigest(p), b, *output)
	return nil
}
