package rest

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime/debug"

	"github.com/bww/go-rest/v1/errors"

	"github.com/bww/go-router/v1"
	"github.com/bww/go-util/text"
	"github.com/sirupsen/logrus"
)

type Service struct {
	router.Router
	log     *logrus.Logger
	verbose bool
	debug   bool
}

func New(opts ...Option) (*Service, error) {
	s := &Service{
		Router: router.New(),
	}
	var err error
	for _, o := range opts {
		s, err = o(s)
		if err != nil {
			return nil, err
		}
	}
	if s.log == nil {
		s.log = logrus.StandardLogger()
	}
	return s, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	defer func() {
		if err := recover(); err != nil {
			s.log.WithFields(logrus.Fields{"because": err}).Errorf("PANIC: %s\n", resource((*router.Request)(req)))
			fmt.Println(string(debug.Stack()))
		}
	}()

	var reqdump string
	if s.debug && !isMimetypeBinary(req.Header.Get("Content-Type")) {
		data := &bytes.Buffer{}
		_, err := io.Copy(data, req.Body)
		if err != nil {
			s.log.WithFields(logrus.Fields{"because": err}).Error("Could not read request entity")
		} else {
			reqdump = text.Indent(string(data.Bytes()), "  > ")
		}
		req.Body = ioutil.NopCloser(data)
	}

	rsp, err := s.handle((*router.Request)(req))
	if err != nil {
		s.log.Errorf("Handler failed: %v", err)
		return
	}
	if rsp == nil {
		w.WriteHeader(http.StatusOK) // no response is an empty 200
		return
	}

	h := w.Header()
	for k, v := range rsp.Header {
		h[k] = v
	}

	w.WriteHeader(rsp.Status)
	if s.debug {
		fmt.Println(reqdump)
		fmt.Printf("  * %d / %s\n", rsp.Status, http.StatusText(rsp.Status))
	}

	if entity := rsp.Entity; entity != nil {
		defer entity.Close()
		if s.debug && !isMimetypeBinary(req.Header.Get("Content-Type")) {
			data := &bytes.Buffer{}
			_, err := io.Copy(data, entity)
			if err != nil {
				s.log.WithFields(logrus.Fields{"because": err}).Error("Could not read response entity")
			} else {
				fmt.Println(text.Indent(string(data.Bytes()), "  < "))
			}
			entity = ioutil.NopCloser(data)
		}
		_, err := io.Copy(w, entity)
		if err != nil {
			s.log.WithFields(logrus.Fields{"because": err}).Errorf("Could not write entity")
		}
	}
}

func (s *Service) handle(req *router.Request) (*router.Response, error) {
	if s.verbose {
		s.log.Info(resource(req))
	}

	rsp, err := s.Router.Handle(req)
	if err == nil {
		return rsp, nil
	}

	var cause error
	if c, ok := err.(*errors.Error); ok {
		cause = c.Cause
	}

	var elog *logrus.Entry
	if cause == nil {
		elog = s.log.WithFields(logrus.Fields{})
	} else {
		elog = s.log.WithFields(logrus.Fields{
			"because": cause.Error(),
		})
	}

	elog.Errorf("%s: %v", resource(req), err)
	if c, ok := err.(*errors.Error); ok {
		return c.Response(), nil
	} else {
		return rsp, err
	}
}

// Format a resource name
func resource(req *router.Request) string {
	r := fmt.Sprintf("%s %s", req.Method, req.URL.Path)
	var p string
	for k, v := range req.URL.Query() {
		for _, e := range v {
			if len(p) > 0 {
				p += "&"
			}
			p += url.QueryEscape(k)
			if len(e) > 0 {
				p += fmt.Sprintf("=%s", url.QueryEscape(e))
			}
		}
	}
	if len(p) > 0 {
		r += fmt.Sprintf("?%s", p)
	}
	return r
}
