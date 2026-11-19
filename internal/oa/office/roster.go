package office

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"workflow/internal/oa/platform"
)

type rosterRow struct {
	Row                int            `json:"row"`
	Sheet              string         `json:"sheet"`
	Name               string         `json:"name"`
	Error              string         `json:"error"`
	Profile            *profileUpdate `json:"profile,omitempty"`
	PersonID           int64          `json:"id"`
	Version            int            `json:"version"`
	EmploymentType     string         `json:"employment_type"`
	EmployeeNo         *int           `json:"employee_no,omitempty"`
	PreviousEmployeeNo *int           `json:"previous_employee_no,omitempty"`
	PreviousName       string         `json:"previous_name,omitempty"`
	NewName            string         `json:"new_name,omitempty"`
	Result             string         `json:"result,omitempty"`
	Failure            string         `json:"failure,omitempty"`
	ShopID             string         `json:"shop_id"`
	From               string         `json:"from"`
}

var peopleHeader = []any{"用户ID", "工号", "姓名", "用工类型", "直属负责人用户ID", "直属负责人工号或姓名", "入职日期", "岗位", "职级", "成本中心", "生效直属负责人（只读）"}
var termsHeader = []any{"用户ID", "店铺ID", "生效日期", "结束日期"}

func (app office) exportPeople(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	people, err := app.selectPeople(r, actor)
	if err != nil {
		internal(w)
		return
	}
	book := excelize.NewFile()
	defer book.Close()
	if book.SetSheetName("Sheet1", "人员") != nil {
		internal(w)
		return
	}
	if _, err := book.NewSheet("店铺任职"); err != nil {
		internal(w)
		return
	}
	if book.SetSheetRow("人员", "A1", &peopleHeader) != nil || book.SetSheetRow("店铺任职", "A1", &termsHeader) != nil {
		internal(w)
		return
	}
	index := 2
	peopleByID := make(map[int64]person, len(people))
	for _, p := range people {
		peopleByID[p.ID] = p
	}
	for row, p := range people {
		no := ""
		if p.EmployeeNo != nil {
			no = strconv.Itoa(*p.EmployeeNo)
		}
		var managerID, managerLabel string
		if p.ManagerOverrideID > 0 {
			managerID = decimal(p.ManagerOverrideID)
			if manager, visible := peopleByID[p.ManagerOverrideID]; visible {
				managerLabel = manager.Name
				if manager.EmployeeNo != nil {
					managerLabel = strconv.Itoa(*manager.EmployeeNo)
				}
			}
		}
		values := []any{decimal(p.ID), no, p.Name, p.EmploymentType, managerID, managerLabel, p.Joined, p.JobTitle, p.JobLevel, p.CostCenter, p.ManagerName}
		if book.SetSheetRow("人员", "A"+strconv.Itoa(row+2), &values) != nil {
			internal(w)
			return
		}
		for _, term := range p.Terms {
			values := []any{decimal(p.ID), term.ShopID, term.From, term.To}
			if book.SetSheetRow("店铺任职", "A"+strconv.Itoa(index), &values) != nil {
				internal(w)
				return
			}
			index++
		}
	}
	data, err := book.WriteToBuffer()
	if err != nil {
		internal(w)
		return
	}
	w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	w.Header().Set("Content-Disposition", "attachment; filename=oa-roster.xlsx")
	w.Header().Set("Cache-Control", "private, no-store")
	_, _ = w.Write(data.Bytes())
}

