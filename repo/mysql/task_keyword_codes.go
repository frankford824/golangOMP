package mysqlrepo

import (
	"strings"
	"unicode"
)

// Only split a list entirely made of codes; spaces in product names retain
// their existing phrase-search meaning. A repeated code still uses list mode.
func taskListKeywordCodes(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return unicode.IsSpace(r) || strings.ContainsRune(",，;；、", r)
	})
	if len(parts) < 2 {
		return nil
	}
	codes := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))
	for _, part := range parts {
		code := strings.ToUpper(part)
		letter, digit := false, false
		for _, r := range code {
			switch {
			case r >= 'A' && r <= 'Z':
				letter = true
			case r >= '0' && r <= '9':
				digit = true
			case r == '-' || r == '_' || r == '.':
			default:
				return nil
			}
		}
		if !letter || !digit {
			return nil
		}
		if !seen[code] {
			codes = append(codes, code)
			seen[code] = true
		}
	}
	return codes
}
