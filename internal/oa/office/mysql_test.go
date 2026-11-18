package office

import (
	"context"
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/go-sql-driver/mysql"
)

func TestMySQLIdentityQueue(t *testing.T) {
	dsn := os.Getenv("OA_TEST_DSN")
	if dsn == "" {
		t.Skip("OA_TEST_DSN requires an empty oa_office_fixture_* schema")
	}
	config, err := mysql.ParseDSN(dsn)
	if err != nil || !strings.HasPrefix(config.DBName, "oa_office_fixture_") {
		t.Fatal("use a dedicated oa_office_fixture_* schema")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	var tables int
	if err = db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE()").Scan(&tables); err != nil || tables != 0 {
		t.Fatalf("expected an empty schema: tables=%d, error=%v", tables, err)
	}
	run := func(query string, args ...any) {
		t.Helper()
		if _, err := db.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	run("CREATE TABLE users(id BIGINT PRIMARY KEY,status VARCHAR(16))")
	run("CREATE TABLE user_sessions(user_id BIGINT,revoked_at DATETIME NULL)")
	run("CREATE TABLE auth_roles(id BIGINT PRIMARY KEY,code VARCHAR(50))")
	run("CREATE TABLE auth_policy_state(singleton_id BIGINT PRIMARY KEY,policy_revision BIGINT)")
	run("CREATE TABLE auth_policy_events(policy_revision BIGINT,actor_id BIGINT,action VARCHAR(50),target_type VARCHAR(16),target_id VARCHAR(16),reason VARCHAR(50),after_json JSON)")
	run("CREATE TABLE auth_user_role_assignments(id BIGINT AUTO_INCREMENT PRIMARY KEY,user_id BIGINT,role_id BIGINT,scope_mode VARCHAR(32),source_type VARCHAR(32),source_ref_id BIGINT,assigned_by BIGINT)")
	if err = EnsureSchema(ctx, db); err != nil {
		t.Fatal(err)
	}
	run("INSERT INTO users VALUES(2,'active')")
	run("INSERT INTO user_sessions VALUES(2,NULL)")
	run("INSERT INTO auth_roles VALUES(1,'operations'),(2,'auditor')")
	run("INSERT INTO auth_policy_state VALUES(1,1)")
	run("INSERT INTO auth_user_role_assignments(user_id,role_id,source_type,source_ref_id) VALUES(2,2,'migration',0)")
	today := businessDate().Format(time.DateOnly)
	run("INSERT INTO oa_office_role_changes(user_id,effective_on,payload) VALUES(2,?,?)", today, jsonData(change{Roles: []string{"运营"}, ActorID: 9}))
	if err = ApplyDueIdentityChanges(ctx, db); err != nil {
		t.Fatal(err)
	}
	run("INSERT INTO oa_office_role_changes(user_id,effective_on,payload) VALUES(2,?,?)", today, jsonData(change{Kind: "leave", ActorID: 9}))
	run("INSERT INTO oa_office_identity_changes(user_id,effective_on,status) VALUES(2,?,'disabled')", today)
	for range 2 {
		if err = ApplyDueIdentityChanges(ctx, db); err != nil {
			t.Fatal(err)
		}
	}
	var status string
	var revoked bool
	var legacy, owned, events int
	if err = db.QueryRowContext(ctx, "SELECT (SELECT status FROM users WHERE id=2), (SELECT revoked_at IS NOT NULL FROM user_sessions WHERE user_id=2), (SELECT COUNT(*) FROM auth_user_role_assignments WHERE source_type='migration'), (SELECT COUNT(*) FROM auth_user_role_assignments WHERE source_ref_id=?), (SELECT COUNT(*) FROM auth_policy_events)", officeAssignmentRef).Scan(&status, &revoked, &legacy, &owned, &events); err != nil {
		t.Fatal(err)
	}
	if status != "disabled" || !revoked || legacy != 1 || owned != 0 || events != 2 {
		t.Fatalf("status=%s revoked=%v legacy=%d owned=%d events=%d", status, revoked, legacy, owned, events)
	}
}
