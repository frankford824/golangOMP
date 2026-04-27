package transport

import "fake/transport/handler"

type Router struct{}
type Group struct{}

func (r *Router) Group(path string) *Group { return &Group{} }
func (g *Group) GET(path string, h any)    {}

func NewRouter(itemsH *handler.ItemHandler) {
	r := &Router{}
	v1 := r.Group("/v1")
	v1.GET("/items/:id", itemsH.Get)
}
