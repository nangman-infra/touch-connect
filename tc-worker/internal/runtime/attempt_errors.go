package runtime

import (
	"errors"
	"net/http"

	"github.com/nangman-infra/touch-connect/internal/communication/contracts"
)

type AttemptDrop struct {
	Code       string
	StatusCode int
	Err        error
}

func (d AttemptDrop) Error() string {
	if d.Code != "" {
		return d.Code
	}
	if d.Err != nil {
		return d.Err.Error()
	}
	return "attempt_dropped"
}

func (d AttemptDrop) Unwrap() error {
	return d.Err
}

func recoverableAttemptDrop(err error) (AttemptDrop, bool) {
	if err == nil {
		return AttemptDrop{}, false
	}
	var drop AttemptDrop
	if errors.As(err, &drop) {
		return drop, true
	}
	var apiErr contracts.APIError
	if !errors.As(err, &apiErr) {
		return AttemptDrop{}, false
	}
	if recoverableAttemptStatus(apiErr.StatusCode) || recoverableAttemptCode(apiErr.Response.Code) {
		code := apiErr.Response.Code
		if code == "" {
			code = http.StatusText(apiErr.StatusCode)
		}
		return AttemptDrop{Code: code, StatusCode: apiErr.StatusCode, Err: err}, true
	}
	return AttemptDrop{}, false
}

func recoverableAttemptStatus(status int) bool {
	switch status {
	case http.StatusUnauthorized, http.StatusForbidden, http.StatusConflict:
		return true
	default:
		return false
	}
}

func recoverableAttemptCode(code string) bool {
	switch code {
	case "lease_expired", "stale_attempt", "message_dead_lettered":
		return true
	default:
		return false
	}
}
