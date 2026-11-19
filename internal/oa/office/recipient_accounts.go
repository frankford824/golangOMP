package office

import (
	"database/sql"
	"errors"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/go-sql-driver/mysql"
	"workflow/internal/oa/platform"
)

type recipientAccount struct {
	ID       string `json:"id"`
	Holder   string `json:"holder"`
	Bank     string `json:"bank"`
	LastFour string `json:"last_four"`
	Version  int64  `json:"version"`
}
type recipientInput struct {
	Scope   string `json:"scope"`
	Holder  string `json:"holder"`
	Bank    string `json:"bank"`
	Account string `json:"account"`
	Version int64  `json:"version"`
}

var accountFormat = regexp.MustCompile("^[0-9]{6,34}$")

func accountDirectory(actor platform.Actor, scope string, owner int64, write bool) bool {
	if scope == "personal" {
		return actor.ID == owner || !write && permit(actor, "finance.pay")
	}
	if scope != "supplier" || owner != 0 {
		return false
	}
	if write {
		return permit(actor, "finance.admin") || permit(actor, "finance.pay")
	}
	return permit(actor, "finance.apply") || permit(actor, "finance.pay") || permit(actor, "finance.admin")
}
func mountAccounts(mux *http.ServeMux, app office) {
	const path = "/oa-api/finance/accounts"
	mux.HandleFunc("GET "+path, app.listAccounts)
	mux.HandleFunc("POST "+path, app.writeAccount)
	mux.HandleFunc("PUT "+path+"/{id}", app.writeAccount)
	mux.HandleFunc("POST "+path+"/{id}/retire", app.writeAccount)
	mux.HandleFunc("POST "+path+"/{id}/use", app.useAccount)
}
func (app office) listAccounts(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	q := r.URL.Query()
	scope := q.Get("scope")
	if scope != "personal" && scope != "supplier" {
		bad(w, "请选择本人账户或供应商账户")
		return
	}
	var owner int64
	if scope == "personal" {
		owner = actor.ID
	}
	if q.Has("owner_id") {
		n, err := strconv.ParseInt(q.Get("owner_id"), 10, 64)
		if err != nil || n <= 0 || scope != "personal" {
			bad(w, "人员编号无效")
			return
		}
		owner = n
	}
	if !accountDirectory(actor, scope, owner, false) {
		deny(w)
		return
	}
	term := strings.TrimSpace(q.Get("q"))
	page := 1
	if q.Has("page") {
		var err error
		page, err = strconv.Atoi(q.Get("page"))
		if err != nil || page < 1 || page > 100000 {
			bad(w, "页码无效")
			return
		}
	}
	if utf8.RuneCountInString(term) > 100 {
		bad(w, "搜索词过长")
		return
	}
	const filter = " FROM oa_office_recipient_accounts WHERE scope=? AND owner_id=? AND active=1 AND (LOCATE(?,holder)>0 OR LOCATE(?,bank)>0)"
	rows, err := app.db.QueryContext(r.Context(), "SELECT id,holder,bank,last_four,version"+filter+" ORDER BY holder,id LIMIT 21 OFFSET ?", scope, owner, term, term, (page-1)*20)
	if err != nil {
		internal(w)
		return
	}
	defer rows.Close()
	items := []recipientAccount{}
	for rows.Next() {
		var a recipientAccount
		if err = rows.Scan(&a.ID, &a.Holder, &a.Bank, &a.LastFour, &a.Version); err != nil {
			internal(w)
			return
		}
		items = append(items, a)
	}
	if rows.Err() != nil {
		internal(w)
		return
	}
	more := len(items) > 20
	if more {
		items = items[:20]
	}
	w.Header().Set("Cache-Control", "private, no-store")
	platform.JSON(w, 200, map[string]any{"items": items, "page": page, "has_more": more})
}

