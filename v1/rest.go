package rest

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"runtime/debug"
	"time"

	"github.com/bww/go-rest/v1/errors"

	"github.com/bww/go-metrics/v1"
	"github.com/bww/go-router/v1"
	"github.com/bww/go-util/v1/text"
	"github.com/sirupsen/logrus"
)

type Service struct {
	router.Router

	pline   *Pipeline
	log     *logrus.Logger
	verbose bool
	debug   bool

	metrics        *metrics.Metrics
	requestSampler metrics.SamplerVec
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
	if s.metrics != nil {
		s.requestSampler = s.metrics.RegisterSamplerVec("rest_request", "Request sampler", []string{"status"})
	}
	return s, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var rsp *router.Response
	var err error

	start := time.Now()
	defer func() {
		if err := recover(); err != nil {
			s.log.WithFields(logrus.Fields{"because": err}).Errorf("PANIC: %s\n", resource((*router.Request)(req)))
			fmt.Println(string(debug.Stack()))
			return
		}
		if rsp != nil {
			if s.requestSampler != nil {
				s.requestSampler.With(metrics.Tags{"status": fmt.Sprint(rsp.Status)}).Observe(float64(time.Since(start)))
			}
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

	route, match, err := s.Router.Find((*router.Request)(req))
	if err != nil {
		s.log.WithFields(logrus.Fields{"because": err}).Error("Could not lookup route")
		rsp = errors.Errorf(http.StatusInternalServerError, "Internal server error").Response()
	} else if route == nil {
		rsp = errors.Errorf(http.StatusNotFound, "Not found").Response()
	} else {
		req := (*router.Request)((*http.Request)(req).WithContext(router.NewMatchContext(req.Context(), match)))
		cxt := route.Context(match)
		hdl := s.handler(route)
		if s.pline != nil {
			rsp, err = s.pline.With(hdl).Handle(req, cxt)
		} else {
			rsp, err = hdl.Handle(req, cxt, nil)
		}
		if err != nil {
			s.log.WithFields(logrus.Fields{"because": err}).Error("Handler failed")
			return
		}
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
			s.log.WithFields(logrus.Fields{"because": err}).Errorf("Could not write response entity")
		}
	}
}

func (s *Service) handler(route *router.Route) Handler {
	return HandlerFunc(func(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
		if s.verbose {
			s.log.Info(resource(req))
		}
		if next.Len() > 0 {
			return next.Handle(req, cxt)
		}

		rsp, err := route.Handle(req, cxt)
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
	})
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
