package service

import (
	"context"
	"strings"
	"testing"
	"time"

	"workflow/domain"
)

func TestERPBridgeUpsertProductWithCostRetryFirstMismatchThenMatched(t *testing.T) {
	expected := 3.3
	actual := 1.06
	attempt := 0
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostVerificationSleep = time.Sleep })

	result, upsertAttempts, appErr := erpBridgeUpsertProductWithCostRetry(context.Background(), func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		attempt++
		if attempt == 1 {
			return &domain.ERPProductUpsertResult{
				CostVerification: &domain.ERPCostVerificationResult{
					Status:       "mismatched",
					ExpectedCost: float64Ptr(expected),
					ActualCost:   float64Ptr(actual),
				},
			}, nil
		}
		return &domain.ERPProductUpsertResult{
			CostVerification: &domain.ERPCostVerificationResult{
				Status:       "matched",
				ExpectedCost: float64Ptr(expected),
				ActualCost:   float64Ptr(expected),
			},
		}, nil
	}, domain.ERPProductUpsertPayload{SKUID: "SKU-1", CostPrice: float64Ptr(expected)})
	if appErr != nil {
		t.Fatalf("unexpected appErr: %+v", appErr)
	}
	if upsertAttempts != 2 {
		t.Fatalf("upsertAttempts = %d, want 2", upsertAttempts)
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, upsertAttempts); failure != "" {
		t.Fatalf("failure message = %q, want empty after matched readback", failure)
	}
}

func TestERPBridgeUpsertProductWithCostRetryExhaustedMismatch(t *testing.T) {
	expected := 3.3
	actual := 1.06
	erpBridgeCostVerificationSleep = func(time.Duration) {}
	t.Cleanup(func() { erpBridgeCostVerificationSleep = time.Sleep })

	_, upsertAttempts, appErr := erpBridgeUpsertProductWithCostRetry(context.Background(), func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		return &domain.ERPProductUpsertResult{
			CostVerification: &domain.ERPCostVerificationResult{
				Status:       "mismatched",
				ExpectedCost: float64Ptr(expected),
				ActualCost:   float64Ptr(actual),
			},
		}, nil
	}, domain.ERPProductUpsertPayload{SKUID: "SKU-1", CostPrice: float64Ptr(expected)})
	if appErr != nil {
		t.Fatalf("unexpected appErr: %+v", appErr)
	}
	maxAttempts := 1 + len(erpBridgeCostVerificationRetryDelays)
	if upsertAttempts != maxAttempts {
		t.Fatalf("upsertAttempts = %d, want %d", upsertAttempts, maxAttempts)
	}
}

func TestERPBridgeCostVerificationFailureMessageIncludesRetryCount(t *testing.T) {
	expected := 3.3
	actual := 1.06
	result := &domain.ERPProductUpsertResult{
		CostVerification: &domain.ERPCostVerificationResult{
			Status:       "mismatched",
			ExpectedCost: float64Ptr(expected),
			ActualCost:   float64Ptr(actual),
		},
	}
	failure := erpBridgeCostVerificationFailureMessage(result, 1+len(erpBridgeCostVerificationRetryDelays))
	if !strings.Contains(failure, "3.3000") || !strings.Contains(failure, "1.0600") {
		t.Fatalf("failure = %q, want expected/actual costs", failure)
	}
	if !strings.Contains(failure, "系统成本覆盖重试 3 次后仍不一致") {
		t.Fatalf("failure = %q, want retry count", failure)
	}
}

