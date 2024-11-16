package response

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"net/http"
	"os"
	"strconv"

	resterrs "github.com/bww/go-rest/v2/errors"
	"github.com/bww/go-router/v2"
	"github.com/bww/go-router/v2/entity"
	lru "github.com/hashicorp/golang-lru/v2"
)

var tmplcache *lru.Cache[string, *template.Template]

func init() {
	var err error

	// determine the size of the template cache; this can be overridden via the environment
	n := 512
	if v := os.Getenv("GOREST_TEMPLATE_CACHE_COUNT"); v != "" {
		n, err = strconv.Atoi(v)
		if err != nil {
			panic(fmt.Errorf("GOREST_TEMPLATE_CACHE_COUNT: %w", err))
		}
	}

	// a length of zero (or less) disables the template cache entirely
	if n > 0 {
		tmplcache, err = lru.New[string, *template.Template](n)
		if err != nil {
			panic(fmt.Errorf("Could not initialize template LRU cache: %w", err))
		}
	}
}

// Take the first valid status
func statusOr(status, or int) int {
	if status > 0 {
		return status
	} else {
		return or
	}
}

// Produce a successful response, optionally including a payload, which will be
// marshaled to JSON. The status code used is 200 unless otherwise specified via
// an option.
//
// Deprecated: This method has been deprecated in favor of JSON(), which it
// simply calls.
func Success(body interface{}, opts ...Option) *router.Response {
	return JSON(body, opts...)
}

// Produce a 302/Found redirect response
func Redirect(dest string, opts ...Option) *router.Response {
	conf := Config{}.WithOptions(opts)
	rsp := router.NewResponse(statusOr(conf.Status, http.StatusFound))
	// start with the provided header, if any
	if len(conf.Header) > 0 {
		rsp.Header = conf.Header
	}
	// update the location header for the redirect
	rsp.SetHeader("Location", dest)
	return rsp
}

// Produce a successful response with a text entity. The status code used is 200
// unless otherwise specified via an option.
func Text(ctype, text string, opts ...Option) (*router.Response, error) {
	conf := Config{}.WithOptions(opts)
	rsp := router.NewResponse(statusOr(conf.Status, http.StatusOK))
	// start with the provided header, if any
	if len(conf.Header) > 0 {
		rsp.Header = conf.Header
	}
	// setting the body will update the content type
	if text != "" {
		ent, err := entity.NewString(ctype, text)
		if err != nil {
			return nil, resterrs.New(http.StatusInternalServerError, "Could not create text response entity", err)
		}
		_, err = rsp.SetEntity(ent)
		if err != nil {
			return nil, resterrs.New(http.StatusInternalServerError, "Could not set text response entity", err)
		}
	}
	return rsp, nil
}

// Produce a successful response, optionally including a payload, which will be
// marshaled to JSON. The status code used is 200 unless otherwise specified via
// an option.
func JSON(body interface{}, opts ...Option) *router.Response {
	conf := Config{}.WithOptions(opts)
	rsp := router.NewResponse(statusOr(conf.Status, http.StatusOK))
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

// Produce a successful 200 response with HTML entity content. The template
// string is expected to use the Go template (HTML variant) format, and it will
// be evaluated with the provided context value. The result of this evaluation
// is the response entity.
func HTML(fstr string, data interface{}, opts ...Option) (*router.Response, error) {
	rsp, _, err := renderHTML(fstr, data, tmplcache, opts...)
	return rsp, err
}

// Render an HTML response: this is decomposed in order to make caching more
// testable
func renderHTML(fstr string, data interface{}, cache *lru.Cache[string, *template.Template], opts ...Option) (*router.Response, bool, error) {
	conf := Config{}.WithOptions(opts)
	var err error
	var hit bool

	var tmpl *template.Template
	if cache != nil {
		tmpl, _ = cache.Get(fstr)
	}
	if tmpl != nil {
		hit = true
	} else {
		tmpl = template.New("_")
		if conf.Funcs != nil {
			tmpl.Funcs(conf.Funcs)
		}
		tmpl, err = tmpl.Parse(fstr)
		if err != nil {
			return nil, hit, resterrs.New(http.StatusInternalServerError, "Could not parse HTML template", err)
		}
	}
	if cache != nil {
		cache.Add(fstr, tmpl)
	}

	body := &bytes.Buffer{}
	err = tmpl.Execute(body, data)
	if err != nil {
		return nil, hit, resterrs.New(http.StatusInternalServerError, "Could not execute HTML template", err)
	}
	ent, err := entity.New("text/html", body)
	if err != nil {
		return nil, hit, resterrs.New(http.StatusInternalServerError, "Could not create HTML entity", err)
	}

	rsp := router.NewResponse(statusOr(conf.Status, http.StatusOK))
	// set explicit provided headers first, if any
	if len(conf.Header) > 0 {
		rsp.Header = conf.Header
	}
	// setting the body will update the content type header
	_, err = rsp.SetEntity(ent)
	if err != nil {
		return nil, hit, resterrs.New(http.StatusInternalServerError, "Could not set HTML response entity", err)
	}

	return rsp, hit, nil
}

// Produce a successful response with a byte entity. The status code used is 200
// unless otherwise specified via an option.
func Bytes(ctype string, data []byte, opts ...Option) (*router.Response, error) {
	conf := Config{}.WithOptions(opts)
	rsp := router.NewResponse(statusOr(conf.Status, http.StatusOK))
	// start with the provided header, if any
	if len(conf.Header) > 0 {
		rsp.Header = conf.Header
	}
	// setting the body will update the content type
	if len(data) > 0 {
		ent, err := entity.NewBytes(ctype, data)
		if err != nil {
			return nil, resterrs.New(http.StatusInternalServerError, "Could not create bytes response entity", err)
		}
		_, err = rsp.SetEntity(ent)
		if err != nil {
			return nil, resterrs.New(http.StatusInternalServerError, "Could not set bytes response entity", err)
		}
	}
	return rsp, nil
}

// Produce a successful response with a reader entity. The status code used is
// 200 unless otherwise specified via an option.
func Reader(ctype string, rdr io.Reader, opts ...Option) (*router.Response, error) {
	conf := Config{}.WithOptions(opts)
	rsp := router.NewResponse(statusOr(conf.Status, http.StatusOK))
	// start with the provided header, if any
	if len(conf.Header) > 0 {
		rsp.Header = conf.Header
	}
	// setting the body will update the content type
	if rdr != nil {
		ent, err := entity.New(ctype, rdr)
		if err != nil {
			return nil, resterrs.New(http.StatusInternalServerError, "Could not create stream response entity", err)
		}
		_, err = rsp.SetEntity(ent)
		if err != nil {
			return nil, resterrs.New(http.StatusInternalServerError, "Could not set stream response entity", err)
		}
	}
	return rsp, nil
}
