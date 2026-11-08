package server

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"

	"github.com/go-chi/chi/v5/middleware"
)

func TestResponseErrorHandler(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantBody   string
		wantLogged bool
	}{
		{"not implemented", ErrNotImplemented, http.StatusNotImplemented, `{"title":"Not Implemented","status":501}`, false},
		{"wrapped not implemented", fmt.Errorf("list: %w", ErrNotImplemented), http.StatusNotImplemented, `{"title":"Not Implemented","status":501}`, false},
		{"unexpected error", errors.New("db down"), http.StatusInternalServerError, `{"title":"Internal Server Error","status":500}`, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var logs bytes.Buffer
			handler := responseErrorHandler(slog.New(slog.NewJSONHandler(&logs, nil)))
			rec := httptest.NewRecorder()
			var requestID string
			chain := middleware.RequestID(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				requestID = middleware.GetReqID(r.Context())
				handler(w, r, tc.err)
			}))
			chain.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/v1/demo", nil))

			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
				t.Fatalf("content-type = %q, want application/problem+json", got)
			}
			assertJSONEqual(t, rec.Body.Bytes(), tc.wantBody)
			if tc.wantLogged {
				if strings.Contains(rec.Body.String(), "db down") {
					t.Fatalf("response leaks the internal error: %s", rec.Body.String())
				}
				if !strings.Contains(logs.String(), "db down") {
					t.Fatalf("internal error not logged: %s", logs.String())
				}
				if requestID == "" || !strings.Contains(logs.String(), requestID) {
					t.Fatalf("log lacks request id %q: %s", requestID, logs.String())
				}
			} else if logs.Len() != 0 {
				t.Fatalf("unexpected log output: %s", logs.String())
			}
		})
	}
}

func TestRequestErrorHandler(t *testing.T) {
	rec := httptest.NewRecorder()
	requestErrorHandler(rec, httptest.NewRequest(http.MethodGet, "/api/v1/demo?page=x", nil), errors.New("invalid page"))

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusBadRequest)
	}
	if got := rec.Header().Get("Content-Type"); got != "application/problem+json" {
		t.Fatalf("content-type = %q, want application/problem+json", got)
	}
	want := `{"title":"Bad Request","status":400,"detail":"invalid page"}`
	assertJSONEqual(t, rec.Body.Bytes(), want)
}

// assertJSONEqual compares JSON documents ignoring key order (the generated struct orders keys alphabetically).
func assertJSONEqual(t *testing.T, got []byte, want string) {
	t.Helper()
	var g, w map[string]any
	if err := json.Unmarshal(got, &g); err != nil {
		t.Fatalf("response is not JSON: %v: %s", err, got)
	}
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatalf("bad expectation: %v", err)
	}
	if !reflect.DeepEqual(g, w) {
		t.Fatalf("body = %s, want %s", strings.TrimSpace(string(got)), want)
	}
}
