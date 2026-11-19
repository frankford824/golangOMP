package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"workflow/internal/oa/platform"
)

func (doc document) payableAmount() int64 {
	if doc.ApprovedAmount != nil {
		return *doc.ApprovedAmount
	}
	return doc.Amount
}

func newFinanceHistory(actor platform.Actor, action string) history {
	now := time.Now().UTC()
	return history{
		Actor: actor.ID, Action: action, At: now,
		OperatorID: actor.ID, OperatorName: actor.Name, OperatorNo: actor.EmployeeNo,
		OperatorDepartment: actor.Department, CreatedAt: &now,
	}
}

// Document state, its append-only event, and any result notification are
// committed in the same transaction while holding the document row lock.
func appendDocumentEvent(ctx context.Context, tx *sql.Tx, doc document, entry history) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO oa_office_document_events
        (document_id, document_version, operator_id, action, occurred_at, payload) VALUES (?, ?, ?, ?, ?, ?)`,
		doc.ID, doc.Version, entry.OperatorID, entry.Action, entry.At, jsonData(entry))
	return err
}

// Historical entries recorded before the events table remain readable from
// the old document payload. Their missing fields must stay unknown.
func (app office) readDocumentEvents(ctx context.Context, doc *document) error {
	rows, err := app.db.QueryContext(ctx, `SELECT payload FROM oa_office_document_events
        WHERE document_id=? AND document_version<=? ORDER BY document_version`, doc.ID, doc.Version)
	if err != nil {
		return err
	}
	defer rows.Close()

	entries := make([]history, 0, len(doc.History))
	for _, entry := range doc.History {
		if entry.CreatedAt == nil {
			entries = append(entries, entry)
		}
	}
	for rows.Next() {
		var raw []byte
		var entry history
		if err := rows.Scan(&raw); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &entry); err != nil {
			return err
		}
		entries = append(entries, entry)
	}
	doc.History = entries
	return rows.Err()
}

// Event- and CC-based readers can review document status and amounts. Current
// document participants retain the existing evidence and account policies.
func (doc document) limitedDetail() document {
	doc.RequestID = ""
	doc.Reason = ""
	doc.Holder = ""
	doc.Account = ""
	doc.Bank = ""
	doc.Extra = map[string]string{}
	doc.Files = []string{}
	doc.Template.Fields = []string{}
	doc.Template.Required = []string{}
	doc.Template.Evidence = ""
	doc.Template.Nodes = []string{}
	doc.Template.Extra = ""
	doc.Template.Threshold = 0
	doc.Route = []int64{}
	doc.RouteNodes = nil
	doc.History = []history{}
	if doc.Payment != nil {
		doc.Payment = &payment{Amount: doc.Payment.Amount, Date: doc.Payment.Date}
	}
	return doc
}
