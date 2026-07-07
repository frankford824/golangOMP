package domain

import (
	"strings"
	"unicode"
)

// NormalizeIID canonicalizes ERP style i_id values before binding lookup.
func NormalizeIID(value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	var b strings.Builder
	b.Grow(len(value))
	for _, r := range value {
		switch {
		case r == '\u3000':
			continue
		case r >= '\uff01' && r <= '\uff5e':
			r -= 0xfee0
		case unicode.IsSpace(r):
			continue
		}
		b.WriteRune(unicode.ToUpper(r))
	}
	return b.String()
}
