package service

import (
	"context"
	"encoding/json"
	"strings"

	"workflow/domain"
	"workflow/repo"
)

// PolicyService manages system policies stored in DB (spec §7.2, §9.2).
// Policy changes are dangerous actions and MUST include a reason.
type PolicyService interface {
	ListAll(ctx context.Context) ([]*domain.SystemPolicy, *domain.AppError)
	Update(ctx context.Context, id int64, value, reason string) *domain.AppError
}

type policyService struct {
	policyRepo repo.PolicyRepo
}

func NewPolicyService(policyRepo repo.PolicyRepo) PolicyService {
	return &policyService{policyRepo: policyRepo}
}

func (s *policyService) ListAll(ctx context.Context) ([]*domain.SystemPolicy, *domain.AppError) {
	policies, err := s.policyRepo.ListAll(ctx)
	if err != nil {
		return nil, infraError("list policies", err)
	}
	return policies, nil
}

func (s *policyService) Update(ctx context.Context, id int64, value, reason string) *domain.AppError {
	if strings.TrimSpace(reason) == "" {
		return domain.ErrReasonRequired
	}
	if !json.Valid([]byte(value)) {
		return domain.NewAppError(domain.ErrCodeInvalidRequest, "policy value must be valid JSON", nil)
	}

	existing, err := s.policyRepo.GetByID(ctx, id)
	if err != nil {
		return infraError("get policy by id", err)
	}
	if existing == nil {
		return domain.ErrNotFound
	}

	if err = s.policyRepo.Upsert(ctx, &domain.SystemPolicy{
		Key:       existing.Key,
		Value:     value,
		UpdatedBy: callerFromCtx(ctx),
	}); err != nil {
		return infraError("update policy", err)
	}
	return nil
}
