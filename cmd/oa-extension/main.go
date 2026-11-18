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
	"workflow/transport"
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
	public := transport.NewOAExtensionRouter(db, source, core)
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
