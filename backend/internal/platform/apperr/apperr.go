// Package apperr defines the application errors that cross layer boundaries.
// Services return them; the HTTP layer maps each Kind to a status code.
package apperr

import (
	"errors"
	"fmt"
)

// Kind classifies an error. Its string value is the public error code.
type Kind string

const (
	KindUnauthorized Kind = "unauthorized"
	KindForbidden    Kind = "forbidden"
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict"
	KindInvalid      Kind = "invalid"
	KindRateLimited  Kind = "rate_limited"
	KindUpstream     Kind = "upstream_error"
	KindInternal     Kind = "internal"
)

// Error is an expected, user-facing failure.
type Error struct {
	Kind    Kind
	Message string
	// Fields maps a request field to its validation message (KindInvalid only).
	Fields map[string]string
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Kind, e.Message) }

func Unauthorized(msg string) *Error { return &Error{Kind: KindUnauthorized, Message: msg} }
func Forbidden(msg string) *Error    { return &Error{Kind: KindForbidden, Message: msg} }
func NotFound(msg string) *Error     { return &Error{Kind: KindNotFound, Message: msg} }
func Conflict(msg string) *Error     { return &Error{Kind: KindConflict, Message: msg} }
func RateLimited(msg string) *Error  { return &Error{Kind: KindRateLimited, Message: msg} }

// Upstream reports that a model provider failed or answered something unusable.
func Upstream(msg string) *Error { return &Error{Kind: KindUpstream, Message: msg} }

// Invalid builds a validation error. It returns nil when fields is empty so
// callers can write `if err := apperr.Invalid(fields); err != nil`.
func Invalid(fields map[string]string) *Error {
	if len(fields) == 0 {
		return nil
	}
	return &Error{Kind: KindInvalid, Message: "Some fields are invalid.", Fields: fields}
}

// As extracts an *Error from err.
func As(err error) (*Error, bool) {
	var e *Error
	ok := errors.As(err, &e)
	return e, ok
}
