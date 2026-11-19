package office

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"workflow/internal/oa/platform"
)

type profileUpdate struct {
	EmploymentType   *string           `json:"employment_type,omitempty"`
	Version          int64             `json:"version"`
	EmergencyContact *emergencyContact `json:"emergency_contact,omitempty"`
	EffectiveDate    string            `json:"effective_date,omitempty"`
	ManagerID        int64             `json:"manager_id"`
	Joined           string            `json:"joined"`
	JobTitle         string            `json:"job_title"`
	JobLevel         string            `json:"job_level"`
	CostCenter       string            `json:"cost_center"`
}

type orgManager struct {
	Kind         string `json:"kind"`
	ID           int64  `json:"id"`
	Name         string `json:"name"`
	DepartmentID int64  `json:"department_id"`
	Version      int64  `json:"version"`
	ManagerID    int64  `json:"manager_id"`
	ManagerName  string `json:"manager_name,omitempty"`
}

// Current OA data combines native membership and status with legacy effective
// events and explicit profile overrides. A team lead inherits from the department. A
// department lead skips automatic reporting within their own department.
const directoryPeople = `WITH base AS (
	SELECT u.id, u.display_name AS name, u.employee_no, u.status,
		COALESCE(u.department_id,0) AS department_id, COALESCE(u.team_id,0) AS team_id,
		COALESCE(u.department,'') AS department, COALESCE(u.team,'') AS team,
		COALESCE(NULLIF(c.employment_type,''), NULLIF((SELECT e.employment_type FROM JSON_TABLE(o.payload, '$.events[*]' COLUMNS
			(seq FOR ORDINALITY, employment_type VARCHAR(48) PATH '$.employment_type', effective_date VARCHAR(10) PATH '$.effective_date')) e
			WHERE e.employment_type<>'' AND e.effective_date<=? ORDER BY seq DESC LIMIT 1),''),
			NULLIF(JSON_UNQUOTE(JSON_EXTRACT(o.payload,'$.employment_type')),''),u.employment_type,'') AS employment_type,
		CASE WHEN p.user_id IS NOT NULL THEN COALESCE(DATE_FORMAT(p.joined,'%Y-%m-%d'),'')
			ELSE COALESCE((SELECT e.effective_date FROM JSON_TABLE(o.payload, '$.events[*]' COLUMNS
			(seq FOR ORDINALITY, kind VARCHAR(16) PATH '$.kind', effective_date VARCHAR(10) PATH '$.effective_date')) e
			WHERE e.kind IN ('hire','rehire') AND e.effective_date<=? ORDER BY seq DESC LIMIT 1),
			JSON_UNQUOTE(JSON_EXTRACT(o.payload,'$.joined')),'') END AS joined,
		COALESCE(p.manager_id,(SELECT e.manager_id FROM JSON_TABLE(o.payload, '$.events[*]' COLUMNS
			(seq FOR ORDINALITY, manager_id BIGINT PATH '$.manager_id', effective_date VARCHAR(10) PATH '$.effective_date')) e
			WHERE e.manager_id>0 AND e.effective_date<=? ORDER BY seq DESC LIMIT 1),
			CAST(NULLIF(JSON_UNQUOTE(JSON_EXTRACT(o.payload,'$.manager_id')),'null') AS SIGNED),0) AS manager_override_id,
		COALESCE(p.version,0) AS profile_version, COALESCE(p.job_title,'') AS job_title,
		COALESCE(p.job_level,'') AS job_level, COALESCE(p.cost_center,'') AS cost_center,
		(u.is_config_super_admin OR EXISTS (SELECT 1 FROM user_roles ur WHERE ur.user_id=u.id AND LOWER(ur.role)='superadmin')
			OR EXISTS (SELECT 1 FROM auth_user_role_assignments ar JOIN auth_roles r ON r.id=ar.role_id
				WHERE ar.user_id=u.id AND r.code='super_admin')
			OR EXISTS (SELECT 1 FROM auth_org_role_policies op JOIN auth_roles r ON r.id=op.role_id
				WHERE r.code='super_admin' AND r.archived_at IS NULL AND op.enabled=1
				AND ((op.subject_type='department' AND op.subject_id=u.department_id) OR (op.subject_type='team' AND op.subject_id=u.team_id)))) AS protected
	FROM users u LEFT JOIN oa_office_people o ON o.id=u.id
	LEFT JOIN oa_office_person_profiles p ON p.user_id=u.id
	LEFT JOIN oa_office_person_contacts c ON c.user_id=u.id
	WHERE u.status<>'deleted'
), lifecycle AS (
	SELECT person.id,
		COALESCE(MAX(CASE WHEN e.kind='leave' AND e.day<=? THEN e.day END),'') AS left_on,
		COALESCE(MAX(CASE WHEN e.kind IN ('hire','rehire') AND e.day<=? THEN e.day END),'') AS hired_on,
		COALESCE(MAX(CASE WHEN e.kind='confirm' AND e.day<=? THEN e.day END),'') AS confirmed_on,
		COALESCE(MAX(CASE WHEN e.kind='leave_plan' THEN e.day END),'') AS leaving_on
	FROM oa_office_people person JOIN JSON_TABLE(person.payload,'$.events[*]' COLUMNS (
		kind VARCHAR(32) PATH '$.kind', day VARCHAR(10) PATH '$.effective_date')) e ON TRUE
	GROUP BY person.id
), directory AS (
	SELECT b.*,
		CASE WHEN l.left_on<>'' AND l.left_on>=l.hired_on AND b.status<>'active' THEN 'left'
			WHEN b.status<>'active' THEN 'disabled'
			WHEN l.leaving_on>l.left_on AND l.leaving_on>=l.hired_on THEN 'leaving'
			WHEN l.confirmed_on<>'' AND l.confirmed_on>=l.hired_on THEN 'regular'
			WHEN l.hired_on<>'' THEN 'probation' ELSE 'active' END AS hr_status,
		CASE WHEN b.manager_override_id>0 THEN COALESCE(om.id,0)
			WHEN tm.id IS NOT NULL THEN tm.id WHEN dm.id IS NOT NULL THEN dm.id ELSE 0 END AS manager_id,
		CASE WHEN b.manager_override_id>0 THEN 'person'
			WHEN tm.id IS NOT NULL THEN 'team' WHEN dm.id IS NOT NULL THEN 'department' ELSE '' END AS manager_source,
		CASE WHEN b.manager_override_id>0 THEN COALESCE(om.display_name,'') ELSE COALESCE(tm.display_name,dm.display_name,'') END AS manager_name
	FROM base b LEFT JOIN lifecycle l ON l.id=b.id LEFT JOIN org_teams t ON t.id=b.team_id AND t.department_id=b.department_id AND t.enabled=1
	LEFT JOIN org_departments d ON d.id=b.department_id AND d.enabled=1
	LEFT JOIN oa_office_org_managers ot ON ot.org_kind='team' AND ot.org_id=t.id AND d.id IS NOT NULL
	LEFT JOIN oa_office_org_managers od ON od.org_kind='department' AND od.org_id=d.id
	LEFT JOIN users om ON om.id=b.manager_override_id AND om.status='active' AND om.id<>b.id
	LEFT JOIN users tm ON tm.id=ot.manager_id AND tm.status='active' AND tm.id<>b.id AND tm.team_id=t.id AND tm.department_id=d.id AND COALESCE(od.manager_id,0)<>b.id
	LEFT JOIN users dm ON dm.id=od.manager_id AND dm.status='active' AND dm.id<>b.id AND dm.department_id=d.id
) `

