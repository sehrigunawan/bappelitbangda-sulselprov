package server

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/go-chi/chi/v5/middleware"

	"github.com/kindaMid72/bappelitbangda-sulselprov/api/internal/api"
)

// ErrNotImplemented is returned by contract-PR stubs until the BE lane implements the operation.
// The response error handler maps it to 501.
var ErrNotImplemented = errors.New("not implemented")

// ErrUnauthenticated is returned when the request has no session or the session stage does not allow the operation.
var ErrUnauthenticated = errors.New("unauthenticated")

// ErrForbidden is returned when the session lacks the permission for the operation.
var ErrForbidden = errors.New("forbidden")

// ErrStepUpRequired is returned when the operation needs a recent TOTP re-verification.
var ErrStepUpRequired = errors.New("step-up required")

// Indonesian detail texts that the client shows or matches on (NFR-16, docs/modules/auth.md).
const (
	detailUnauthorized         = "Sesi tidak valid atau telah berakhir."
	detailForbidden            = "Akses ditolak."
	detailStepUp               = "Verifikasi ulang dengan kode autentikator diperlukan."
	detailCrossOrigin          = "Permintaan lintas situs ditolak."
	detailBadCredentials       = "Nama pengguna atau kata sandi salah." //nolint:gosec // response text, not a credential
	detailWrongCurrentPassword = "Kata sandi saat ini salah."           //nolint:gosec // response text, not a credential
	detailInvalidCode          = "Kode tidak valid."
	detailUsernameTaken        = "Nama pengguna sudah dipakai."
)

// problem builds a Problem body for typed handler responses. detail is omitted when empty.
func problem(status int, detail string) api.Problem {
	p := api.Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	return p
}

// writeProblem writes an RFC 9457 problem details body (components.schemas.Problem in openapi.yaml).
func writeProblem(w http.ResponseWriter, status int, detail string) {
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(problem(status, detail))
}

// responseErrorHandler maps errors returned by handlers. Anything unexpected is logged with the
// request id (NFR-15) and answered with a generic 500: the error text never reaches the client.
func responseErrorHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		// Authorization failures are expected outcomes, not bugs: answered without logging.
		switch {
		case errors.Is(err, ErrNotImplemented):
			writeProblem(w, http.StatusNotImplemented, "")
			return
		case errors.Is(err, ErrUnauthenticated):
			writeProblem(w, http.StatusUnauthorized, detailUnauthorized)
			return
		case errors.Is(err, ErrForbidden):
			writeProblem(w, http.StatusForbidden, detailForbidden)
			return
		case errors.Is(err, ErrStepUpRequired):
			writeProblem(w, http.StatusForbidden, detailStepUp)
			return
		}
		logger.ErrorContext(r.Context(), "handler error", "error", err, "request_id", middleware.GetReqID(r.Context()))
		writeProblem(w, http.StatusInternalServerError, "")
	}
}

// requestErrorHandler answers requests the generated code could not decode or validate.
func requestErrorHandler(w http.ResponseWriter, _ *http.Request, err error) {
	writeProblem(w, http.StatusBadRequest, err.Error())
}
