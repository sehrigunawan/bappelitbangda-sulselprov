package server_test

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/server"
)

func TestHealth(t *testing.T) {
	tests := []struct {
		name       string
		method     string
		path       string
		wantStatus int
		wantBody   string
	}{
		{"ok", http.MethodGet, "/api/v1/health", http.StatusOK, "ok"},
		{"unknown path", http.MethodGet, "/api/v1/nope", http.StatusNotFound, ""},
		{"wrong method", http.MethodPost, "/api/v1/health", http.StatusMethodNotAllowed, ""},
	}
	h := server.NewHandler(slog.New(slog.DiscardHandler))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, httptest.NewRequest(tc.method, tc.path, nil))
			if rec.Code != tc.wantStatus {
				t.Fatalf("status = %d, want %d", rec.Code, tc.wantStatus)
			}
			if tc.wantBody == "" {
				return
			}
			var body struct {
				Status string `json:"status"`
			}
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
				t.Fatalf("decode: %v", err)
			}
			if body.Status != tc.wantBody {
				t.Fatalf("status field = %q, want %q", body.Status, tc.wantBody)
			}
		})
	}
}