func directoryDates() []any {
	today := businessDate().Format(time.DateOnly)
	return []any{today, today, today, today, today, today}
}

func (app office) effectiveManager(r *http.Request, id int64) (int64, error) {
	var manager int64
	err := app.db.QueryRowContext(r.Context(), directoryPeople+`SELECT manager_id FROM directory WHERE id=?`, append(directoryDates(), id)...).Scan(&manager)
	return manager, err
}

func (app office) applyProfile(ctx context.Context, p *person) error {
	return app.db.QueryRowContext(ctx, directoryPeople+`SELECT manager_id,manager_override_id,manager_source,manager_name,joined,job_title,job_level,cost_center,profile_version,employment_type FROM directory WHERE id=?`, append(directoryDates(), p.ID)...).
		Scan(&p.ManagerID, &p.ManagerOverrideID, &p.ManagerSource, &p.ManagerName, &p.Joined, &p.JobTitle, &p.JobLevel, &p.CostCenter, &p.ProfileVersion, &p.EmploymentType)
}

func validProfile(input profileUpdate) bool {
	if input.Version < 0 || input.ManagerID < 0 || utf8.RuneCountInString(input.JobTitle) > 100 || utf8.RuneCountInString(input.JobLevel) > 100 || utf8.RuneCountInString(input.CostCenter) > 100 {
		return false
	}
	if input.Joined == "" {
		return true
	}
	day, err := time.Parse(time.DateOnly, input.Joined)
	return err == nil && day.Year() >= 1000 && input.Joined <= businessDate().Format(time.DateOnly)
}

