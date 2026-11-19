package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"slices"
	"strings"
	"time"

	"workflow/internal/oa/platform"
)

type approvalNode struct {
	Name      string   `json:"name"`
	Mode      string   `json:"mode"`
	Approvers []string `json:"approvers"`
}
type amountRule struct {
	Min       int64    `json:"min_cents"`
	Max       *int64   `json:"max_cents,omitempty"`
	Mode      string   `json:"mode,omitempty"`
	Approvers []string `json:"approvers"`
}
type approvalStep struct {
	Name      string  `json:"name"`
	Mode      string  `json:"mode"`
	Approvers []int64 `json:"approver_ids"`
}
type fieldDefinition struct {
	Name     string   `json:"name"`
	Type     string   `json:"type"`
	Required bool     `json:"required"`
	Options  []string `json:"options,omitempty"`
}
type sourceReference struct {
	Kind   string `json:"kind"`
	Number string `json:"number"`
	URL    string `json:"url,omitempty"`
}

const maxFinanceAmount = 100000000000

// The legacy column remains the first pending user for older readers.
const financePendingScope = "(d.approver=? OR JSON_CONTAINS(d.payload,CAST(? AS JSON),'$.pending_approver_ids'))"

func (doc document) pendingFor(id int64) bool {
	if doc.State != "waiting" || doc.Current < 0 || doc.Current >= len(doc.Route) {
		return false
	}
	if len(doc.ApprovalSteps) > 0 {
		return slices.Contains(doc.PendingApprovers, id)
	}
	return doc.Route[doc.Current] == id
}

func (doc *document) setCurrentApprovalStep() {
	doc.StepVersion = doc.Version + 1
	doc.PendingApprovers = nil
	if doc.Current < len(doc.ApprovalSteps) {
		doc.PendingApprovers = slices.Clone(doc.ApprovalSteps[doc.Current].Approvers)
	}
}

func (doc document) hasUnresolvedApprover(id int64) bool {
	if doc.pendingFor(id) {
		return true
	}
	if doc.State != "waiting" || doc.Current < 0 || doc.Current >= len(doc.Route) {
		return false
	}
	if len(doc.ApprovalSteps) == 0 {
		return slices.Contains(doc.Route[doc.Current+1:], id)
	}
	for _, step := range doc.ApprovalSteps[doc.Current+1:] {
		if slices.Contains(step.Approvers, id) {
			return true
		}
	}
	return false
}

func (doc *document) transferPendingApprover(old, replacement int64) bool {
	if !doc.hasUnresolvedApprover(old) || old == replacement || doc.Applicant == replacement {
		return false
	}
	if len(doc.ApprovalSteps) == 0 {
		for i := doc.Current; i < len(doc.Route); i++ {
			if doc.Route[i] == old {
				doc.Route[i] = replacement
			}
		}
		return true
	}
	pending := doc.pendingFor(old)
	for i := doc.Current; i < len(doc.ApprovalSteps); i++ {
		step := doc.ApprovalSteps[i]
		if (i != doc.Current || pending) && slices.Contains(step.Approvers, old) && slices.Contains(step.Approvers, replacement) {
			return false // A transfer must preserve the number of votes at each step.
		}
	}
	for i := doc.Current; i < len(doc.ApprovalSteps); i++ {
		step := &doc.ApprovalSteps[i]
		if pos := slices.Index(step.Approvers, old); pos >= 0 && (i != doc.Current || pending) {
			step.Approvers[pos] = replacement
			doc.Route[i] = step.Approvers[0]
		}
	}
	if pending {
		doc.PendingApprovers[slices.Index(doc.PendingApprovers, old)] = replacement
	}
	doc.StepVersion = doc.Version + 1
	return true
}

func (doc document) participates(id int64) bool {
	if slices.Contains(doc.Route, id) {
		return true
	}
	for _, step := range doc.ApprovalSteps {
		if slices.Contains(step.Approvers, id) {
			return true
		}
	}
	return false
}

type approvalQuery interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func activeFinancePerson(r *http.Request, query approvalQuery, id int64) bool {
	if id <= 0 {
		return false
	}
	var status string
	return query.QueryRowContext(r.Context(), "SELECT status FROM users WHERE id=?", id).Scan(&status) == nil && status == "active"
}

