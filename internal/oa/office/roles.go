package office

import (
	"context"
	"database/sql"
	"slices"
)

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

const companyPeopleScope int64 = -1

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
			if access && !slices.Contains(departments, companyPeopleScope) {
				departments = append(departments, companyPeopleScope)
			} else if code == "department_admin" && dept > 0 && !slices.Contains(departments, dept) {
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
