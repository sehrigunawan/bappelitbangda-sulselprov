package server

import (
	"log/slog"
	"net/http"
)

// NewHandlerWithoutRule builds the HTTP handler with operationRules minus op,
// used to test that unlisted operations are denied by default.
func NewHandlerWithoutRule(logger *slog.Logger, deps Deps, op string) http.Handler {
	rules := make(map[string]rule, len(operationRules))
	for k, v := range operationRules {
		if k != op {
			rules[k] = v
		}
	}
	return newHandler(logger, deps, rules)
}
