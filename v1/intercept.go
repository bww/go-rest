package rest

import (
	"github.com/bww/go-router/v1"
)

type Interceptor interface {
	Intercept(*router.Request, *router.Response) (*router.Response, error)
}

type InterceptorFunc func(*router.Request, *router.Response) (*router.Response, error)

func (f InterceptorFunc) Intercept(req *router.Request, rsp *router.Response) (*router.Response, error) {
	return f(req, rsp)
}
