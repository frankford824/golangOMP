package service

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"workflow/domain"
	"workflow/repo"
)

func TestTaskEventServiceListByTaskIDEnrichesActorNames(t *testing.T) {
	operatorID := int64(9)
	taskRepo := &taskEventTaskRepoStub{
		task: &domain.Task{
			ID:        603,
			CreatorID: 1,
		},
	}
	eventRepo := &taskEventRepoStub{
		events: []*domain.TaskEvent{
			{
				ID:         "event-1",
				TaskID:     603,
				Sequence:   1,
				EventType:  domain.TaskEventCreated,
				OperatorID: &operatorID,
				Payload:    json.RawMessage(`{"creator_id":1}`),
				CreatedAt:  time.Now().UTC(),
			},
		},
	}
	svc := NewTaskEventService(eventRepo, taskRepo, WithTaskEventUserDisplayNameResolver(taskEventNameResolverStub{
		names: map[int64]string{
			1: "系统管理员",
			9: "设计师九",
		},
	}))

	events, appErr := svc.ListByTaskID(context.Background(), 603)
	if appErr != nil {
		t.Fatalf("ListByTaskID() appErr = %v", appErr)
	}
	if len(events) != 1 {
		t.Fatalf("events length = %d, want 1", len(events))
	}
	if events[0].CreatorID == nil || *events[0].CreatorID != 1 || events[0].CreatorName != "系统管理员" {
		t.Fatalf("creator enrichment = (%v, %q), want (1, 系统管理员)", events[0].CreatorID, events[0].CreatorName)
	}
	if events[0].OperatorName != "设计师九" {
		t.Fatalf("operator_name = %q, want 设计师九", events[0].OperatorName)
	}
}

type taskEventNameResolverStub struct {
	names map[int64]string
}

func (r taskEventNameResolverStub) GetDisplayName(_ context.Context, userID int64) string {
	return r.names[userID]
}

type taskEventRepoStub struct {
	events []*domain.TaskEvent
}

func (r *taskEventRepoStub) Append(context.Context, repo.Tx, int64, string, *int64, interface{}) (*domain.TaskEvent, error) {
	panic("not used")
}

func (r *taskEventRepoStub) ListByTaskID(context.Context, int64) ([]*domain.TaskEvent, error) {
	return r.events, nil
}

func (r *taskEventRepoStub) ListRecent(context.Context, repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	panic("not used")
}

type taskEventTaskRepoStub struct {
	task *domain.Task
}

func (r *taskEventTaskRepoStub) Create(context.Context, repo.Tx, *domain.Task, *domain.TaskDetail) (int64, error) {
	panic("not used")
}

func (r *taskEventTaskRepoStub) CreateSKUItems(context.Context, repo.Tx, []*domain.TaskSKUItem) error {
	panic("not used")
}

func (r *taskEventTaskRepoStub) GetByID(context.Context, int64) (*domain.Task, error) {
	return r.task, nil
}

func (r *taskEventTaskRepoStub) GetDetailByTaskID(context.Context, int64) (*domain.TaskDetail, error) {
	panic("not used")
}

func (r *taskEventTaskRepoStub) GetSKUItemBySKUCode(context.Context, string) (*domain.TaskSKUItem, error) {
	panic("not used")
}

func (r *taskEventTaskRepoStub) ListSKUItemsByTaskID(context.Context, int64) ([]*domain.TaskSKUItem, error) {
	panic("not used")
}

func (r *taskEventTaskRepoStub) List(context.Context, repo.TaskListFilter) ([]*domain.TaskListItem, int64, error) {
	panic("not used")
}

func (r *taskEventTaskRepoStub) ListBoardCandidates(context.Context, repo.TaskBoardCandidateFilter) ([]*domain.TaskListItem, error) {
	panic("not used")
}

func (r *taskEventTaskRepoStub) UpdateDetailBusinessInfo(context.Context, repo.Tx, *domain.TaskDetail) error {
	panic("not used")
}

func (r *taskEventTaskRepoStub) UpdateProductBinding(context.Context, repo.Tx, *domain.Task) error {
	panic("not used")
}

func (r *taskEventTaskRepoStub) UpdateStatus(context.Context, repo.Tx, int64, domain.TaskStatus) error {
	panic("not used")
}

func (r *taskEventTaskRepoStub) UpdateDesigner(context.Context, repo.Tx, int64, *int64) error {
	panic("not used")
}

func (r *taskEventTaskRepoStub) UpdateHandler(context.Context, repo.Tx, int64, *int64) error {
	panic("not used")
}

func (r *taskEventTaskRepoStub) UpdateCustomizationState(context.Context, repo.Tx, int64, *int64, string, string) error {
	panic("not used")
}
