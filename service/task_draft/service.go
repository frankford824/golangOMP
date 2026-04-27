package task_draft

import (
	"context"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

const (
	CodeDraftNotOwner = "draft_not_owner"
	defaultTaskType   = "unknown"
)

type Service struct {
	drafts repo.TaskDraftRepo
	logs   repo.PermissionLogRepo
	tx     repo.TxRunner
	now    func() time.Time
}

type ListDraftFilter struct {
	TaskType string
	Limit    int
	Cursor   string
}

func NewService(drafts repo.TaskDraftRepo, logs repo.PermissionLogRepo, tx repo.TxRunner) *Service {
	return &Service{drafts: drafts, logs: logs, tx: tx, now: time.Now}
}

func (s *Service) CreateOrUpdate(ctx context.Context, actor domain.RequestActor, raw json.RawMessage) (*domain.TaskDraft, *domain.AppError) {
	if s == nil || s.drafts == nil || s.tx == nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, "task draft service is not configured", nil)
	}
	if !json.Valid(raw) || len(raw) == 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "draft payload must be valid json", nil)
	}
	draftID, taskType := parseDraftPayload(raw)
	if taskType == "" {
		taskType = defaultTaskType
	}
	expiresAt := s.now().UTC().Add(7 * 24 * time.Hour)
	var saved *domain.TaskDraft
	err := s.tx.RunInTx(ctx, func(tx repo.Tx) error {
		if draftID > 0 {
			current, err := s.drafts.GetForUpdate(ctx, tx, draftID)
			if err != nil {
				return err
			}
			if current.OwnerUserID != actor.ID {
				return errDraftNotOwner{}
			}
			saved, err = s.drafts.Update(ctx, tx, &domain.TaskDraft{
				ID:          draftID,
				OwnerUserID: actor.ID,
				TaskType:    taskType,
				Payload:     cloneRaw(raw),
				ExpiresAt:   expiresAt,
			})
			return err
		}
		count, err := s.drafts.CountByOwnerAndType(ctx, tx, actor.ID, taskType)
		if err != nil {
			return err
		}
		if count >= 20 {
			if err := s.drafts.DeleteOldestByOwnerAndType(ctx, tx, actor.ID, taskType); err != nil {
				return err
			}
		}
		saved, err = s.drafts.Create(ctx, tx, &domain.TaskDraft{
			OwnerUserID: actor.ID,
			TaskType:    taskType,
			Payload:     cloneRaw(raw),
			ExpiresAt:   expiresAt,
		})
		return err
	})
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, domain.ErrNotFound
		}
		if _, ok := err.(errDraftNotOwner); ok {
			return nil, domain.NewAppError(CodeDraftNotOwner, "not the draft owner", nil)
		}
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return saved, nil
}

