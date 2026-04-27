package service

import (
	"context"
	"fmt"
	"strings"
	"time"

	"workflow/domain"
	"workflow/repo"
)

// CreateOutsourceParams carries all fields needed to create an OutsourceOrder.
type CreateOutsourceParams struct {
	TaskID              int64
	OperatorID          int64
	VendorName          string
	OutsourceType       string
	DeliveryRequirement string
	SettlementNote      string
}

// OutsourceFilter for list queries.
type OutsourceFilter struct {
	TaskID   *int64
	Status   string
	Vendor   string
	Page     int
	PageSize int
}

type ReturnOutsourceParams struct {
	OrderID    int64
	OperatorID int64
	Remark     string
}

type ReviewOutsourceParams struct {
	OrderID    int64
	ReviewerID int64
	Result     string
	Comment    string
	IssueTypes []string
}

// OutsourceService manages outsource order lifecycle (V7 §6.2).
type OutsourceService interface {
	Create(ctx context.Context, p CreateOutsourceParams) (*domain.OutsourceOrder, *domain.AppError)
	List(ctx context.Context, filter OutsourceFilter) ([]*domain.OutsourceOrder, domain.PaginationMeta, *domain.AppError)
	GetByID(ctx context.Context, id int64) (*domain.OutsourceOrder, *domain.AppError)
	Return(ctx context.Context, p ReturnOutsourceParams) (*domain.OutsourceOrder, *domain.AppError)
	Review(ctx context.Context, p ReviewOutsourceParams) (*domain.OutsourceOrder, *domain.AppError)
}

type outsourceService struct {
	outsourceRepo repo.OutsourceRepo
	taskRepo      repo.TaskRepo
	auditV7Repo   repo.AuditV7Repo
	taskEventRepo repo.TaskEventRepo
	codeRuleSvc   CodeRuleService
	txRunner      repo.TxRunner
}

func NewOutsourceService(
	outsourceRepo repo.OutsourceRepo,
	taskRepo repo.TaskRepo,
	auditV7Repo repo.AuditV7Repo,
	taskEventRepo repo.TaskEventRepo,
	codeRuleSvc CodeRuleService,
	txRunner repo.TxRunner,
) OutsourceService {
	return &outsourceService{
		outsourceRepo: outsourceRepo,
		taskRepo:      taskRepo,
		auditV7Repo:   auditV7Repo,
		taskEventRepo: taskEventRepo,
		codeRuleSvc:   codeRuleSvc,
		txRunner:      txRunner,
	}
}

func (s *outsourceService) Create(ctx context.Context, p CreateOutsourceParams) (*domain.OutsourceOrder, *domain.AppError) {
	// Validate task exists and is in PendingOutsource state.
	task, err := s.taskRepo.GetByID(ctx, p.TaskID)
	if err != nil {
		return nil, infraError("get task for outsource", err)
	}
	if task == nil {
		return nil, domain.ErrNotFound
	}
	if task.TaskStatus != domain.TaskStatusPendingOutsource {
		return nil, domain.NewAppError(domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d is in status %q, must be PendingOutsource to create outsource order",
				p.TaskID, task.TaskStatus), nil)
	}

	// Generate outsource number.
	outsourceNo, appErr := s.codeRuleSvc.GenerateCode(ctx, domain.CodeRuleTypeOutsourceNo)
	if appErr != nil {
		return nil, appErr
	}

	order := &domain.OutsourceOrder{
		OutsourceNo:         outsourceNo,
		TaskID:              p.TaskID,
		VendorName:          p.VendorName,
		OutsourceType:       p.OutsourceType,
		DeliveryRequirement: p.DeliveryRequirement,
		SettlementNote:      p.SettlementNote,
		Status:              domain.OutsourceStatusCreated,
	}

	var newID int64
	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		id, err := s.outsourceRepo.Create(ctx, tx, order)
		if err != nil {
			return fmt.Errorf("create outsource_order: %w", err)
		}
		newID = id

		// Transition task to Outsourcing.
		if err := s.taskRepo.UpdateStatus(ctx, tx, p.TaskID, domain.TaskStatusOutsourcing); err != nil {
			return err
		}

		// Write task event.
		_, err = s.taskEventRepo.Append(ctx, tx, p.TaskID, domain.TaskEventOutsourceCreated, &p.OperatorID,
			map[string]interface{}{
				"outsource_no":   outsourceNo,
				"vendor_name":    p.VendorName,
				"outsource_type": p.OutsourceType,
			})
		return err
	})
	if txErr != nil {
		return nil, infraError("create outsource tx", txErr)
	}

	created, err := s.outsourceRepo.GetByID(ctx, newID)
	if err != nil || created == nil {
		return nil, infraError("re-read outsource order", err)
	}
	return created, nil
}

func (s *outsourceService) List(ctx context.Context, filter OutsourceFilter) ([]*domain.OutsourceOrder, domain.PaginationMeta, *domain.AppError) {
	f := repo.OutsourceListFilter{
		TaskID:   filter.TaskID,
		Vendor:   filter.Vendor,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	}
	if filter.Status != "" {
		st := domain.OutsourceStatus(filter.Status)
		f.Status = &st
	}
	orders, total, err := s.outsourceRepo.List(ctx, f)
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list outsource orders", err)
	}
	if orders == nil {
		orders = []*domain.OutsourceOrder{}
	}
	return orders, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *outsourceService) GetByID(ctx context.Context, id int64) (*domain.OutsourceOrder, *domain.AppError) {
	order, err := s.outsourceRepo.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("get outsource order", err)
	}
	if order == nil {
		return nil, domain.ErrNotFound
	}
	return order, nil
}

