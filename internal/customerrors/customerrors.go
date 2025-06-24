package customerrors

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrInvalidValue = errors.New("invalid value")
	ErrKeyNotFound  = errors.New("key not found")
	ErrInvalidType  = errors.New("invalid metric type")
	ErrNotConnected = errors.New("database not connected")
)

type CommonError struct {
	Title   string `json:"title"`
	Status  int    `json:"status"`
	Details string `json:"detail"`
}

func WriteError(w http.ResponseWriter, status int, customDetail string) {
	title, defaultDetail := statusText(status)

	detail := defaultDetail
	if customDetail != "" {
		detail = customDetail
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(CommonError{
		Title:   title,
		Status:  status,
		Details: detail,
	})
}

func statusText(status int) (title, detail string) {
	switch status {
	case http.StatusBadRequest:
		return "Validation Error", "The request could not be understood or was missing required parameters"
	case http.StatusNotFound:
		return "Not Found", "The requested resource could not be found"
	case http.StatusInternalServerError:
		return "Resource temporarily unavailable", "Resource temporarily unavailable"
	default:
		return http.StatusText(status), "An error occurred while processing the request"
	}
}
