package handler

type Context struct{}
type ItemHandler struct{}

func respondOK(c *Context, data any) {}

func (h *ItemHandler) Get(c *Context) {
	if true {
		respondOK(c, ItemDTO{ID: 1})
		return
	}
	respondOK(c, ErrorDTO{Code: "missing"})
}

type ItemDTO struct {
	ID int `json:"id"`
}

type ErrorDTO struct {
	Code string `json:"code"`
}
