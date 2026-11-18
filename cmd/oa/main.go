// Command oa serves the isolated OA validation environment.
package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	"workflow/internal/oa/office"
	"workflow/internal/oa/orders"
	"workflow/internal/oa/platform"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	cfg, err := mysql.ParseDSN(os.Getenv("OA_DB_DSN"))
	if err != nil || cfg.DBName != "yongbo_oa" || cfg.User != "yongbo_oa_app" {
		return fmt.Errorf("OA requires its dedicated validation database")
	}
	if os.Getenv("OA_CORE_URL") != "http://127.0.0.1:18087" {
		return fmt.Errorf("OA requires the private validation core gateway")
	}
	connector, err := mysql.NewConnector(cfg)
	if err != nil {
		return fmt.Errorf("invalid validation DB configuration")
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	db.SetMaxOpenConns(16)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(4 * time.Minute)
	if err = db.PingContext(ctx); err != nil {
		return fmt.Errorf("OA database unavailable: %w", err)
	}
	if err = office.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("OA office schema: %w", err)
	}
	if err = orders.EnsureSchema(ctx, db); err != nil {
		return fmt.Errorf("OA order schema: %w", err)
	}
	if _, err = db.ExecContext(ctx, `CREATE OR REPLACE SQL SECURITY INVOKER VIEW oa_current_sku_costs AS
	SELECT s.sku_code,s.cost_price FROM task_sku_items s JOIN tasks t ON t.id=s.task_id
	WHERE s.sku_code<>'' AND (t.task_status IN ('Completed','Archived') OR EXISTS
	(SELECT 1 FROM tasks h WHERE h.sku_code=s.sku_code AND h.task_status IN ('Completed','Archived')))`); err != nil {
		return fmt.Errorf("OA cost snapshot view: %w", err)
	}
	protected := http.NewServeMux()
	office.Mount(protected, db, db, orders.BaselineShops)
	orders.Mount(protected, db, db)
	protected.HandleFunc("GET /oa-api/me", func(w http.ResponseWriter, r *http.Request) { platform.JSON(w, 200, platform.ActorFrom(r)) })
	id := platform.NewIdentity(os.Getenv("OA_CORE_URL"))
	public := http.NewServeMux()
	// Apply scheduled personnel changes before serving the first request of a new
	// Beijing day. Changes effective today are applied by their writing transaction.
	var scheduleMu sync.Mutex
	var appliedDay string
	applyDue := func(ctx context.Context) error {
		day := time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600)).Format(time.DateOnly)
		scheduleMu.Lock()
		defer scheduleMu.Unlock()
		if appliedDay == day {
			return nil
		}
		if err := office.ApplyDueIdentityChanges(ctx, db); err != nil {
			return err
		}
		appliedDay = day
		return nil
	}
	if err := applyDue(ctx); err != nil {
		return err
	}
	go func() {
		tick := time.NewTicker(time.Minute)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := applyDue(ctx); err != nil {
					log.Print("OA personnel schedule temporarily unavailable")
				}
			}
		}
	}()
	dueFirst := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if err := applyDue(r.Context()); err != nil {
				platform.Error(w, 503, "任职生效处理中，请稍后重试")
				return
			}
			next.ServeHTTP(w, r)
		})
	}
	coreURL, _ := url.Parse(id.Base)
	proxy := httputil.NewSingleHostReverseProxy(coreURL)
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		platform.Error(w, 502, "验证站业务服务暂不可用")
	}
	public.Handle("/v1/", dueFirst(proxy))
	public.HandleFunc("GET /oa-api/health", func(w http.ResponseWriter, r *http.Request) {
		check, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if db.PingContext(check) != nil {
			platform.Error(w, 503, "数据库未就绪")
			return
		}
		platform.JSON(w, 200, map[string]string{"status": "ok", "environment": "validation"})
	})
	public.Handle("/oa-api/", dueFirst(id.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		a, ok := office.ApplyIdentity(r.Context(), db, platform.ActorFrom(r))
		if !ok {
			platform.Error(w, 403, "验证账号未启用或授权暂不可用")
			return
		}
		protected.ServeHTTP(w, platform.WithActor(r, a))
	}))))
	srv := &http.Server{Addr: "127.0.0.1:18085", Handler: public, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 65 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32 << 10}
	go func() {
		<-ctx.Done()
		drain, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = srv.Shutdown(drain)
	}()
	if err = srv.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
