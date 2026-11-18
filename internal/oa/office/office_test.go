package office

import (
	"net/http"
	"testing"
	"time"

	"workflow/internal/oa/platform"
)

func TestPersonnelProjectsByEffectiveDate(t *testing.T) {
	today := businessDate().Format(time.DateOnly)
	tomorrow := businessDate().AddDate(0, 0, 1).Format(time.DateOnly)
	p := person{ID: 2, Status: "active", DepartmentID: 1, Events: []change{
		{Kind: "roles", EffectiveDate: today, Roles: []string{"运营"}, Management: "department"},
		{Kind: "leave", EffectiveDate: today},
		{Kind: "rehire", EffectiveDate: tomorrow, DepartmentID: 2},
	}}
	p.project()
	if p.Status != "inactive" || p.Left != today || len(p.Roles) != 0 || p.Management != "none" || p.DepartmentID != 1 {
		t.Fatalf("unexpected effective person: %+v", p)
	}
}

func TestConfidentialAccountVisibility(t *testing.T) {
	doc := document{application: application{Account: "123456789"}, Applicant: 4, State: "approved", Route: []int64{6}}
	if doc.canRead(platform.Actor{ID: 5}) {
		t.Fatal("unrelated employee can read")
	}
	if !doc.canRead(platform.Actor{ID: 6}) {
		t.Fatal("assigned reviewer cannot read")
	}
	if doc.visible(platform.Actor{ID: 6}, true).Account != "•••• 6789" {
		t.Fatal("approver receives full account")
	}
	actor := platform.Actor{ID: 8, Actions: map[string]bool{"finance.pay": true}}
	if doc.visible(actor, false).Account != "•••• 6789" || doc.visible(actor, true).Account != "123456789" {
		t.Fatal("payer visibility must require an explicit reveal")
	}
	doc.State = "paid"
	if doc.visible(actor, true).Account != "•••• 6789" {
		t.Fatal("paid account can still be revealed")
	}
}

func TestApplicationEnforcesLockedTemplate(t *testing.T) {
	template := template{Channels: []string{"bank"}, Fields: []string{"contract"}, Required: []string{"contract"}}
	input := application{Amount: 2500, Reason: "合同", Channel: "bank", Holder: "供应商", Account: "123456", Bank: "开户行", Extra: map[string]string{"contract": "C-12"}, Files: []string{"proof"}, Site: "main"}
	if !validateApplication(input, template) {
		t.Fatal("valid template input rejected")
	}
	input.Extra["contract"] = ""
	if validateApplication(input, template) {
		t.Fatal("missing required field accepted")
	}
	input.Extra["contract"] = "C-12"
	input.Channel = "wallet"
	if validateApplication(input, template) {
		t.Fatal("unconfigured payment channel accepted")
	}
}

func TestRoutePatternsMountWithoutConflict(t *testing.T) {
	Mount(http.NewServeMux(), nil, nil)
}

func TestShopOverridesEffectivePerShop(t *testing.T) {
	terms := []term{
		{ShopID: "removed", From: "2026-11-18", To: "2026-11-18"},
		{ShopID: "later", From: "2026-11-19"},
		{ShopID: "reopened", From: "2026-11-17", To: "2026-11-18"},
		{ShopID: "reopened", From: "2026-11-18"},
	}
	got := shopOverrides(terms, "2026-11-18")
	if active, exists := got["removed"]; active || !exists {
		t.Fatalf("ended shop must explicitly override the roster: %v", got)
	}
	if _, exists := got["later"]; exists || !got["reopened"] {
		t.Fatalf("future and reopened shops projected incorrectly: %v", got)
	}
}
