package service

import (
	"context"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

type CostRuleBindingFilter struct {
	Keyword   string
	RuleGroup string
	IsActive  *bool
	Page      int
	PageSize  int
}

type CreateCostRuleBindingParams struct {
	IIDRaw      string
	RuleGroup   string
	DisplayName string
	Source      string
	IsActive    *bool
	ActorID     int64
}

type PatchCostRuleBindingParams struct {
	ID          int64
	IIDRaw      *string
	RuleGroup   *string
	DisplayName *string
	Source      *string
	IsActive    *bool
	ActorID     int64
}

type UnboundCostRuleCandidateFilter struct {
	Keyword  string
	Limit    int
	Page     int
	PageSize int
}

type CostRuleBindingService interface {
	List(ctx context.Context, filter CostRuleBindingFilter) ([]*domain.CostRuleBinding, domain.PaginationMeta, *domain.AppError)
	Create(ctx context.Context, p CreateCostRuleBindingParams) (*domain.CostRuleBinding, *domain.AppError)
	Patch(ctx context.Context, p PatchCostRuleBindingParams) (*domain.CostRuleBinding, *domain.AppError)
	ListUnboundCandidates(ctx context.Context, filter UnboundCostRuleCandidateFilter) ([]*domain.UnboundCostRuleCandidate, domain.PaginationMeta, *domain.AppError)
}

type costRuleBindingService struct {
	bindings repo.CostRuleBindingRepo
	rules    repo.CostRuleRepo
	txRunner repo.TxRunner
}

func NewCostRuleBindingService(bindings repo.CostRuleBindingRepo, rules repo.CostRuleRepo, txRunner repo.TxRunner) CostRuleBindingService {
	return &costRuleBindingService{bindings: bindings, rules: rules, txRunner: txRunner}
}

func (s *costRuleBindingService) List(ctx context.Context, filter CostRuleBindingFilter) ([]*domain.CostRuleBinding, domain.PaginationMeta, *domain.AppError) {
	items, total, err := s.bindings.List(ctx, repo.CostRuleBindingListFilter{
		Keyword:   strings.TrimSpace(filter.Keyword),
		RuleGroup: strings.TrimSpace(filter.RuleGroup),
		IsActive:  filter.IsActive,
		Page:      filter.Page,
		PageSize:  filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list cost rule bindings", err)
	}
	return items, buildPaginationMeta(filter.Page, filter.PageSize, total), nil
}

func (s *costRuleBindingService) Create(ctx context.Context, p CreateCostRuleBindingParams) (*domain.CostRuleBinding, *domain.AppError) {
	iidRaw := strings.TrimSpace(p.IIDRaw)
	normalized := domain.NormalizeIID(iidRaw)
	if normalized == "" {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "i_id is required", map[string]string{"field": "i_id_raw"})
	}
	ruleGroup := strings.TrimSpace(p.RuleGroup)
	if appErr := s.validateRuleGroup(ctx, ruleGroup); appErr != nil {
		return nil, appErr
	}
	active := true
	if p.IsActive != nil {
		active = *p.IsActive
	}
	if active {
		existing, err := s.bindings.GetActiveByNormalizedIID(ctx, normalized)
		if err != nil {
			return nil, infraError("check active cost rule binding", err)
		}
		if existing != nil {
			return nil, domain.NewAppError(domain.ErrCodeConflict, "i_id already has an active cost rule binding", map[string]interface{}{
				"normalized_i_id": normalized,
				"binding_id":      existing.ID,
			})
		}
	}
	actorID := positiveActorID(p.ActorID)
	binding := &domain.CostRuleBinding{
		IIDRaw:        iidRaw,
		NormalizedIID: normalized,
		RuleGroup:     ruleGroup,
		DisplayName:   firstNonEmptyString(strings.TrimSpace(p.DisplayName), ruleGroup),
		Source:        firstNonEmptyString(strings.TrimSpace(p.Source), "manual"),
		IsActive:      active,
		CreatedBy:     actorID,
		UpdatedBy:     actorID,
	}
	var id int64
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		createdID, err := s.bindings.Create(ctx, tx, binding)
		id = createdID
		return err
	}); err != nil {
		return nil, infraError("create cost rule binding", err)
	}
	return s.getBindingFromList(ctx, id)
}

