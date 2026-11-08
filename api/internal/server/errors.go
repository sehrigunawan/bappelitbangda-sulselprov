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

// writeProblem writes an RFC 9457 problem details body (components.schemas.Problem in openapi.yaml).
func writeProblem(w http.ResponseWriter, status int, detail string) {
	p := api.Problem{Title: http.StatusText(status), Status: status}
	if detail != "" {
		p.Detail = &detail
	}
	w.Header().Set("Content-Type", "application/problem+json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(p)
}

// responseErrorHandler maps errors returned by handlers. Anything unexpected is logged with the
// request id (NFR-15) and answered with a generic 500: the error text never reaches the client.
func responseErrorHandler(logger *slog.Logger) func(http.ResponseWriter, *http.Request, error) {
	return func(w http.ResponseWriter, r *http.Request, err error) {
		if errors.Is(err, ErrNotImplemented) {
			writeProblem(w, http.StatusNotImplemented, "")
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
