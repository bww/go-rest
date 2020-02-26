package errors

import (
	"fmt"
	"net/http"

	"github.com/bww/go-router/v1"
	"github.com/bww/go-validate"
)

const fieldErrorsKey = "field_errors"

type Code string

type Error struct {
	Status  int                    `json:"-"`
	Code    Code                   `json:"code,omitempty"`
	Message string                 `json:"message,omitempty"`
	Cause   error                  `json:"-"`
	Detail  map[string]interface{} `json:"detail,omitempty"`
}

func New(s int, m string, c error) *Error {
	return &Error{
		Status:  s,
		Message: m,
		Cause:   c,
	}
}

// Conditionally wrap the parameter error in a REST Error. If the
// parameter error is already a REST Error, it is simply returned.
// Otherwise, a new REST Error is created with the provided status,
// message and the parameter as the cause.
//
// This is intended to be used in cases where the caller receives
// an error that may or may not be a REST Error and needs to report
// the result to the REST client. If the error is already a REST
// Error, that error should be reported, and if not, the underlying
// error should be wrapped in a more generic error.
func Wrap(s int, m string, e error) *Error {
	if c, ok := e.(*Error); ok {
		return c // the parameter is already an Error, just return it
	} else {
		return New(s, m, e)
	}
}

func Errorf(s int, f string, a ...interface{}) *Error {
	return &Error{
		Status:  s,
		Message: fmt.Sprintf(f, a...),
	}
}

func (e Error) Error() string {
	return e.Message
}

func (e Error) String() string {
	return fmt.Sprintf("%d %s: %v", e.Status, http.StatusText(e.Status), e.Message)
}

func (e *Error) SetCause(c error) *Error {
	e.Cause = c
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

func (e *Error) SetFieldErrors(errs validate.Errors) *Error {
	return e.SetDetail(map[string]interface{}{fieldErrorsKey: errs})
}

func (e *Error) Response() *router.Response {
	rsp, _ := router.NewResponse(e.Status).SetJSON(e)
	return rsp
}
