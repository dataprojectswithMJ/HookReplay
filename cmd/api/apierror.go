package main

import (
	"errors"
	"net/http"
)

// apiError is an HTTP error with the §2 envelope fields.
type apiError struct {
	status int
	code   string
	msg    string
}

func (e *apiError) Error() string { return e.msg }

func errAPI(status int, code, msg string) error {
	return &apiError{status: status, code: code, msg: msg}
}

// writeAPIError writes an apiError (or a generic 500) using the §2 envelope.
func writeAPIError(w http.ResponseWriter, err error, reqID string) {
	var ae *apiError
	if errors.As(err, &ae) {
		writeError(w, ae.status, ae.code, ae.msg, reqID)
		return
	}
	writeError(w, http.StatusInternalServerError, "internal_error", err.Error(), reqID)
}
