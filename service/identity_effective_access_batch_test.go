package service

import (
	"context"
	"testing"

	"workflow/domain"
)

type identityEffectiveAccessBatchStub struct {
	singleCalls int
	batchCalls  int
}

func (s *identityEffectiveAccessBatchStub) EffectiveAccess(context.Context, int64) (*domain.EffectiveAccess, error) {
	s.singleCalls++
	return nil, nil
}

func (s *identityEffectiveAccessBatchStub) EffectiveAccessMany(_ context.Context, userIDs []int64) (map[int64]*domain.EffectiveAccess, error) {
	s.batchCalls++
	out := make(map[int64]*domain.EffectiveAccess, len(userIDs))
	for _, userID := range userIDs {
		out[userID] = &domain.EffectiveAccess{UserID: userID, Permissions: []domain.PermissionCode{domain.PermissionTaskView}}
	}
	return out, nil
}

func TestPrepareUsersForResponseUsesOneBatchEffectiveAccessRead(t *testing.T) {
	reader := &identityEffectiveAccessBatchStub{}
	service := &identityService{effectiveAccessReader: reader}
	users := []*domain.User{{ID: 1, Username: "one"}, {ID: 2, Username: "two"}}

	if err := service.prepareUsersForResponse(context.Background(), users); err != nil {
		t.Fatalf("prepareUsersForResponse() error = %v", err)
	}
	if reader.batchCalls != 1 || reader.singleCalls != 0 {
		t.Fatalf("effective access calls batch/single = %d/%d", reader.batchCalls, reader.singleCalls)
	}
	for _, user := range users {
		if !containsPermissionAction(user.FrontendAccess.Actions, string(domain.PermissionTaskView)) {
			t.Fatalf("user %d frontend actions = %+v", user.ID, user.FrontendAccess.Actions)
		}
	}
}

func containsPermissionAction(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}
