package office

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
	"unicode"

	mysql "github.com/go-sql-driver/mysql"
	"golang.org/x/text/unicode/norm"
	"workflow/internal/oa/platform"
)

const financeDraftPrefix = "draft-"

type expenseLine struct {
	Category    string `json:"category"`
	OccurredOn  string `json:"occurred_on"`
	Amount      int64  `json:"amount_cents"`
	Tax         int64  `json:"tax_cents"`
	Invoice     string `json:"invoice_no"`
	Description string `json:"description"`
}

func (input *application) normalizeExpenseLines() string {
	if input.Items == nil {
		return ""
	}
	if len(input.Items) == 0 || len(input.Items) > 100 {
		return "请填写 1 至 100 条费用明细"
	}
	var total int64
	seen := make(map[string]bool)
	for i := range input.Items {
		line := &input.Items[i]
		line.Category = strings.TrimSpace(line.Category)
		line.Description = strings.TrimSpace(line.Description)
		line.Invoice = strings.ToUpper(strings.Map(func(c rune) rune {
			if unicode.IsSpace(c) {
				return -1
			}
			return c
		}, norm.NFKC.String(line.Invoice)))
		if _, err := time.Parse(time.DateOnly, line.OccurredOn); err != nil || line.OccurredOn > businessDate().Format(time.DateOnly) || line.OccurredOn < "1000-01-01" {
			return "费用发生日期无效或晚于今天"
		}
		if !textOK(line.Category, 80) || len(line.Description) > 1000 || len(line.Invoice) > 64 {
			return "请核对费用明细的科目、说明和发票号码"
		}
		if line.Amount <= 0 || line.Amount > maxFinanceAmount-total || line.Tax < 0 || line.Tax > line.Amount {
			return "请核对明细金额与税额，税额不得超过含税金额"
		}
		total += line.Amount
		if line.Invoice != "" {
			if seen[line.Invoice] {
				return "同张单据不能重复填写同一个发票号码"
			}
			seen[line.Invoice] = true
		}
	}
	input.Amount = total
	return ""
}

func reimbursement(t template) bool { return t.Type == "reimbursement" || t.Type == "报销" }

var errInvoiceClaimed = errors.New("invoice already claimed")

func saveExpenseLines(r *http.Request, tx *sql.Tx, doc document) error {
	if doc.Items == nil {
		return nil
	}
	ctx := r.Context()
	if _, err := tx.ExecContext(ctx, "DELETE FROM oa_office_document_items WHERE document_id=?", doc.ID); err != nil {
		return err
	}
	invoices := make([]string, 0, len(doc.Items))
	for index, line := range doc.Items {
		if _, err := tx.ExecContext(ctx, "INSERT INTO oa_office_document_items(document_id,line_no,category,occurred_on,amount_cents,tax_cents,invoice_no,description) VALUES(?,?,?,?,?,?,?,?)", doc.ID, index+1, line.Category, line.OccurredOn, line.Amount, line.Tax, line.Invoice, line.Description); err != nil {
			return err
		}
		if line.Invoice != "" {
			invoices = append(invoices, line.Invoice)
		}
	}
	sort.Strings(invoices)
	for _, invoice := range invoices {
		_, err := tx.ExecContext(ctx, "INSERT INTO oa_office_invoice_locks(document_id,invoice_no,state) VALUES(?,?,'LOCKED')", doc.ID, invoice)
		if err == nil {
			continue
		}
		var dbErr *mysql.MySQLError
		if !errors.As(err, &dbErr) || dbErr.Number != 1062 {
			return err
		}
		result, err := tx.ExecContext(ctx, "UPDATE oa_office_invoice_locks SET state='LOCKED' WHERE document_id=? AND invoice_no=? AND state='RELEASED'", doc.ID, invoice)
		if err != nil {
			if errors.As(err, &dbErr) && dbErr.Number == 1062 {
				return errInvoiceClaimed
			}
			return err
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			return errInvoiceClaimed
		}
	}
	return nil
}

func writeExpenseError(w http.ResponseWriter, err error) {
	if errors.Is(err, errInvoiceClaimed) {
		platform.ErrorWithCode(w, http.StatusConflict, "INVOICE_ALREADY_CLAIMED", "发票号码已被其他单据占用，请核对明细")
		return
	}
	internal(w)
}

