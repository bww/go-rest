package rest

import (
	"github.com/bww/go-router/v1"
)

type Handler interface {
	Handle(*router.Request, router.Context, *Pipeline) (*router.Response, error)
}

type HandlerFunc func(*router.Request, router.Context, *Pipeline) (*router.Response, error)

func (f HandlerFunc) Handle(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
	return f(req, cxt, next)
}

type Pipeline struct {
	h []Handler
}

func newPipeline(h ...Handler) *Pipeline {
	return &Pipeline{h}
}

func (c *Pipeline) With(h ...Handler) *Pipeline {
	return &Pipeline{append(c.h, h...)}
}

func (c *Pipeline) Len() int {
	if c != nil {
		return len(c.h)
	} else {
		return 0
	}
}

func (c *Pipeline) Next() Handler {
	var n Handler
	if c != nil && len(c.h) > 0 {
		n, c.h = c.h[0], c.h[1:]
	}
	return n
}

func (c *Pipeline) Handle(req *router.Request, cxt router.Context) (*router.Response, error) {
	if n := c.Next(); n != nil {
		return n.Handle(req, cxt, c)
	} else {
		return nil, nil // end of chain
	}
}
