package office

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"workflow/internal/oa/platform"
)

type template struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Channels  []string `json:"channels"`
	Fields    []string `json:"fields"`
	Required  []string `json:"required"`
	Evidence  string   `json:"evidence"`
	Nodes     []string `json:"nodes"`
	Threshold int64    `json:"threshold_cents"`
	Extra     string   `json:"extra"`
	Version   int      `json:"version"`
}
type application struct {
	RequestID  string            `json:"request_id"`
	TemplateID string            `json:"template_id"`
	Amount     int64             `json:"amount_cents"`
	Reason     string            `json:"reason"`
	Channel    string            `json:"channel"`
	Holder     string            `json:"holder"`
	Account    string            `json:"account"`
	Bank       string            `json:"bank"`
	Extra      map[string]string `json:"extra"`
	Files      []string          `json:"files"`
	Site       string            `json:"site"`
}
type payment struct {
	Amount  int64  `json:"amount_cents"`
	Date    string `json:"date"`
	Receipt string `json:"receipt"`
}
type history struct {
	Actor   int64     `json:"actor_id"`
	Action  string    `json:"action"`
	Comment string    `json:"comment"`
	At      time.Time `json:"at"`
}
type document struct {
	application
	ID        string    `json:"id"`
	Applicant int64     `json:"applicant_id"`
	Template  template  `json:"template"`
	State     string    `json:"state"`
	Route     []int64   `json:"route"`
	Current   int       `json:"current"`
	Version   int       `json:"version"`
	History   []history `json:"history"`
	Payment   *payment  `json:"payment"`
}

func (app office) mountFinance(mux *http.ServeMux) {
	const path = "/oa-api/finance/"
	mux.HandleFunc("GET "+path+"templates", app.templates)
	mux.HandleFunc("GET "+path+"approvers", app.approvers)
	mux.HandleFunc("POST "+path+"approvers/search", app.searchApprovers)
	mux.HandleFunc("GET "+path+"notifications", app.notifications)
	mux.HandleFunc("GET "+path+"templates/{id}", app.templateDetail)
	mux.HandleFunc("PUT "+path+"templates/{id}", app.saveTemplate)
	mux.HandleFunc("GET "+path+"delegates", app.delegates)
	mux.HandleFunc("GET "+path+"grants", app.financeGrants)
	mux.HandleFunc("PUT "+path+"delegates/{id}", app.saveDelegate)
	mux.HandleFunc("PUT "+path+"actions/{id}", app.saveFinanceActions)
	mux.HandleFunc("GET "+path+"documents", app.documents)
	mux.HandleFunc("POST "+path+"documents", app.submit)
	mux.HandleFunc("GET "+path+"documents/{id}", app.detail)
	mux.HandleFunc("POST "+path+"documents/{id}/{operation}", app.mutateDocument)
	mux.HandleFunc("GET "+path+"report", app.report)
	mux.HandleFunc("POST "+path+"files", app.upload)
	mux.HandleFunc("GET "+path+"files/{id}", app.download)
}

func (app office) approvers(w http.ResponseWriter, r *http.Request) {
	cursor := r.URL.Query().Get("after_id")
	if cursor == "" {
		cursor = "0"
	}
	afterID, err := strconv.ParseInt(cursor, 10, 64)
	if err != nil {
		bad(w, "候选分页位置无效")
		return
	}
	app.findApprovers(w, r, r.URL.Query().Get("q"), r.URL.Query().Get("ids"), afterID, false)
}

