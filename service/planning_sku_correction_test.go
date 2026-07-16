package service

import (
	"context"
	"testing"

	"workflow/domain"
	"workflow/repo"
)

type planningCorrectionTx struct{}

func (planningCorrectionTx) IsTx() {}

type planningCorrectionTxRunner struct{}

func (planningCorrectionTxRunner) RunInTx(_ context.Context, fn func(repo.Tx) error) error {
	return fn(planningCorrectionTx{})
}

type planningCorrectionRepoStub struct {
	PlanningSKURepository
	lock      domain.PlanningSKUUpdateLock
	updated   *domain.PlanningSKURevision
	reindexed bool
}

func (s *planningCorrectionRepoStub) GetTaskAccessSubject(context.Context, int64) (domain.TaskAccessSubject, error) {
	return domain.TaskAccessSubject{TaskID: s.lock.TaskID, CreatorID: 99}, nil
}

func (s *planningCorrectionRepoStub) GetUpdateLock(context.Context, repo.Tx, int64, int64) (*domain.PlanningSKUUpdateLock, error) {
	lock := s.lock
	return &lock, nil
}

func (s *planningCorrectionRepoStub) UpdateRevision(context.Context, repo.Tx, domain.PlanningSKUUpdateLock, domain.UpdatePlanningSKURequest, int64) (*domain.PlanningSKURevision, error) {
	return s.updated, nil
}

func (s *planningCorrectionRepoStub) ReindexTask(context.Context, repo.Tx, int64) error {
	s.reindexed = true
	return nil
}

type planningCorrectionEventRepo struct {
	eventType  string
	operatorID int64
	payload    map[string]interface{}
}

func (s *planningCorrectionEventRepo) Append(_ context.Context, _ repo.Tx, _ int64, eventType string, operatorID *int64, payload interface{}) (*domain.TaskEvent, error) {
	s.eventType = eventType
	if operatorID != nil {
		s.operatorID = *operatorID
	}
	s.payload, _ = payload.(map[string]interface{})
	return &domain.TaskEvent{}, nil
}

func (s *planningCorrectionEventRepo) ListByTaskID(context.Context, int64) ([]*domain.TaskEvent, error) {
	return nil, nil
}

func (s *planningCorrectionEventRepo) ListRecent(context.Context, repo.TaskEventListFilter) ([]*domain.TaskEvent, int64, error) {
	return nil, 0, nil
}

func TestPlanningSKUCorrectionWritesAuditEventInTransaction(t *testing.T) {
	repository := &planningCorrectionRepoStub{
		lock: domain.PlanningSKUUpdateLock{
			TaskID: 42, TaskSKUItemID: 7, SKUCode: "PLAN-001", LockVersion: 3,
			CurrentRevision: domain.PlanningSKURevision{ID: 10, VersionNo: 3, DescriptionSpec: "旧规格", Quantity: 1},
		},
		updated: &domain.PlanningSKURevision{ID: 11, TaskSKUItemID: 7, VersionNo: 4, DescriptionSpec: "新规格", Quantity: 2, Reason: "调整数量"},
	}
	events := &planningCorrectionEventRepo{}
	svc := &planningSKUService{repo: repository, eventRepo: events, txRunner: planningCorrectionTxRunner{}}
	actor := domain.RequestActor{ID: 99, Permissions: []domain.PermissionCode{domain.PermissionPlanningSKUEdit}, EffectiveAccess: &domain.EffectiveAccess{
		Permissions: []domain.PermissionCode{domain.PermissionPlanningSKUEdit},
		Assignments: []domain.AccessAssignment{{RoleID: 1, ScopeMode: domain.AccessScopeGlobal}},
		Sources:     []domain.EffectiveAccessNote{{RoleID: 1, Permission: domain.PermissionPlanningSKUEdit}},
	}}

	updated, appErr := svc.Update(context.Background(), actor, 42, 7, domain.UpdatePlanningSKURequest{
		ExpectedVersion: 3, Reason: "调整数量", DescriptionSpec: "新规格", Quantity: 2,
	})
	if appErr != nil {
		t.Fatalf("Update() error = %+v", appErr)
	}
	if updated == nil || updated.ID != 11 || !repository.reindexed {
		t.Fatalf("updated=%+v reindexed=%v", updated, repository.reindexed)
	}
	if events.eventType != domain.TaskEventPlanningSKUCorrected || events.operatorID != 99 {
		t.Fatalf("event type/operator = %q/%d", events.eventType, events.operatorID)
	}
	if events.payload["previous_revision"] != int64(10) || events.payload["current_revision"] != int64(11) || events.payload["reason"] != "调整数量" {
		t.Fatalf("event payload = %+v", events.payload)
	}
}