func transitionExpenseInvoices(r *http.Request, tx *sql.Tx, doc document) error {
	if len(doc.Items) == 0 {
		return nil
	}
	switch doc.State {
	case "approved", "paid":
		_, err := tx.ExecContext(r.Context(), "UPDATE oa_office_invoice_locks SET state='USED' WHERE document_id=? AND state='LOCKED'", doc.ID)
		return err
	case "returned", "rejected", "withdrawn", "voided":
		_, err := tx.ExecContext(r.Context(), "UPDATE oa_office_invoice_locks SET state='RELEASED' WHERE document_id=? AND state<>'RELEASED'", doc.ID)
		return err
	}
	return nil
}

type draftEvidence struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type financeDraft struct {
	application
	AmountInput string          `json:"amount_input"`
	ItemAmounts []string        `json:"item_amounts"`
	ItemTaxes   []string        `json:"item_taxes"`
	Evidence    []draftEvidence `json:"evidence"`
}

func (app office) financeDraft(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if actor.ID <= 0 || !permit(actor, "finance.apply") {
		deny(w)
		return
	}
	site := r.URL.Query().Get("site")
	if site != "main" && site != "asset" {
		bad(w, "申请场景无效")
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	ctx := r.Context()
	cipherContext := "draft/" + decimal(actor.ID) + "/" + site
	switch r.Method {
	case http.MethodGet:
		var sealed, version string
		err := app.db.QueryRowContext(ctx, "SELECT payload,request_id FROM oa_office_finance_drafts WHERE owner_id=? AND site=?", actor.ID, site).Scan(&sealed, &version)
		if errors.Is(err, sql.ErrNoRows) {
			platform.JSON(w, http.StatusOK, map[string]any{"draft": nil, "version": ""})
		} else if err != nil {
			internal(w)
		} else {
			raw, err := openBoundOfficeValue(sealed, cipherContext)
			if err != nil {
				internal(w)
				return
			}
			platform.JSON(w, http.StatusOK, map[string]any{"draft": json.RawMessage(raw), "version": version})
		}
	case http.MethodPut:
		var input struct {
			financeDraft
			Version string `json:"version"`
		}
		if !decode(w, r, &input) {
			return
		}
		draft := input.financeDraft
		if len(draft.Evidence) > 15 || len(draft.Items) > 100 ||
			len(draft.ItemAmounts) > len(draft.Items) || len(draft.ItemTaxes) > len(draft.Items) {
			bad(w, "请核对草稿编号、凭证和明细")
			return
		}
		draft.Site = site
		draft.Files = nil
		for index := range draft.Evidence {
			file := &draft.Evidence[index]
			err := app.db.QueryRowContext(ctx, "SELECT name FROM oa_office_files WHERE id=? AND owner_id=? AND document_id IS NULL", file.ID, actor.ID).Scan(&file.Name)
			if errors.Is(err, sql.ErrNoRows) {
				bad(w, "草稿附件无效或已用于单据")
				return
			}
			if err != nil {
				internal(w)
				return
			}
		}
		draft.RequestID = financeDraftPrefix + identifier()
		sealed, err := sealBoundOfficeValue(jsonData(draft), cipherContext)
		if err != nil {
			internal(w)
			return
		}
		var result sql.Result
		if input.Version == "" {
			result, err = app.db.ExecContext(ctx, "INSERT INTO oa_office_finance_drafts(owner_id,site,request_id,payload) VALUES(?,?,?,?)", actor.ID, site, draft.RequestID, sealed)
			var dbErr *mysql.MySQLError
			if errors.As(err, &dbErr) && dbErr.Number == 1062 {
				conflict(w)
				return
			}
		} else {
			result, err = app.db.ExecContext(ctx, "UPDATE oa_office_finance_drafts SET request_id=?,payload=? WHERE owner_id=? AND site=? AND request_id=?", draft.RequestID, sealed, actor.ID, site, input.Version)
		}
		if err != nil {
			internal(w)
			return
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			conflict(w)
			return
		}
		platform.JSON(w, http.StatusOK, map[string]any{"draft": draft, "version": draft.RequestID})
	case http.MethodDelete:
		version := r.URL.Query().Get("version")
		if version == "" {
			bad(w, "缺少草稿版本")
			return
		}
		result, err := app.db.ExecContext(ctx, "DELETE FROM oa_office_finance_drafts WHERE owner_id=? AND site=? AND request_id=?", actor.ID, site, version)
		if err != nil {
			internal(w)
			return
		}
		if changed, _ := result.RowsAffected(); changed != 1 {
			conflict(w)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