func (app office) findApprovers(w http.ResponseWriter, r *http.Request, text, selectedIDs string, afterID int64, extended bool) {
	if afterID < 0 {
		bad(w, "候选分页位置无效")
		return
	}
	if utf8.RuneCountInString(text) > 256 {
		bad(w, "搜索词过长")
		return
	}
	actor := platform.ActorFrom(r)
	if !financeAdmin(actor) {
		level := app.delegate(r)
		if level != "draft" && level != "publish" {
			deny(w)
			return
		}
	}
	// Delegated maintainers keep their own department and any personnel scopes
	// they already hold. This search exposes directory labels only.
	text = strings.TrimSpace(text)
	scope, args := financeCandidateScope(actor)
	var ids []string
	if raw := selectedIDs; raw != "" {
		ids = strings.SplitN(raw, ",", 17)
		if len(ids) > 16 {
			bad(w, "最多查询 16 名已选审批人")
			return
		}
	}
	selected := []approverOption{}
	if len(ids) > 0 {
		selectedArgs := slices.Clone(args)
		for _, value := range ids {
			id, err := strconv.ParseInt(value, 10, 64)
			if err != nil || id <= 0 {
				bad(w, "已选审批人 ID 无效")
				return
			}
			selectedArgs = append(selectedArgs, id)
		}
		var err error
		selected, err = app.queryApprovers(r, scope+" AND id IN ("+strings.Repeat("?,", len(ids)-1)+"?) ORDER BY display_name,id", selectedArgs)
		if err != nil {
			internal(w)
			return
		}
	}
	var items []approverOption
	var err error
	if extended {
		items, err = app.searchApproverRows(r.Context(), scope, args, text, afterID)
	} else {
		q := "%" + text + "%"
		items, err = app.queryApprovers(r, scope+` AND id>? AND (display_name LIKE ? OR CAST(employee_no AS CHAR) LIKE ? OR department LIKE ?) ORDER BY id LIMIT 51`, append(args, afterID, q, q, q))
	}
	if err != nil {
		internal(w)
		return
	}
	var next int64
	if len(items) > approverPageSize {
		items = items[:approverPageSize]
		next = items[len(items)-1].ID
	}
	platform.JSON(w, 200, map[string]any{"items": items, "selected": selected, "next_after_id": next})
}

type approverOption struct {
	Department string `json:"department"`
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	EmployeeNo *int   `json:"employee_no"`
}

func (app office) queryApprovers(r *http.Request, query string, args []any) ([]approverOption, error) {
	rows, err := app.source.QueryContext(r.Context(), query, args...)
	if err != nil {
		return nil, err
	}
	result := []approverOption{}
	defer rows.Close()
	for rows.Next() {
		var item approverOption
		if err := rows.Scan(&item.ID, &item.Name, &item.EmployeeNo, &item.Department); err != nil {
			return nil, err
		}
		result = append(result, item)
	}
	return result, rows.Err()
}

func (app office) saveFinanceActions(w http.ResponseWriter, r *http.Request) {
	if !financeAdmin(platform.ActorFrom(r)) {
		deny(w)
		return
	}
	var input struct {
		Pay    *bool `json:"pay"`
		Report *bool `json:"report"`
	}
	if !decode(w, r, &input) {
		return
	}
	id := number(r.PathValue("id"))
	if id <= 0 || input.Pay == nil || input.Report == nil {
		bad(w, "请提供有效账号及付款、报表权限")
		return
	}
	var err error
	if !*input.Pay && !*input.Report {
		_, err = app.db.ExecContext(r.Context(), "UPDATE oa_office_finance_actions SET can_pay=FALSE,can_report=FALSE WHERE user_id=?", id)
	} else {
		if !app.active(r, id) {
			bad(w, "启用付款或报表权限须选择在职人员")
			return
		}
		_, err = app.db.ExecContext(r.Context(), "INSERT INTO oa_office_finance_actions(user_id,can_pay,can_report) VALUES(?,?,?) ON DUPLICATE KEY UPDATE can_pay=?,can_report=?", id, *input.Pay, *input.Report, *input.Pay, *input.Report)
	}
	if err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, input)
}

func (app office) templateLevel(r *http.Request) string {
	if financeAdmin(platform.ActorFrom(r)) {
		return "publish"
	}
	return app.delegate(r)
}

