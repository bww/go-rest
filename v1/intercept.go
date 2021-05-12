package rest

import (
	"github.com/bww/go-router/v1"
)

type Interceptor interface {
	Intercept(*router.Response) (*router.Response, error)
}

type InterceptorFunc func(*router.Response) (*router.Response, error)

func (f InterceptorFunc) Intercept(rsp *router.Response) (*router.Response, error) {
	return f(rsp)
}
