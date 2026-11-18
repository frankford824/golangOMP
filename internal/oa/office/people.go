package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"workflow/internal/oa/platform"
)

type person struct {
	ID             int64    `json:"id"`
	Name           string   `json:"name"`
	EmployeeNo     *int     `json:"employee_no"`
	DepartmentID   int64    `json:"department_id"`
	TeamID         int64    `json:"team_id"`
	Department     string   `json:"department"`
	Team           string   `json:"team"`
	Status         string   `json:"status"`
	EmploymentType string   `json:"employment_type"`
	Joined         string   `json:"joined"`
	Left           string   `json:"left"`
	Roles          []string `json:"roles"`
	Management     string   `json:"management"`
	ManagerID      int64    `json:"manager_id"`
	Version        int      `json:"version"`
	Events         []change `json:"events"`
	Terms          []term   `json:"terms"`
}
type term struct {
	ShopID          string `json:"shop_id"`
	From            string `json:"from"`
	To              string `json:"to"`
	PendingHandover bool   `json:"pending_handover,omitempty"`
}

var businessRoles = map[string]string{"定制设计师": "customization_operator", "审核": "auditor", "常规设计师": "designer", "运营": "operations", "素材提交": "asset_submitter", "素材管理": "asset_manager", "素材结算": "asset_settlement"}

// ShopTerms returns an OA override only after at least one local term was recorded.
func ShopTerms(ctx context.Context, db *sql.DB, actorID int64) ([]string, bool, error) {
	var raw []byte
	err := db.QueryRowContext(ctx, "SELECT payload FROM oa_office_people WHERE id=?", actorID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	var p person
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, false, err
	}
	today := businessDate().Format(time.DateOnly)
	shops := []string{}
	known := false
	for _, term := range p.Terms {
		if term.From <= today {
			known = true
			if (term.To == "" || term.To > today) && !slices.Contains(shops, term.ShopID) {
				shops = append(shops, term.ShopID)
			}
		}
	}
	return shops, known, nil
}

