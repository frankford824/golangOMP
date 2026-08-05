package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// rejectingTaskCreateRequestRepo fails the test if the idempotency guard lets an
// over-long client_create_id reach the repository, where it would only surface as
// an opaque INSERT error.
type rejectingTaskCreateRequestRepo struct {
	t        *testing.T
	reserved bool
}

func (r *rejectingTaskCreateRequestRepo) Reserve(context.Context, int64, string, string, string, time.Time) (*domain.TaskCreateRequest, string, error) {
	r.reserved = true
	return nil, "", nil
}

func (r *rejectingTaskCreateRequestRepo) FindRecentActiveByActorPayloadHash(context.Context, int64, string, time.Time) (*domain.TaskCreateRequest, error) {
	return nil, nil
}

func (r *rejectingTaskCreateRequestRepo) MarkSucceeded(context.Context, repo.Tx, int64, string, string, int64) error {
	return nil
}

func (r *rejectingTaskCreateRequestRepo) MarkFailed(context.Context, int64, string, string, string) error {
	return nil
}

func TestReserveTaskCreateIdempotencyRejectsOverLongClientCreateID(t *testing.T) {
	requestRepo := &rejectingTaskCreateRequestRepo{t: t}
	svc := &taskService{taskCreateRequestRepo: requestRepo}
	params := CreateTaskParams{
		TaskType:       domain.TaskTypeNewProductDevelopment,
		ClientCreateID: "compose:" + strings.Repeat("a", 130),
	}

	task, reservation, appErr := svc.reserveTaskCreateIdempotency(context.Background(), &params)

	if appErr == nil {
		t.Fatal("reserveTaskCreateIdempotency() appErr = nil, want validation error")
	}
	if appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("appErr.Code = %q, want %q", appErr.Code, domain.ErrCodeInvalidRequest)
	}
	if task != nil {
		t.Fatalf("task = %+v, want nil", task)
	}
	if reservation.started {
		t.Fatal("reservation.started = true, want false")
	}
	if requestRepo.reserved {
		t.Fatal("Reserve() was called, want the guard to short-circuit before the repository")
	}
	details, ok := appErr.Details.(map[string]interface{})
	if !ok {
		t.Fatalf("appErr.Details = %+v, want map details", appErr.Details)
	}
	violations, ok := details["violations"].([]map[string]interface{})
	if !ok || len(violations) != 1 {
		t.Fatalf("details[violations] = %+v, want exactly one violation", details["violations"])
	}
	if got := violations[0]["field"]; got != "client_create_id" {
		t.Fatalf("violation field = %v, want client_create_id", got)
	}
}

func TestReserveTaskCreateIdempotencyAcceptsMaxLengthClientCreateID(t *testing.T) {
	requestRepo := &rejectingTaskCreateRequestRepo{t: t}
	svc := &taskService{taskCreateRequestRepo: requestRepo}
	params := CreateTaskParams{
		TaskType:       domain.TaskTypeNewProductDevelopment,
		ClientCreateID: strings.Repeat("b", taskCreateClientCreateIDMaxLen),
	}

	if _, _, appErr := svc.reserveTaskCreateIdempotency(context.Background(), &params); appErr != nil {
		t.Fatalf("reserveTaskCreateIdempotency() appErr = %+v, want nil", appErr)
	}
	if !requestRepo.reserved {
		t.Fatal("Reserve() was not called, want the guard to allow a 128-character key")
	}
}

func TestReserveTaskCreateIdempotencyCountsUnicodeCharacters(t *testing.T) {
	t.Run("accepts 128 unicode characters", func(t *testing.T) {
		requestRepo := &rejectingTaskCreateRequestRepo{t: t}
		svc := &taskService{taskCreateRequestRepo: requestRepo}
		params := CreateTaskParams{
			TaskType:       domain.TaskTypeNewProductDevelopment,
			ClientCreateID: strings.Repeat("批", taskCreateClientCreateIDMaxLen),
		}

		if _, _, appErr := svc.reserveTaskCreateIdempotency(context.Background(), &params); appErr != nil {
			t.Fatalf("reserveTaskCreateIdempotency() appErr = %+v, want nil", appErr)
		}
		if !requestRepo.reserved {
			t.Fatal("Reserve() was not called, want the guard to allow 128 unicode characters")
		}
	})

	t.Run("rejects 129 unicode characters", func(t *testing.T) {
		requestRepo := &rejectingTaskCreateRequestRepo{t: t}
		svc := &taskService{taskCreateRequestRepo: requestRepo}
		params := CreateTaskParams{
			TaskType:       domain.TaskTypeNewProductDevelopment,
			ClientCreateID: strings.Repeat("批", taskCreateClientCreateIDMaxLen+1),
		}

		if _, _, appErr := svc.reserveTaskCreateIdempotency(context.Background(), &params); appErr == nil {
			t.Fatal("reserveTaskCreateIdempotency() appErr = nil, want validation error")
		}
		if requestRepo.reserved {
			t.Fatal("Reserve() was called, want the unicode length guard to short-circuit")
		}
	})
}
