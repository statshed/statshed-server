package api

import "net/http"

// corsMiddleware reproduces flask_cors's observable behavior for the API routes (M1,
// spec.md §9): reflect an allowed Origin into Access-Control-Allow-Origin, set
// Vary: Origin, and answer the OPTIONS preflight with Allow-Methods/Headers. Production is
// same-origin, so this is best-effort fidelity, not a load-bearing contract — the reused
// pytest suite never issues cross-origin requests.
func corsMiddleware(allowedOrigins []string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(allowedOrigins))
	for _, o := range allowedOrigins {
		allowed[o] = struct{}{}
	}
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")
			_, originAllowed := allowed[origin]
			if origin != "" && originAllowed {
				h := w.Header()
				h.Set("Access-Control-Allow-Origin", origin)
				h.Add("Vary", "Origin")
			}

			if r.Method == http.MethodOptions {
				if origin != "" && originAllowed {
					h := w.Header()
					h.Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
					reqHeaders := r.Header.Get("Access-Control-Request-Headers")
					if reqHeaders == "" {
						reqHeaders = "Content-Type"
					}
					h.Set("Access-Control-Allow-Headers", reqHeaders)
				}
				w.WriteHeader(http.StatusOK)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