var errManagerCycle = errors.New("直属负责人关系形成循环")
var errManagerInvalid = errors.New("负责人必须为本人以外的在职人员，并在管理范围内")

func lockManagers(ctx context.Context, tx *sql.Tx) error {
	var version int64
	return tx.QueryRowContext(ctx, `SELECT version FROM oa_office_org_managers WHERE org_kind='lock' AND org_id=0 FOR UPDATE`).Scan(&version)
}

func checkManagerGraph(ctx context.Context, tx *sql.Tx) error {
	rows, err := tx.QueryContext(ctx, directoryPeople+`SELECT id,manager_id FROM directory WHERE status='active'`, directoryDates()...)
	if err != nil {
		return err
	}
	defer rows.Close()
	graph := map[int64]int64{}
	for rows.Next() {
		var id, manager int64
		if err := rows.Scan(&id, &manager); err != nil {
			return err
		}
		graph[id] = manager
	}
	if err := rows.Err(); err != nil {
		return err
	}
	return validateManagerGraph(graph)
}

func validateManagerGraph(graph map[int64]int64) error {
	checked := map[int64]bool{}
	for id := range graph {
		path := map[int64]bool{}
		for current := id; current != 0 && !checked[current]; current = graph[current] {
			if path[current] {
				return errManagerCycle
			}
			path[current] = true
		}
		for current := range path {
			checked[current] = true
		}
	}
	return nil
}

func (app office) allowedManager(r *http.Request, actor platform.Actor, id int64) (person, error) {
	p, err := app.readPerson(r, id)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return p, err
	}
	if errors.Is(err, sql.ErrNoRows) || p.Status != "active" || !scopePerson(actor, p) {
		return p, errManagerInvalid
	}
	return p, nil
}

