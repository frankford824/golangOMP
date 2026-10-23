// Reconcile historical memberships after unifying the two applications' access
// model. Default is a transactionally rolled-back dry run. No grants are edited.
package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"workflow/repo"
	mysqlrepo "workflow/repo/mysql"
)

var dryRun = errors.New("dry run rollback")

func main() {
	apply := flag.Bool("apply", false, "commit the reviewed repair (requires explicit production authorization)")
	actorID := flag.Int64("actor-id", 0, "protected super administrator authorizing this repair")
	flag.Parse()
	if err := run(*apply, *actorID); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(apply bool, actorID int64) error {
	dsn := os.Getenv("MYSQL_DSN")
	if dsn == "" {
		return errors.New("MYSQL_DSN is required")
	}
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return errors.New("invalid MySQL configuration")
	}
	defer db.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	mdb := mysqlrepo.New(db)
	policy := mysqlrepo.NewAccessPolicyRepo(mdb)
	report := map[string]interface{}{"apply": apply, "actor_id": actorID}
	_, total, err := mysqlrepo.NewAssetWorkbenchRepo(mdb).ListMembers(ctx, repo.AssetWorkbenchMemberFilter{Page: 1, PageSize: 20, Status: "active"})
	if err != nil {
		return fmt.Errorf("validate canonical member query: %w", err)
	}
	report["visible_active_members"] = total
	err = mdb.RunInTx(ctx, func(tx repo.Tx) error {
		sqlTx := mysqlrepo.Unwrap(tx)
		if apply {
			var allowed int
			if err := sqlTx.QueryRowContext(ctx, `SELECT COUNT(*) FROM auth_user_role_assignments a JOIN auth_roles r ON r.id = a.role_id AND r.code = 'super_admin' AND r.system_protected = 1 AND r.archived_at IS NULL JOIN users u ON u.id = a.user_id AND u.status = 'active' WHERE a.user_id = ?`, actorID).Scan(&allowed); err != nil {
				return err
			}
			if allowed == 0 {
				return errors.New("apply requires an active protected super administrator")
			}
		}
		counts := func() (map[string]int64, error) {
			values := map[string]int64{}
			for key, query := range map[string]string{
				"memberships":           `SELECT COUNT(*) FROM app_memberships WHERE app_code = 'asset_workbench'`,
				"profiles":              `SELECT COUNT(*) FROM asset_workbench_profiles`,
				"active_memberships":    `SELECT COUNT(*) FROM app_memberships WHERE app_code = 'asset_workbench' AND status = 'active'`,
				"pending_memberships":   `SELECT COUNT(*) FROM app_memberships WHERE app_code = 'asset_workbench' AND status = 'pending'`,
				"protected_memberships": `SELECT COUNT(*) FROM app_memberships WHERE app_code = 'asset_workbench' AND status IN ('disabled','merged')`,
			} {
				var count int64
				if err := sqlTx.QueryRowContext(ctx, query).Scan(&count); err != nil {
					return nil, err
				}
				values[key] = count
			}
			return values, nil
		}
		before, err := counts()
		if err != nil {
			return err
		}
		if err := policy.ReconcileWorkbenchAccess(ctx, tx, actorID); err != nil {
			return err
		}
		after, err := counts()
		if err != nil {
			return err
		}
		if before["protected_memberships"] != after["protected_memberships"] {
			return errors.New("protected membership invariant failed")
		}
		report["before"], report["after"] = before, after
		if !apply {
			return dryRun
		}
		beforeJSON, _ := json.Marshal(before)
		afterJSON, _ := json.Marshal(after)
		_, err = sqlTx.ExecContext(ctx, `INSERT INTO app_identity_events (actor_user_id, source_app, target_app, action, before_json, after_json, reason) VALUES (?, 'main_ops', 'asset_workbench', 'access.reconciled', ?, ?, '统一显式权限，修复历史缺失开通记录和待完善档案；不修改角色和工资')`, actorID, beforeJSON, afterJSON)
		return err
	})
	if err != nil && !errors.Is(err, dryRun) {
		return err
	}
	return json.NewEncoder(os.Stdout).Encode(report)
}