func (s *outsourceService) Return(ctx context.Context, p ReturnOutsourceParams) (*domain.OutsourceOrder, *domain.AppError) {
	order, task, appErr := s.getOrderWithTask(ctx, p.OrderID)
	if appErr != nil {
		return nil, appErr
	}
	if task.TaskStatus != domain.TaskStatusOutsourcing {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d is in status %q, must be Outsourcing to return outsource order", task.ID, task.TaskStatus),
			nil,
		)
	}
	if !canReturnOutsource(order.Status) {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("outsource order %d is in status %q and cannot be returned", order.ID, order.Status),
			nil,
		)
	}

	now := time.Now()
	fromOrderStatus := order.Status
	order.Status = domain.OutsourceStatusReturned
	order.ReturnedAt = &now

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.outsourceRepo.Update(ctx, tx, order); err != nil {
			return err
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, task.ID, domain.TaskStatusPendingOutsourceReview); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, task.ID, domain.TaskEventOutsourceReturned, &p.OperatorID,
			map[string]interface{}{
				"outsource_order_id": order.ID,
				"outsource_no":       order.OutsourceNo,
				"remark":             p.Remark,
				"from_order_status":  string(fromOrderStatus),
				"to_order_status":    string(domain.OutsourceStatusReturned),
				"from_task_status":   string(task.TaskStatus),
				"to_task_status":     string(domain.TaskStatusPendingOutsourceReview),
			},
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("return outsource tx", txErr)
	}

	return s.GetByID(ctx, p.OrderID)
}

func (s *outsourceService) Review(ctx context.Context, p ReviewOutsourceParams) (*domain.OutsourceOrder, *domain.AppError) {
	order, task, appErr := s.getOrderWithTask(ctx, p.OrderID)
	if appErr != nil {
		return nil, appErr
	}
	if task.TaskStatus != domain.TaskStatusPendingOutsourceReview {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("task %d is in status %q, must be PendingOutsourceReview to review outsource order", task.ID, task.TaskStatus),
			nil,
		)
	}
	if order.Status != domain.OutsourceStatusReturned {
		return nil, domain.NewAppError(
			domain.ErrCodeInvalidStateTransition,
			fmt.Sprintf("outsource order %d is in status %q, must be returned before review", order.ID, order.Status),
			nil,
		)
	}

	result := strings.ToLower(strings.TrimSpace(p.Result))
	var nextOrderStatus domain.OutsourceStatus
	var nextTaskStatus domain.TaskStatus
	var action domain.AuditActionType
	switch result {
	case "approved":
		nextOrderStatus = domain.OutsourceStatusApproved
		nextTaskStatus = domain.TaskStatusPendingWarehouseReceive
		action = domain.AuditActionTypeApprove
	case "rejected":
		if strings.TrimSpace(p.Comment) == "" {
			return nil, domain.NewAppError(domain.ErrCodeReasonRequired, "comment is required when review result is rejected", nil)
		}
		nextOrderStatus = domain.OutsourceStatusRejected
		nextTaskStatus = domain.TaskStatusOutsourcing
		action = domain.AuditActionTypeReject
	default:
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "result must be approved or rejected", nil)
	}

	fromOrderStatus := order.Status
	order.Status = nextOrderStatus

	txErr := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		if err := s.outsourceRepo.Update(ctx, tx, order); err != nil {
			return err
		}
		if _, err := s.auditV7Repo.CreateRecord(ctx, tx, &domain.AuditRecord{
			TaskID:         task.ID,
			Stage:          domain.AuditRecordStageOutsourceReview,
			Action:         action,
			AuditorID:      p.ReviewerID,
			IssueTypesJSON: issueTypesToJSON(p.IssueTypes),
			Comment:        p.Comment,
		}); err != nil {
			return fmt.Errorf("outsource review audit record: %w", err)
		}
		if err := s.taskRepo.UpdateStatus(ctx, tx, task.ID, nextTaskStatus); err != nil {
			return err
		}
		_, err := s.taskEventRepo.Append(ctx, tx, task.ID, domain.TaskEventOutsourceReviewed, &p.ReviewerID,
			map[string]interface{}{
				"outsource_order_id": order.ID,
				"outsource_no":       order.OutsourceNo,
				"result":             result,
				"comment":            p.Comment,
				"issue_types":        p.IssueTypes,
				"from_order_status":  string(fromOrderStatus),
				"to_order_status":    string(nextOrderStatus),
				"from_task_status":   string(task.TaskStatus),
				"to_task_status":     string(nextTaskStatus),
			},
		)
		return err
	})
	if txErr != nil {
		return nil, infraError("review outsource tx", txErr)
	}

	return s.GetByID(ctx, p.OrderID)
}

func (s *outsourceService) getOrderWithTask(ctx context.Context, orderID int64) (*domain.OutsourceOrder, *domain.Task, *domain.AppError) {
	order, err := s.outsourceRepo.GetByID(ctx, orderID)
	if err != nil {
		return nil, nil, infraError("get outsource order", err)
	}
	if order == nil {
		return nil, nil, domain.ErrNotFound
	}

	task, err := s.taskRepo.GetByID(ctx, order.TaskID)
	if err != nil {
		return nil, nil, infraError("get task for outsource order", err)
	}
	if task == nil {
		return nil, nil, domain.ErrNotFound
	}
	return order, task, nil
}

func canReturnOutsource(status domain.OutsourceStatus) bool {
	switch status {
	case domain.OutsourceStatusCreated,
		domain.OutsourceStatusPackaged,
		domain.OutsourceStatusSent,
		domain.OutsourceStatusInProduction,
		domain.OutsourceStatusRejected:
		return true
	default:
		return false
	}
}
