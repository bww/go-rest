package rest

import (
	"bytes"
	syserrs "errors"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
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

	var dump *bytes.Buffer
	if s.debug {
		dump = &bytes.Buffer{}
		dump.WriteString(text.Indent(resource((*router.Request)(req)), "  > "))
		dump.WriteString("\n")

		data := &bytes.Buffer{}
		req.Header.Write(data)
		dump.WriteString(text.Indent(string(data.Bytes()), "  > "))
		dump.WriteString("\n")

		mtype := req.Header.Get("Content-Type")
		if !isMimetypeBinary(mtype) {
			data := &bytes.Buffer{}
			_, err := io.Copy(data, req.Body)
			if err != nil {
				s.log.WithFields(logrus.Fields{"because": err}).Error("Could not read request entity")
			} else {
				dump.WriteString(text.Indent(string(data.Bytes()), "  > "))
				dump.WriteString("\n")
			}
			req.Body = ioutil.NopCloser(data)
		} else if mtype != "" {
			dump.WriteString(text.Indent("[binary data]", "  > "))
			dump.WriteString("\n")
		}
	}

	var (
		rrq *router.Request
		cxt router.Context
		hdl Handler
	)
	route, match, err := s.Router.Find((*router.Request)(req))
	if err != nil {
		s.log.WithFields(logrus.Fields{"because": err}).Error("Could not lookup route")
		rrq = (*router.Request)(req)
		hdl = HandlerFunc(s.handle500)
	} else if route == nil {
		s.log.Errorf("Not found: %s", resource((*router.Request)(req)))
		rrq = (*router.Request)(req)
		hdl = HandlerFunc(s.handle404)
	} else {
		rrq = (*router.Request)((*http.Request)(req).WithContext(router.NewMatchContext(req.Context(), match)))
		cxt = route.Context(match)
		hdl = s.handler(route)
	}
	if s.pline != nil {
		rsp, err = s.pline.With(hdl).Handle(rrq, cxt)
	} else {
		rsp, err = hdl.Handle(rrq, cxt, nil)
	}
	if err != nil {
		s.log.WithFields(logrus.Fields{"because": err}).Error("Handler failed")
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
		dump.WriteString(fmt.Sprintf("  *\n  < %d / %s\n", rsp.Status, http.StatusText(rsp.Status)))
	}

	if entity := rsp.Entity; entity != nil {
		defer entity.Close()
		if dump != nil {
			data := &bytes.Buffer{}
			rsp.Header.Write(data)
			dump.WriteString(text.Indent(string(data.Bytes()), "  < "))
			dump.WriteString("\n")

			mtype := rsp.Header.Get("Content-Type")
			if !isMimetypeBinary(mtype) {
				data := &bytes.Buffer{}
				_, err := io.Copy(data, entity)
				if err != nil {
					s.log.WithFields(logrus.Fields{"because": err}).Error("Could not read response entity")
				} else {
					dump.WriteString(text.Indent(string(data.Bytes()), "  < "))
					dump.WriteString("\n")
				}
				entity = ioutil.NopCloser(data)
			} else if mtype != "" {
				dump.WriteString(text.Indent("[binary data]", "  < "))
				dump.WriteString("\n")
			}

			_, err := io.Copy(os.Stdout, dump)
			if err != nil {
				s.log.WithFields(logrus.Fields{"because": err}).Errorf("Could not dump request")
			}
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

		elog := errlog(s.log, err)
		elog.Errorf("%s: %v", resource(req), err)

		var rsperr errors.Responder
		if syserrs.As(err, &rsperr) {
			return rsperr.Response(), nil
		} else {
			return rsp, err
		}
	})
}

var (
	err404 = errors.Errorf(http.StatusNotFound, "Not found")
	err500 = errors.Errorf(http.StatusInternalServerError, "Internal server error")
)

func (s *Service) handle404(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
	return err404.Response(), nil
}
func (s *Service) handle500(req *router.Request, cxt router.Context, next *Pipeline) (*router.Response, error) {
	return err500.Response(), nil
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

// Produce a logger for an error
func errlog(log *logrus.Logger, err error) *logrus.Entry {
	fields := make(logrus.Fields)

	for n := 0; err != nil; n++ {
		var resterr *errors.Error
		if !syserrs.As(err, &resterr) {
			break
		}
		cause := resterr.Unwrap()
		name := "because"
		if n > 0 {
			name = name + fmt.Sprintf("_%d", n)
		}
		fields[name] = cause.Error()
		err = cause
	}

	return log.WithFields(fields)
}
