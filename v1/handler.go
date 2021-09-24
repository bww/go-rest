package rest

import (
	"github.com/bww/go-router/v1"
)

type Handler interface {
	Handle(*router.Request, router.Context, Handler) (*router.Response, error)
}

type HandlerFunc func(*router.Request, router.Context, Handler) (*router.Response, error)

func (f HandlerFunc) Handle(req *router.Request, cxt router.Context, next Handler) (*router.Response, error) {
	return f(req, cxt, next)
}