func (app office) previewPeople(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 5<<20)
	reader, err := r.MultipartReader()
	if err != nil {
		bad(w, "请选择 Excel 花名册")
		return
	}
	part, err := reader.NextPart()
	if err != nil || part.FormName() != "file" {
		bad(w, "请选择 Excel 花名册")
		return
	}
	defer part.Close()
	book, err := excelize.OpenReader(part, excelize.Options{UnzipSizeLimit: 32 << 20, UnzipXMLSizeLimit: 4 << 20})
	if err != nil {
		bad(w, "Excel 文件无法读取")
		return
	}
	defer book.Close()
	result := []rosterRow{}
	seen := map[string]bool{}
	for _, sheet := range []string{"人员", "店铺任职"} {
		rows, err := book.GetRows(sheet)
		if err != nil || len(rows) == 0 || len(rows) > 2001 {
			bad(w, "须包含人员与店铺任职表，每张最多 2000 行")
			return
		}
		header := peopleHeader
		if sheet == "店铺任职" {
			header = termsHeader
		}
		extended := sheet == "人员" && len(rows[0]) != 4
		if sheet == "人员" && !extended {
			header = peopleHeader[:4]
		}
		if len(rows[0]) < len(header) {
			bad(w, "花名册表头与导出模板不一致")
			return
		}
		for index, label := range header {
			if rows[0][index] != label {
				bad(w, "花名册表头与导出模板不一致")
				return
			}
		}
		for index, cells := range rows[1:] {
			if len(cells) == 0 {
				continue
			}
			values := make([]string, len(header))
			copy(values, cells)
			for cell := range values {
				values[cell] = strings.TrimSpace(values[cell])
			}
			if strings.Join(values, "") == "" {
				continue
			}
			row := rosterRow{Row: index + 2, Sheet: sheet, PersonID: number(values[0]), Name: values[2]}
			p, err := app.readPerson(r, row.PersonID)
			if errors.Is(err, sql.ErrNoRows) {
				row.Error = "请先在 OA 账号管理建立稳定用户 ID"
			} else if err != nil {
				internal(w)
				return
			} else if !scopePerson(actor, p) {
				row.Error = "用户不在管理范围"
			} else {
				row.Version = p.Version
				row.Name = p.Name
			}
			if row.Error == "" && extended {
				if err := app.applyProfile(r.Context(), &p); err != nil {
					internal(w)
					return
				}
				input, err := app.rosterProfile(r, actor, p, values)
				if errors.Is(err, errManagerInvalid) || errors.Is(err, errRosterProfile) || errors.Is(err, errManagerCycle) {
					row.Error = err.Error()
				} else if err != nil {
					internal(w)
					return
				} else {
					row.Profile = input
				}
			}
			if row.Error == "" && sheet == "人员" {
				row.EmploymentType = values[3]
				row.PreviousEmployeeNo = p.EmployeeNo
				row.PreviousName = p.Name
				if values[1] != "" {
					no, parseErr := strconv.Atoi(values[1])
					if parseErr != nil || no < 0 || no > 9999 || len(values[1]) > 4 || strings.Trim(values[1], "0123456789") != "" {
						row.Error = "工号必须是 0–9999 的数字"
					} else if p.EmployeeNo == nil || no != *p.EmployeeNo {
						var duplicate bool
						if err := app.source.QueryRowContext(r.Context(), "SELECT EXISTS(SELECT 1 FROM users WHERE employee_no=? AND id<>?)", no, p.ID).Scan(&duplicate); err != nil {
							internal(w)
							return
						}
						key := "工号/" + strconv.Itoa(no)
						if duplicate || seen[key] {
							row.Error = "工号已占用"
						} else {
							row.EmployeeNo = &no
							seen[key] = true
						}
					}
				}
				if values[2] != "" && values[2] != p.Name {
					if !textOK(values[2], 128) {
						row.Error = "姓名长度无效"
					} else {
						row.NewName = values[2]
					}
				}
				if len(row.EmploymentType) > 48 {
					row.Error = "用工类型过长"
				}
			} else if row.Error == "" {
				row.ShopID = values[1]
				row.From = values[2]
				day, dateErr := time.Parse(time.DateOnly, row.From)
				unchanged := false
				for _, term := range p.Terms {
					if term.ShopID == row.ShopID && term.From == row.From && term.To == values[3] {
						unchanged = true
					}
				}
				if unchanged {
					row.ShopID = ""
				} else if !textOK(row.ShopID, 128) || dateErr != nil || day.Format(time.DateOnly) < businessDate().Format(time.DateOnly) || p.Status != "active" || values[3] != "" {
					row.Error = "只允许为在职人员新增今日或未来的店铺任职"
				}
			}
			key := sheet + "/" + decimal(row.PersonID) + "/" + values[1] + "/" + row.From
			if sheet == "人员" {
				key = sheet + "/" + decimal(row.PersonID)
			}
			if seen[key] && row.Error == "" {
				row.Error = "文件内重复记录"
			}
			seen[key] = true
			result = append(result, row)
		}
	}
	token := identifier()
	_, err = app.db.ExecContext(r.Context(), "INSERT INTO oa_office_imports(token,actor,payload,expires_at) VALUES(?,?,?,CURRENT_TIMESTAMP+INTERVAL 1 HOUR)", token, actor.ID, jsonData(result))
	if err != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"token": token, "items": result})
}

