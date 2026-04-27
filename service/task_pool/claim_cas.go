package task_pool

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"workflow/domain"
	"workflow/repo"
	"workflow/service/permission"
)

type ClaimService struct {
	tasks           taskGetter
	modules         repo.TaskModuleRepo
	events          repo.TaskModuleEventRepo
	txRunner        repo.TxRunner
	authorizer      *permission.Authorizer
	notificationGen claimNotificationGenerator
	wsHub           claimWebSocketHub
}

type taskGetter interface {
	GetByID(ctx context.Context, id int64) (*domain.Task, error)
}

type claimNotificationGenerator interface {
	GenerateForEvent(ctx context.Context, tx repo.Tx, evt domain.TaskModuleEvent) error
}

type claimWebSocketHub interface {
	BroadcastPoolCountChanged(teamCode string, poolCount int)
}

type Option func(*ClaimService)

func WithNotificationGenerator(gen claimNotificationGenerator) Option {
	return func(s *ClaimService) { s.notificationGen = gen }
}

func WithWebSocketHub(hub claimWebSocketHub) Option {
	return func(s *ClaimService) { s.wsHub = hub }
}

func NewClaimService(tasks taskGetter, modules repo.TaskModuleRepo, events repo.TaskModuleEventRepo, txRunner repo.TxRunner, opts ...Option) *ClaimService {
	s := &ClaimService{tasks: tasks, modules: modules, events: events, txRunner: txRunner, authorizer: permission.NewAuthorizer(tasks, modules)}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

func (s *ClaimService) Claim(ctx context.Context, actor domain.RequestActor, taskID int64, moduleKey, confirmPoolTeamCode string) permission.Decision {
	task, err := s.tasks.GetByID(ctx, taskID)
	if err != nil {
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	if task == nil {
		return permission.Deny("task_not_found", "task not found")
	}
	tm, err := s.modules.GetByTaskAndKey(ctx, taskID, moduleKey)
	if err != nil {
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	if tm == nil {
		return permission.Deny(domain.DenyModuleNotInstantiated, "module is not instantiated")
	}
	if confirmPoolTeamCode == "" && tm.PoolTeamCode != nil {
		confirmPoolTeamCode = *tm.PoolTeamCode
	}
	claimedTeam := matchedTeam(actor, confirmPoolTeamCode)
	if claimedTeam == "" {
		return permission.Deny(domain.DenyModuleOutOfScope, "actor is not in pool team")
	}
	snapshot := actorSnapshot(actor)
	err = s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		ok, err := s.modules.ClaimCAS(ctx, tx, taskID, moduleKey, confirmPoolTeamCode, actor.ID, claimedTeam, snapshot)
		if err != nil {
			return err
		}
		if !ok {
			return claimConflictError{}
		}
		from := domain.ModuleStatePendingClaim
		to := domain.ModuleStateInProgress
		event := domain.TaskModuleEvent{
			TaskID:        taskID,
			TaskModuleID:  tm.ID,
			ModuleKey:     moduleKey,
			EventType:     domain.ModuleEventClaimed,
			FromState:     &from,
			ToState:       &to,
			ActorID:       &actor.ID,
			ActorSnapshot: snapshot,
			Payload:       mustJSON(map[string]interface{}{"claimed_team_code": claimedTeam}),
		}
		eventID, err := s.events.Insert(ctx, tx, &event)
		if err != nil {
			return err
		}
		event.ID = eventID
		if s.notificationGen != nil {
			_ = s.notificationGen.GenerateForEvent(ctx, tx, event)
		}
		return nil
	})
	if err != nil {
		if _, ok := err.(claimConflictError); ok {
			return permission.Deny(domain.DenyModuleClaimConflict, "module claim conflict")
		}
		return permission.Deny(domain.ErrCodeInternalError, err.Error())
	}
	if s.wsHub != nil {
		s.wsHub.BroadcastPoolCountChanged(claimedTeam, 0)
	}
	return permission.Allow()
}

type claimConflictError struct{}

func (claimConflictError) Error() string { return "module claim conflict" }

func matchedTeam(actor domain.RequestActor, pool string) string {
	pool = strings.TrimSpace(pool)
	if pool == "" {
		return ""
	}
	for _, team := range actorTeams(actor) {
		if strings.EqualFold(team, pool) {
			return team
		}
	}
	return ""
}

func actorSnapshot(actor domain.RequestActor) json.RawMessage {
	return mustJSON(map[string]interface{}{
		"user_id":    actor.ID,
		"username":   actor.Username,
		"department": actor.Department,
		"team":       actor.Team,
		"roles":      actor.Roles,
	})
}

func mustJSON(v interface{}) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(fmt.Sprintf("marshal json: %v", err))
	}
	return raw
}
