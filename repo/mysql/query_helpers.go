package mysqlrepo

import (
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
)

func marshalOptionalJSON(value interface{}) ([]byte, error) {
	if value == nil {
		return []byte("null"), nil
	}
	return json.Marshal(value)
}

func unmarshalOptionalRoles(raw string) ([]domain.Role, error) {
	if strings.TrimSpace(raw) == "" || strings.TrimSpace(raw) == "null" {
		return nil, nil
	}
	var roles []domain.Role
	if err := json.Unmarshal([]byte(raw), &roles); err != nil {
		return nil, fmt.Errorf("unmarshal roles: %w", err)
	}
	return roles, nil
}

func buildInt64InClause(column string, values []int64) (string, []interface{}) {
	placeholders := make([]string, 0, len(values))
	args := make([]interface{}, 0, len(values))
	for _, value := range values {
		placeholders = append(placeholders, "?")
		args = append(args, value)
	}
	return column + " IN (" + strings.Join(placeholders, ", ") + ")", args
}

func nullIfEmpty(value string) interface{} {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return value
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if text := strings.TrimSpace(value); text != "" {
			return text
		}
	}
	return ""
}
