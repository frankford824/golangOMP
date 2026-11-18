package office

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
	"workflow/internal/oa/platform"
)

type rosterRow struct {
	Row                int    `json:"row"`
	Sheet              string `json:"sheet"`
	Name               string `json:"name"`
	Error              string `json:"error"`
	PersonID           int64  `json:"id"`
	Version            int    `json:"version"`
	EmploymentType     string `json:"employment_type"`
	EmployeeNo         *int   `json:"employee_no,omitempty"`
	PreviousEmployeeNo *int   `json:"previous_employee_no,omitempty"`
	PreviousName       string `json:"previous_name,omitempty"`
	NewName            string `json:"new_name,omitempty"`
	ShopID             string `json:"shop_id"`
	From               string `json:"from"`
}

var peopleHeader = []any{"用户ID", "工号", "姓名", "用工类型"}
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
	for row, p := range people {
		no := ""
		if p.EmployeeNo != nil {
			no = strconv.Itoa(*p.EmployeeNo)
		}
		values := []any{decimal(p.ID), no, p.Name, p.EmploymentType}
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
			values := make([]string, 4)
			copy(values, cells)
			for cell := range values {
				values[cell] = strings.TrimSpace(values[cell])
			}
			if values[0] == "" && values[1] == "" && values[2] == "" && values[3] == "" {
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
		platform.JSON(w, 200, map[string]any{"count": count.Int64})
		return
	}
	var rows []rosterRow
	if json.Unmarshal(raw, &rows) != nil {
		internal(w)
		return
	}
	originals := map[int64]person{}
	for _, row := range rows {
		if row.Error != "" {
			bad(w, "请先修正冲突行并重新预览")
			return
		}
		if _, exists := originals[row.PersonID]; exists {
			continue
		}
		p, err := app.sourcePerson(r, row.PersonID)
		if err != nil {
			conflict(w)
			return
		}
		originals[p.ID] = p
	}
	tx, err := app.db.BeginTx(ctx, nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	err = tx.QueryRowContext(ctx, "SELECT confirmed_count FROM oa_office_imports WHERE token=? AND expires_at>CURRENT_TIMESTAMP FOR UPDATE", input.Token).Scan(&count)
	if err != nil {
		conflict(w)
		return
	}
	if count.Valid {
		platform.JSON(w, 200, map[string]any{"count": count.Int64})
		return
	}
	changed := 0
	for _, row := range rows {
		original := originals[row.PersonID]
		p, err := lockPerson(r, tx, original)
		if err != nil {
			internal(w)
			return
		}
		current := p
		current.project()
		if !scopePerson(actor, current) {
			deny(w)
			return
		}
		if original.Version != 0 && p.Version != original.Version || original.Version == 0 && p.Version != row.Version {
			conflict(w)
			return
		}
		dirty := false
		if row.EmployeeNo != nil || row.NewName != "" {
			updated, err := tx.ExecContext(ctx, "UPDATE users SET employee_no=COALESCE(?,employee_no),display_name=COALESCE(NULLIF(?,''),display_name) WHERE id=? AND employee_no <=> ? AND display_name=? AND status<>'deleted'", row.EmployeeNo, row.NewName, p.ID, row.PreviousEmployeeNo, row.PreviousName)
			if err != nil {
				conflict(w)
				return
			}
			affected, err := updated.RowsAffected()
			if err != nil || affected != 1 {
				conflict(w)
				return
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
			changed++
		}
		original.Version = p.Version
		originals[row.PersonID] = original
	}
	if _, err = tx.ExecContext(ctx, "UPDATE oa_office_imports SET confirmed_count=? WHERE token=?", changed, input.Token); err != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, map[string]any{"count": changed})
}
