//go:build integration

package search

import (
	"testing"

	"workflow/domain"
)

func TestSADI5_SearchInvalidQuery(t *testing.T) {
	_, svc := sadSearchDBSvc(t)
	ctx, cancel := sadCtx(t)
	defer cancel()
	if _, appErr := svc.Search(ctx, sadActor(50005, domain.RoleMember), "", "all", 20); appErr == nil || appErr.Code != CodeInvalidQuery {
		t.Fatalf("blank q appErr=%+v", appErr)
	}
	if _, appErr := svc.Search(ctx, sadActor(50005, domain.RoleMember), "x", "all", 20); appErr != nil {
		t.Fatalf("one-char q appErr=%+v", appErr)
	}
}
