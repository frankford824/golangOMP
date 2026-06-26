package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"log"
	"strings"
	"time"

	"workflow/domain"
)

const taskCreateIdempotencyTTL = 10 * time.Minute

type taskCreateIdempotencyReservation struct {
	clientCreateID string
	payloadHash    string
	started        bool
}

func (s *taskService) reserveTaskCreateIdempotency(ctx context.Context, p *CreateTaskParams) (*domain.Task, taskCreateIdempotencyReservation, *domain.AppError) {
	if s.taskCreateRequestRepo == nil || p == nil {
		return nil, taskCreateIdempotencyReservation{}, nil
	}
	p.ClientCreateID = strings.TrimSpace(p.ClientCreateID)
	if p.ClientCreateID == "" {
		return nil, taskCreateIdempotencyReservation{}, nil
	}
	p.CreatePayloadHash = strings.TrimSpace(p.CreatePayloadHash)
	if p.CreatePayloadHash == "" {
		p.CreatePayloadHash = computeTaskCreatePayloadHash(*p)
	}
	p.CreateRequestJSON = strings.TrimSpace(p.CreateRequestJSON)
	if p.CreateRequestJSON == "" {
		p.CreateRequestJSON = marshalTaskCreateRequestSnapshot(*p)
	}
	if recent, appErr := s.findRecentTaskCreateByPayloadHash(ctx, *p); appErr != nil || recent != nil {
		return recent, taskCreateIdempotencyReservation{}, appErr
	}

	record, state, err := s.taskCreateRequestRepo.Reserve(
		ctx,
		p.CreatorID,
		p.ClientCreateID,
		p.CreatePayloadHash,
		p.CreateRequestJSON,
		time.Now().Add(taskCreateIdempotencyTTL),
	)
	if err != nil {
		return nil, taskCreateIdempotencyReservation{}, infraError("reserve task create request", err)
	}

	switch state {
	case domain.TaskCreateRequestReserveReplay:
		if record == nil || record.TaskID == nil || *record.TaskID <= 0 {
			return nil, taskCreateIdempotencyReservation{}, domain.NewAppError(domain.ErrCodeConflict, "该创建请求已完成，但暂时无法定位任务，请刷新任务列表确认。", nil)
		}
		task, err := s.taskRepo.GetByID(ctx, *record.TaskID)
		if err != nil {
			return nil, taskCreateIdempotencyReservation{}, infraError("load idempotent task create result", err)
		}
		if task == nil {
			return nil, taskCreateIdempotencyReservation{}, domain.NewAppError(domain.ErrCodeConflict, "该创建请求记录异常，请刷新后重新提交。", map[string]interface{}{
				"client_create_id": p.ClientCreateID,
				"task_id":          *record.TaskID,
			})
		}
		return task, taskCreateIdempotencyReservation{}, nil
	case domain.TaskCreateRequestReserveInProgress:
		return nil, taskCreateIdempotencyReservation{}, domain.NewAppError(domain.ErrCodeConflict, "这次任务提交正在处理中，请稍后刷新任务列表或再次重试。", map[string]interface{}{
			"client_create_id": p.ClientCreateID,
		})
	case domain.TaskCreateRequestReservePayloadConflict:
		return nil, taskCreateIdempotencyReservation{}, domain.NewAppError(domain.ErrCodeConflict, "检测到同一次提交编号已用于另一份任务内容，请刷新页面后重新提交。", map[string]interface{}{
			"client_create_id": p.ClientCreateID,
		})
	default:
		return nil, taskCreateIdempotencyReservation{
			clientCreateID: p.ClientCreateID,
			payloadHash:    p.CreatePayloadHash,
			started:        true,
		}, nil
	}
}

func (s *taskService) findRecentTaskCreateByPayloadHash(ctx context.Context, p CreateTaskParams) (*domain.Task, *domain.AppError) {
	if s.taskCreateRequestRepo == nil || strings.TrimSpace(p.CreatePayloadHash) == "" {
		return nil, nil
	}
	record, err := s.taskCreateRequestRepo.FindRecentActiveByActorPayloadHash(ctx, p.CreatorID, p.CreatePayloadHash, time.Now().Add(-taskCreateIdempotencyTTL))
	if err != nil {
		return nil, infraError("find recent task create request", err)
	}
	if record == nil {
		return nil, nil
	}
	if record.Status == domain.TaskCreateRequestStatusInProgress && record.ExpiresAt != nil && record.ExpiresAt.After(time.Now()) {
		return nil, domain.NewAppError(domain.ErrCodeConflict, "检测到相同任务内容正在提交中，请稍后刷新任务列表或再次重试。", map[string]interface{}{
			"client_create_id": record.ClientCreateID,
		})
	}
	if record.Status == domain.TaskCreateRequestStatusSucceeded && record.TaskID != nil && *record.TaskID > 0 {
		task, err := s.taskRepo.GetByID(ctx, *record.TaskID)
		if err != nil {
			return nil, infraError("load recent task create result", err)
		}
		if task != nil {
			return task, nil
		}
	}
	return nil, nil
}

func (s *taskService) markTaskCreateIdempotencyFailed(reservation taskCreateIdempotencyReservation, actorID int64, message string) {
	if s.taskCreateRequestRepo == nil || !reservation.started {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := s.taskCreateRequestRepo.MarkFailed(ctx, actorID, reservation.clientCreateID, reservation.payloadHash, message); err != nil {
		log.Printf("task_create_idempotency_mark_failed_error actor_id=%d client_create_id=%s err=%v", actorID, reservation.clientCreateID, err)
	}
}

func computeTaskCreatePayloadHash(p CreateTaskParams) string {
	p.ClientCreateID = ""
	p.CreatePayloadHash = ""
	p.CreateRequestJSON = ""
	raw, err := json.Marshal(p)
	if err != nil {
		raw = []byte(strings.Join([]string{
			string(p.TaskType),
			string(p.SourceMode),
			string(p.BusinessLane),
			strings.TrimSpace(p.ProductNameSnapshot),
			strings.TrimSpace(p.ProductShortName),
			strings.TrimSpace(p.OwnerTeam),
			strings.TrimSpace(p.OwnerDepartment),
			strings.TrimSpace(p.OwnerOrgTeam),
		}, "|"))
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:])
}

func marshalTaskCreateRequestSnapshot(p CreateTaskParams) string {
	p.CreateRequestJSON = ""
	raw, err := json.Marshal(p)
	if err != nil {
		return "{}"
	}
	return string(raw)
}