func (app office) confirmPeople(w http.ResponseWriter, r *http.Request) {
	actor := platform.ActorFrom(r)
	if !permit(actor, "people.manage") {
		deny(w)
		return
	}
	var input struct {
		Token string `json:"token"`
	}
	if !decode(w, r, &input) {
		return
	}
	ctx := r.Context()
	var raw []byte
	var count sql.NullInt64
	err := app.db.QueryRowContext(ctx, "SELECT payload,confirmed_count FROM oa_office_imports WHERE token=? AND actor=? AND expires_at>CURRENT_TIMESTAMP", input.Token, actor.ID).Scan(&raw, &count)
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if count.Valid {
		platform.JSON(w, 200, map[string]any{"count": count.Int64, "items": json.RawMessage(raw)})
		return
	}
	var rows []rosterRow
	if json.Unmarshal(raw, &rows) != nil {
		internal(w)
		return
	}
	originals := map[int64]person{}
	for _, row := range rows {
		if row.Error != "" || row.Result == "applied" {
			continue
		}
		if _, exists := originals[row.PersonID]; exists {
			continue
		}
		p, err := app.sourcePerson(r, row.PersonID)
		if err == nil {
			originals[p.ID] = p
		}
	}
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	if err := lockManagers(ctx, tx); err != nil {
		internal(w)
		return
	}
	err = tx.QueryRowContext(ctx, "SELECT payload,confirmed_count FROM oa_office_imports WHERE token=? AND actor=? AND expires_at>CURRENT_TIMESTAMP FOR UPDATE", input.Token, actor.ID).Scan(&raw, &count)
	if err != nil {
		conflict(w)
		return
	}
	if count.Valid {
		platform.JSON(w, 200, map[string]any{"count": count.Int64, "items": json.RawMessage(raw)})
		return
	}
	if json.Unmarshal(raw, &rows) != nil {
		internal(w)
		return
	}
	changed := 0
	failures := 0
	for index := range rows {
		row := &rows[index]
		if row.Error != "" {
			continue
		}
		if row.Result == "applied" {
			changed++
			continue
		}
		row.Failure = ""
		original := originals[row.PersonID]
		if original.ID == 0 {
			row.Failure = "人员已不存在或暂无法读取"
			failures++
			continue
		}
		p, err := lockPerson(r, tx, original)
		if err != nil {
			internal(w)
			return
		}
		current := p
		current.project()
		if !scopePerson(actor, current) {
			row.Failure = "用户不在管理范围"
			failures++
			continue
		}
		if original.Version != 0 && p.Version != original.Version || original.Version == 0 && p.Version != row.Version {
			row.Failure = "人员任职已变化，请重新预览"
			failures++
			continue
		}
		dirty := false
		if row.Profile != nil {
			if !validProfile(*row.Profile) || row.Profile.ManagerID == p.ID {
				row.Failure = "人员档案参数无效"
				failures++
				continue
			}
			if row.Profile.ManagerID > 0 {
				if _, err := app.allowedManager(r, actor, row.Profile.ManagerID); err != nil {
					if !errors.Is(err, errManagerInvalid) {
						internal(w)
						return
					}
					row.Failure = err.Error()
					failures++
					continue
				}
			}
			if _, err := tx.ExecContext(ctx, "SAVEPOINT oa_profile_row"); err != nil {
				internal(w)
				return
			}
			err := writeProfile(ctx, tx, p.ID, actor.ID, *row.Profile)
			if err == nil {
				err = checkManagerGraph(ctx, tx)
			}
			if err != nil {
				if !errors.Is(err, errProfileVersion) && !errors.Is(err, errManagerCycle) {
					internal(w)
					return
				}
				if _, rollback := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT oa_profile_row"); rollback != nil {
					internal(w)
					return
				}
				row.Failure = err.Error()
				failures++
				continue
			}
		}
		if row.EmployeeNo != nil || row.NewName != "" {
			if problem := app.patchRosterIdentity(r, *row, p); problem != "" {
				if row.Profile != nil {
					if _, err := tx.ExecContext(ctx, "ROLLBACK TO SAVEPOINT oa_profile_row"); err != nil {
						internal(w)
						return
					}
				}
				row.Failure = problem
				failures++
				continue
			}
			dirty = true
		}
		if row.EmploymentType != "" && row.EmploymentType != p.EmploymentType {
			p.EmploymentType = row.EmploymentType
			dirty = true
		}
		if row.ShopID != "" {
			prior := len(p.Terms)
			addTerm(&p, row.ShopID, row.From)
			dirty = len(p.Terms) != prior || dirty
		}
		if dirty {
			p.Version++
			if savePerson(r, tx, p) != nil {
				internal(w)
				return
			}
		}
		if dirty || row.Profile != nil {
			changed++
			row.Result = "applied"
		} else {
			row.Result = "unchanged"
		}
		original.Version = p.Version
		originals[row.PersonID] = original
	}
	var finished any
	if failures == 0 {
		finished = changed
	}
	if _, err = tx.ExecContext(ctx, "UPDATE oa_office_imports SET payload=?,confirmed_count=? WHERE token=?", jsonData(rows), finished, input.Token); err != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"count": changed, "items": rows})
}

