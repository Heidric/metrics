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
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		return
	}
}

func NotFoundError(w http.ResponseWriter) {
	res := CommonError{
		Title:   "Not found",
		Status:  http.StatusNotFound,
		Details: "Requested resource not found",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusNotFound)

	// Добавим обработку ошибки кодирования
	if err := json.NewEncoder(w).Encode(res); err != nil {
		// Если не удалось закодировать JSON, пишем простой текст
		w.Write([]byte(`{"title":"Encoding Error","status":500,"detail":"Failed to encode error response"}`))
	}
}

func InternalError(w http.ResponseWriter) {
	res := CommonError{
		Title:   "Resource temporarily unavailable",
		Status:  http.StatusInternalServerError,
		Details: "Resource temporarily unavailable",
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusInternalServerError)
	err := json.NewEncoder(w).Encode(res)
	if err != nil {
		return
	}
}
