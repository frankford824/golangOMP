package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"

	"go.uber.org/zap"

	"workflow/domain"
)

func TestErpRemoteFailureAllowsLocalFallback(t *testing.T) {
	t.Parallel()
	if erpRemoteFailureAllowsLocalFallback(nil) {
		t.Fatal("nil err should not allow fallback")
	}
	if erpRemoteFailureAllowsLocalFallback(fmt.Errorf("%w", ErrERPRemoteOpenWebAuthRequired)) {
		t.Fatal("auth required should not allow fallback")
	}
	if erpRemoteFailureAllowsLocalFallback(&erpBridgeRemoteProductNotFoundError{QueryID: "x"}) {
		t.Fatal("remote not found should not allow fallback")
	}
	if erpRemoteFailureAllowsLocalFallback(&erpBridgeOpenWebError{Code: 100, Message: "biz"}) {
		t.Fatal("openweb business error should not allow fallback")
	}
	if !erpRemoteFailureAllowsLocalFallback(&erpBridgeHTTPError{StatusCode: http.StatusBadGateway, Retryable: true}) {
		t.Fatal("502 should allow fallback")
	}
	if !erpRemoteFailureAllowsLocalFallback(&erpBridgeRequestError{Timeout: true, Cause: errors.New("i/o timeout")}) {
		t.Fatal("timeout should allow fallback")
	}
	if erpRemoteFailureAllowsLocalFallback(&erpBridgeHTTPError{StatusCode: http.StatusNotFound}) {
		t.Fatal("404 should not allow fallback")
	}
	if erpRemoteFailureAllowsLocalFallback(fmt.Errorf("jst sku query business code 12: x")) {
		t.Fatal("jst business string should not allow fallback")
	}
	if erpRemoteFailureAllowsLocalFallback(fmt.Errorf("decode jst sku response: %w", errors.New("eof"))) {
		t.Fatal("decode error should not allow fallback")
	}
}

func TestClassifyERPRemoteErr(t *testing.T) {
	t.Parallel()
	if classifyERPRemoteErr(&erpBridgeRequestError{Timeout: true, Duration: time.Second, Cause: errors.New("x")}) != "request_timeout" {
		t.Fatal("expected request_timeout")
	}
}

func TestHybridERPBridgeUpsertDoesNotFallbackOnOpenWebBusinessError(t *testing.T) {
	t.Parallel()
	remoteErr := &erpBridgeOpenWebError{Code: 10015, Message: "字段 ShortName 必须是最大长度为 40 的字符串。"}
	remote := &hybridERPBridgeTestClient{upsertErr: remoteErr}
	local := &hybridERPBridgeTestClient{}
	client := NewHybridERPBridgeClient(local, remote, true, zap.NewNop())

	_, err := client.UpsertProduct(context.Background(), domain.ERPProductUpsertPayload{SKUID: "DZK000013", Name: "too long"})
	if err == nil {
		t.Fatal("UpsertProduct() expected remote business error")
	}
	if !errors.As(err, &remoteErr) {
		t.Fatalf("UpsertProduct() err = %v, want openweb error", err)
	}
	if local.upsertCalls != 0 {
		t.Fatalf("local fallback upsert calls = %d, want 0", local.upsertCalls)
	}
}

func TestHybridERPBridgeQueryCombineSKUsDoesNotFallbackToLocal(t *testing.T) {
	t.Parallel()
	remoteErr := &erpBridgeHTTPError{StatusCode: http.StatusBadGateway, Retryable: true}
	remote := &hybridERPBridgeTestClient{queryCombineErr: remoteErr}
	local := &hybridERPBridgeTestClient{
		queryCombineResult: &domain.JSTCombineSKUListResponse{
			Items: []domain.JSTCombineSKUItem{{ComboSKUCode: "COMBO-LOCAL"}},
		},
	}
	client := NewHybridERPBridgeClient(local, remote, true, zap.NewNop())

	_, err := client.QueryCombineSKUs(context.Background(), domain.JSTCombineSKUFilter{PageIndex: 1, PageSize: 50})
	if err == nil {
		t.Fatal("QueryCombineSKUs() expected remote error")
	}
	if !errors.As(err, &remoteErr) {
		t.Fatalf("QueryCombineSKUs() err = %v, want remote error", err)
	}
	if local.queryCombineCalls != 0 {
		t.Fatalf("local fallback combine calls = %d, want 0", local.queryCombineCalls)
	}
}

type hybridERPBridgeTestClient struct {
	upsertErr          error
	upsertCalls        int
	queryCombineErr    error
	queryCombineResult *domain.JSTCombineSKUListResponse
	queryCombineCalls  int
}

func (c *hybridERPBridgeTestClient) SearchProducts(context.Context, domain.ERPProductSearchFilter) (*domain.ERPProductListResponse, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) GetProductByID(context.Context, string) (*domain.ERPProduct, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) QueryCombineSKUs(context.Context, domain.JSTCombineSKUFilter) (*domain.JSTCombineSKUListResponse, error) {
	c.queryCombineCalls++
	if c.queryCombineErr != nil {
		return nil, c.queryCombineErr
	}
	if c.queryCombineResult != nil {
		return c.queryCombineResult, nil
	}
	return &domain.JSTCombineSKUListResponse{
		Items:      []domain.JSTCombineSKUItem{},
		Pagination: domain.PaginationMeta{Page: 1, PageSize: 50, Total: 0},
	}, nil
}

func (c *hybridERPBridgeTestClient) ListCategories(context.Context) ([]*domain.ERPCategory, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) ListSyncLogs(context.Context, domain.ERPSyncLogFilter) (*domain.ERPSyncLogListResponse, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) GetSyncLogByID(context.Context, string) (*domain.ERPSyncLog, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) UpsertProduct(context.Context, domain.ERPProductUpsertPayload) (*domain.ERPProductUpsertResult, error) {
	c.upsertCalls++
	if c.upsertErr != nil {
		return nil, c.upsertErr
	}
	return &domain.ERPProductUpsertResult{Status: "accepted"}, nil
}

func (c *hybridERPBridgeTestClient) UpdateItemStyle(context.Context, domain.ERPItemStyleUpdatePayload) (*domain.ERPItemStyleUpdateResult, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) ShelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) UnshelveProductsBatch(context.Context, domain.ERPProductBatchMutationPayload) (*domain.ERPProductBatchMutationResult, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) UpdateVirtualInventory(context.Context, domain.ERPVirtualInventoryUpdatePayload) (*domain.ERPVirtualInventoryUpdateResult, error) {
	return nil, nil
}

func (c *hybridERPBridgeTestClient) GetCompanyUsers(context.Context, domain.JSTUserListFilter) (*domain.JSTUserListResponse, error) {
	return nil, nil
}
