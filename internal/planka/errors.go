package planka

import (
	"encoding/json"
	"errors"
	"fmt"
)

// ErrorKind classifies a Planka API failure. It ports the PlankaError subclass
// hierarchy from common/errors.ts into a single typed error with a discriminant.
type ErrorKind string

const (
	KindAuth       ErrorKind = "auth"       // 401
	KindPermission ErrorKind = "permission" // 403
	KindNotFound   ErrorKind = "not_found"  // 404
	KindConflict   ErrorKind = "conflict"   // 409
	KindValidation ErrorKind = "validation" // 422
	KindRateLimit  ErrorKind = "rate_limit" // 429
	KindAPI        ErrorKind = "api"        // anything else
)

// APIError is a non-2xx response from Planka. The raw body is retained for
// diagnostics; Message is the server-supplied message when one could be parsed.
type APIError struct {
	Status  int
	Kind    ErrorKind
	Message string
	Body    string
}

func (e *APIError) Error() string {
	if e.Message != "" {
		return fmt.Sprintf("planka %s error (%d): %s", e.Kind, e.Status, e.Message)
	}
	return fmt.Sprintf("planka %s error (%d)", e.Kind, e.Status)
}

// classifyKind maps an HTTP status to its ErrorKind, mirroring createPlankaError.
func classifyKind(status int) ErrorKind {
	switch status {
	case 401:
		return KindAuth
	case 403:
		return KindPermission
	case 404:
		return KindNotFound
	case 409:
		return KindConflict
	case 422:
		return KindValidation
	case 429:
		return KindRateLimit
	default:
		return KindAPI
	}
}

// newAPIError builds an APIError from a non-2xx response, extracting the
// server's {"message": "..."} field when the body is JSON.
func newAPIError(status int, body []byte) *APIError {
	msg := extractMessage(body)
	return &APIError{
		Status:  status,
		Kind:    classifyKind(status),
		Message: msg,
		Body:    string(body),
	}
}

// extractMessage pulls a human-readable message out of a Planka error body.
// Planka returns either {"message": "..."} or {"code": "...", "message": "..."};
// fall back to empty so callers render a status-only error.
func extractMessage(body []byte) string {
	if len(body) == 0 {
		return ""
	}
	var env struct {
		Message string `json:"message"`
		Problem string `json:"problem"`
	}
	if err := json.Unmarshal(body, &env); err == nil {
		if env.Message != "" {
			return env.Message
		}
		if env.Problem != "" {
			return env.Problem
		}
	}
	return ""
}

// ErrTermsNotAccepted is returned when Planka 2.1.x gates login behind
// terms-of-service acceptance (HTTP 403 with {"step":"accept-terms"}). The
// message is actionable, mirroring authenticateAgent in common/utils.ts.
var ErrTermsNotAccepted = errors.New(
	"Planka requires terms-of-service acceptance for this account before an " +
		"access token can be issued. Accept terms once (in the Planka UI, or run " +
		"scripts/accept-planka-terms.sh <baseUrl> <email> <password>), then retry.",
)

// IsNotFound reports whether err is (or wraps) a 404 APIError. Composites use
// this to treat "no comments / no tasks" as empty rather than failure.
func IsNotFound(err error) bool {
	var apiErr *APIError
	return errors.As(err, &apiErr) && apiErr.Status == 404
}

// AsAPIError extracts an *APIError from err's chain, if present.
func AsAPIError(err error) (*APIError, bool) {
	var apiErr *APIError
	if errors.As(err, &apiErr) {
		return apiErr, true
	}
	return nil, false
}
