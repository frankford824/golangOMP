package mysqlrepo

import (
	"strings"
	"testing"
)

func TestTaskListOrderByHonorsSortToken(t *testing.T) {
	cases := []struct {
		name  string
		token string
		want  string
	}{
		{"empty defaults to updated_at desc", "", "t.updated_at DESC, t.id DESC"},
		{"updated_at ascending", "updated_at", "t.updated_at ASC, t.id DESC"},
		{"updated_at descending", "-updated_at", "t.updated_at DESC, t.id DESC"},
		{"created_at ascending", "created_at", "t.created_at ASC, t.id DESC"},
		{"created_at descending", "-created_at", "t.created_at DESC, t.id DESC"},
		{"due_at maps to deadline", "due_at", "t.deadline_at ASC, t.id DESC"},
		{"due_at descending", "-due_at", "t.deadline_at DESC, t.id DESC"},
		{"task_no ascending", "task_no", "t.task_no ASC, t.id DESC"},
		{"unknown token is coerced to updated_at desc", "; DROP TABLE tasks", "t.updated_at DESC, t.id DESC"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := taskListOrderBy(tc.token)
			if got != tc.want {
				t.Fatalf("taskListOrderBy(%q) = %q, want %q", tc.token, got, tc.want)
			}
			// Guard against injection: only whitelisted column identifiers may appear.
			if strings.ContainsAny(got, ";()'\"") {
				t.Fatalf("taskListOrderBy(%q) produced unsafe clause %q", tc.token, got)
			}
		})
	}
}
