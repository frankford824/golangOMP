package office

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"workflow/internal/oa/platform"
)

type office struct{ db, source *sql.DB }

func EnsureSchema(ctx context.Context, db *sql.DB) error {
	for _, statement := range []string{
		"CREATE TABLE IF NOT EXISTS oa_office_templates (id VARCHAR(64) PRIMARY KEY, version INT NOT NULL, draft JSON, published JSON)",
		"CREATE TABLE IF NOT EXISTS oa_office_template_versions (id VARCHAR(64) NOT NULL, version INT NOT NULL, publisher BIGINT NOT NULL, published_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP, payload JSON NOT NULL, PRIMARY KEY(id,version))",
		"CREATE TABLE IF NOT EXISTS oa_office_delegates (user_id BIGINT PRIMARY KEY, level VARCHAR(16) NOT NULL)",
		"CREATE TABLE IF NOT EXISTS oa_office_finance_actions (user_id BIGINT PRIMARY KEY, can_pay BOOLEAN NOT NULL, can_report BOOLEAN NOT NULL)",
		"CREATE TABLE IF NOT EXISTS oa_office_documents (id CHAR(32) PRIMARY KEY, applicant BIGINT NOT NULL, request_id VARCHAR(80) NOT NULL, state VARCHAR(16) NOT NULL, approver BIGINT NULL, paid_date DATE NULL, payload JSON NOT NULL, UNIQUE KEY (applicant,request_id), KEY (state,approver), KEY (paid_date))",
		"CREATE TABLE IF NOT EXISTS oa_office_files (id CHAR(32) PRIMARY KEY, owner_id BIGINT NOT NULL, name VARCHAR(255) NOT NULL, mime VARCHAR(64) NOT NULL, document_id CHAR(32) NULL)",
		"CREATE TABLE IF NOT EXISTS oa_office_people (id BIGINT PRIMARY KEY, version INT NOT NULL, payload JSON NOT NULL)",
		"CREATE TABLE IF NOT EXISTS oa_office_identity_changes (id BIGINT AUTO_INCREMENT PRIMARY KEY, user_id BIGINT NOT NULL, effective_on DATE NOT NULL, status VARCHAR(16) NOT NULL, applied BOOLEAN NOT NULL DEFAULT FALSE, KEY (applied,effective_on,id))",
		"CREATE TABLE IF NOT EXISTS oa_office_role_changes (id BIGINT AUTO_INCREMENT PRIMARY KEY, user_id BIGINT NOT NULL, effective_on DATE NOT NULL, payload JSON NOT NULL, applied BOOLEAN NOT NULL DEFAULT FALSE, KEY (applied,effective_on,id))",
		"CREATE TABLE IF NOT EXISTS oa_office_imports (token CHAR(32) PRIMARY KEY, actor BIGINT NOT NULL, payload JSON NOT NULL, confirmed_count INT NULL, expires_at TIMESTAMP NOT NULL)",
	} {
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func Mount(mux *http.ServeMux, db, source *sql.DB) {
	app := office{db, source}
	app.mountFinance(mux)
	app.mountPeople(mux)
}

func permit(actor platform.Actor, action string) bool {
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