func (app office) patchRosterIdentity(r *http.Request, row rosterRow, current person) string {
	fields := map[string]any{}
	if row.EmployeeNo != nil && !sameEmployeeNo(current.EmployeeNo, row.EmployeeNo) {
		if !sameEmployeeNo(current.EmployeeNo, row.PreviousEmployeeNo) {
			return "现有工号已变化，请重新预览"
		}
		fields["employee_no"] = *row.EmployeeNo
	}
	if row.NewName != "" && current.Name != row.NewName {
		if current.Name != row.PreviousName {
			return "现有姓名已变化，请重新预览"
		}
		fields["display_name"] = row.NewName
	}
	if len(fields) == 0 {
		return ""
	}
	if app.native == nil {
		return "原生人员服务尚未接入，可重试"
	}
	req, err := http.NewRequestWithContext(r.Context(), http.MethodPatch, "/v1/users/"+decimal(row.PersonID), bytes.NewBufferString(jsonData(fields)))
	if err != nil {
		return "无法构造人员维护请求"
	}
	req.Header.Set("Authorization", r.Header.Get("Authorization"))
	req.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	app.native.ServeHTTP(response, req)
	switch response.Code {
	case 200, 204:
		return ""
	case 400, 409, 422:
		return "主站拒绝该资料变更，请核对姓名和工号占用"
	case 401, 403:
		return "当前账号没有通过主站人员变更授权"
	default:
		return "主站人员服务暂不可用，可重试"
	}
}

func sameEmployeeNo(left, right *int) bool {
	return left == nil && right == nil || left != nil && right != nil && *left == *right
}

var errRosterProfile = errors.New("请检查入职日期（YYYY-MM-DD，今日或过去）及岗位、职级、成本中心（各限 100 字）")

func (app office) rosterProfile(r *http.Request, actor platform.Actor, p person, values []string) (*profileUpdate, error) {
	input := profileUpdate{Version: p.ProfileVersion, Joined: values[6], JobTitle: values[7], JobLevel: values[8], CostCenter: values[9]}
	switch {
	case values[4] != "":
		id, err := strconv.ParseInt(values[4], 10, 64)
		if err != nil || id <= 0 {
			return nil, errManagerInvalid
		}
		input.ManagerID = id
	case values[5] != "":
		id, err := app.rosterManagerReference(r, actor, values[5])
		if err != nil {
			return nil, err
		}
		input.ManagerID = id
	}
	if input.ManagerID == p.ManagerOverrideID && input.Joined == p.Joined && input.JobTitle == p.JobTitle &&
		input.JobLevel == p.JobLevel && input.CostCenter == p.CostCenter {
		return nil, nil
	}
	if !validProfile(input) {
		return nil, errRosterProfile
	}
	if input.ManagerID == p.ID {
		return nil, errManagerInvalid
	}
	if input.ManagerID > 0 {
		if _, err := app.allowedManager(r, actor, input.ManagerID); err != nil {
			return nil, err
		}
		seen := map[int64]bool{p.ID: true}
		for current := input.ManagerID; current > 0; {
			if seen[current] {
				return nil, errManagerCycle
			}
			seen[current] = true
			id, err := app.effectiveManager(r, current)
			if err != nil {
				return nil, err
			}
			current = id
		}
	}
	return &input, nil
}

func (app office) rosterManagerReference(r *http.Request, actor platform.Actor, ref string) (int64, error) {
	rows, err := app.source.QueryContext(r.Context(), `SELECT id FROM users WHERE status='active' AND
		(display_name=? OR employee_no=?) LIMIT 3`, ref, employeeSearchNumber(ref))
	if err != nil {
		return 0, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err = rows.Scan(&id); err != nil {
			return 0, err
		}
		ids = append(ids, id)
	}
	if err = rows.Err(); err != nil {
		return 0, err
	}
	if err = rows.Close(); err != nil {
		return 0, err
	}
	if len(ids) != 1 {
		return 0, errManagerInvalid
	}
	if _, err = app.allowedManager(r, actor, ids[0]); err != nil {
		return 0, err
	}
	return ids[0], nil
}
