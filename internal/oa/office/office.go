package office

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"workflow/internal/oa/platform"
)

type office struct {
	db, source *sql.DB
	native     http.Handler
	baseline   func(context.Context, *int) (map[string]bool, error)
	bindings   ShopBindingDirectory
}

func CheckSchema(ctx context.Context, db *sql.DB) error {
	var count int
	err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM information_schema.tables WHERE table_schema=DATABASE() AND table_name IN ('oa_office_templates','oa_office_template_versions','oa_office_delegates','oa_office_finance_actions','oa_office_documents','oa_office_files','oa_office_people','oa_office_notifications','oa_office_imports','oa_office_document_events','oa_office_person_profiles','oa_office_org_managers','oa_office_finance_drafts','oa_office_document_items','oa_office_invoice_locks','oa_office_person_contacts','oa_office_person_events','oa_office_recipient_accounts','oa_office_recipient_access')`).Scan(&count)
	if err != nil {
		return err
	}
	if count != 19 {
		return fmt.Errorf("OA office schema incomplete: found %d of 19 tables", count)
	}
	var locks int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM oa_office_org_managers WHERE org_kind='lock' AND org_id IN (0,1)").Scan(&locks); err != nil {
		return err
	}
	if locks != 2 {
		return fmt.Errorf("missing OA office coordination rows")
	}
	return nil
}

func Mount(mux *http.ServeMux, db, source *sql.DB, native http.Handler, baseline func(context.Context, *int) (map[string]bool, error), bindings ShopBindingDirectory) {
	app := office{db: db, source: source, native: native, baseline: baseline, bindings: bindings}
	app.mountFinance(mux)
	mountAccounts(mux, app)
	app.mountPeople(mux)
}

func permit(actor platform.Actor, action string) bool {
	if action == "finance.pay" || action == "finance.report" {
		return actor.Actions[action]
	}
	return actor.SuperAdmin || actor.Actions[action]
}
func identifier() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(id[:])
}
func number(value string) int64 { id, _ := strconv.ParseInt(value, 10, 64); return id }
func decode(w http.ResponseWriter, r *http.Request, value any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 512<<10)
	if err := json.NewDecoder(r.Body).Decode(value); err != nil {
		platform.Error(w, 400, "请求格式无效")
		return false
	}
	return true
}
func internal(w http.ResponseWriter)            { platform.Error(w, 500, "暂时无法完成操作") }
func deny(w http.ResponseWriter)                { platform.Error(w, 403, "当前账号无权办理") }
func bad(w http.ResponseWriter, message string) { platform.Error(w, 400, message) }
func missing(w http.ResponseWriter)             { platform.Error(w, 404, "记录不存在") }
func conflict(w http.ResponseWriter)            { platform.Error(w, 409, "记录已变化，请刷新后重试") }
func jsonData(value any) string {
	data, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return string(data)
}
func textOK(value string, max int) bool { return strings.TrimSpace(value) != "" && len(value) <= max }
