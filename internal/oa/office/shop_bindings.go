package office

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"
	"strings"
	"time"

	"workflow/internal/oa/platform"
)

type BindingShop struct {
	ID         string `json:"shop_id"`
	Name       string `json:"shop_name"`
	Platform   string `json:"platform"`
	Selectable bool   `json:"selectable"`
}

type ShopBindingDirectory func(context.Context, *int) (map[string]bool, string, []BindingShop, error)

type shopBinding struct {
	BindingShop
	Baseline bool   `json:"baseline"`
	Override string `json:"override,omitempty"`
	Visible  bool   `json:"visible"`
}

type shopBindingView struct {
	Editable bool          `json:"editable"`
	Version  int           `json:"version"`
	Match    string        `json:"match"`
	Shops    []shopBinding `json:"shops"`
}

type shopBindingChange struct {
	Kind    string `json:"kind"`
	Shop    string `json:"shop_id,omitempty"`
	From    string `json:"from_shop_id,omitempty"`
	Version *int   `json:"version"`
}

func bindingsView(p person, baseline map[string]bool, matched string, shops []BindingShop, day string) shopBindingView {
	out := shopBindingView{Editable: p.Status == "active" && p.Management != "super_admin", Version: p.Version, Match: "matched", Shops: []shopBinding{}}
	if p.EmployeeNo == nil {
		out.Match = "missing_employee"
	} else if matched == "" {
		out.Match = "unmatched"
	}
	overrides := shopOverrides(p.Terms, day)
	seen := map[string]bool{}
	appendShop := func(shop BindingShop) {
		local, has := overrides[shop.ID]
		b := shopBinding{BindingShop: shop, Baseline: baseline[shop.ID], Visible: baseline[shop.ID]}
		if has {
			b.Visible = local
			if local {
				b.Override = "granted"
			} else {
				b.Override = "revoked"
			}
		}
		if p.Management == "super_admin" && shop.Selectable {
			b.Visible = true
		}
		out.Shops = append(out.Shops, b)
		seen[shop.ID] = true
	}
	for _, shop := range shops {
		appendShop(shop)
	}
	for _, shop := range slices.Sorted(func(yield func(string) bool) {
		for key := range overrides {
			if !seen[key] && !yield(key) {
				return
			}
		}
	}) {
		appendShop(BindingShop{ID: shop})
	}
	return out
}

func (app office) bindingPerson(w http.ResponseWriter, r *http.Request) (person, bool) {
	actor := platform.ActorFrom(r)
	if !actor.SuperAdmin || actor.ID <= 0 {
		deny(w)
		return person{}, false
	}
	if app.bindings == nil || app.db == nil || app.source == nil {
		platform.Error(w, 503, "店铺绑定暂不可用")
		return person{}, false
	}
	p, err := app.sourcePerson(r, number(r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return person{}, false
	}
	if err != nil {
		internal(w)
		return person{}, false
	}
	roles, _, _, err := effectivePeopleAccess(r.Context(), app.source, p.ID)
	if err != nil {
		internal(w)
		return person{}, false
	}
	if slices.Contains(roles, "super_admin") {
		p.Management = "super_admin"
	}
	return p, true
}

func (app office) readShopBindings(w http.ResponseWriter, r *http.Request) {
	p, ok := app.bindingPerson(w, r)
	if !ok {
		return
	}
	p, err := mergePerson(p, app.db.QueryRowContext(r.Context(), "SELECT payload FROM oa_office_people WHERE id=?", p.ID))
	if err != nil {
		internal(w)
		return
	}
	baseline, matched, shops, err := app.bindings(r.Context(), p.EmployeeNo)
	if err != nil {
		platform.Error(w, 503, "暂时无法核对正式店铺资料")
		return
	}
	platform.JSON(w, 200, bindingsView(p, baseline, matched, shops, businessDate().Format(time.DateOnly)))
}

func (app office) changeShopBindings(w http.ResponseWriter, r *http.Request) {
	p, ok := app.bindingPerson(w, r)
	if !ok {
		return
	}
	if p.Status != "active" || p.Management == "super_admin" {
		platform.Error(w, 422, "只能调整在职非超级管理员的店铺范围")
		return
	}
	var input shopBindingChange
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 8192))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&input) != nil || decoder.Decode(&struct{}{}) != io.EOF || input.Version == nil || *input.Version < 0 {
		bad(w, "店铺变更格式无效")
		return
	}
	input.Shop = strings.TrimSpace(input.Shop)
	input.From = strings.TrimSpace(input.From)
	switch input.Kind {
	case "bind":
		if !textOK(input.Shop, 128) || input.From != "" {
			bad(w, "请选择要绑定的店铺")
			return
		}
	case "move":
		if !textOK(input.Shop, 128) || !textOK(input.From, 128) || input.Shop == input.From {
			bad(w, "请选择两个不同的店铺")
			return
		}
	case "unbind":
		if !textOK(input.From, 128) || input.Shop != "" {
			bad(w, "请选择要解除的店铺")
			return
		}
	default:
		bad(w, "店铺变更动作无效")
		return
	}
	baseline, matched, shops, err := app.bindings(r.Context(), p.EmployeeNo)
	if err != nil {
		platform.Error(w, 503, "暂时无法核对正式店铺资料")
		return
	}
	if input.Shop != "" && !slices.ContainsFunc(shops, func(s BindingShop) bool { return s.ID == input.Shop && s.Selectable }) {
		bad(w, "请选择正式电商店铺")
		return
	}
	tx, err := app.db.BeginTx(r.Context(), nil)
	if err != nil {
		internal(w)
		return
	}
	defer tx.Rollback()
	p, err = lockPerson(r, tx, p)
	if err != nil {
		internal(w)
		return
	}
	today := businessDate().Format(time.DateOnly)
	if p.Version != *input.Version || !applyBindingChange(&p, baseline, input, today) {
		conflict(w)
		return
	}
	p.Version++
	p.Events = append(p.Events, change{Kind: "shop_transfer", ActorID: platform.ActorFrom(r).ID, EffectiveDate: today, ShopID: input.Shop, FromShopID: input.From})
	if savePerson(r, tx, p) != nil || tx.Commit() != nil {
		internal(w)
		return
	}
	platform.JSON(w, 200, bindingsView(p, baseline, matched, shops, today))
}

func applyBindingChange(p *person, baseline map[string]bool, input shopBindingChange, day string) bool {
	overrides := shopOverrides(p.Terms, day)
	visible := func(shop string) bool {
		if local, found := overrides[shop]; found {
			return local
		}
		return baseline[shop]
	}
	if input.From != "" && !visible(input.From) || input.Shop != "" && visible(input.Shop) {
		return false
	}
	for _, term := range p.Terms {
		if (term.ShopID == input.From || term.ShopID == input.Shop) && (term.From > day || term.To > day) {
			return false
		}
	}
	if input.From != "" {
		for index := range p.Terms {
			item := &p.Terms[index]
			if item.ShopID == input.From && item.From <= day && item.To == "" {
				item.To = day
			}
		}
		// An explicit ended term masks the data-layer baseline from today onward.
		if _, found := overrides[input.From]; !found {
			p.Terms = append(p.Terms, term{ShopID: input.From, From: day, To: day})
		}
	}
	if input.Shop != "" {
		addTerm(p, input.Shop, day)
	}
	return true
}
