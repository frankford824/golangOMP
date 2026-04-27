package handler

import (
	"testing"

	"workflow/domain"
)

func TestValidateCreateTaskPriority(t *testing.T) {
	passCases := []struct {
		name string
		in   string
		want string
	}{
		{name: "empty defaults normal", in: "", want: string(domain.TaskPriorityNormal)},
		{name: "low", in: "low", want: string(domain.TaskPriorityLow)},
		{name: "normal", in: "normal", want: string(domain.TaskPriorityNormal)},
		{name: "high", in: "high", want: string(domain.TaskPriorityHigh)},
		{name: "critical", in: "critical", want: string(domain.TaskPriorityCritical)},
	}
	for _, tc := range passCases {
		t.Run(tc.name, func(t *testing.T) {
			got, appErr := validateCreateTaskPriority(tc.in)
			if appErr != nil {
				t.Fatalf("validateCreateTaskPriority(%q) error = %+v", tc.in, appErr)
			}
			if got != tc.want {
				t.Fatalf("validateCreateTaskPriority(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}

	failCases := []string{"urgent", "random", "LOW"}
	for _, in := range failCases {
		t.Run(in, func(t *testing.T) {
			_, appErr := validateCreateTaskPriority(in)
			if appErr == nil {
				t.Fatalf("validateCreateTaskPriority(%q) returned nil error", in)
			}
			if appErr.Code != domain.ErrCodeInvalidRequest {
				t.Fatalf("AppError.Code = %s, want %s", appErr.Code, domain.ErrCodeInvalidRequest)
			}
			if appErr.Message != "task_priority_invalid" {
				t.Fatalf("AppError.Message = %q, want task_priority_invalid", appErr.Message)
			}
		})
	}
}
