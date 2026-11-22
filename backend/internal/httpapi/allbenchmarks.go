package httpapi

import (
	"net/http"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// allAccountsBenchmarks returns the comparison of all Accounts with each Benchmark in the period of the query
// parameter "period", in PLN. The return of the user is the Time-Weighted Return of the Value History of all
// Accounts. If that history is not available, the comparison is not available and the warnings tell why.
func (a *api) allAccountsBenchmarks(w http.ResponseWriter, r *http.Request) {
	period, ok := benchmarkPeriod(w, r)
	if !ok {
		return
	}
	history, err := a.allValueHistoryOf(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	body, err := a.benchmarksBody(r.Context(), ledger.PLN, period, history.points)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	if len(history.Warnings) > 0 {
		body.Warnings = history.Warnings
	}
	writeJSON(w, http.StatusOK, body)
}