// Use the caller's transaction when resolving a submission so that the
// personnel guard and the identity lookup use one connection and snapshot.
func effectiveApprovalManager(r *http.Request, query approvalQuery, id int64) (int64, error) {
	var manager int64
	err := query.QueryRowContext(r.Context(), directoryPeople+"SELECT manager_id FROM directory WHERE id=?", append(directoryDates(), id)...).Scan(&manager)
	return manager, err
}

func (app office) validateApprovalTemplate(r *http.Request, query approvalQuery, t *template) string {
	validNode := func(mode string, references []string) bool {
		if (mode != "AND" && mode != "OR") || len(references) == 0 || len(references) > 12 {
			return false
		}
		seen := map[string]bool{}
		for _, reference := range references {
			fixed := strings.TrimPrefix(reference, "fixed:")
			id := number(fixed)
			if seen[reference] || (reference != "manager" && (!strings.HasPrefix(reference, "fixed:") || decimal(id) != fixed || !activeFinancePerson(r, query, id))) {
				return false
			}
			seen[reference] = true
		}
		return true
	}
	if len(t.ApprovalNodes) > 12 || len(t.AmountRules) > 12 || (len(t.Nodes) == 0 && len(t.ApprovalNodes) == 0) {
		return "请设置 1 至 12 个审批节点，金额规则最多 12 条"
	}
	for _, node := range t.ApprovalNodes {
		if (node.Name != "" && !textOK(node.Name, 120)) || !validNode(node.Mode, node.Approvers) {
			return "请核对审批节点名称、会签方式和在职审批人"
		}
	}
	for i := range t.AmountRules {
		rule := &t.AmountRules[i]
		if rule.Mode == "" {
			rule.Mode = "AND"
		}
		if rule.Min < 0 || rule.Min > maxFinanceAmount || (rule.Max != nil && (*rule.Max < rule.Min || *rule.Max > maxFinanceAmount)) || !validNode(rule.Mode, rule.Approvers) {
			return "请核对金额规则的区间、会签方式和在职审批人"
		}
		if i > 0 {
			previous := t.AmountRules[i-1]
			if previous.Max == nil || *previous.Max >= rule.Min {
				return "金额规则须按金额从低到高排列，区间不得重叠"
			}
		}
	}
	for _, recipients := range [][]int64{t.CCAfterApproval, t.CCAfterPayment} {
		if len(recipients) > 16 {
			return "每种抄送最多 16 人"
		}
		seen := map[int64]bool{}
		for _, id := range recipients {
			if seen[id] || !activeFinancePerson(r, query, id) {
				return "请为每种抄送选择不同的在职人员"
			}
			seen[id] = true
		}
	}
	return ""
}

func validateTemplateFields(t *template) bool {
	if len(t.FieldDefinitions) == 0 {
		return true
	}
	if len(t.FieldDefinitions) > 30 {
		return false
	}
	fields, required := []string{}, []string{}
	for _, field := range t.FieldDefinitions {
		if !textOK(field.Name, 120) || strings.TrimSpace(field.Name) != field.Name || slices.Contains(fields, field.Name) {
			return false
		}
		switch field.Type {
		case "text", "number", "date":
			if len(field.Options) > 0 {
				return false
			}
		case "select":
			if len(field.Options) == 0 || len(field.Options) > 60 {
				return false
			}
			seen := map[string]bool{}
			for _, option := range field.Options {
				if !textOK(option, 120) || strings.TrimSpace(option) != option || seen[option] {
					return false
				}
				seen[option] = true
			}
		default:
			return false
		}
		fields = append(fields, field.Name)
		if field.Required {
			required = append(required, field.Name)
		}
	}
	t.Fields, t.Required = fields, required
	return true
}

var financeDecimal = regexp.MustCompile("^-?[0-9]+(\\.[0-9]+)?$")

