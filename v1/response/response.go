package response

import (
	"net/http"

	"github.com/bww/go-router/v1"
)

// Produce a successful 200 response, optionally with a payload, which will be marshaled to JSON
func Success(body interface{}, opts ...Option) *router.Response {
	conf := Config{}.WithOptions(opts)
	rsp := router.NewResponse(http.StatusOK)
	// start with the provided header, if any
	if len(conf.Header) > 0 {
		rsp.Header = conf.Header
	}
	// setting the body will update the content type
	if body != nil {
		_, err := rsp.SetJSON(body)
		if err != nil {
			panic(err)
		}
	}
	return rsp
}

// Produce a 302/Found redirect response
func Redirect(dest string, opts ...Option) *router.Response {
	conf := Config{}.WithOptions(opts)
	rsp := router.NewResponse(http.StatusFound)
	// start with the provided header, if any
	if len(conf.Header) > 0 {
		rsp.Header = conf.Header
	}
	// update the location header for the redirect
	rsp.SetHeader("Location", dest)
	return rsp
}
