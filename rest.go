package rest

import (
	"bytes"
	syserrs "errors"
	"fmt"
	"io"
	"io/ioutil"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"runtime/debug"
	"time"

	"github.com/bww/go-rest/v2/errors"

	"github.com/bww/go-metrics/v1"
	"github.com/bww/go-router/v2"
	"github.com/bww/go-util/v1/ext"
	"github.com/bww/go-util/v1/text"
)

type Service struct {
	router.Router

	dflt    router.Handler
	log     *slog.Logger
	verbose bool
	debug   bool

	metrics        *metrics.Metrics
	requestSampler metrics.SamplerVec
}

func New(opts ...Option) (*Service, error) {
	conf, err := Config{}.WithOptions(opts)
	if err != nil {
		return nil, err
	}

	s := &Service{
		Router:  router.New(),
		dflt:    conf.Default,
		log:     ext.Coalesce(conf.Logger, slog.Default()),
		verbose: conf.Verbose,
		debug:   conf.Debug,
	}

	if conf.Metrics != nil {
		s.requestSampler = conf.Metrics.RegisterSamplerVec("rest_request", "Request sampler", []string{"status"})
	}
	return s, nil
}

func (s *Service) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	var rsp *router.Response
	var err error

	method, rcname := resource((*router.Request)(req))
	log := s.log.With("method", method, "resource", rcname)
	start := time.Now()
	defer func() {
		if err := recover(); err != nil {
			log.With("because", err).Error("PANIC")
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
		method, rcname := resource((*router.Request)(req))
		dump = &bytes.Buffer{}
		dump.WriteString(text.Indent(fmt.Sprintf("%s %s", method, rcname), "  > "))
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
				log.With("because", err).Error("Could not read request entity")
			} else {
				dump.WriteString(text.Indent(string(data.Bytes()), "  > "))
				dump.WriteString("\n")
			}
			req.Body = io.NopCloser(data)
		} else if mtype != "" {
			dump.WriteString(text.Indent("[binary data]", "  > "))
			dump.WriteString("\n")
		}
	}

	var (
		rrq *router.Request
		cxt router.Context
		hdl router.Handler
	)
	route, match, err := s.Router.Find((*router.Request)(req))
	if err != nil {
		log.With("because", err).Error("Error finding route")
		rrq = (*router.Request)(req)
		hdl = first(s.dflt, s.handle500)
	} else if route == nil {
		log.Error("Route not found")
		rrq = (*router.Request)(req)
		hdl = first(s.dflt, s.handle404)
	} else {
		rrq = (*router.Request)((*http.Request)(req).WithContext(router.NewMatchContext(req.Context(), match)))
		cxt = route.Context(match)
		hdl = s.handler(route)
	}

	rsp, err = hdl(rrq, cxt)
	if err != nil {
		log.With("because", err).Error("Handler failed")
		return
	}
	if rsp == nil {
		w.WriteHeader(http.StatusOK) // nil response is an empty 200
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
					log.With("because", err).Error("Could not read response entity")
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
				log.With("because", err).Error("Could not dump request")
			}
		}
		_, err := io.Copy(w, entity)
		if err != nil {
			log.With("because", err).Error("Could not write response entity")
		}
	}
}

func (s *Service) handler(route *router.Route) router.Handler {
	return router.Handler(func(req *router.Request, cxt router.Context) (*router.Response, error) {
		method, rcname := resource((*router.Request)(req))
		log := s.log.With("method", method, "resource", rcname)
		if s.verbose {
			log.Info(req.OriginAddr())
		}

		rsp, err := route.Handle(req, cxt)
		if err == nil { // short circuit on success; error handling follows
			return rsp, nil
		}

		errlog(log, err).Error(err.Error())

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

func (s *Service) handle404(req *router.Request, cxt router.Context) (*router.Response, error) {
	return err404.Response(), nil
}
func (s *Service) handle500(req *router.Request, cxt router.Context) (*router.Response, error) {
	return err500.Response(), nil
}

// Take the first valid handler
func first(h ...router.Handler) router.Handler {
	for _, e := range h {
		if e != nil {
			return e
		}
	}
	return nil
}

// Format a resource name
func resource(req *router.Request) (string, string) {
	r := req.URL.Path
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
	return req.Method, r
}

// Produce a logger for an error
func errlog(log *slog.Logger, err error) *slog.Logger {
	for n := 0; err != nil; n++ {
		var resterr *errors.Error
		if !syserrs.As(err, &resterr) {
			break
		}
		err = resterr.Unwrap()
		if err != nil {
			name := "because"
			if n > 0 {
				name = name + fmt.Sprintf(" #%d", n+1)
			}
			log = log.With(name, err.Error())
		}
	}

	return log
}
