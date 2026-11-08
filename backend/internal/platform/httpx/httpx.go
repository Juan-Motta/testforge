// Package httpx holds JSON helpers and generic middleware for the HTTP layer.
package httpx

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"runtime/debug"
	"time"

	"testforge/internal/platform/apperr"
)

const maxBodyBytes = 1 << 20

var statusByKind = map[apperr.Kind]int{
	apperr.KindUnauthorized: http.StatusUnauthorized,
	apperr.KindForbidden:    http.StatusForbidden,
	apperr.KindNotFound:     http.StatusNotFound,
	apperr.KindConflict:     http.StatusConflict,
	apperr.KindInvalid:      http.StatusUnprocessableEntity,
	apperr.KindRateLimited:  http.StatusTooManyRequests,
	apperr.KindUpstream:     http.StatusBadGateway,
	apperr.KindInternal:     http.StatusInternalServerError,
}

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    apperr.Kind       `json:"code"`
	Message string            `json:"message"`
	Fields  map[string]string `json:"fields,omitempty"`
}

// JSON writes v as a JSON response with the given status.
func JSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

// NoContent writes a 204 response.
func NoContent(w http.ResponseWriter) { w.WriteHeader(http.StatusNoContent) }

// Error writes err using the public error shape. Unexpected errors are logged
// and reported as a generic internal error.
func Error(w http.ResponseWriter, r *http.Request, err error) {
	appErr, ok := apperr.As(err)
	if !ok {
		slog.ErrorContext(r.Context(), "request failed", "method", r.Method, "path", r.URL.Path, "err", err)
		appErr = &apperr.Error{Kind: apperr.KindInternal, Message: "Something went wrong."}
	}
	status, known := statusByKind[appErr.Kind]
	if !known {
		status = http.StatusInternalServerError
	}
	JSON(w, status, errorBody{Error: errorPayload{Code: appErr.Kind, Message: appErr.Message, Fields: appErr.Fields}})
}

// Decode reads a JSON body into dst, limited to 1 MiB.
func Decode(r *http.Request, dst any) error {
	dec := json.NewDecoder(io.LimitReader(r.Body, maxBodyBytes))
	if err := dec.Decode(dst); err != nil {
		if errors.Is(err, io.EOF) {
			return &apperr.Error{Kind: apperr.KindInvalid, Message: "Request body is required."}
		}
		return &apperr.Error{Kind: apperr.KindInvalid, Message: "Request body is not valid JSON."}
	}
	return nil
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (s *statusRecorder) WriteHeader(code int) {
	s.status = code
	s.ResponseWriter.WriteHeader(code)
}

// Unwrap lets http.ResponseController reach the underlying writer (flush, deadlines).
func (s *statusRecorder) Unwrap() http.ResponseWriter { return s.ResponseWriter }

// Logging logs one line per request.
func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		next.ServeHTTP(rec, r)
		slog.InfoContext(r.Context(), "http", "method", r.Method, "path", r.URL.Path,
			"status", rec.status, "duration_ms", time.Since(start).Milliseconds())
	})
}

// Recover turns panics into 500 responses.
func Recover(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if v := recover(); v != nil {
				if v == http.ErrAbortHandler {
					panic(v)
				}
				slog.ErrorContext(r.Context(), "panic", "value", v, "stack", string(debug.Stack()))
				Error(w, r, errors.New("panic"))
			}
		}()
		next.ServeHTTP(w, r)
	})
}
