package transport

import "fake/transport/handler"

type Router struct{}
type Group struct{}

func (r *Router) Group(path string) *Group { return &Group{} }
func (g *Group) GET(path string, h any)    {}

func NewRouter(tasksH *handler.TaskHandler) {
	r := &Router{}
	v1 := r.Group("/v1")
	tasks := v1.Group("/tasks")
	tasks.GET("/:id", tasksH.Get)
}
