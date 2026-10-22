package httpapi

import "net/http"

// healthResponse is the body of GET /api/health.
type healthResponse struct {
	Status        string `json:"status"`
	SchemaVersion int    `json:"schemaVersion,omitempty"`
}

// health tells if the server and the database answer.
func (a *api) health(w http.ResponseWriter, r *http.Request) {
	version, err := a.store.SchemaVersion(r.Context())
	if err != nil {
		a.logger.ErrorContext(r.Context(), "health check: database does not answer", "error", err)
		writeJSON(w, http.StatusServiceUnavailable, healthResponse{Status: "unavailable"})
		return
	}
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", SchemaVersion: version})
}