func (s *costRuleBindingService) Patch(ctx context.Context, p PatchCostRuleBindingParams) (*domain.CostRuleBinding, *domain.AppError) {
	if p.ID <= 0 {
		return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid binding id", nil)
	}
	if p.IIDRaw != nil {
		normalized := domain.NormalizeIID(*p.IIDRaw)
		if normalized == "" {
			return nil, domain.NewAppError(domain.ErrCodeInvalidRequest, "i_id is required", map[string]string{"field": "i_id_raw"})
		}
		if p.IsActive == nil || *p.IsActive {
			existing, err := s.bindings.GetActiveByNormalizedIID(ctx, normalized)
			if err != nil {
				return nil, infraError("check active cost rule binding", err)
			}
			if existing != nil && existing.ID != p.ID {
				return nil, domain.NewAppError(domain.ErrCodeConflict, "i_id already has an active cost rule binding", map[string]interface{}{
					"normalized_i_id": normalized,
					"binding_id":      existing.ID,
				})
			}
		}
	}
	if p.RuleGroup != nil {
		if appErr := s.validateRuleGroup(ctx, strings.TrimSpace(*p.RuleGroup)); appErr != nil {
			return nil, appErr
		}
	}
	current, err := s.bindings.GetByID(ctx, p.ID)
	if err != nil {
		return nil, infraError("get cost rule binding for patch", err)
	}
	if current == nil {
		return nil, domain.ErrNotFound
	}
	if p.IIDRaw != nil {
		current.IIDRaw = strings.TrimSpace(*p.IIDRaw)
		current.NormalizedIID = domain.NormalizeIID(current.IIDRaw)
	}
	if p.RuleGroup != nil {
		current.RuleGroup = strings.TrimSpace(*p.RuleGroup)
	}
	if p.DisplayName != nil {
		current.DisplayName = strings.TrimSpace(*p.DisplayName)
	}
	if p.Source != nil {
		current.Source = strings.TrimSpace(*p.Source)
	}
	if p.IsActive != nil {
		current.IsActive = *p.IsActive
	}
	current.UpdatedBy = positiveActorID(p.ActorID)
	if err := s.txRunner.RunInTx(ctx, func(tx repo.Tx) error {
		return s.bindings.Update(ctx, tx, current)
	}); err != nil {
		return nil, infraError("patch cost rule binding", err)
	}
	return s.getBindingFromList(ctx, p.ID)
}

func (s *costRuleBindingService) ListUnboundCandidates(ctx context.Context, filter UnboundCostRuleCandidateFilter) ([]*domain.UnboundCostRuleCandidate, domain.PaginationMeta, *domain.AppError) {
	items, total, err := s.bindings.ListUnboundCandidates(ctx, repo.UnboundCostRuleCandidateFilter{
		Keyword:  strings.TrimSpace(filter.Keyword),
		Limit:    filter.Limit,
		Page:     filter.Page,
		PageSize: filter.PageSize,
	})
	if err != nil {
		return nil, domain.PaginationMeta{}, infraError("list unbound cost rule candidates", err)
	}
	page, pageSize := filter.Page, filter.PageSize
	if filter.Limit > 0 && filter.Limit <= 100 {
		page, pageSize = 1, filter.Limit
	}
	return items, buildPaginationMeta(page, pageSize, total), nil
}

func (s *costRuleBindingService) validateRuleGroup(ctx context.Context, ruleGroup string) *domain.AppError {
	if ruleGroup == "" {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_group is required", map[string]string{"field": "rule_group"})
	}
	exists, err := s.bindings.RuleGroupExists(ctx, ruleGroup)
	if err != nil {
		return infraError("validate cost rule binding rule group", err)
	}
	if !exists {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "rule_group must reference an active cost rule category", map[string]string{"rule_group": ruleGroup})
	}
	return nil
}

func (s *costRuleBindingService) getBindingFromList(ctx context.Context, id int64) (*domain.CostRuleBinding, *domain.AppError) {
	item, err := s.bindings.GetByID(ctx, id)
	if err != nil {
		return nil, infraError("reload cost rule binding", err)
	}
	if item != nil {
		return item, nil
	}
	return nil, domain.ErrNotFound
}

func positiveActorID(actorID int64) *int64 {
	if actorID <= 0 {
		return nil
	}
	return &actorID
}
