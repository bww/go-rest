package rest

import (
	"net/http"

	"github.com/bww/go-router/v1"
)

// Produce a successful 200 response, optionally with a payload, which will be marshaled to JSON
func Success(body interface{}) *router.Response {
	rsp := router.NewResponse(http.StatusOK)
	if body != nil {
		_, err := rsp.SetJSON(body)
		if err != nil {
			panic(err)
		}
	}
	return rsp
}

// Produce a 302/Found redirect response
func Redirect(dest string) *router.Response {
	return router.NewResponse(http.StatusFound).SetHeader("Location", dest)
}