func writeProfile(ctx context.Context, tx *sql.Tx, id, actor int64, input profileUpdate) error {
	var version int64
	var oldJob, oldLevel, oldJoined string
	var oldManager int64
	err := tx.QueryRowContext(ctx, `SELECT version,job_title,job_level,COALESCE(DATE_FORMAT(joined,'%Y-%m-%d'),''),manager_id
		FROM oa_office_person_profiles WHERE user_id=? FOR UPDATE`, id).Scan(&version, &oldJob, &oldLevel, &oldJoined, &oldManager)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if version != input.Version {
		return errProfileVersion
	}
	input.JobTitle = strings.TrimSpace(input.JobTitle)
	input.JobLevel = strings.TrimSpace(input.JobLevel)
	_, err = tx.ExecContext(ctx, `INSERT INTO oa_office_person_profiles(user_id,version,manager_id,joined,job_title,job_level,cost_center,updated_by)
		VALUES(?,?,?,NULLIF(?,''),?,?,?,?) ON DUPLICATE KEY UPDATE version=VALUES(version),manager_id=VALUES(manager_id),joined=VALUES(joined),
		job_title=VALUES(job_title),job_level=VALUES(job_level),cost_center=VALUES(cost_center),updated_by=VALUES(updated_by)`,
		id, version+1, input.ManagerID, input.Joined, input.JobTitle, input.JobLevel, strings.TrimSpace(input.CostCenter), actor)
	if err != nil {
		return err
	}
	if input.EmergencyContact != nil || input.EmploymentType != nil {
		contact := any(nil)
		if input.EmergencyContact != nil {
			contact = jsonData(input.EmergencyContact)
		}
		_, err = tx.ExecContext(ctx, `INSERT INTO oa_office_person_contacts(user_id,payload,employment_type)
			VALUES(?,COALESCE(?,JSON_OBJECT()),COALESCE(?,''))
			ON DUPLICATE KEY UPDATE payload=COALESCE(?,payload),employment_type=COALESCE(?,employment_type)`,
			id, contact, input.EmploymentType, contact, input.EmploymentType)
		if err != nil {
			return err
		}
	}
	if oldJoined == "" && input.Joined != "" {
		var exists bool
		err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM oa_office_people WHERE id=?
			AND JSON_SEARCH(payload,'one','hire',NULL,'$.events[*].kind') IS NOT NULL)`, id).Scan(&exists)
		if err != nil {
			return err
		}
		if !exists {
			if err := recordPersonEvent(ctx, tx, id, change{Kind: "hire_profile", EffectiveDate: input.Joined, ActorID: actor}); err != nil {
				return err
			}
		}
	}
	day := input.EffectiveDate
	if day == "" {
		day = businessDate().Format(time.DateOnly)
	}
	if oldJob != input.JobTitle || oldLevel != input.JobLevel {
		kind := "job_change"
		if oldJob == input.JobTitle {
			kind = "level_change"
		}
		if err := recordPersonEvent(ctx, tx, id, change{Kind: kind, EffectiveDate: day, ActorID: actor,
			PreviousJobTitle: oldJob, PreviousJobLevel: oldLevel, JobTitle: input.JobTitle, JobLevel: input.JobLevel}); err != nil {
			return err
		}
	}
	if oldManager != input.ManagerID {
		return recordPersonEvent(ctx, tx, id, change{Kind: "manager_change", EffectiveDate: day, ActorID: actor, ManagerID: input.ManagerID})
	}
	return nil
}

var errProfileVersion = errors.New("档案已变化，请刷新后重试")

func (app office) updateProfile(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), platform.ActorFrom(r)
	p, err := app.readPerson(r, number(r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !scopePerson(actor, p) {
		deny(w)
		return
	}
	var input profileUpdate
	if !decode(w, r, &input) {
		return
	}
	input.Joined = strings.TrimSpace(input.Joined)
	if !validProfile(input) {
		bad(w, "入职日期须为今日或过去，岗位、职级、成本中心各限 100 字")
		return
	}
	if input.EmploymentType != nil && !slices.Contains([]string{"", "full_time", "part_time", "outsourced"}, *input.EmploymentType) {
		bad(w, "用工类型无效")
		return
	}
	if input.EmergencyContact != nil && !input.EmergencyContact.normalize() {
		bad(w, "紧急联系人须同时填写姓名和电话")
		return
	}
	if input.EffectiveDate != "" {
		day, err := time.Parse(time.DateOnly, input.EffectiveDate)
		if err != nil || day.Year() < 1000 || input.EffectiveDate > businessDate().Format(time.DateOnly) {
			bad(w, "档案变更生效日期须为今日或过去")
			return
		}
	}
	if input.ManagerID == p.ID {
		bad(w, errManagerInvalid.Error())
		return
	}
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if err = lockManagers(ctx, tx); err != nil {
		internal(w)
		return
	}
	if input.ManagerID != 0 {
		_, err = app.allowedManager(r, actor, input.ManagerID)
		if err != nil {
			profileFailure(w, err)
			return
		}
	}
	if err = writeProfile(ctx, tx, p.ID, actor.ID, input); err != nil {
		profileFailure(w, err)
		return
	}
	if err = checkManagerGraph(ctx, tx); err != nil {
		profileFailure(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		internal(w)
		return
	}
	p, err = app.readPerson(r, p.ID)
	if err != nil {
		internal(w)
		return
	}
	if err := app.applyProfile(ctx, &p); err != nil {
		internal(w)
		return
	}
	if err := app.applyPersonRecords(r, &p); err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, p)
}

func profileFailure(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, errProfileVersion):
		conflict(w)
	case errors.Is(err, errManagerCycle), errors.Is(err, errManagerInvalid):
		bad(w, err.Error())
	default:
		internal(w)
	}
}

func orgManagerScope(actor platform.Actor, p orgManager) bool {
	if actor.SuperAdmin || slices.Contains(actor.PeopleDepartments, companyPeopleScope) {
		return true
	}
	if !permit(actor, "people.manage") {
		return false
	}
	if slices.Contains(actor.PeopleDepartments, p.DepartmentID) {
		return true
	}
	if p.Kind == "team" && slices.Contains(actor.PeopleTeams, p.ID) {
		return true
	}
	return len(actor.PeopleDepartments) == 0 && len(actor.PeopleTeams) == 0 && actor.DepartmentID == p.DepartmentID
}

const orgManagerDirectory = `
	SELECT o.kind,o.id,o.name,o.department_id,COALESCE(m.version,0),COALESCE(m.manager_id,0),COALESCE(u.display_name,'')
	FROM (
		SELECT 'department' AS kind,id,name,id AS department_id FROM org_departments WHERE enabled=1
		UNION ALL
		SELECT 'team',t.id,t.name,t.department_id FROM org_teams t JOIN org_departments d ON d.id=t.department_id AND d.enabled=1 WHERE t.enabled=1
	) o LEFT JOIN oa_office_org_managers m ON m.org_kind=o.kind AND m.org_id=o.id
	LEFT JOIN users u ON u.id=m.manager_id AND u.status<>'deleted' `

func (app office) orgManagers(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	rows, err := app.db.QueryContext(r.Context(), orgManagerDirectory+`ORDER BY o.department_id,o.kind,o.name,o.id`)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []orgManager{}
	for rows.Next() {
		var item orgManager
		if err = rows.Scan(&item.Kind, &item.ID, &item.Name, &item.DepartmentID, &item.Version, &item.ManagerID, &item.ManagerName); err != nil {
			internal(w)
			return
		}
		if orgManagerScope(actor, item) {
			items = append(items, item)
		}
	}
	if rows.Err() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items})
}

func (app office) updateOrgManager(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	kind, id := r.PathValue("kind"), number(r.PathValue("id"))
	if !slices.Contains([]string{"department", "team"}, kind) || id <= 0 {
		missing(w)
		return
	}
	input := struct {
		Version   int64 `json:"version"`
		ManagerID int64 `json:"manager_id"`
	}{}
	if !decode(w, r, &input) {
		return
	}
	if input.Version < 0 || input.ManagerID < 0 {
		bad(w, "负责人配置无效")
		return
	}
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if err = lockManagers(ctx, tx); err != nil {
		internal(w)
		return
	}
	var item orgManager
	err = tx.QueryRowContext(ctx, orgManagerDirectory+`WHERE o.kind=? AND o.id=?`, kind, id).
		Scan(&item.Kind, &item.ID, &item.Name, &item.DepartmentID, &item.Version, &item.ManagerID, &item.ManagerName)
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !orgManagerScope(actor, item) {
		deny(w)
		return
	}
	if item.Version != input.Version {
		conflict(w)
		return
	}
	item.ManagerName = ""
	if input.ManagerID != 0 {
		p, err := app.allowedManager(r, actor, input.ManagerID)
		if err != nil {
			profileFailure(w, err)
			return
		}
		if p.DepartmentID != item.DepartmentID || kind == "team" && p.TeamID != item.ID {
			bad(w, "请选择属于该组织的在职负责人")
			return
		}
		item.ManagerName = p.Name
	}
	item.Version++
	item.ManagerID = input.ManagerID
	_, err = tx.ExecContext(ctx, `INSERT INTO oa_office_org_managers(org_kind,org_id,manager_id,version,updated_by) VALUES(?,?,?,?,?)
		ON DUPLICATE KEY UPDATE manager_id=VALUES(manager_id),version=VALUES(version),updated_by=VALUES(updated_by)`, kind, id, item.ManagerID, item.Version, actor.ID)
	if err != nil {
		internal(w)
		return
	}
	if err = checkManagerGraph(ctx, tx); err != nil {
		profileFailure(w, err)
		return
	}
	if err = tx.Commit(); err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, item)
}

func rosterScope(actor platform.Actor) (string, []any) {
	if actor.SuperAdmin {
		return "", nil
	}
	if slices.Contains(actor.PeopleDepartments, companyPeopleScope) {
		return " AND NOT protected", nil
	}
	var terms []string
	var args []any
	for _, id := range actor.PeopleDepartments {
		terms = append(terms, "department_id=?")
		args = append(args, id)
	}
	for _, id := range actor.PeopleTeams {
		terms = append(terms, "team_id=?")
		args = append(args, id)
	}
	if len(terms) == 0 && actor.DepartmentID > 0 {
		terms = append(terms, "department_id=?")
		args = append(args, actor.DepartmentID)
	}
	if len(terms) == 0 {
		return " AND 1=0", nil
	}
	return " AND NOT protected AND (" + strings.Join(terms, " OR ") + ")", args
}

func employeeSearchNumber(value string) any {
	if len(value) == 0 || len(value) > 4 || strings.Trim(value, "0123456789") != "" {
		return nil
	}
	number, err := strconv.Atoi(value)
	if err != nil {
		return nil
	}
	return number
}

func pageValue(s string, fallback int) (int, error) {
	if s == "" {
		return fallback, nil
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return 0, fmt.Errorf("invalid page")
	}
	return v, nil
}

func rosterOrder(field, direction string) (string, bool) {
	order := " ASC"
	switch direction {
	case "", "asc":
	case "desc":
		order = " DESC"
	default:
		return "", false
	}
	switch field {
	case "":
		return "id", direction == ""
	case "name":
		return "(name=''),name" + order + ",id", true
	case "employee_no":
		return "(employee_no IS NULL),employee_no" + order + ",id", true
	case "joined":
		return "(joined=''),joined" + order + ",id", true
	default:
		return "", false
	}
}

func (app office) rosterPeople(w http.ResponseWriter, r *http.Request) {
	ctx, actor := r.Context(), platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	q, queryErr := url.ParseQuery(r.URL.RawQuery)
	if queryErr != nil {
		bad(w, "花名册查询参数无效")
		return
	}
	page, e1 := pageValue(q.Get("page"), 1)
	size, e2 := pageValue(q.Get("page_size"), 25)
	if e1 != nil || e2 != nil || size > 100 || page > 1000000 {
		bad(w, "分页参数无效")
		return
	}
	order, ok := rosterOrder(q.Get("sort"), q.Get("direction"))
	if !ok {
		bad(w, "花名册排序参数无效")
		return
	}
	where, scopeArgs := rosterScope(actor)
	where = " WHERE 1=1" + where
	args := append(directoryDates(), scopeArgs...)
	if query := strings.TrimSpace(q.Get("q")); query != "" {
		where += " AND (LOCATE(?,name)>0 OR LOCATE(?,CAST(employee_no AS CHAR))>0 OR employee_no=?)"
		args = append(args, query, query, employeeSearchNumber(query))
	}
	if status := q.Get("status"); status != "" {
		if status != "active" && status != "disabled" {
			bad(w, "人员状态无效")
			return
		}
		where += " AND status=?"
		args = append(args, status)
	}
	for _, field := range []string{"department_id", "team_id"} {
		if value := q.Get(field); value != "" {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil || id <= 0 {
				bad(w, "组织筛选无效")
				return
			}
			where += " AND " + field + "=?"
			args = append(args, id)
		}
	}
	if hrStatus := q.Get("hr_status"); hrStatus != "" {
		if !slices.Contains([]string{"active", "probation", "regular", "leaving", "left", "disabled"}, hrStatus) {
			bad(w, "在职状态无效")
			return
		}
		where += " AND hr_status=?"
		args = append(args, hrStatus)
	}
	if employment := q.Get("employment_type"); employment != "" {
		where += " AND employment_type=?"
		args = append(args, employment)
	}
	countWhere := where
	switch q.Get("missing") {
	case "":
	case "manager":
		where += " AND manager_id=0"
	case "joined":
		where += " AND joined=''"
	case "employee_no":
		where += " AND employee_no IS NULL"
	default:
		bad(w, "档案缺项筛选无效")
		return
	}
	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true, Isolation: sql.LevelRepeatableRead})
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	var managerMissing, joinedMissing, employeeMissing int
	if err = tx.QueryRowContext(ctx, directoryPeople+`SELECT COALESCE(SUM(manager_id=0),0),COALESCE(SUM(joined=''),0),
		COALESCE(SUM(employee_no IS NULL),0) FROM directory`+countWhere, args...).Scan(&managerMissing, &joinedMissing, &employeeMissing); err != nil {
		internal(w)
		return
	}
	var total int
	if err = tx.QueryRowContext(ctx, directoryPeople+`SELECT COUNT(*) FROM directory`+where, args...).Scan(&total); err != nil {
		internal(w)
		return
	}
	rows, err := tx.QueryContext(ctx, directoryPeople+`SELECT id,name,employee_no,status,hr_status,department_id,team_id,department,team,employment_type,joined,
		manager_override_id,manager_id,manager_source,manager_name,job_title,job_level,cost_center,profile_version
		FROM directory`+where+" ORDER BY "+order+" LIMIT ? OFFSET ?", append(args, size, (page-1)*size)...)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []person{}
	for rows.Next() {
		var p person
		if err = rows.Scan(&p.ID, &p.Name, &p.EmployeeNo, &p.Status, &p.HRStatus, &p.DepartmentID, &p.TeamID, &p.Department, &p.Team, &p.EmploymentType, &p.Joined,
			&p.ManagerOverrideID, &p.ManagerID, &p.ManagerSource, &p.ManagerName, &p.JobTitle, &p.JobLevel, &p.CostCenter, &p.ProfileVersion); err != nil {
			internal(w)
			return
		}
		items = append(items, p)
	}
	if err = rows.Err(); err != nil {
		internal(w)
		return
	}
	if err = rows.Close(); err != nil {
		internal(w)
		return
	}
	if err = tx.Commit(); err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items, "total": total, "page": page, "page_size": size, "completeness": map[string]int{"manager": managerMissing, "joined": joinedMissing, "employee_no": employeeMissing}})
}
