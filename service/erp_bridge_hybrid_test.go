package service

import (
	"errors"
	"fmt"
	"net/http"
	"testing"
	"time"
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
