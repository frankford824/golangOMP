package service

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

func TestWorkflowTraceEventServiceRecordNormalizesEvent(t *testing.T) {
	repoStub := &workflowTraceEventRepoStub{id: 123}
	svc := NewWorkflowTraceEventService(repoStub)
	actorID := int64(42)

	created, appErr := svc.RecordTraceEvent(context.Background(), &domain.WorkflowTraceEvent{
		EventType:     domain.WorkflowTraceEventUserAction,
		ActorID:       &actorID,
		ActorRoles:    []domain.Role{domain.RoleOps},
		ActorUsername: "  ops-user  ",
		Payload:       []byte(`{"task_id":733}`),
	})
	if appErr != nil {
		t.Fatalf("RecordTraceEvent appErr=%+v", appErr)
	}
	if created.ID != 123 {
		t.Fatalf("created.ID = %d, want 123", created.ID)
	}
	if created.EventID == "" {
		t.Fatal("EventID should be generated")
	}
	if created.EventSource != domain.WorkflowTraceSourceSystem {
		t.Fatalf("EventSource = %q, want %q", created.EventSource, domain.WorkflowTraceSourceSystem)
	}
	if created.ActorUsername != "ops-user" {
		t.Fatalf("ActorUsername = %q, want trimmed", created.ActorUsername)
	}
	if created.OccurredAt.IsZero() {
		t.Fatal("OccurredAt should be set")
	}
	if repoStub.created == nil || repoStub.created.EventID != created.EventID {
		t.Fatal("repo did not receive normalized event")
	}
}

func TestWorkflowTraceEventServiceRecordRejectsInvalidPayload(t *testing.T) {
	svc := NewWorkflowTraceEventService(&workflowTraceEventRepoStub{})
	_, appErr := svc.RecordTraceEvent(context.Background(), &domain.WorkflowTraceEvent{
		EventType: domain.WorkflowTraceEventUserAction,
		Payload:   []byte(`{"broken"`),
	})
	if appErr == nil || appErr.Code != domain.ErrCodeInvalidRequest {
		t.Fatalf("appErr = %+v, want invalid request", appErr)
	}
}

func TestWorkflowTraceEventServiceListPassesBusinessFilters(t *testing.T) {
	repoStub := &workflowTraceEventRepoStub{}
	svc := NewWorkflowTraceEventService(repoStub)

	_, _, appErr := svc.ListTraceEvents(context.Background(), WorkflowTraceEventFilter{
		ActorUsername: "  admin  ",
		ActorSource:   "  session_token  ",
		BusinessOnly:  true,
		Page:          0,
		PageSize:      200,
	})
	if appErr != nil {
		t.Fatalf("ListTraceEvents appErr=%+v", appErr)
	}
	if repoStub.lastFilter.ActorUsername != "admin" {
		t.Fatalf("ActorUsername = %q, want admin", repoStub.lastFilter.ActorUsername)
	}
	if repoStub.lastFilter.ActorSource != "session_token" {
		t.Fatalf("ActorSource = %q, want session_token", repoStub.lastFilter.ActorSource)
	}
	if !repoStub.lastFilter.BusinessOnly {
		t.Fatal("BusinessOnly = false, want true")
	}
	if repoStub.lastFilter.Page != 1 || repoStub.lastFilter.PageSize != 20 {
		t.Fatalf("pagination = (%d,%d), want (1,20)", repoStub.lastFilter.Page, repoStub.lastFilter.PageSize)
	}
}

type workflowTraceEventRepoStub struct {
	id         int64
	created    *domain.WorkflowTraceEvent
	lastFilter repo.WorkflowTraceEventListFilter
}

func (s *workflowTraceEventRepoStub) Create(_ context.Context, _ repo.Tx, event *domain.WorkflowTraceEvent) (int64, error) {
	s.created = event
	if s.id == 0 {
		return 1, nil
	}
	return s.id, nil
}

func (s *workflowTraceEventRepoStub) List(_ context.Context, filter repo.WorkflowTraceEventListFilter) ([]*domain.WorkflowTraceEvent, int64, error) {
	s.lastFilter = filter
	return []*domain.WorkflowTraceEvent{}, 0, nil
}