func (app office) writeAccount(w http.ResponseWriter, r *http.Request) {
	var in recipientInput
	if !decode(w, r, &in) {
		return
	}
	actor := platform.ActorFrom(r)
	id := r.PathValue("id")
	create := id == ""
	retire := strings.HasSuffix(r.URL.Path, "/retire")
	if !retire {
		in.Holder = strings.TrimSpace(in.Holder)
		in.Bank = strings.TrimSpace(in.Bank)
		in.Account = strings.Map(func(ch rune) rune {
			if unicode.IsSpace(ch) {
				return -1
			}
			return ch
		}, in.Account)
		if in.Holder == "" || in.Bank == "" || utf8.RuneCountInString(in.Holder) > 160 || utf8.RuneCountInString(in.Bank) > 160 || !accountFormat.MatchString(in.Account) {
			bad(w, "请填写收款户名、开户行及 6 至 34 位数字账号")
			return
		}
	}
	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	scope := in.Scope
	var owner, version int64
	if create {
		id = identifier()
		if scope == "personal" {
			owner = actor.ID
		}
	} else {
		err = tx.QueryRowContext(r.Context(), "SELECT scope,owner_id,version FROM oa_office_recipient_accounts WHERE id=? AND active=1 FOR UPDATE", id).Scan(&scope, &owner, &version)
		if errors.Is(err, sql.ErrNoRows) {
			missing(w)
			return
		}
		if err != nil {
			internal(w)
			return
		}
	}
	if !accountDirectory(actor, scope, owner, true) {
		deny(w)
		return
	}
	if in.Version != version {
		conflict(w)
		return
	}
	action := "update"
	if retire {
		action = "retire"
		_, err = tx.ExecContext(r.Context(), "UPDATE oa_office_recipient_accounts SET active=0,version=version+1,updated_by=? WHERE id=?", actor.ID, id)
	} else {
		var encrypted string
		encrypted, err = sealBoundOfficeValue(in.Account, "recipient:"+id)
		if err != nil {
			internal(w)
			return
		}
		last := in.Account[len(in.Account)-4:]
		if create {
			action = "create"
			_, err = tx.ExecContext(r.Context(), "INSERT INTO oa_office_recipient_accounts(id,scope,owner_id,holder,bank,last_four,account_cipher,updated_by) VALUES(?,?,?,?,?,?,?,?)", id, scope, owner, in.Holder, in.Bank, last, encrypted, actor.ID)
		} else {
			_, err = tx.ExecContext(r.Context(), "UPDATE oa_office_recipient_accounts SET holder=?,bank=?,last_four=?,account_cipher=?,version=version+1,updated_by=? WHERE id=?", in.Holder, in.Bank, last, encrypted, actor.ID, id)
		}
	}
	var m *mysql.MySQLError
	if errors.As(err, &m) && m.Number == 1062 {
		platform.Error(w, 409, "本人已设默认账户，请刷新后修改")
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO oa_office_recipient_access(recipient_id,actor_id,action) VALUES(?,?,?)", id, actor.ID, action); err != nil {
		internal(w)
		return
	}
	if tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"id": id, "version": version + 1})
}

func (app office) useAccount(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	id := r.PathValue("id")
	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	var scope, holder, bank, ciphertext string
	var owner int64
	err = tx.QueryRowContext(r.Context(), "SELECT scope,owner_id,holder,bank,account_cipher FROM oa_office_recipient_accounts WHERE id=? AND active=1 FOR SHARE", id).Scan(&scope, &owner, &holder, &bank, &ciphertext)
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !accountDirectory(actor, scope, owner, false) {
		deny(w)
		return
	}
	plain, err := openBoundOfficeValue(ciphertext, "recipient:"+id)
	if err != nil {
		internal(w)
		return
	}
	if _, err = tx.ExecContext(r.Context(), "INSERT INTO oa_office_recipient_access(recipient_id,actor_id,action) VALUES(?,?,'use')", id, actor.ID); err != nil {
		internal(w)
		return
	}
	if tx.Commit() != nil {
		internal(w)
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	platform.JSON(w, 200, map[string]string{"holder": holder, "bank": bank, "account": plain})
}
