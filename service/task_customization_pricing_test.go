package service

import (
	"context"
	"testing"

	"workflow/domain"
)

type pricingTestUserReader struct {
	user *domain.User
}

func (r pricingTestUserReader) GetByID(_ context.Context, _ int64) (*domain.User, error) {
	return r.user, nil
}

type pricingTestRuleRepo struct {
	rule *domain.CustomizationPricingRule
}

func (r pricingTestRuleRepo) GetActiveByLevelAndEmploymentType(_ context.Context, _ string, _ domain.EmploymentType) (*domain.CustomizationPricingRule, error) {
	return r.rule, nil
}

func TestResolveCustomizationPricingSnapshotByEmploymentType(t *testing.T) {
	tests := []struct {
		name                 string
		employmentType       domain.EmploymentType
		expectedWorkerType   domain.EmploymentType
		expectedUnitPrice    float64
		expectedWeightFactor float64
	}{
		{
			name:                 "full time operator pricing",
			employmentType:       domain.EmploymentTypeFullTime,
			expectedWorkerType:   domain.EmploymentTypeFullTime,
			expectedUnitPrice:    19.9,
			expectedWeightFactor: 1.1,
		},
		{
			name:                 "part time operator pricing",
			employmentType:       domain.EmploymentTypePartTime,
			expectedWorkerType:   domain.EmploymentTypePartTime,
			expectedUnitPrice:    16.8,
			expectedWeightFactor: 0.9,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			svc := &taskService{
				customizationPricingUserRepo: pricingTestUserReader{
					user: &domain.User{ID: 7, EmploymentType: tc.employmentType},
				},
				customizationPricingRuleRepo: pricingTestRuleRepo{
					rule: &domain.CustomizationPricingRule{
						CustomizationLevelCode: "L1",
						EmploymentType:         tc.employmentType,
						UnitPrice:              tc.expectedUnitPrice,
						WeightFactor:           tc.expectedWeightFactor,
						IsEnabled:              true,
					},
				},
			}
			workerType, unitPrice, weightFactor, appErr := svc.resolveCustomizationPricingSnapshot(context.Background(), 7, "L1")
			if appErr != nil {
				t.Fatalf("resolveCustomizationPricingSnapshot() appErr = %+v", appErr)
			}
			if workerType != tc.expectedWorkerType {
				t.Fatalf("workerType = %q, want %q", workerType, tc.expectedWorkerType)
			}
			if unitPrice == nil || *unitPrice != tc.expectedUnitPrice {
				t.Fatalf("unitPrice = %v, want %v", unitPrice, tc.expectedUnitPrice)
			}
			if weightFactor == nil || *weightFactor != tc.expectedWeightFactor {
				t.Fatalf("weightFactor = %v, want %v", weightFactor, tc.expectedWeightFactor)
			}
		})
	}
}
