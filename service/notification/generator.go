package notification

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"workflow/domain"
	"workflow/repo"
)

type Generator struct {
	svc     *Service
	modules repo.ModuleNotificationRepo
	logger  *zap.Logger
}

func NewGenerator(notificationSvc *Service, modules repo.ModuleNotificationRepo, logger *zap.Logger) *Generator {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Generator{svc: notificationSvc, modules: modules, logger: logger}
}

func (g *Generator) GenerateForEvent(ctx context.Context, tx repo.Tx, evt domain.TaskModuleEvent) (err error) {
	defer func() {
		if r := recover(); r != nil {
			g.logger.Warn("notification generator recovered panic", zap.Any("panic", r))
			err = nil
		}
	}()
	if g == nil || g.svc == nil {
		return nil
	}
	candidates, err := g.candidates(ctx, tx, evt)
	if err != nil {
		g.logger.Warn("notification generator skipped event", zap.String("event_type", string(evt.EventType)), zap.Error(err))
		return nil
	}
	for _, c := range candidates {
		if c.UserID <= 0 || !c.Type.Valid() {
			g.logger.Warn("notification generator skipped invalid candidate", zap.Int64("user_id", c.UserID), zap.String("type", string(c.Type)))
			continue
		}
		if _, createErr := g.svc.CreateNotification(ctx, tx, c.UserID, c.Type, c.Payload); createErr != nil {
			g.logger.Warn("notification generator create failed", zap.Int64("user_id", c.UserID), zap.String("type", string(c.Type)), zap.Error(createErr))
		}
	}
	return nil
}

func (g *Generator) NotifyClaimConflict(ctx context.Context, tx repo.Tx, userID, taskID int64, moduleKey string) error {
	return g.GenerateForEvent(ctx, tx, domain.TaskModuleEvent{
		TaskID:    taskID,
		ModuleKey: moduleKey,
		EventType: domain.ModuleEventType("claim_conflict"),
		Payload:   mustRaw(map[string]interface{}{"task_id": taskID, "module_key": moduleKey, "user_id": userID}),
	})
}

func (g *Generator) candidates(ctx context.Context, tx repo.Tx, evt domain.TaskModuleEvent) ([]Candidate, error) {
	if evt.TaskID == 0 || evt.ModuleKey == "" {
		if g.modules == nil {
			return nil, nil
		}
		m, err := g.modules.GetTaskModuleByID(ctx, tx, evt.TaskModuleID)
		if err != nil {
			return nil, err
		}
		if m != nil {
			evt.TaskID = m.TaskID
			evt.ModuleKey = m.ModuleKey
		}
	}
	p := payloadMap(evt.Payload)
	actorID := int64(0)
	if evt.ActorID != nil {
		actorID = *evt.ActorID
	}
	switch evt.EventType {
	case domain.ModuleEventReassigned, domain.ModuleEventClaimed:
		userID := payloadInt64(p, "claimed_by_user_id", "claimed_by", "user_id")
		if userID == 0 && evt.EventType == domain.ModuleEventClaimed {
			userID = actorID
		}
		if userID == 0 {
			return nil, nil
		}
		return []Candidate{{UserID: userID, Type: domain.NotificationTypeTaskAssignedToMe, Payload: mustRaw(map[string]interface{}{
			"task_id": evt.TaskID, "module_key": evt.ModuleKey, "assigned_by": actorID, "reason": payloadString(p, "reason", "reassign_reason"),
		})}}, nil
	case domain.ModuleEventRejected:
		userID := payloadInt64(p, "claimed_by_user_id", "claimed_by")
		if userID == 0 {
			return nil, nil
		}
		return []Candidate{{UserID: userID, Type: domain.NotificationTypeTaskRejected, Payload: mustRaw(map[string]interface{}{
			"task_id": evt.TaskID, "reject_reason": payloadString(p, "reason", "reject_reason"),
		})}}, nil
	case domain.ModuleEventType("claim_conflict"):
		userID := payloadInt64(p, "user_id", "loser_user_id")
		if userID == 0 {
			return nil, nil
		}
		return []Candidate{{UserID: userID, Type: domain.NotificationTypeClaimConflict, Payload: mustRaw(map[string]interface{}{
			"task_id": evt.TaskID, "module_key": evt.ModuleKey,
		})}}, nil
	case domain.ModuleEventEntered, domain.ModuleEventPoolReassignedByAdmin:
		team := payloadString(p, "team_code", "pool_team_code", "target_team_code")
		if team == "" || g.modules == nil {
			return nil, nil
		}
		users, err := g.modules.ListActiveUserIDsByTeam(ctx, tx, team, evt.ActorID)
		if err != nil {
			return nil, err
		}
		out := make([]Candidate, 0, len(users))
		for _, userID := range users {
			out = append(out, Candidate{UserID: userID, Type: domain.NotificationTypePoolReassigned, Payload: mustRaw(map[string]interface{}{
				"task_id": evt.TaskID, "module_key": evt.ModuleKey,
			})})
		}
		return out, nil
	case domain.ModuleEventTaskCancelled:
		if g.modules == nil {
			return nil, nil
		}
		users, err := g.modules.ListClaimedUserIDsByTask(ctx, tx, evt.TaskID, evt.ActorID)
		if err != nil {
			return nil, err
		}
		out := make([]Candidate, 0, len(users))
		for _, userID := range users {
			out = append(out, Candidate{UserID: userID, Type: domain.NotificationTypeTaskCancelled, Payload: mustRaw(map[string]interface{}{
				"task_id": evt.TaskID, "cancel_reason": payloadString(p, "reason", "cancel_reason"), "cancelled_by": actorID,
			})})
		}
		return out, nil
	default:
		g.logger.Warn("notification generator unknown event", zap.String("event_type", fmt.Sprint(evt.EventType)))
		return nil, nil
	}
}
