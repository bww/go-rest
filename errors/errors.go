package errors

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/bww/go-router/v2"
	"github.com/bww/go-validate/v1"
	"github.com/google/uuid"
)

const (
	helpKey        = "help"
	fieldErrorsKey = "field_errors"
)

func mkref() string {
	ref, err := uuid.NewRandom()
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("err-%v", ref)
}

type Code string

type Responder interface {
	error
	Response() *router.Response
}

type Error struct {
	Status  int                    `json:"-"`
	Code    Code                   `json:"code,omitempty"`
	Message string                 `json:"message,omitempty"`
	Cause   error                  `json:"-"`
	Detail  map[string]interface{} `json:"detail,omitempty"`
	Ref     string                 `json:"ref,omitempty"`
}

func New(s int, m string, c error) *Error {
	return &Error{
		Status:  s,
		Message: m,
		Cause:   c,
		Ref:     mkref(),
	}
}

// Conditionally wrap the parameter error in a REST Error. If the
// parameter error can be unwrapped as a REST Error, that error is
// simply returned. Otherwise, a new REST Error is created with the
// provided status, message and the parameter as the cause.
//
// This is intended to be used in cases where the caller receives
// an error that may or may not be a REST Error and needs to report
// the result to the REST client. If the error is already a REST
// Error, that error should be reported, and if not, the underlying
// error should be wrapped in a more generic error.
func Wrap(s int, m string, e error) *Error {
	var resterr *Error
	if errors.As(e, &resterr) {
		return resterr // the parameter is already an Error, just return it
	} else {
		return New(s, m, e)
	}
}

func Errorf(s int, f string, a ...interface{}) *Error {
	return &Error{
		Status:  s,
		Message: fmt.Sprintf(f, a...),
		Ref:     mkref(),
	}
}

func (e *Error) Copy() *Error {
	d := *e
	return &d
}

func (e *Error) Error() string {
	if e.Ref != "" {
		return fmt.Sprintf("%s (ref: %s)", e.Message, e.Ref)
	} else {
		return e.Message
	}
}

func (e *Error) Unwrap() error {
	return e.Cause
}

func (e *Error) Reference() string {
	return e.Ref
}

func (e *Error) String() string {
	return fmt.Sprintf("%d %s: %v", e.Status, http.StatusText(e.Status), e.Error())
}

func (e *Error) SetCause(c error) *Error {
	if c == e {
		panic("errors: Attempting to set error as its own cause; this will result in infinite recursion")
	}
	e.Cause = c
	return e
}

func (e *Error) SetStatus(v int) *Error {
	e.Status = v
	return e
}

func (e *Error) SetCode(c Code) *Error {
	e.Code = c
	return e
}

func (e *Error) SetDetail(d map[string]interface{}) *Error {
	e.Detail = d
	return e
}

func (e *Error) AddDetail(d map[string]interface{}) *Error {
	if e.Detail == nil {
		e.Detail = make(map[string]interface{})
	}
	for k, v := range d {
		e.Detail[k] = v
	}
	return e
}

func (e *Error) SetHelp(help string) *Error {
	return e.AddDetail(map[string]interface{}{
		helpKey: help,
	})
}

func (e *Error) SetFieldErrors(errs validate.Errors) *Error {
	return e.AddDetail(map[string]interface{}{
		fieldErrorsKey: errs,
	})
}

func (e *Error) Response() *router.Response {
	rsp, _ := router.NewResponse(e.Status).SetJSON(e)
	return rsp
}
