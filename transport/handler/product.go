package handler

import (
	"github.com/gin-gonic/gin"

	"workflow/domain"
	"workflow/service"
)

type ProductHandler struct {
	svc service.ProductService
}

func NewProductHandler(svc service.ProductService) *ProductHandler {
	return &ProductHandler{svc: svc}
}

// Search handles GET /v1/products/search?keyword=&category=&page=&page_size=
func (h *ProductHandler) Search(c *gin.Context) {
	filter := service.ProductFilter{
		Keyword:         c.Query("keyword"),
		Category:        c.Query("category"),
		CategoryCode:    c.Query("category_code"),
		SearchEntryCode: c.Query("search_entry_code"),
		MappingMatch:    domain.ProductMappingMatchMode(c.Query("mapping_match")),
		SecondaryKey:    c.Query("secondary_key"),
		SecondaryValue:  c.Query("secondary_value"),
		TertiaryKey:     c.Query("tertiary_key"),
		TertiaryValue:   c.Query("tertiary_value"),
	}
	if raw := c.Query("category_id"); raw != "" {
		value, err := parseInt64(raw)
		if err != nil {
			respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid category_id", nil))
			return
		}
		filter.CategoryID = &value
	}
	filter.Page, _ = parseInt(c.Query("page"))
	filter.PageSize, _ = parseInt(c.Query("page_size"))

	products, pagination, appErr := h.svc.Search(c.Request.Context(), filter)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOKWithPagination(c, products, pagination)
}

// GetByID handles GET /v1/products/:id
func (h *ProductHandler) GetByID(c *gin.Context) {
	id, err := parseID(c)
	if err != nil {
		respondError(c, domain.NewAppError(domain.ErrCodeInvalidRequest, "invalid id", nil))
		return
	}
	product, appErr := h.svc.GetByID(c.Request.Context(), id)
	if appErr != nil {
		respondError(c, appErr)
		return
	}
	respondOK(c, product)
}