func (s *Service) List(ctx context.Context, actor domain.RequestActor, filter ListDraftFilter) ([]domain.TaskDraftListItem, string, *domain.AppError) {
	beforeTime, beforeID, appErr := decodeCursor(filter.Cursor)
	if appErr != nil {
		return nil, "", appErr
	}
	limit := filter.Limit
	if limit <= 0 || limit > 50 {
		limit = 20
	}
	items, err := s.drafts.List(ctx, repo.TaskDraftListFilter{
		OwnerUserID: actor.ID,
		TaskType:    strings.TrimSpace(filter.TaskType),
		Limit:       limit + 1,
		BeforeTime:  beforeTime,
		BeforeID:    beforeID,
	})
	if err != nil {
		return nil, "", domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	next := ""
	if len(items) > limit {
		last := items[limit-1]
		next = encodeCursor(last.UpdatedAt, last.ID)
		items = items[:limit]
	}
	return items, next, nil
}

func (s *Service) Get(ctx context.Context, actor domain.RequestActor, draftID int64) (*domain.TaskDraft, *domain.AppError) {
	draft, err := s.drafts.Get(ctx, draftID)
	if err == sql.ErrNoRows {
		return nil, domain.ErrNotFound
	}
	if err != nil {
		return nil, domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if draft.OwnerUserID != actor.ID {
		return nil, domain.NewAppError(CodeDraftNotOwner, "not the draft owner", nil)
	}
	return draft, nil
}

func (s *Service) Delete(ctx context.Context, actor domain.RequestActor, draftID int64) *domain.AppError {
	draft, err := s.drafts.Get(ctx, draftID)
	if err == sql.ErrNoRows {
		return domain.ErrNotFound
	}
	if err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	if draft.OwnerUserID != actor.ID {
		s.recordDenied(ctx, actor, draftID)
		return domain.NewAppError(CodeDraftNotOwner, "not the draft owner", nil)
	}
	if err := s.tx.RunInTx(ctx, func(tx repo.Tx) error { return s.drafts.Delete(ctx, tx, draftID) }); err != nil {
		return domain.NewAppError(domain.ErrCodeInternalError, err.Error(), nil)
	}
	return nil
}

func (s *Service) DeleteBySourceDraftID(ctx context.Context, tx repo.Tx, draftID int64) error {
	if draftID <= 0 {
		return nil
	}
	return s.drafts.Delete(ctx, tx, draftID)
}

func (s *Service) CleanupExpired(ctx context.Context) (int, error) {
	n, err := s.drafts.DeleteExpired(ctx, s.now().UTC())
	return int(n), err
}

func (s *Service) recordDenied(ctx context.Context, actor domain.RequestActor, draftID int64) {
	if s.logs == nil {
		return
	}
	now := s.now().UTC()
	_ = s.logs.Create(ctx, &domain.PermissionLog{
		ActorID:       &actor.ID,
		ActorUsername: actor.Username,
		ActorSource:   actor.Source,
		AuthMode:      actor.AuthMode,
		Readiness:     domain.APIReadinessReadyForFrontend,
		ActionType:    "draft_access_denied",
		ActorRoles:    actor.Roles,
		Method:        "DELETE",
		RoutePath:     "/v1/task-drafts/{draft_id}",
		Granted:       false,
		Reason:        fmt.Sprintf(`{"actor":%d,"draft_id":%d,"reason":"not_owner"}`, actor.ID, draftID),
		CreatedAt:     now,
	})
}

type errDraftNotOwner struct{}

func (errDraftNotOwner) Error() string { return CodeDraftNotOwner }

func parseDraftPayload(raw json.RawMessage) (int64, string) {
	var payload map[string]interface{}
	_ = json.Unmarshal(raw, &payload)
	var id int64
	switch v := payload["draft_id"].(type) {
	case float64:
		id = int64(v)
	case string:
		id, _ = strconv.ParseInt(strings.TrimSpace(v), 10, 64)
	}
	taskType, _ := payload["task_type"].(string)
	return id, strings.TrimSpace(taskType)
}

func cloneRaw(raw json.RawMessage) json.RawMessage {
	out := make([]byte, len(raw))
	copy(out, raw)
	return out
}

func encodeCursor(t time.Time, id int64) string {
	return base64.RawURLEncoding.EncodeToString([]byte(fmt.Sprintf("%d:%d", t.UnixMilli(), id)))
}

func decodeCursor(raw string) (*time.Time, int64, *domain.AppError) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, 0, nil
	}
	decoded, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cursor", nil)
	}
	parts := strings.SplitN(string(decoded), ":", 2)
	if len(parts) != 2 {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cursor", nil)
	}
	ms, err1 := strconv.ParseInt(parts[0], 10, 64)
	id, err2 := strconv.ParseInt(parts[1], 10, 64)
	if err1 != nil || err2 != nil || id <= 0 {
		return nil, 0, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid cursor", nil)
	}
	t := time.UnixMilli(ms).UTC()
	return &t, id, nil
}
