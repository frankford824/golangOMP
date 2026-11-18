package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"slices"
	"time"
)

const officeAssignmentRef int64 = -90117

const grantsQuery = `WITH grants AS (
 SELECT a.id,u.id AS user_id,r.code,a.scope_mode,u.department_id,u.team_id
 FROM users u JOIN auth_user_role_assignments a ON a.user_id=u.id
 JOIN auth_roles r ON r.id=a.role_id AND r.archived_at IS NULL WHERE u.id=?
 UNION ALL
 SELECT 0,u.id,r.code,p.scope_mode,u.department_id,u.team_id
 FROM users u JOIN auth_org_role_policies p ON p.enabled=1 AND
 ((p.subject_type='department' AND p.subject_id=u.department_id) OR (p.subject_type='team' AND p.subject_id=u.team_id))
 JOIN auth_roles r ON r.id=p.role_id AND r.archived_at IS NULL WHERE u.id=?
)`

func effectivePeopleAccess(ctx context.Context, db *sql.DB, id int64) (roles []string, departments, teams []int64, err error) {
	rows, err := db.QueryContext(ctx, grantsQuery+`
 SELECT DISTINCT g.code,g.scope_mode,COALESCE(g.department_id,0),COALESCE(g.team_id,0),
 COALESCE(s.subject_type,''),COALESCE(s.subject_id,0),EXISTS(
 SELECT 1 FROM auth_roles r JOIN auth_role_permissions p ON p.role_id=r.id
 WHERE r.code=g.code AND p.permission_code='access.manage')
 FROM grants g LEFT JOIN auth_assignment_scope_subjects s ON g.id>0 AND g.id=s.assignment_id AND g.scope_mode='selected_org'`, id, id)
	if err != nil {
		return nil, nil, nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var code, mode, subject string
		var dept, team, subjectID int64
		var access bool
		if err = rows.Scan(&code, &mode, &dept, &team, &subject, &subjectID, &access); err != nil {
			return nil, nil, nil, err
		}
		if !slices.Contains(roles, code) {
			roles = append(roles, code)
		}
		if code != "department_admin" && !access {
			continue
		}
		switch mode {
		case "own_department":
			if dept > 0 && !slices.Contains(departments, dept) {
				departments = append(departments, dept)
			}
		case "global":
			if code == "department_admin" && dept > 0 && !slices.Contains(departments, dept) {
				departments = append(departments, dept)
			}
		case "own_team":
			if team > 0 && !slices.Contains(teams, team) {
				teams = append(teams, team)
			}
		case "selected_org":
			if subject == "department" && subjectID > 0 && !slices.Contains(departments, subjectID) {
				departments = append(departments, subjectID)
			}
			if subject == "team" && subjectID > 0 && !slices.Contains(teams, subjectID) {
				teams = append(teams, subjectID)
			}
		}
	}
	return roles, departments, teams, rows.Err()
}

func applyDueRoles(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, "SELECT id,user_id,payload FROM oa_office_role_changes WHERE applied=FALSE AND effective_on<=? ORDER BY effective_on,id FOR UPDATE", businessDate().Format(time.DateOnly))
	if err != nil {
		return err
	}
	type pending struct {
		id, user int64
		change   change
	}
	items := []pending{}
	for rows.Next() {
		var item pending
		var raw []byte
		if err = rows.Scan(&item.id, &item.user, &raw); err != nil {
			rows.Close()
			return err
		}
		if err = json.Unmarshal(raw, &item.change); err != nil {
			rows.Close()
			return err
		}
		items = append(items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(items) == 0 {
		return err
	}
	var revision int64
	if err = tx.QueryRowContext(ctx, "SELECT policy_revision FROM auth_policy_state WHERE singleton_id=1 FOR UPDATE").Scan(&revision); err != nil {
		return err
	}
	for _, item := range items {
		if _, err = tx.ExecContext(ctx, "DELETE FROM auth_user_role_assignments WHERE user_id=? AND source_type='direct' AND source_ref_id=?", item.user, officeAssignmentRef); err != nil {
			return err
		}
		grants := map[string]string{}
		for _, label := range item.change.Roles {
			if code := businessRoles[label]; code != "" {
				grants[code] = "self"
			}
		}
		if item.change.Management == "department" {
			grants["department_admin"] = "own_department"
		}
		for role, scope := range grants {
			var roleID int64
			if err = tx.QueryRowContext(ctx, "SELECT id FROM auth_roles WHERE code=?", role).Scan(&roleID); err != nil {
				return err
			}
			var existing int64
			err = tx.QueryRowContext(ctx, "SELECT id FROM auth_user_role_assignments WHERE user_id=? AND role_id=? LIMIT 1", item.user, roleID).Scan(&existing)
			if err == nil {
				continue
			}
			if !errors.Is(err, sql.ErrNoRows) {
				return err
			}
			if _, err = tx.ExecContext(ctx, "INSERT INTO auth_user_role_assignments(user_id,role_id,scope_mode,source_type,source_ref_id,assigned_by) VALUES(?,?,?,'direct',?,?)", item.user, roleID, scope, officeAssignmentRef, item.change.ActorID); err != nil {
				return err
			}
		}
		revision++
		if _, err = tx.ExecContext(ctx, "INSERT INTO auth_policy_events(policy_revision,actor_id,action,target_type,target_id,reason,after_json) VALUES(?,?,'oa.office.roles','user',?,'OA personnel change',?)", revision, item.change.ActorID, decimal(item.user), jsonData(grants)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, "UPDATE oa_office_role_changes SET applied=TRUE WHERE id=?", item.id); err != nil {
			return err
		}
	}
	_, err = tx.ExecContext(ctx, "UPDATE auth_policy_state SET policy_revision=? WHERE singleton_id=1", revision)
	return err
}