func TestERPBridgeUpsertProductWithCostRetryMatchedOnFirstAttempt(t *testing.T) {
	expected := 5.69
	result, upsertAttempts, appErr := erpBridgeUpsertProductWithCostRetry(context.Background(), func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		return &domain.ERPProductUpsertResult{
			CostVerification: &domain.ERPCostVerificationResult{
				Status:       "matched",
				ExpectedCost: float64Ptr(expected),
				ActualCost:   float64Ptr(expected),
			},
		}, nil
	}, domain.ERPProductUpsertPayload{SKUID: "SKU-1", CostPrice: float64Ptr(expected)})
	if appErr != nil {
		t.Fatalf("unexpected appErr: %+v", appErr)
	}
	if upsertAttempts != 1 {
		t.Fatalf("upsertAttempts = %d, want 1", upsertAttempts)
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, upsertAttempts); failure != "" {
		t.Fatalf("failure = %q, want empty", failure)
	}
}

func TestERPBridgeUpsertProductWithCostRetryUnverifiedDoesNotRetry(t *testing.T) {
	attempts := 0
	erpBridgeCostVerificationSleep = func(time.Duration) {
		t.Fatal("should not sleep when readback is unverified")
	}
	t.Cleanup(func() { erpBridgeCostVerificationSleep = time.Sleep })

	result, upsertAttempts, appErr := erpBridgeUpsertProductWithCostRetry(context.Background(), func(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		attempts++
		return &domain.ERPProductUpsertResult{
			CostVerification: &domain.ERPCostVerificationResult{
				Status:  "unverified",
				Message: "ERP cost readback failed after upsert",
			},
		}, nil
	}, domain.ERPProductUpsertPayload{SKUID: "SKU-1", CostPrice: float64Ptr(1.2)})
	if appErr != nil {
		t.Fatalf("unexpected appErr: %+v", appErr)
	}
	if attempts != 1 {
		t.Fatalf("upsert calls = %d, want 1", attempts)
	}
	if upsertAttempts != 1 {
		t.Fatalf("upsertAttempts = %d, want 1", upsertAttempts)
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, upsertAttempts); failure == "" || !strings.Contains(failure, "ERP成本回查失败") {
		t.Fatalf("failure = %q, want explicit unverified failure", failure)
	}
}

func TestERPBridgeCostVerificationFailureMessageReadbackNotFound(t *testing.T) {
	result := &domain.ERPProductUpsertResult{
		CostVerification: &domain.ERPCostVerificationResult{
			Status:  "readback_not_found",
			Message: erpBridgeCostReadbackNotFoundExhaustedMessage,
		},
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, 1); failure != "" {
		t.Fatalf("failure = %q, want empty because readback_not_found is pending confirmation", failure)
	}
	want := "ERP已提交，等待系统回查确认"
	if got := erpBridgeCostVerificationPendingMessage(result); got != want {
		t.Fatalf("pending message = %q, want %q", got, want)
	}
}

func TestERPBridgeUpsertProductWithCostRetryZeroCostMatched(t *testing.T) {
	attempts := 0
	result, upsertAttempts, appErr := erpBridgeUpsertProductWithCostRetry(context.Background(), func(_ context.Context, payload domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, *domain.AppError) {
		attempts++
		if payload.CostPrice == nil {
			t.Fatal("zero cost payload should still send explicit 0 cost_price")
		}
		if *payload.CostPrice != 0 {
			t.Fatalf("cost_price = %.4f, want 0", *payload.CostPrice)
		}
		return &domain.ERPProductUpsertResult{
			CostVerification: &domain.ERPCostVerificationResult{
				Status:       "matched",
				ExpectedCost: float64Ptr(0),
				ActualCost:   float64Ptr(0),
			},
		}, nil
	}, domain.ERPProductUpsertPayload{SKUID: "SKU-0", CostPrice: erpCostPriceForFiling(float64Ptr(0))})
	if appErr != nil {
		t.Fatalf("unexpected appErr: %+v", appErr)
	}
	if attempts != 1 || upsertAttempts != 1 {
		t.Fatalf("attempts=%d upsertAttempts=%d, want single upsert", attempts, upsertAttempts)
	}
	if failure := erpBridgeCostVerificationFailureMessage(result, upsertAttempts); failure != "" {
		t.Fatalf("failure = %q, want empty for zero-cost match", failure)
	}
}
