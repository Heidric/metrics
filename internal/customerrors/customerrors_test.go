package customerrors

import (
	"net/http"
	"testing"
)

func TestStatusText_Mappings(t *testing.T) {
	cases := []struct {
		title  string
		detail string

		status int
	}{{
		status: http.StatusBadRequest,
		title:  "Validation Error",
		detail: "The request could not be understood or was missing required parameters",
	}, {
		status: http.StatusNotFound,
		title:  "Not Found",
		detail: "The requested resource could not be found",
	}, {
		status: http.StatusInternalServerError,
		title:  "Resource temporarily unavailable",
		detail: "Resource temporarily unavailable",
	}, {
		status: http.StatusTeapot,
		title:  http.StatusText(http.StatusTeapot),
		detail: "An error occurred while processing the request",
	}}

	for _, tc := range cases {
		t.Run(http.StatusText(tc.status), func(t *testing.T) {
			title, detail := statusText(tc.status)
			if title != tc.title || detail != tc.detail {
				t.Fatalf("statusText(%d) = (%q,%q), want (%q,%q)",
					tc.status, title, detail, tc.title, tc.detail)
			}
		})
	}
}

func TestErrorVars_Available(t *testing.T) {
	if ErrInvalidValue == nil {
		t.Fatal("ErrInvalidValue nil")
	}
	if ErrKeyNotFound == nil {
		t.Fatal("ErrKeyNotFound nil")
	}
	if ErrInvalidType == nil {
		t.Fatal("ErrInvalidType nil")
	}
	if ErrNotConnected == nil {
		t.Fatal("ErrNotConnected nil")
	}
}