func validateFieldValues(t template, values map[string]string) bool {
	for _, field := range t.FieldDefinitions {
		value := values[field.Name]
		if strings.TrimSpace(value) == "" && !field.Required {
			continue
		}
		switch field.Type {
		case "number":
			if !financeDecimal.MatchString(value) {
				return false
			}
		case "date":
			d, err := time.Parse(time.DateOnly, value)
			if err != nil || d.Year() < 1000 || d.Format(time.DateOnly) != value {
				return false
			}
		case "select":
			if !slices.Contains(field.Options, value) {
				return false
			}
		}
	}
	return true
}

func safeFinanceSource(ref *sourceReference) bool {
	if ref == nil {
		return true
	}
	if !slices.Contains([]string{"travel", "loan", "erp_purchase", "contract", "other"}, ref.Kind) || !textOK(ref.Number, 160) {
		return false
	}
	if ref.URL == "" {
		return true
	}
	// This value is a navigation link. The server never fetches user URLs.
	if len(ref.URL) > 1024 || strings.ContainsAny(ref.URL, "\\\r\n\t ") {
		return false
	}
	u, err := url.Parse(ref.URL)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" && u.User == nil
}

func matchAmountRule(rules []amountRule, amount int64) *amountRule {
	for i := range rules {
		rule := &rules[i]
		if amount >= rule.Min && (rule.Max == nil || amount <= *rule.Max) {
			return rule
		}
	}
	return nil
}

func (app office) resolveApprovalSteps(r *http.Request, query approvalQuery, t template, amount, applicant int64) ([]approvalStep, error) {
	if len(t.ApprovalNodes) == 0 && len(t.AmountRules) == 0 {
		route, names, ok := app.approvalRoute(r, query, t, amount, applicant)
		if !ok {
			return nil, errors.New("审批路线缺少有效直属负责人，或指定审批人已停用")
		}
		steps := make([]approvalStep, len(route))
		for i, id := range route {
			steps[i] = approvalStep{names[i], "AND", []int64{id}}
		}
		return steps, nil
	}
	nodes := slices.Clone(t.ApprovalNodes)
	if len(nodes) == 0 {
		for _, reference := range t.Nodes {
			nodes = append(nodes, approvalNode{Mode: "AND", Approvers: []string{reference}})
		}
	}
	if rule := matchAmountRule(t.AmountRules, amount); rule != nil {
		mode := rule.Mode
		if mode == "" {
			mode = "AND"
		}
		nodes = append(nodes, approvalNode{Name: "金额加签", Mode: mode, Approvers: rule.Approvers})
	}
	steps := make([]approvalStep, 0, len(nodes))
	var managerID int64
	for i, node := range nodes {
		name := node.Name
		if name == "" {
			name = "指定审批"
			if len(node.Approvers) == 1 && node.Approvers[0] == "manager" {
				name = "直属负责人审批"
			}
		}
		step := approvalStep{Name: name, Mode: node.Mode, Approvers: []int64{}}
		for _, reference := range node.Approvers {
			id := number(strings.TrimPrefix(reference, "fixed:"))
			if reference == "manager" {
				if managerID == 0 {
					managerID, _ = effectiveApprovalManager(r, query, applicant)
				}
				id = managerID
				if id == 0 || id == applicant || !activeFinancePerson(r, query, id) {
					return nil, fmt.Errorf("审批第 %d 步缺少有效直属负责人，请联系人员管理员", i+1)
				}
			} else if id == applicant || !activeFinancePerson(r, query, id) {
				return nil, fmt.Errorf("审批第 %d 步的指定审批人已停用或为申请人本人", i+1)
			}
			if !slices.Contains(step.Approvers, id) {
				step.Approvers = append(step.Approvers, id)
			}
		}
		if len(step.Approvers) == 0 {
			return nil, errors.New("审批路线缺少有效审批人")
		}
		steps = append(steps, step)
	}
	if len(steps) == 0 {
		return nil, errors.New("审批路线缺少有效审批人")
	}
	return steps, nil
}

