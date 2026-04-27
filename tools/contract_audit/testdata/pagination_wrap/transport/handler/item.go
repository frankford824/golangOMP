package handler

import "fake/domain"

type Context struct{}
type ItemHandler struct{}

func respondOKWithPagination(c *Context, data any, pagination any) {}

func (h *ItemHandler) List(c *Context) {
	respondOKWithPagination(c, []domain.Item{}, Pagination{})
}

type Pagination struct {
	Total int `json:"total"`
}
