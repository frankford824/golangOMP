package handler

import "fake/domain"

type Context struct{}
type TaskHandler struct{}

func respondOK(c *Context, data any) {}

func (h *TaskHandler) Get(c *Context) {
	respondOK(c, domain.TaskReadModel{})
}
