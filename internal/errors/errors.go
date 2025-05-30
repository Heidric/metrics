package errors

import (
	"encoding/json"
	"errors"
	"net/http"
)

var (
	ErrInvalidValue           = errors.New("invalid value")
	ErrKeyNotFound            = errors.New("key not found")
	ErrUnexpectedResponseType = errors.New("unexpected response type")
	ErrInvalidType            = errors.New("invalid metric type")
	ErrStoreClosed            = errors.New("store is closed")
)

type CommonError struct {
	Title   string `json:"title"`
	Status  int    `json:"status"`
	Details string `json:"detail"`
}

func ValidationError(w http.ResponseWriter, details string) {
	res := CommonError{
		Title:   "Validation error occurred",
		Status:  http.StatusBadRequest,
		Details: details,
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	json.NewEncoder(w).Encode(res)
}

func NotFoundError(w http.ResponseWriter) {
	res := CommonError{
		Title:   "Not found",
		Status:  http.StatusNotFound,
		Details: "Requested resource not found",
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)
	json.NewEncoder(w).Encode(res)
}

func InternalError(w http.ResponseWriter) {
	res := CommonError{
		Title:   "Resource temporarily unavailable",
		Status:  http.StatusInternalServerError,
		Details: "Resource temporarily unavailable",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(res)
}
