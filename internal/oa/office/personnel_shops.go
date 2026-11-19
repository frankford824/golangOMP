package office

import (
	"database/sql"
	"errors"
	"net/http"
	"slices"
	"strings"
	"time"

	"workflow/internal/oa/platform"
)

type termShop struct {
	BindingShop
	Bound    bool `json:"bound"`
	Handover bool `json:"handover"`
}

func (app office) termShopDirectory(r *http.Request, p person, day string) ([]termShop, error) {
	baseline, match, directory, err := app.bindings(r.Context(), p.EmployeeNo)
	if err != nil {
		return nil, err
	}
	bindings := bindingsView(p, baseline, match, directory, day)
	result := make([]termShop, 0, len(bindings.Shops))
	for _, item := range bindings.Shops {
		pending := slices.ContainsFunc(p.Terms, func(t term) bool {
			return t.ShopID == item.ID && t.From <= day && t.PendingHandover
		})
		result = append(result, termShop{BindingShop: item.BindingShop, Bound: item.Visible, Handover: pending})
	}
	return result, nil
}

func (app office) termShops(w http.ResponseWriter, r *http.Request) {
	p, err := app.readPerson(r, number(r.PathValue("id")))
	if errors.Is(err, sql.ErrNoRows) {
		missing(w)
		return
	}
	if err != nil {
		internal(w)
		return
	}
	if !scopePerson(platform.ActorFrom(r), p) || p.Management == "super_admin" {
		deny(w)
		return
	}
	day := strings.TrimSpace(r.URL.Query().Get("effective_date"))
	if day == "" {
		day = businessDate().Format(time.DateOnly)
	}
	if _, err := time.Parse(time.DateOnly, day); err != nil {
		bad(w, "生效日期无效")
		return
	}
	if app.bindings == nil {
		platform.Error(w, 503, "暂时无法核对正式店铺资料")
		return
	}
	items, err := app.termShopDirectory(r, p, day)
	if err != nil {
		platform.Error(w, 503, "暂时无法核对正式店铺资料")
		return
	}
	query := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("q")))
	choices, bound := []termShop{}, []termShop{}
	for _, item := range items {
		if item.Bound || item.Handover {
			bound = append(bound, item)
		}
		if item.Selectable && !item.Bound && query != "" && len(choices) < 40 &&
			strings.Contains(strings.ToLower(item.Name+" "+item.Platform+" "+item.ID), query) {
			choices = append(choices, item)
		}
	}
	platform.JSON(w, 200, map[string]any{"bound": bound, "items": choices})
}

func (app office) validateTermShopChange(w http.ResponseWriter, r *http.Request, p person, input change) bool {
	if app.bindings == nil {
		platform.Error(w, 503, "暂时无法核对正式店铺资料")
		return false
	}
	items, err := app.termShopDirectory(r, p, input.EffectiveDate)
	if err != nil {
		platform.Error(w, 503, "暂时无法核对正式店铺资料")
		return false
	}
	find := func(shop string) termShop {
		for _, item := range items {
			if item.ID == shop {
				return item
			}
		}
		return termShop{}
	}
	for _, shop := range input.FromShopIDs {
		if !find(shop).Bound || slices.Contains(input.ShopIDs, shop) {
			bad(w, "请选择有效的已绑定店铺")
			return false
		}
	}
	for _, shop := range input.ShopIDs {
		item := find(shop)
		if input.Kind == "handover" && !item.Bound && !item.Handover || input.Kind != "handover" && (!item.Selectable || item.Bound) {
			bad(w, "请选择有效的正式店铺")
			return false
		}
	}
	for _, item := range p.Terms {
		if (slices.Contains(input.FromShopIDs, item.ShopID) || slices.Contains(input.ShopIDs, item.ShopID)) &&
			(item.From > input.EffectiveDate || item.To > input.EffectiveDate && !item.PendingHandover) {
			conflict(w)
			return false
		}
	}
	return true
}