func (app office) delegate(r *http.Request) string {
	actor := platform.ActorFrom(r)
	var level string
	_ = app.db.QueryRowContext(r.Context(), "SELECT level FROM oa_office_delegates WHERE user_id=?", actor.ID).Scan(&level)
	return level
}
func (app office) delegates(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !financeAdmin(actor) {
		platform.JSON(w, 200, map[string]any{"level": app.templateLevel(r)})
		return
	}
	rows, err := app.db.QueryContext(r.Context(), "SELECT user_id,level FROM oa_office_delegates ORDER BY user_id")
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var level string
		if rows.Scan(&id, &level) != nil {
			internal(w)
			return
		}
		items = append(items, map[string]any{"id": id, "level": level})
	}
	if rows.Err() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items, "level": app.templateLevel(r)})
}
func (app office) saveDelegate(w http.ResponseWriter, r *http.Request) {
	if !financeAdmin(platform.ActorFrom(r)) {
		deny(w)
		return
	}
	var input struct {
		Level string `json:"level"`
	}
	if !decode(w, r, &input) {
		return
	}
	id := number(r.PathValue("id"))
	if id <= 0 || !slices.Contains([]string{"none", "draft", "publish"}, input.Level) {
		bad(w, "委托对象或级别无效")
		return
	}
	var err error
	if input.Level == "none" {
		_, err = app.db.ExecContext(r.Context(), "UPDATE oa_office_delegates SET level='none' WHERE user_id=?", id)
	} else {
		if !app.active(r, id) {
			bad(w, "授予模板维护权限须选择在职人员")
			return
		}
		_, err = app.db.ExecContext(r.Context(), "INSERT INTO oa_office_delegates(user_id,level) VALUES(?,?) ON DUPLICATE KEY UPDATE level=?", id, input.Level, input.Level)
	}
	if err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, input)
}
func (app office) templates(w http.ResponseWriter, r *http.Request) {
	expression := "published"
	level := app.templateLevel(r)
	if r.URL.Query().Get("draft") == "1" && (level == "draft" || level == "publish") {
		expression = "COALESCE(draft,published)"
	}
	rows, err := app.db.QueryContext(r.Context(), "SELECT "+expression+" FROM oa_office_templates WHERE "+expression+" IS NOT NULL ORDER BY id")
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []template{}
	for rows.Next() {
		var raw []byte
		var item template
		if rows.Scan(&raw) != nil || json.Unmarshal(raw, &item) != nil {
			internal(w)
			return
		}
		items = append(items, item)
	}
	if rows.Err() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items, "level": level})
}
func (app office) templateDetail(w http.ResponseWriter, r *http.Request) {
	if level := app.templateLevel(r); level != "draft" && level != "publish" {
		deny(w)
		return
	}
	var raw []byte
	err := app.db.QueryRowContext(r.Context(), "SELECT COALESCE(draft,published) FROM oa_office_templates WHERE id=?", r.PathValue("id")).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	var item template
	if err != nil || json.Unmarshal(raw, &item) != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, item)
}
func (app office) saveTemplate(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	var input struct {
		template
		Publish bool `json:"publish"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.ID = r.PathValue("id")
	if !textOK(input.ID, 64) || !textOK(input.Name, 120) || !slices.Contains([]string{"报销", "付款", "reimbursement", "payment"}, input.Type) || len(input.Channels) == 0 || len(input.Nodes) == 0 || len(input.Nodes) > 12 || len(input.Fields) > 30 || input.Threshold < 0 || !textOK(input.Evidence, 240) {
		bad(w, "请完整填写模板、凭证和审批节点")
		return
	}
	for _, channel := range input.Channels {
		if !slices.Contains([]string{"bank", "person", "wallet"}, channel) {
			bad(w, "支付渠道无效")
			return
		}
	}
	for _, field := range input.Required {
		if !slices.Contains(input.Fields, field) {
			bad(w, "必填字段未纳入模板")
			return
		}
	}
	nodes := slices.Clone(input.Nodes)
	if input.Threshold > 0 {
		nodes = append(nodes, input.Extra)
	}
	for _, node := range nodes {
		if node != "manager" && (!strings.HasPrefix(node, "fixed:") || !app.active(r, number(strings.TrimPrefix(node, "fixed:")))) {
			bad(w, "审批节点须为直属负责人或有效的指定人员 fixed:<用户ID>")
			return
		}
	}
	ctx := r.Context()
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if !financeAdmin(actor) {
		var level string
		err = tx.QueryRowContext(ctx, "SELECT level FROM oa_office_delegates WHERE user_id=? FOR UPDATE", actor.ID).Scan(&level)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			internal(w)
			return
		}
		if level != "publish" && (level != "draft" || input.Publish) {
			deny(w)
			return
		}
	}
	_, err = tx.ExecContext(ctx, "INSERT INTO oa_office_templates(id,version) VALUES(?,0) ON DUPLICATE KEY UPDATE id=id", input.ID)
	if err != nil {
		internal(w)
		return
	}
	var version int
	if tx.QueryRowContext(ctx, "SELECT version FROM oa_office_templates WHERE id=? FOR UPDATE", input.ID).Scan(&version) != nil {
		internal(w)
		return
	}
	if version != input.Version {
		conflict(w)
		return
	}
	if input.Publish {
		input.Version++
		if _, err = tx.ExecContext(ctx, "INSERT INTO oa_office_template_versions(id,version,publisher,payload) VALUES(?,?,?,?)", input.ID, input.Version, actor.ID, jsonData(input.template)); err != nil {
			internal(w)
			return
		}
		_, err = tx.ExecContext(ctx, "UPDATE oa_office_templates SET version=?,published=?,draft=NULL WHERE id=?", input.Version, jsonData(input.template), input.ID)
	} else {
		_, err = tx.ExecContext(ctx, "UPDATE oa_office_templates SET draft=? WHERE id=?", jsonData(input.template), input.ID)
	}
	if err != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, input.template)
}

func (app office) active(r *http.Request, id int64) bool {
	if id <= 0 || app.source == nil {
		return false
	}
	var status string
	if app.source.QueryRowContext(r.Context(), "SELECT status FROM users WHERE id=?", id).Scan(&status) != nil || status != "active" {
		return false
	}
	return true
}
func (app office) approvalRoute(r *http.Request, t template, amount, applicant int64) ([]int64, bool) {
	nodes := slices.Clone(t.Nodes)
	if t.Threshold > 0 && amount >= t.Threshold {
		nodes = append(nodes, t.Extra)
	}
	route := []int64{}
	for _, node := range nodes {
		id := number(strings.TrimPrefix(node, "fixed:"))
		if node == "manager" || id == applicant {
			person, err := app.readPerson(r, applicant)
			if err == nil {
				id = person.ManagerID
			}
			if id == 0 {
				id = number(templateFallback())
			}
		}
		if id == applicant || !app.active(r, id) {
			return nil, false
		}
		if !slices.Contains(route, id) {
			route = append(route, id)
		}
	}
	return route, len(route) > 0
}
func validateApplication(input application, t template) bool {
	if input.Amount <= 0 || input.Amount > 100000000000 || !textOK(input.Reason, 2000) || !slices.Contains(t.Channels, input.Channel) || !textOK(input.Holder, 240) || !textOK(input.Account, 128) || !textOK(input.Bank, 240) || len(input.Files) == 0 || len(input.Files) > 15 || !slices.Contains([]string{"main", "asset"}, input.Site) {
		return false
	}
	for key, value := range input.Extra {
		if !slices.Contains(t.Fields, key) || len(value) > 1000 {
			return false
		}
	}
	for _, key := range t.Required {
		if !textOK(input.Extra[key], 1000) {
			return false
		}
	}
	return true
}

func (doc document) canRead(actor platform.Actor) bool {
	return actor.ID == doc.Applicant || slices.Contains(doc.Route, actor.ID) || ((doc.State == "approved" || doc.State == "paid") && permit(actor, "finance.pay")) || (doc.State == "paid" && permit(actor, "finance.report"))
}
func (doc document) visible(actor platform.Actor, reveal bool) document {
	if !(reveal && doc.State == "approved" && actor.ID != doc.Applicant && permit(actor, "finance.pay")) {
		account := []rune(doc.Account)
		if len(account) > 4 {
			doc.Account = "•••• " + string(account[len(account)-4:])
		} else {
			doc.Account = "••••"
		}
	}
	return doc
}
func scanDocument(row interface{ Scan(...any) error }) (document, error) {
	var doc document
	var raw []byte
	if err := row.Scan(&raw); err != nil {
		return doc, err
	}
	return doc, json.Unmarshal(raw, &doc)
}
func (app office) detail(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	doc, err := scanDocument(app.db.QueryRowContext(r.Context(), "SELECT payload FROM oa_office_documents WHERE id=?", r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !doc.canRead(actor) {
		deny(w)
		return
	}
	platform.JSON(w, 200, doc.visible(actor, r.URL.Query().Get("reveal") == "1"))
}
func (app office) documents(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	where := "applicant=?"
	args := []any{actor.ID}
	switch r.URL.Query().Get("queue") {
	case "", "mine":
	case "todo":
		where = "state='waiting' AND approver=?"
	case "pay":
		if !permit(actor, "finance.pay") {
			deny(w)
			return
		}
		where = "state='approved' AND applicant<>?"
	default:
		bad(w, "队列无效")
		return
	}
	app.selectDocuments(w, r, where, args, false)
}
func (app office) report(w http.ResponseWriter, r *http.Request) {
	if !permit(platform.ActorFrom(r), "finance.report") {
		deny(w)
		return
	}
	value := r.URL.Query().Get("period")
	if value == "" {
		value = businessDate().Format("2006-01")
	}
	var format string
	switch len(value) {
	case 4:
		format = "2006"
	case 7:
		format = "2006-01"
	case 10:
		format = "2006-01-02"
	default:
		bad(w, "统计期间无效")
		return
	}
	start, err := time.Parse(format, value)
	if err != nil {
		bad(w, "统计期间无效")
		return
	}
	end := start.AddDate(0, 0, 1)
	if len(value) == 4 {
		end = start.AddDate(1, 0, 0)
	} else if len(value) == 7 {
		end = start.AddDate(0, 1, 0)
	}
	app.selectDocuments(w, r, "paid_date>=? AND paid_date<?", []any{start.Format(time.DateOnly), end.Format(time.DateOnly)}, true)
}
func (app office) selectDocuments(w http.ResponseWriter, r *http.Request, where string, args []any, isReport bool) {
	actor := platform.ActorFrom(r)
	rows, err := app.db.QueryContext(r.Context(), "SELECT payload FROM oa_office_documents WHERE "+where+" ORDER BY paid_date DESC,id", args...)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []document{}
	var sum int64
	for rows.Next() {
		doc, err := scanDocument(rows)
		if err != nil {
			internal(w)
			return
		}
		items = append(items, doc.visible(actor, false))
		if isReport && doc.Payment != nil {
			sum += doc.Payment.Amount
		}
	}
	if rows.Err() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"items": items, "amount_cents": sum, "count": len(items)})
}
func (app office) submit(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	var input application
	if !decode(w, r, &input) {
		return
	}
	if !textOK(input.RequestID, 80) {
		bad(w, "缺少请求编号")
		return
	}
	ctx := r.Context()
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	id := identifier()
	_, err = tx.ExecContext(ctx, "INSERT INTO oa_office_documents(id,applicant,request_id,state,payload) VALUES(?,?,?,'waiting',JSON_OBJECT()) ON DUPLICATE KEY UPDATE id=id", id, actor.ID, input.RequestID)
	if err != nil {
		internal(w)
		return
	}
	var found string
	var raw []byte
	err = tx.QueryRowContext(ctx, "SELECT id,payload FROM oa_office_documents WHERE applicant=? AND request_id=? FOR UPDATE", actor.ID, input.RequestID).Scan(&found, &raw)
	if err != nil {
		internal(w)
		return
	}
	if found != id {
		var doc document
		if json.Unmarshal(raw, &doc) != nil {
			internal(w)
			return
		}
		if jsonData(doc.application) != jsonData(input) {
			conflict(w)
			return
		}
		platform.JSON(w, 200, doc.visible(actor, false))
		return
	}
	var t template
	err = tx.QueryRowContext(ctx, "SELECT published FROM oa_office_templates WHERE id=? AND published IS NOT NULL", input.TemplateID).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		bad(w, "模板尚未发布")
		return
	}
	if err != nil || json.Unmarshal(raw, &t) != nil {
		internal(w)
		return
	}
	if !validateApplication(input, t) {
		bad(w, "请核对金额、用途、收款信息、模板必填字段和附件")
		return
	}
	route, ok := app.approvalRoute(r, t, input.Amount, actor.ID)
	if !ok {
		bad(w, "审批路线尚未配置有效负责人")
		return
	}
	doc := document{application: input, ID: id, Applicant: actor.ID, Template: t, State: "waiting", Route: route, Version: 1, History: []history{{actor.ID, "submit", "", time.Now().UTC()}}}
	if !linkFiles(r, tx, input.Files, actor.ID, id) {
		bad(w, "附件无效或已用于其他单据")
		return
	}
	if persistDocument(r, tx, doc) != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 201, doc.visible(actor, false))
}
func persistDocument(r *http.Request, tx *sql.Tx, doc document) error {
	var approver, date any
	if doc.State == "waiting" {
		approver = doc.Route[doc.Current]
	}
	if doc.Payment != nil {
		date = doc.Payment.Date
	}
	_, err := tx.ExecContext(r.Context(), "UPDATE oa_office_documents SET state=?,approver=?,paid_date=?,payload=? WHERE id=?", doc.State, approver, date, jsonData(doc), doc.ID)
	return err
}

func (app office) mutateDocument(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	ctx := r.Context()
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	doc, err := scanDocument(tx.QueryRowContext(ctx, "SELECT payload FROM oa_office_documents WHERE id=? FOR UPDATE", r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	operation := r.PathValue("operation")
	entry := history{Actor: actor.ID, Action: operation, At: time.Now().UTC()}
	switch operation {
	case "resubmit":
		if doc.Applicant != actor.ID {
			deny(w)
			return
		}
		if doc.State != "returned" {
			conflict(w)
			return
		}
		var input application
		if !decode(w, r, &input) {
			return
		}
		input.RequestID = doc.RequestID
		input.TemplateID = doc.TemplateID
		input.Site = doc.Site
		if input.Account == "" && input.Channel == doc.Channel {
			input.Account = doc.Account
		}
		if !validateApplication(input, doc.Template) {
			bad(w, "请按原模板版本补全申请")
			return
		}
		route, ok := app.approvalRoute(r, doc.Template, input.Amount, actor.ID)
		if !ok {
			bad(w, "审批路线尚未配置有效负责人")
			return
		}
		if !linkFiles(r, tx, input.Files, actor.ID, doc.ID) {
			bad(w, "附件无效或已用于其他单据")
			return
		}
		doc.application = input
		doc.Route = route
		doc.Current = 0
		doc.State = "waiting"
	case "decision":
		if doc.State != "waiting" {
			conflict(w)
			return
		}
		if doc.Applicant == actor.ID || doc.Route[doc.Current] != actor.ID {
			deny(w)
			return
		}
		var input struct {
			Decision string `json:"decision"`
			Comment  string `json:"comment"`
			Current  *int   `json:"current"`
			Version  *int   `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		if input.Current == nil || *input.Current != doc.Current || (input.Version != nil && *input.Version != doc.Version) {
			conflict(w)
			return
		}
		if len(input.Comment) > 2000 || (input.Decision != "approve" && !textOK(input.Comment, 2000)) {
			bad(w, "请填写有效审批意见")
			return
		}
		entry.Action = input.Decision
		entry.Comment = input.Comment
		switch input.Decision {
		case "approve":
			doc.Current++
			if doc.Current == len(doc.Route) {
				doc.State = "approved"
			}
		case "return":
			doc.State = "returned"
		case "reject":
			doc.State = "rejected"
		default:
			bad(w, "审批动作无效")
			return
		}
	case "payment":
		if !permit(actor, "finance.pay") || actor.ID == doc.Applicant {
			deny(w)
			return
		}
		var input payment
		if !decode(w, r, &input) {
			return
		}
		if doc.State == "paid" && doc.Payment != nil && *doc.Payment == input {
			platform.JSON(w, 200, doc.visible(actor, false))
			return
		}
		if doc.State != "approved" {
			conflict(w)
			return
		}
		paidDay, err := time.Parse(time.DateOnly, input.Date)
		if err != nil || paidDay.Format(time.DateOnly) > businessDate().Format(time.DateOnly) || input.Amount != doc.Amount || !linkFiles(r, tx, []string{input.Receipt}, actor.ID, doc.ID) {
			bad(w, "须提供审批金额、有效实付日及本人上传的回单")
			return
		}
		doc.Payment = &input
		doc.State = "paid"
	default:
		missing(w)
		return
	}
	doc.Version++
	doc.History = append(doc.History, entry)
	if persistDocument(r, tx, doc) != nil || recordFinanceResult(ctx, tx, doc) != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, doc.visible(actor, false))
}
func businessDate() time.Time { return time.Now().In(time.FixedZone("CST", 8*3600)) }
func decimal(id int64) string { return strconv.FormatInt(id, 10) }
