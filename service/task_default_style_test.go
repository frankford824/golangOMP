package service

import (
	"context"
	"testing"

	"workflow/domain"
)

func TestDefaultStyleUploadRequiresScopedAuditAndSessionOwner(t *testing.T) {
	ownerTeam, otherTeam := int64(11), int64(22)
	task := &domain.Task{ID: 41, TaskType: domain.TaskTypeSKUPlanning, TaskStatus: domain.TaskStatusCompleted, OwnerTeamID: &ownerTeam}
	auditor := globalCapabilityActor(7, domain.PermissionTaskAuditDecision)
	ctx := domain.WithRequestActor(context.Background(), auditor)
	if err := authorizeDefaultStyleUpload(ctx, task, 7); err != nil {
		t.Fatalf("completed planning upload: %v", err)
	}
	if err := authorizeDefaultStyleUpload(ctx, task, 8); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("other session owner: %v", err)
	}
	outsider := scopedCapabilityActor(7, domain.PermissionTaskAuditDecision, domain.AccessScopeOwnTeam, nil, &otherTeam, nil)
	if err := authorizeDefaultStyleUpload(domain.WithRequestActor(context.Background(), outsider), task, 7); err == nil || err.Code != domain.ErrCodePermissionDenied {
		t.Fatalf("other organization: %v", err)
	}
	task.TaskStatus = domain.TaskStatusArchived
	if err := authorizeDefaultStyleUpload(ctx, task, 7); err == nil || err.Code != domain.ErrCodeInvalidStateTransition {
		t.Fatalf("archived task: %v", err)
	}
}
