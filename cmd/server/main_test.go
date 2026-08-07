package main

import (
	"testing"

	mysqldriver "github.com/go-sql-driver/mysql"
)

func TestEnvFlagAcceptsDeploymentBooleanValues(t *testing.T) {
	cases := []struct {
		name string
		raw  string
		want bool
	}{
		{name: "one", raw: "1", want: true},
		{name: "true", raw: "true", want: true},
		{name: "yes", raw: "yes", want: true},
		{name: "on", raw: "on", want: true},
		{name: "false", raw: "false", want: false},
		{name: "empty", raw: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("ENABLE_CRON_TEST", tc.raw)
			if got := envFlag("ENABLE_CRON_TEST"); got != tc.want {
				t.Fatalf("envFlag() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestMySQLRuntimeDSNEnablesSafeClientInterpolation(t *testing.T) {
	runtimeDSN, err := mysqlRuntimeDSN("workflow_user:safe-pass@tcp(db.internal:3306)/workflow?charset=utf8mb4&parseTime=true&loc=Local")
	if err != nil {
		t.Fatalf("mysqlRuntimeDSN() error = %v", err)
	}
	cfg, err := mysqldriver.ParseDSN(runtimeDSN)
	if err != nil {
		t.Fatalf("ParseDSN(runtimeDSN) error = %v", err)
	}
	if !cfg.InterpolateParams {
		t.Fatal("InterpolateParams = false, want true")
	}
	if cfg.Params["charset"] != "utf8mb4" || !cfg.ParseTime || cfg.Loc.String() != "Local" {
		t.Fatalf("runtime config lost existing options: charset=%q parseTime=%v loc=%v", cfg.Params["charset"], cfg.ParseTime, cfg.Loc)
	}
	if cfg.User != "workflow_user" || cfg.Passwd != "safe-pass" || cfg.Addr != "db.internal:3306" || cfg.DBName != "workflow" {
		t.Fatalf("runtime config changed connection identity: user=%q addr=%q db=%q", cfg.User, cfg.Addr, cfg.DBName)
	}
}

func TestMySQLRuntimeDSNRejectsMalformedInput(t *testing.T) {
	if _, err := mysqlRuntimeDSN("not a valid dsn%%%"); err == nil {
		t.Fatal("mysqlRuntimeDSN() error = nil, want malformed DSN error")
	}
}
