package api

import (
	"net/http"

	"github.com/go-chi/chi/v5"

	"github.com/statshed/statshed-server/internal/config"
	"github.com/statshed/statshed-server/internal/realtime"
	"github.com/statshed/statshed-server/internal/staticfs"
	"github.com/statshed/statshed-server/internal/store"
)

// handlers carries the dependencies shared by the REST handlers.
type handlers struct {
	store *store.Store
	cfg   config.Config
}

// NewRouter builds the HTTP handler: the global middleware stack plus the /api subrouter.
// The hub backs GET /api/events (Phase 5.2 wires the handlers' real-time broadcasts).
func NewRouter(cfg config.Config, st *store.Store, hub *realtime.Hub) http.Handler {
	h := &handlers{store: st, cfg: cfg}
	r := chi.NewRouter()

	// Middleware order, outer -> inner: requestLogger first so it records the final status
	// of EVERY response (including a recovered 500); then recover; then the security
	// headers (set before the handler so even a 413/404/500 carries them); then gzip; then
	// the body-size limit nearest the handler.
	r.Use(requestLogger)
	r.Use(recoverer)
	r.Use(securityHeaders)
	r.Use(gzipResponses)

	jsonNotFound := func(w http.ResponseWriter, _ *http.Request) {
		writeHTTPError(w, http.StatusNotFound, "Not found")
	}
	jsonMethodNotAllowed := func(w http.ResponseWriter, _ *http.Request) {
		writeHTTPError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}

	r.Route("/api", func(apiRouter chi.Router) {
		// CORS before the body limit, so an over-limit 413 to an allowed Origin still
		// carries the CORS headers (flask_cors applies CORS to error responses too). The
		// limit lives here because only /api has request bodies.
		apiRouter.Use(corsMiddleware(cfg.CORSOrigins))
		apiRouter.Use(bodyLimit(config.MaxContentLength))
		apiRouter.Get("/health", h.health)
		apiRouter.Get("/events", hub.ServeEvents)
		apiRouter.Post("/status", h.postStatus)
		apiRouter.Get("/jobs", h.listJobs)
		apiRouter.Post("/jobs/{id:[0-9]+}/ack", h.ackJob)
		apiRouter.Delete("/jobs/{id:[0-9]+}", h.deleteJob)
		apiRouter.Post("/groups/{name}/ack", h.ackGroup)
		apiRouter.Post("/ack-all", h.ackAll)
		apiRouter.Get("/admin/stats", h.getAdminStats)
		apiRouter.Delete("/admin/cleanup", h.adminCleanup)
		if cfg.TestHooks {
			// Guarded test-only tick hook; absent (-> JSON 404) in production.
			apiRouter.Post("/admin/run-checks", h.runChecks)
		}
		apiRouter.Get("/groups", h.listGroups)
		apiRouter.Get("/groups/{name}/jobs", h.getGroupJobs)
		apiRouter.Get("/groups/{name}/jobs/{job}/log", h.getJobLog)
		apiRouter.Get("/config", h.getConfig)
		apiRouter.Put("/config", h.updateConfig)
		apiRouter.Get("/groups/{name}/config", h.getGroupConfig)
		apiRouter.Put("/groups/{name}/config", h.updateGroupConfig)
		// Unknown /api/* paths get the JSON 404 envelope (never SPA HTML).
		apiRouter.NotFound(jsonNotFound)
		apiRouter.MethodNotAllowed(jsonMethodNotAllowed)
	})

	// Non-/api paths fall through here: serve the SPA when enabled (D9), else JSON 404.
	// Unknown /api/* paths are handled by the subrouter above and never reach this.
	if spa := staticfs.Handler(cfg.StaticDir, cfg.StaticDisabled); spa != nil {
		r.NotFound(spa.ServeHTTP)
	} else {
		r.NotFound(jsonNotFound)
	}
	r.MethodNotAllowed(jsonMethodNotAllowed)

	return r
}