func ShopOverrides(ctx context.Context, db *sql.DB, actorID int64) (map[string]bool, error) {
	var raw []byte
	err := db.QueryRowContext(ctx, "SELECT payload FROM oa_office_people WHERE id=?", actorID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p person
	if err = json.Unmarshal(raw, &p); err != nil {
		return nil, err
	}
	return shopOverrides(p.Terms, businessDate().Format(time.DateOnly)), nil
}

func shopOverrides(terms []term, today string) map[string]bool {
	overrides := map[string]bool{}
	for _, item := range terms {
		if item.From > today {
			continue
		}
		overrides[item.ShopID] = overrides[item.ShopID] || item.To == "" || item.To > today
	}
	return overrides
}

type change struct {
	Kind           string   `json:"kind"`
	EffectiveDate  string   `json:"effective_date"`
	DepartmentID   int64    `json:"department_id"`
	TeamID         int64    `json:"team_id"`
	ShopID         string   `json:"shop_id"`
	FromShopID     string   `json:"from_shop_id"`
	TargetID       int64    `json:"target_id"`
	Roles          []string `json:"roles"`
	Management     string   `json:"management"`
	EmploymentType string   `json:"employment_type"`
	ManagerID      int64    `json:"manager_id"`
	Version        int      `json:"version"`
	ActorID        int64    `json:"actor_id"`
}

func (p *person) project() {
	now := businessDate().Format(time.DateOnly)
	for _, event := range p.Events {
		if event.EffectiveDate > now {
			continue
		}
		switch event.Kind {
		case "hire", "rehire":
			p.Joined = event.EffectiveDate
			p.Left = ""
			p.Status = "active"
			p.Roles = []string{}
			p.Management = "none"
		case "leave":
			p.Left = event.EffectiveDate
			p.Status = "inactive"
			p.Roles = []string{}
			p.Management = "none"
		case "roles":
			p.Roles = event.Roles
			p.Management = event.Management
		}
		if event.DepartmentID != 0 {
			p.DepartmentID = event.DepartmentID
			p.TeamID = event.TeamID
		}
		if event.EmploymentType != "" {
			p.EmploymentType = event.EmploymentType
		}
		if event.ManagerID != 0 {
			p.ManagerID = event.ManagerID
		}
	}
}

func ApplyIdentity(ctx context.Context, db, source *sql.DB, actor platform.Actor) (platform.Actor, bool) {
	roleCodes, departments, teams, err := effectivePeopleAccess(ctx, source, actor.ID)
	if err != nil {
		return actor, false
	}
	actor.PeopleDepartments, actor.PeopleTeams = departments, teams
	var pay, report, templates bool
	err = db.QueryRowContext(ctx, "SELECT COALESCE((SELECT can_pay FROM oa_office_finance_actions WHERE user_id=?), FALSE), COALESCE((SELECT can_report FROM oa_office_finance_actions WHERE user_id=?), FALSE), EXISTS(SELECT 1 FROM oa_office_delegates WHERE user_id=? AND level IN ('draft','publish'))", actor.ID, actor.ID, actor.ID).Scan(&pay, &report, &templates)
	if err != nil {
		return actor, false
	}
	if actor.Actions == nil {
		actor.Actions = map[string]bool{}
	}
	// Core exposes role-based capabilities even if an old frontend only labels
	// the session "member". Resolve the operating role from its actual grants.
	actor.Roles = slices.Clone(roleCodes)
	if slices.Contains(roleCodes, "operations") {
		actor.Actions["orders.view"] = true
	}
	if len(departments) > 0 || len(teams) > 0 {
		actor.Actions["people.manage"] = true
	}
	delete(actor.Actions, "finance.pay")
	delete(actor.Actions, "finance.report")
	delete(actor.Actions, "finance.templates")
	if pay {
		actor.Actions["finance.pay"] = true
	}
	if report {
		actor.Actions["finance.report"] = true
	}
	if templates || financeAdmin(actor) {
		actor.Actions["finance.templates"] = true
	}
	if actor.SuperAdmin {
		actor.Actions["people.manage"] = true
	}
	return actor, true
}

func (app office) mountPeople(mux *http.ServeMux) {
	const path = "/oa-api/people/"
	mux.HandleFunc("GET "+path+"list", app.peopleList)
	mux.HandleFunc("GET "+path+"{id}", app.personDetail)
	mux.HandleFunc("GET "+path+"{id}/handover", app.handoverItems)
	mux.HandleFunc("GET "+path+"{id}/shop-bindings", app.readShopBindings)
	mux.HandleFunc("POST "+path+"{id}/shop-bindings", app.changeShopBindings)
	mux.HandleFunc("POST "+path+"{id}/change", app.changePerson)
	mux.HandleFunc("GET "+path+"export", app.exportPeople)
	mux.HandleFunc("POST "+path+"preview", app.previewPeople)
	mux.HandleFunc("POST "+path+"confirm", app.confirmPeople)
}
func (app office) sourcePerson(r *http.Request, id int64) (person, error) {
	p := person{Roles: []string{}, Events: []change{}, Terms: []term{}, Management: "none"}
	var protected bool
	err := app.source.QueryRowContext(r.Context(), "SELECT id,display_name,employee_no,COALESCE(department_id,0),COALESCE(team_id,0),COALESCE(department,''),COALESCE(team,''),status,employment_type,is_config_super_admin OR EXISTS(SELECT 1 FROM user_roles WHERE user_id=users.id AND LOWER(role)='superadmin') OR EXISTS(SELECT 1 FROM auth_user_role_assignments a JOIN auth_roles r ON r.id=a.role_id WHERE a.user_id=users.id AND r.code='super_admin') FROM users WHERE id=? AND status<>'deleted'", id).Scan(&p.ID, &p.Name, &p.EmployeeNo, &p.DepartmentID, &p.TeamID, &p.Department, &p.Team, &p.Status, &p.EmploymentType, &protected)
	if protected {
		p.Management = "super_admin"
	}
	return p, err
}
func mergePerson(p person, row interface{ Scan(...any) error }) (person, error) {
	var raw []byte
	err := row.Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	var stored person
	if err := json.Unmarshal(raw, &stored); err != nil {
		return p, err
	}
	p.Version = stored.Version
	p.Events = stored.Events
	p.Terms = stored.Terms
	p.Joined = stored.Joined
	if stored.EmploymentType != "" {
		p.EmploymentType = stored.EmploymentType
	}
	p.ManagerID = stored.ManagerID
	return p, nil
}
func (app office) readPerson(r *http.Request, id int64) (person, error) {
	p, err := app.sourcePerson(r, id)
	if err != nil {
		return p, err
	}
	p, err = mergePerson(p, app.db.QueryRowContext(r.Context(), "SELECT payload FROM oa_office_people WHERE id=?", id))
	if err == nil {
		departmentID, teamID, management, authoritativeStatus := p.DepartmentID, p.TeamID, p.Management, p.Status
		p.project()
		p.Status = authoritativeStatus
		if management == "super_admin" {
			p.Management = management
		}
		if p.DepartmentID != departmentID || p.TeamID != teamID {
			err = app.source.QueryRowContext(r.Context(), "SELECT d.name,COALESCE(t.name,'') FROM org_departments d LEFT JOIN org_teams t ON t.id=? AND t.department_id=d.id WHERE d.id=?", p.TeamID, p.DepartmentID).Scan(&p.Department, &p.Team)
		}
	}
	if err == nil {
		var codes []string
		codes, _, _, err = effectivePeopleAccess(r.Context(), app.source, id)
		for label, code := range businessRoles {
			if slices.Contains(codes, code) && !slices.Contains(p.Roles, label) {
				p.Roles = append(p.Roles, label)
			}
		}
		slices.Sort(p.Roles)
		if slices.Contains(codes, "super_admin") {
			p.Management = "super_admin"
		} else if slices.Contains(codes, "department_admin") {
			p.Management = "department"
		}
	}
	return p, err
}
func scopePerson(actor platform.Actor, p person) bool {
	return actor.SuperAdmin || (p.Management != "super_admin" && permit(actor, "people.manage") && (slices.Contains(actor.PeopleDepartments, companyPeopleScope) || slices.Contains(actor.PeopleDepartments, p.DepartmentID) || slices.Contains(actor.PeopleTeams, p.TeamID) || (len(actor.PeopleDepartments) == 0 && len(actor.PeopleTeams) == 0 && actor.DepartmentID > 0 && actor.DepartmentID == p.DepartmentID)))
}
func (app office) peopleList(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	items, err := app.selectPeople(r, actor)
	if err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items})
}
func (app office) selectPeople(r *http.Request, actor platform.Actor) ([]person, error) {
	q := "%" + strings.TrimSpace(r.URL.Query().Get("q")) + "%"
	rows, err := app.source.QueryContext(r.Context(), "SELECT id FROM users WHERE status<>'deleted' AND (display_name LIKE ? OR CAST(employee_no AS CHAR) LIKE ?) ORDER BY id", q, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []int64{}
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	items := []person{}
	for _, id := range ids {
		p, err := app.readPerson(r, id)
		if err != nil {
			return nil, err
		}
		if scopePerson(actor, p) {
			items = append(items, p)
		}
	}
	return items, nil
}
func (app office) personDetail(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	id := number(r.PathValue("id"))
	p, err := app.readPerson(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if id != actor.ID && !scopePerson(actor, p) {
		deny(w)
		return
	}
	platform.JSON(w, 200, p)
}
func lockPerson(r *http.Request, tx *sql.Tx, p person) (person, error) {
	ctx := r.Context()
	_, err := tx.ExecContext(ctx, "INSERT INTO oa_office_people(id,version,payload) VALUES(?,0,JSON_OBJECT()) ON DUPLICATE KEY UPDATE id=id", p.ID)
	if err != nil {
		return p, err
	}
	return mergePerson(p, tx.QueryRowContext(ctx, "SELECT payload FROM oa_office_people WHERE id=? FOR UPDATE", p.ID))
}
func savePerson(r *http.Request, tx *sql.Tx, p person) error {
	metadata := map[string]any{"version": p.Version, "events": p.Events, "terms": p.Terms, "joined": p.Joined, "employment_type": p.EmploymentType, "manager_id": p.ManagerID}
	_, err := tx.ExecContext(r.Context(), "UPDATE oa_office_people SET version=?,payload=? WHERE id=?", p.Version, jsonData(metadata), p.ID)
	return err
}
func (app office) changePerson(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	var input change
	if !decode(w, r, &input) {
		return
	}
	input.ActorID = actor.ID
	today := businessDate().Format(time.DateOnly)
	if input.EffectiveDate == "" {
		input.EffectiveDate = today
	}
	if _, err := time.Parse(time.DateOnly, input.EffectiveDate); err != nil || input.EffectiveDate < today {
		bad(w, "生效日期必须为今日或未来")
		return
	}
	if !slices.Contains([]string{"hire", "transfer", "shop_transfer", "leave", "handover", "rehire", "roles"}, input.Kind) {
		bad(w, "任职动作无效")
		return
	}
	if input.Management == "super_admin" {
		bad(w, "超级管理员由受保护的 OA 核心授权维护")
		return
	}
	if input.Kind == "roles" {
		platform.Error(w, 422, "业务与管理角色必须通过主站原生权限接口维护")
		return
	}
	if input.Kind == "transfer" {
		platform.Error(w, 422, "组织调动必须通过主站原生人员接口维护")
		return
	}
	if (input.Kind == "leave" || input.Kind == "rehire" || input.Kind == "hire") && input.EffectiveDate != today {
		bad(w, "请在生效日完成主站账号变更后登记")
		return
	}
	input.Roles = nil
	input.Management = ""
	if input.ManagerID != 0 && !app.active(r, input.ManagerID) {
		bad(w, "直属负责人必须为在职人员")
		return
	}
	id := number(r.PathValue("id"))
	if id == actor.ID && input.Kind == "leave" || input.ManagerID == id && id > 0 {
		bad(w, "该动作不能对本人执行")
		return
	}
	if input.DepartmentID > 0 {
		if !scopePerson(actor, person{DepartmentID: input.DepartmentID, TeamID: input.TeamID}) {
			deny(w)
			return
		}
		var found int64
		err := app.source.QueryRowContext(r.Context(), "SELECT id FROM org_departments WHERE id=? AND enabled=1", input.DepartmentID).Scan(&found)
		if err != nil {
			bad(w, "部门无效")
			return
		}
		if input.TeamID > 0 {
			err = app.source.QueryRowContext(r.Context(), "SELECT id FROM org_teams WHERE id=? AND department_id=? AND enabled=1", input.TeamID, input.DepartmentID).Scan(&found)
			if err != nil {
				bad(w, "团队不属于该部门")
				return
			}
		}
	} else if input.TeamID != 0 {
		bad(w, "团队须与部门同时提供")
		return
	}
	p, err := app.sourcePerson(r, id)
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if input.DepartmentID != 0 && (input.DepartmentID != p.DepartmentID || input.TeamID != p.TeamID) {
		platform.Error(w, 422, "请先通过原生人员接口完成组织变更")
		return
	}
	input.DepartmentID, input.TeamID = 0, 0
	if p.Management == "super_admin" && (input.Kind == "leave" || !actor.SuperAdmin) {
		deny(w)
		return
	}
	var target person
	if input.TargetID > 0 {
		target, err = app.sourcePerson(r, input.TargetID)
		if err != nil || target.ID == p.ID {
			bad(w, "接手人无效")
			return
		}
	}
	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if target.ID > 0 && target.ID < p.ID {
		target, err = lockPerson(r, tx, target)
		if err != nil {
			internal(w)
			return
		}
	}
	p, err = lockPerson(r, tx, p)
	if err != nil {
		internal(w)
		return
	}
	if target.ID > p.ID {
		target, err = lockPerson(r, tx, target)
		if err != nil {
			internal(w)
			return
		}
	}
	current := p
	current.project()
	if !scopePerson(actor, current) {
		deny(w)
		return
	}
	if p.Version != input.Version || len(p.Events) > 0 && p.Events[len(p.Events)-1].EffectiveDate > input.EffectiveDate {
		conflict(w)
		return
	}
	if input.Kind == "leave" && p.Status != "disabled" || (input.Kind == "rehire" || input.Kind == "hire") && p.Status != "active" {
		platform.Error(w, 422, "请先通过主站原生接口停用或启用该账号")
		return
	}
	if (input.Kind == "rehire" && current.Status != "inactive") || (input.Kind == "hire" && current.Joined != "") || (input.Kind == "shop_transfer" && current.Status != "active") {
		conflict(w)
		return
	}
	if target.ID > 0 {
		candidate := target
		candidate.project()
		if candidate.Status != "active" || !scopePerson(actor, candidate) {
			deny(w)
			return
		}
	}
	if input.Kind == "shop_transfer" && input.ShopID == "" && input.FromShopID == "" {
		bad(w, "请提供店铺")
		return
	}
	if input.Kind == "handover" && (target.ID == 0 || input.ShopID == "") {
		bad(w, "请选择店铺和接手人")
		return
	}
	if len(input.ShopID) > 128 || len(input.FromShopID) > 128 || len(input.EmploymentType) > 48 {
		bad(w, "店铺或用工类型超出长度限制")
		return
	}
	handed := false
	if app.baseline != nil && (input.Kind == "leave" || input.FromShopID != "" || input.Kind == "handover") {
		base, err := app.baseline(r.Context(), p.EmployeeNo)
		if err != nil {
			platform.Error(w, 503, "暂时无法核对店铺归属")
			return
		}
		overrides := shopOverrides(p.Terms, input.EffectiveDate)
		for shop := range base {
			if _, alreadyLocal := overrides[shop]; alreadyLocal {
				continue
			}
			if input.Kind == "leave" || shop == input.FromShopID || input.Kind == "handover" && shop == input.ShopID {
				addTerm(&p, shop, input.EffectiveDate)
			}
		}
	}
	if input.Kind == "leave" || input.FromShopID != "" || input.Kind == "handover" {
		for index := range p.Terms {
			item := &p.Terms[index]
			if input.Kind == "handover" && item.ShopID == input.ShopID && item.PendingHandover && item.To <= input.EffectiveDate {
				addTerm(&target, item.ShopID, input.EffectiveDate)
				item.PendingHandover = false
				handed = true
				continue
			}
			if (input.Kind == "leave" || item.ShopID == input.FromShopID || input.Kind == "handover" && item.ShopID == input.ShopID) && (item.To == "" || item.To > input.EffectiveDate) {
				item.To = input.EffectiveDate
				if item.From > item.To {
					item.To = item.From
				}
				if target.ID > 0 && item.From <= input.EffectiveDate {
					addTerm(&target, item.ShopID, input.EffectiveDate)
					handed = true
				} else if input.Kind == "leave" && item.From <= input.EffectiveDate {
					item.PendingHandover = true
				}
			}
		}
	}
	if input.Kind == "handover" && !handed {
		conflict(w)
		return
	}
	if (input.Kind == "hire" || input.Kind == "shop_transfer") && input.ShopID != "" {
		addTerm(&p, input.ShopID, input.EffectiveDate)
	}
	p.Version++
	p.Events = append(p.Events, input)
	if target.ID > 0 {
		target.Version++
		if err := savePerson(r, tx, target); err != nil {
			internal(w)
			return
		}
	}
	if savePerson(r, tx, p) != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	authoritativeStatus := p.Status
	p.project()
	p.Status = authoritativeStatus
	platform.JSON(w, 200, p)
}
func addTerm(p *person, shop, date string) {
	for _, old := range p.Terms {
		if old.ShopID == shop && old.From <= date && (old.To == "" || old.To > date) {
			return
		}
	}
	p.Terms = append(p.Terms, term{ShopID: shop, From: date})
}
