package mysqlrepo

import (
	"strconv"
	"strings"
	"unicode"
)

type normalizedSearchKeyword struct {
	Raw       string
	Upper     string
	Like      string
	Prefix    string
	Int64     int64
	HasInt64  bool
	IsCode    bool
	IsTaskNo  bool
	IsSKUCode bool
}

func normalizeSearchKeyword(raw string) normalizedSearchKeyword {
	text := strings.TrimSpace(raw)
	upper := strings.ToUpper(text)
	kw := normalizedSearchKeyword{
		Raw:    text,
		Upper:  upper,
		Like:   "%" + text + "%",
		Prefix: text + "%",
	}
	if text == "" {
		return kw
	}
	if id, err := strconv.ParseInt(text, 10, 64); err == nil && id > 0 {
		kw.Int64 = id
		kw.HasInt64 = true
	}
	kw.IsTaskNo = looksLikeTaskNo(upper)
	kw.IsSKUCode = !kw.IsTaskNo && looksLikeSKUCode(upper)
	kw.IsCode = kw.IsTaskNo || kw.IsSKUCode || looksLikeMixedCode(upper)
	return kw
}

func booleanPhraseSearchQuery(raw string) string {
	value := strings.TrimSpace(raw)
	if value == "" {
		return value
	}
	replacer := strings.NewReplacer(`"`, " ", `\`, " ")
	value = strings.Join(strings.Fields(replacer.Replace(value)), " ")
	if value == "" {
		return strings.TrimSpace(raw)
	}
	return `"` + value + `"`
}

func looksLikeTaskNo(value string) bool {
	return strings.HasPrefix(value, "RW-") || strings.HasPrefix(value, "RW")
}

func looksLikeSKUCode(value string) bool {
	if len(value) < 5 {
		return false
	}
	letters := 0
	digits := 0
	for _, r := range value {
		switch {
		case unicode.IsLetter(r):
			letters++
		case unicode.IsDigit(r):
			digits++
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return letters >= 2 && digits >= 2
}

func looksLikeMixedCode(value string) bool {
	if len(value) < 3 {
		return false
	}
	hasLetter := false
	hasDigit := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r):
			hasLetter = true
		case unicode.IsDigit(r):
			hasDigit = true
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return hasLetter && hasDigit
}

func appendAnyClause(clauses []string, parts ...string) []string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part != "" {
			filtered = append(filtered, part)
		}
	}
	if len(filtered) == 0 {
		return clauses
	}
	return append(clauses, "("+strings.Join(filtered, " OR ")+")")
}
