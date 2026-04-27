package handler

type Context struct{}
type TaskHandler struct{}

func respondOK(c *Context, data any) {}

func (h *TaskHandler) Get(c *Context) {
	respondOK(c, TaskDTO{ID: 1, Extra: "x"})
}

type TaskDTO struct {
	ID    int    `json:"id"`
	Extra string `json:"extra"`
}
