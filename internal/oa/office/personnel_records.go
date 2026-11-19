package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
	"unicode/utf8"

	"workflow/internal/oa/platform"
)

type emergencyContact struct {
	Name         string `json:"name"`
	Relationship string `json:"relationship"`
	Phone        string `json:"phone"`
}

func (e *emergencyContact) normalize() bool {
	e.Name = strings.TrimSpace(e.Name)
	e.Relationship = strings.TrimSpace(e.Relationship)
	e.Phone = strings.TrimSpace(e.Phone)
	if utf8.RuneCountInString(e.Name) > 100 || utf8.RuneCountInString(e.Relationship) > 60 || utf8.RuneCountInString(e.Phone) > 40 {
		return false
	}
	return (e.Name == "" && e.Relationship == "" && e.Phone == "") || (e.Name != "" && e.Phone != "")
}

type personEventWriter interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func recordPersonEvent(ctx context.Context, tx personEventWriter, id int64, e change) error {
	_, err := tx.ExecContext(ctx, `INSERT INTO oa_office_person_events(user_id,actor_id,effective_date,kind,payload)
		VALUES (?,?,?,?,?)`, id, e.ActorID, e.EffectiveDate, e.Kind, jsonData(e))
	return err
}

func (app office) applyPersonRecords(r *http.Request, p *person) error {
	ctx := r.Context()
	var raw []byte
	err := app.db.QueryRowContext(ctx, "SELECT payload FROM oa_office_person_contacts WHERE user_id=?", p.ID).Scan(&raw)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if len(raw) != 0 {
		if err := json.Unmarshal(raw, &p.EmergencyContact); err != nil {
			return err
		}
	}
	err = app.source.QueryRowContext(ctx, "SELECT COALESCE(mobile,''),COALESCE(email,'') FROM users WHERE id=?", p.ID).Scan(&p.Mobile, &p.Email)
	if err != nil {
		return err
	}
	events := []change{}
	for _, e := range p.Events {
		if !e.Recorded {
			events = append(events, e)
		}
	}
	rows, err := app.db.QueryContext(ctx, `SELECT id,payload,DATE_FORMAT(recorded_at,'%Y-%m-%d %H:%i:%s.%f') FROM oa_office_person_events WHERE user_id=? ORDER BY id`, p.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var e change
		var id int64
		var recordedAt string
		if err := rows.Scan(&id, &raw, &recordedAt); err != nil {
			return err
		}
		if err := json.Unmarshal(raw, &e); err != nil {
			return err
		}
		e.ID, e.RecordedAt = id, recordedAt
		events = append(events, e)
	}
	p.Events = events
	p.HRStatus = personnelStatus(p.Status, p.Left, p.Confirmed, p.Events, businessDate().Format(time.DateOnly))
	return rows.Err()
}

func personnelStatus(status, left, confirmed string, events []change, today string) string {
	if status != "active" && left != "" && left <= today {
		return "left"
	}
	if status != "active" {
		return "disabled"
	}
	var planned, hired string
	for _, event := range events {
		if event.Kind == "leave_plan" && event.EffectiveDate > planned {
			planned = event.EffectiveDate
		}
		if (event.Kind == "hire" || event.Kind == "rehire") && event.EffectiveDate > hired {
			hired = event.EffectiveDate
		}
	}
	if planned > left && planned >= hired {
		return "leaving"
	}
	if confirmed != "" && confirmed <= today {
		return "regular"
	}
	if hired != "" {
		return "probation"
	}
	return "active"
}

func (app office) recordPersonnelTransfer(w http.ResponseWriter, r *http.Request) {
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
	var input struct {
		Department string `json:"department"`
		Team       string `json:"team"`
	}
	if !decode(w, r, &input) {
		return
	}
	input.Department = strings.TrimSpace(input.Department)
	input.Team = strings.TrimSpace(input.Team)
	if !textOK(input.Team, 128) || input.Team != "ungrouped" && !textOK(input.Department, 128) {
		bad(w, "请选择有效的部门和小组")
		return
	}
	tx, err := app.db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelReadCommitted})
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if err := lockManagers(ctx, tx); err != nil {
		internal(w)
		return
	}
	before, err := app.sourcePerson(r, p.ID)
	if err != nil {
		internal(w)
		return
	}
	fields := map[string]string{"team": input.Team}
	if input.Department != "" {
		fields["department"] = input.Department
	}
	operationID := identifier()
	day := businessDate().Format(time.DateOnly)
	intent := change{Kind: "transfer_requested", OperationID: operationID, EffectiveDate: day, ActorID: actor.ID,
		OrgFrom: strings.Trim(before.Department+" / "+before.Team, " /"), OrgTo: strings.Trim(input.Department+" / "+input.Team, " /")}
	var previousID, previousOrigin string
	err = tx.QueryRowContext(ctx, `SELECT JSON_UNQUOTE(JSON_EXTRACT(i.payload,'$.operation_id')),
		COALESCE(JSON_UNQUOTE(JSON_EXTRACT(i.payload,'$.org_from')),'')
		FROM oa_office_person_events i WHERE i.user_id=? AND i.kind='transfer_requested'
		AND COALESCE(JSON_UNQUOTE(JSON_EXTRACT(i.payload,'$.org_to')),'')=?
		AND NOT EXISTS(SELECT 1 FROM oa_office_person_events e WHERE e.user_id=i.user_id
			AND e.kind IN ('transfer','transfer_unchanged','transfer_observed')
			AND JSON_EXTRACT(e.payload,'$.operation_id')=JSON_EXTRACT(i.payload,'$.operation_id'))
		ORDER BY i.id DESC LIMIT 1`, p.ID, intent.OrgTo).Scan(&previousID, &previousOrigin)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		internal(w)
		return
	}
	if err := recordPersonEvent(ctx, app.db, p.ID, intent); err != nil {
		internal(w)
		return
	}
	if _, ok := app.nativePersonnelRequest(w, r, http.MethodPatch, fmt.Sprintf("/v1/users/%d", p.ID), jsonData(fields)); !ok {
		return
	}
	after, err := app.sourcePerson(r, p.ID)
	if err != nil {
		internal(w)
		return
	}
	result := change{Kind: "transfer_unchanged", OperationID: operationID, EffectiveDate: day, ActorID: actor.ID,
		DepartmentID: after.DepartmentID, TeamID: after.TeamID,
		OrgFrom: intent.OrgFrom, OrgTo: strings.Trim(after.Department+" / "+after.Team, " /")}
	if before.DepartmentID != after.DepartmentID || before.TeamID != after.TeamID {
		result.Kind = "transfer"
	} else if previousID != "" && previousOrigin != result.OrgTo {
		observed := result
		observed.Kind, observed.OperationID, observed.OrgFrom = "transfer_observed", previousID, ""
		if err := recordPersonEvent(ctx, tx, p.ID, observed); err != nil {
			internal(w)
			return
		}
	}
	if err := recordPersonEvent(ctx, tx, p.ID, result); err != nil {
		internal(w)
		return
	}
	if err := tx.Commit(); err != nil {
		internal(w)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
