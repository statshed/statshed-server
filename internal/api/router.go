package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/statshed/statshed-server/internal/config"
)

// NewRouter builds the HTTP handler: the global middleware stack plus the /api subrouter.
// In Phase 2 only the stub health route exists; Phase 3 adds the real handlers (wiring in
// the store) and Phase 3.10 replaces the root NotFound with SPA serving.
func NewRouter(cfg config.Config) http.Handler {
	r := chi.NewRouter()

	// Middleware order, outer -> inner: requestLogger first so it records the final status
	// of EVERY response (including a recovered 500); then recover; then the security
	// headers (set before the handler so even a 413/404/500 carries them); then gzip; then
	// the body-size limit nearest the handler.
	r.Use(requestLogger)
	r.Use(recoverer)
	r.Use(securityHeaders)
	r.Use(gzipResponses)
	r.Use(bodyLimit(config.MaxContentLength))

	jsonNotFound := func(w http.ResponseWriter, _ *http.Request) {
		writeHTTPError(w, http.StatusNotFound, "Not found")
	}
	jsonMethodNotAllowed := func(w http.ResponseWriter, _ *http.Request) {
		writeHTTPError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}

	r.Route("/api", func(apiRouter chi.Router) {
		apiRouter.Use(corsMiddleware(cfg.CORSOrigins))
		apiRouter.Get("/health", handleHealthStub)
		// Unknown /api/* paths get the JSON 404 envelope (never SPA HTML).
		apiRouter.NotFound(jsonNotFound)
		apiRouter.MethodNotAllowed(jsonMethodNotAllowed)
	})

	r.NotFound(jsonNotFound)
	r.MethodNotAllowed(jsonMethodNotAllowed)

	return r
}
