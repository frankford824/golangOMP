package office

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"workflow/internal/oa/platform"
)

func TestShopBindingChangesPreserveBaselineAndFutureTerms(t *testing.T) {
	day := "2026-11-18"
	person := person{Status: "active"}
	baseline := map[string]bool{"original": true}
	shops := []BindingShop{{ID: "original", Selectable: true}, {ID: "replacement", Selectable: true}}
	state := bindingsView(person, baseline, "", shops, day)
	if state.Match != "missing_employee" || !state.Shops[0].Visible || state.Shops[1].Visible {
		t.Fatal(state)
	}
	if !applyBindingChange(&person, baseline, shopBindingChange{Kind: "move", From: "original", Shop: "replacement"}, day) {
		t.Fatal("move rejected")
	}
	state = bindingsView(person, baseline, "", shops, day)
	if !state.Shops[0].Baseline || state.Shops[0].Visible || state.Shops[0].Override != "revoked" || !state.Shops[1].Visible || state.Shops[1].Override != "granted" {
		t.Fatal(state)
	}
	if applyBindingChange(&person, baseline, shopBindingChange{Kind: "move", From: "original", Shop: "replacement"}, day) {
		t.Fatal("duplicate accepted")
	}
	if !applyBindingChange(&person, baseline, shopBindingChange{Kind: "unbind", From: "replacement"}, day) {
		t.Fatal("unbind rejected")
	}
	if !applyBindingChange(&person, baseline, shopBindingChange{Kind: "bind", Shop: "original"}, day) {
		t.Fatal("rebind rejected")
	}
	person.Terms = append(person.Terms, term{ShopID: "original", From: "2026-12-01"})
	if applyBindingChange(&person, baseline, shopBindingChange{Kind: "unbind", From: "original"}, day) {
		t.Fatal("change would mask future scheduled grant")
	}
}

func TestShopBindingHandlersRejectDelegatedPeoplePermission(t *testing.T) {
	app := office{}
	for verb, handler := range map[string]http.HandlerFunc{http.MethodGet: app.readShopBindings, http.MethodPost: app.changeShopBindings} {
		req := httptest.NewRequest(verb, "/oa-api/people/42/shop-bindings", nil)
		req = platform.WithActor(req, platform.Actor{ID: 11, Actions: map[string]bool{"people.manage": true}})
		res := httptest.NewRecorder()
		handler(res, req)
		if res.Code != 403 {
			t.Fatalf("%s = %d", verb, res.Code)
		}
	}
}
