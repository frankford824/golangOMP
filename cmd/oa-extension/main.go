// Command oa-extension serves the new OA routes alongside the unchanged production core.
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-sql-driver/mysql"
	"workflow/internal/oa/office"
	"workflow/internal/oa/orders"
	"workflow/internal/oa/platform"
)

const core = "http://127.0.0.1:8080"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}
func open(name, expectedUser string) (*sql.DB, error) {
	cfg, err := mysql.ParseDSN(os.Getenv(name))
	if err != nil || cfg.DBName == "" || cfg.DBName != os.Getenv("OA_FORMAL_DATABASE") || cfg.DBName == "yongbo_oa" || cfg.User != expectedUser {
		return nil, fmt.Errorf("invalid scoped configuration for %s", name)
	}
	c, err := mysql.NewConnector(cfg)
	if err != nil {
		return nil, errors.New("invalid database connector")
	}
	db := sql.OpenDB(c)
	db.SetMaxOpenConns(12)
	db.SetMaxIdleConns(8)
	db.SetConnMaxLifetime(4 * time.Minute)
	return db, nil
}
func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, err := open("OA_EXTENSION_DSN", "oa_formal_extension")
	if err != nil {
		return err
	}
	defer db.Close()
	source, err := open("OA_SOURCE_DSN", "oa_formal_reader")
	if err != nil {
		return err
	}
	defer source.Close()
	if err = office.CheckSchema(ctx, db); err != nil {
		return err
	}
	if err = source.PingContext(ctx); err != nil {
		return err
	}
	native := platform.Native(core)
	api := http.NewServeMux()
	office.Mount(api, db, source, native, orders.BaselineShops)
	orders.Mount(api, db, native)
	api.HandleFunc("GET /oa-api/me", func(w http.ResponseWriter, r *http.Request) { platform.JSON(w, 200, platform.ActorFrom(r)) })
	public := http.NewServeMux()
	identity := platform.NewIdentity(core)
	public.HandleFunc("GET /oa-api/health", func(w http.ResponseWriter, r *http.Request) {
		check, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if db.PingContext(check) != nil {
			platform.Error(w, 503, "OA 扩展暂不可用")
			return
		}
		platform.JSON(w, 200, map[string]string{"status": "ok", "mode": "live", "identity": "main-ops"})
	})
	public.Handle("/oa-api/", identity.Authenticate(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		actor, ok := office.ApplyIdentity(r.Context(), db, source, platform.ActorFrom(r))
		if !ok {
			platform.Error(w, 503, "无法核实当前账号的 OA 授权")
			return
		}
		actor.Activated = true
		actor.Actions["finance.apply"] = true
		actor.Actions["finance.approve"] = true
		api.ServeHTTP(w, platform.WithActor(r, actor))
	})))
	server := http.Server{Addr: "127.0.0.1:18095", Handler: public, ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 65 * time.Second, WriteTimeout: 65 * time.Second, IdleTimeout: 90 * time.Second, MaxHeaderBytes: 32768}
	go func() {
		<-ctx.Done()
		grace, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = server.Shutdown(grace)
	}()
	if err = server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}