func (doc *document) assignApprovalRoute(steps []approvalStep) {
	doc.Route, doc.RouteNodes = []int64{}, []string{}
	for _, step := range steps {
		doc.Route = append(doc.Route, step.Approvers[0])
		doc.RouteNodes = append(doc.RouteNodes, step.Name)
	}
	doc.Current = 0
	doc.StepVersion = 0
	doc.PendingApprovers = nil
	if len(doc.Template.ApprovalNodes) > 0 || len(doc.Template.AmountRules) > 0 {
		doc.ApprovalSteps = steps
		doc.setCurrentApprovalStep()
	}
}

func (app office) previewFinanceRoute(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !permit(actor, "finance.apply") {
		deny(w)
		return
	}
	var input struct {
		TemplateID string `json:"template_id"`
		DocumentID string `json:"document_id"`
		Amount     int64  `json:"amount_cents"`
	}
	if !decode(w, r, &input) {
		return
	}
	if input.Amount <= 0 || input.Amount > maxFinanceAmount || !textOK(input.TemplateID, 64) {
		bad(w, "请选择模板并填写有效金额")
		return
	}
	var t template
	if input.DocumentID != "" {
		doc, err := scanDocument(app.db.QueryRowContext(r.Context(), "SELECT payload FROM oa_office_documents WHERE id=? AND applicant=? AND state IN ('returned','withdrawn')", input.DocumentID, actor.ID))
		if errors.Is(err, sql.ErrNoRows) || (err == nil && doc.Template.ID != input.TemplateID) {
			missing(w)
			return
		}
		if err != nil {
			internal(w)
			return
		}
		t = doc.Template
	} else {
		var raw []byte
		err := app.db.QueryRowContext(r.Context(), "SELECT published FROM oa_office_templates WHERE id=? AND published IS NOT NULL", input.TemplateID).Scan(&raw)
		if errors.Is(err, sql.ErrNoRows) {
			bad(w, "模板尚未发布")
			return
		}
		if err != nil || json.Unmarshal(raw, &t) != nil {
			internal(w)
			return
		}
	}
	steps, err := app.resolveApprovalSteps(r, app.db, t, input.Amount, actor.ID)
	if err != nil {
		bad(w, err.Error())
		return
	}
	type person struct {
		ID     string `json:"id"`
		Name   string `json:"name"`
		Number *int   `json:"employee_no,omitempty"`
		Avatar string `json:"avatar_url,omitempty"`
	}
	type stepPreview struct {
		AmountMin *int64   `json:"amount_min_cents,omitempty"`
		AmountMax *int64   `json:"amount_max_cents,omitempty"`
		Name      string   `json:"name"`
		Mode      string   `json:"mode"`
		Approvers []person `json:"approvers"`
	}
	result := make([]stepPreview, 0, len(steps))
	people := map[int64]person{}
	matched := matchAmountRule(t.AmountRules, input.Amount)
	for i, step := range steps {
		preview := stepPreview{Name: step.Name, Mode: step.Mode, Approvers: []person{}}
		if i == len(steps)-1 {
			if matched != nil {
				preview.AmountMin, preview.AmountMax = &matched.Min, matched.Max
			} else if len(t.ApprovalNodes) == 0 && len(t.AmountRules) == 0 && t.Threshold > 0 && step.Name == "金额加签" {
				preview.AmountMin = &t.Threshold
			}
		}
		for _, id := range step.Approvers {
			info, known := people[id]
			if !known {
				info.ID = decimal(id)
				if err := app.source.QueryRowContext(r.Context(), "SELECT display_name,employee_no,COALESCE(avatar_url,'') FROM users WHERE id=? AND status='active'", id).Scan(&info.Name, &info.Number, &info.Avatar); err != nil {
					bad(w, "审批人员状态已变化，请重新预览")
					return
				}
				if u, err := url.Parse(info.Avatar); err != nil || (!strings.HasPrefix(info.Avatar, "/") && u.Scheme != "https") || strings.HasPrefix(info.Avatar, "//") || strings.Contains(info.Avatar, "\\") {
					info.Avatar = ""
				}
				people[id] = info
			}
			preview.Approvers = append(preview.Approvers, info)
		}
		result = append(result, preview)
	}
	w.Header().Set("Cache-Control", "private, no-store")
	platform.JSON(w, 200, map[string]any{"steps": result})
}
