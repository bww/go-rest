package rest

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bww/go-rest/v1/errors"
	"github.com/bww/go-router/v1"
	"github.com/bww/go-router/v1/entity"
	"github.com/sirupsen/logrus"
	"github.com/stretchr/testify/assert"
)

func mustEntity(t string, d []byte) entity.Entity {
	e, err := entity.NewBytes(t, d)
	if err != nil {
		panic(err)
	}
	return e
}

func mustReq(m, s string, e entity.Entity) *http.Request {
	var b io.Reader
	if e != nil {
		b = e.Data()
	}
	req, err := http.NewRequest(m, s, b)
	if err != nil {
		panic(err)
	}
	if e != nil {
		req.Header.Set("Content-Type", e.Type())
	}
	return req
}

func mustNewResponse(s int, m, e string) *router.Response {
	r, err := router.NewResponse(s).SetString(m, e)
	if err != nil {
		panic(err)
	}
	return r
}

func readAll(v io.Reader) string {
	data, err := io.ReadAll(v)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func TestService(t *testing.T) {
	testService(t, WithVerbose(true), WithDebug(true))
	testService(t, WithVerbose(true))
}

func testService(t *testing.T, opts ...Option) {
	handlerA := HandlerFunc(func(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
		rsp, err := next.Handle(req, cxt)
		rsp.Header.Add("X-Handler-A", "1")
		return rsp, err
	})
	handlerB := HandlerFunc(func(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
		rsp, err := next.Handle(req, cxt)
		rsp.Header.Add("X-Handler-B", "1")
		return rsp, err
	})

	funcA := func(*router.Request, router.Context) (*router.Response, error) {
		return router.NewResponse(http.StatusOK).SetString("text/plain", "A")
	}
	funcB := func(*router.Request, router.Context) (*router.Response, error) {
		return router.NewResponse(http.StatusOK).SetString("text/plain", "B")
	}
	funcC := func(*router.Request, router.Context) (*router.Response, error) {
		return router.NewResponse(http.StatusOK).SetString("binary/data", "10011010")
	}

	s, _ := New(append(opts, WithHandlers(handlerA, handlerB))...)
	s.Add("/a", funcA).Methods("GET")
	s.Add("/b", funcB).Methods("GET")
	s.Add("/c", funcC).Methods("GET", "POST")

	tests := []struct {
		Req *http.Request
		Rsp *router.Response
	}{
		{
			mustReq("GET", "/a", nil),
			mustNewResponse(http.StatusOK, "text/plain", "A").
				SetHeader("X-Handler-A", "1").
				SetHeader("X-Handler-B", "1"),
		},
		{
			mustReq("GET", "/b", nil),
			mustNewResponse(http.StatusOK, "text/plain", "B").
				SetHeader("X-Handler-A", "1").
				SetHeader("X-Handler-B", "1"),
		},
		{
			mustReq("GET", "/c", nil),
			mustNewResponse(http.StatusOK, "binary/data", "10011010").
				SetHeader("X-Handler-A", "1").
				SetHeader("X-Handler-B", "1"),
		},
		{
			mustReq("POST", "/c", mustEntity("text/plain", []byte("Hi"))),
			mustNewResponse(http.StatusOK, "binary/data", "10011010").
				SetHeader("X-Handler-A", "1").
				SetHeader("X-Handler-B", "1"),
		},
		{
			mustReq("POST", "/c", mustEntity("binary/data", []byte("10011010"))),
			mustNewResponse(http.StatusOK, "binary/data", "10011010").
				SetHeader("X-Handler-A", "1").
				SetHeader("X-Handler-B", "1"),
		},
		{
			mustReq("GET", "/missing", nil),
			mustNewResponse(http.StatusNotFound, "application/json", `{"message":"Not found"}`).
				SetHeader("X-Handler-A", "1").
				SetHeader("X-Handler-B", "1"),
		},
	}

	for _, e := range tests {
		rec := httptest.NewRecorder()
		s.ServeHTTP(rec, e.Req)
		rsp := rec.Result()
		assert.Equal(t, e.Rsp.Status, rsp.StatusCode)
		assert.Equal(t, e.Rsp.Header, rsp.Header)
		assert.Equal(t, readAll(e.Rsp.Entity), readAll(rsp.Body))
	}
}

func BenchmarkService(b *testing.B) {
	handlerA := HandlerFunc(func(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
		rsp, err := next.Handle(req, cxt)
		rsp.Header.Add("X-Handler-A", "1")
		return rsp, err
	})
	handlerB := HandlerFunc(func(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
		rsp, err := next.Handle(req, cxt)
		rsp.Header.Add("X-Handler-B", "1")
		return rsp, err
	})

	funcA := func(*router.Request, router.Context) (*router.Response, error) {
		return router.NewResponse(http.StatusOK).SetString("text/plain", "A")
	}
	funcB := func(*router.Request, router.Context) (*router.Response, error) {
		return router.NewResponse(http.StatusOK).SetString("text/plain", "B")
	}

	s, _ := New(WithHandlers(handlerA, handlerB))
	s.Add("/a", funcA).Methods("GET")
	s.Add("/b", funcB).Methods("GET")

	reqA := mustReq("GET", "/a", nil)
	reqB := mustReq("GET", "/b", nil)

	for n := 0; n < b.N; n++ {
		var rec *httptest.ResponseRecorder
		rec = httptest.NewRecorder()
		s.ServeHTTP(rec, reqA)
		rec = httptest.NewRecorder()
		s.ServeHTTP(rec, reqB)
	}
}

func TestUnwrapErrors(t *testing.T) {
	err4 := fmt.Errorf("Error #4")
	err3 := fmt.Errorf("Error #3: %w", err4)
	err2 := errors.Errorf(http.StatusBadRequest, "Error #2").SetCause(err3)
	err1 := errors.Errorf(http.StatusBadRequest, "Error #1").SetCause(err2)
	err0 := errors.Errorf(http.StatusBadRequest, "Error #0").SetCause(err1)

	entry := errlog(&logrus.Logger{}, err0)
	assert.Equal(t, logrus.Fields{
		"because":    "Error #1",
		"because #2": "Error #2",
		"because #3": "Error #3: Error #4", // err4 is not unwrapped because it is not an errors.Error
	}, entry.Data)
}

func TestUnwrapErrorsNoCause(t *testing.T) {
	err1 := errors.Errorf(http.StatusBadRequest, "Error #1")
	err0 := errors.Errorf(http.StatusBadRequest, "Error #0").SetCause(err1)

	entry := errlog(&logrus.Logger{}, err0)
	assert.Equal(t, logrus.Fields{
		"because": "Error #1", // err1 has no cause
	}, entry.Data)
}
