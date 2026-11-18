package office

import (
	"context"
	"net/http"
	"strings"
	"unicode"

	"github.com/mozillazg/go-pinyin"
	"golang.org/x/text/unicode/norm"
)

// New clients put names and phone numbers in a request body rather than access-log URLs.
func (app office) searchApprovers(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Query string `json:"q"`
		IDs   string `json:"ids"`
	}
	if !decode(w, r, &in) {
		return
	}
	app.findApprovers(w, r, in.Query, in.IDs, true)
}

func phoneDigits(text string) string {
	compact := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) || strings.ContainsRune("()-（）", r) {
			return -1
		}
		return r
	}, text)
	compact = strings.TrimPrefix(strings.TrimPrefix(compact, "+86"), "0086")
	if len(compact) == 13 && strings.HasPrefix(compact, "86") {
		compact = compact[2:]
	}
	if len(compact) != 11 || compact[0] != '1' || compact[1] < '3' || compact[1] > '9' {
		return ""
	}
	for _, c := range compact {
		if c < '0' || c > '9' {
			return ""
		}
	}
	return compact
}

func romanQuery(s string) string {
	var b strings.Builder
	s = norm.NFD.String(strings.ToLower(s))
	s = strings.ReplaceAll(strings.ReplaceAll(s, "u\u0308", "v"), "u:", "v")
	for _, c := range s {
		switch {
		case c >= 'a' && c <= 'z':
			b.WriteRune(c)
		case unicode.Is(unicode.Mn, c), unicode.IsSpace(c), c == '-', c == 0x27: // pinyin separators and tone marks
		default:
			return ""
		}
	}
	return b.String()
}

func nameMatches(name, query string) bool {
	// Track matched lengths, so alternate readings do not multiply into a list of all possible names.
	args := pinyin.NewArgs()
	args.Heteronym = true
	positions := [2]map[int]bool{}
	for _, c := range name {
		syllables := pinyin.SinglePinyin(c, args)
		if len(syllables) == 0 {
			fallback := romanQuery(string(c))
			if fallback == "" {
				continue
			}
			syllables = []string{fallback}
		}
		for mode := range positions {
			next := make(map[int]bool)
			if positions[mode] == nil {
				positions[mode] = make(map[int]bool)
			}
			positions[mode][0] = true
			for _, syllable := range syllables {
				if syllable == "" {
					continue
				}
				if mode == 1 {
					syllable = syllable[:1]
				}
				for pos := range positions[mode] {
					tail := query[pos:]
					if strings.HasPrefix(syllable, tail) {
						return true
					}
					if strings.HasPrefix(tail, syllable) {
						next[pos+len(syllable)] = true
					}
				}
			}
			positions[mode] = next
		}
	}
	return false
}

const approverScope = `SELECT id,display_name,employee_no,COALESCE(department,'') FROM users WHERE status='active' AND (? OR department_id=NULLIF(?,0) OR id=?)`

func (app office) searchApproverRows(ctx context.Context, scopeArgs []any, text string) ([]approverOption, error) {
	args := append([]any(nil), scopeArgs...)
	const ordering = " ORDER BY display_name,id"
	var filter string
	roman := romanQuery(text)
	switch {
	case text == "":
		filter = ordering + " LIMIT 50"
	case phoneDigits(text) != "":
		phone := phoneDigits(text)
		// Match complete stored numbers, including the common country-prefix and presentation forms.
		filter = ` AND REPLACE(REPLACE(REPLACE(REPLACE(COALESCE(mobile,''),' ',''),'-',''),'(',''),')','') IN (?,?,?,?)` + ordering + " LIMIT 50"
		args = append(args, phone, "+86"+phone, "86"+phone, "0086"+phone)
	case roman != "":
		// The SQL scope is applied before transliteration. Stop after 50 matching authorized candidates.
		filter = ordering
	default:
		filter = " AND (INSTR(display_name,?)>0 OR INSTR(CAST(employee_no AS CHAR),?)>0 OR INSTR(department,?)>0)" + ordering + " LIMIT 50"
		args = append(args, text, text, text)
	}
	rows, err := app.source.QueryContext(ctx, approverScope+filter, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []approverOption{}
	for rows.Next() {
		var item approverOption
		if err := rows.Scan(&item.ID, &item.Name, &item.EmployeeNo, &item.Department); err != nil {
			return nil, err
		}
		if roman != "" && !strings.Contains(strings.ToLower(item.Department), strings.ToLower(text)) &&
			!strings.Contains(strings.ToLower(item.Name), strings.ToLower(text)) && !nameMatches(item.Name, roman) {
			continue
		}
		out = append(out, item)
		if len(out) == 50 {
			break
		}
	}
	return out, rows.Err()
}
