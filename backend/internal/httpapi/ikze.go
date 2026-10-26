package httpapi

import (
	"fmt"
	"net/http"

	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ikze"
	"github.com/jrojowski/profit-tracker/backend/internal/importer"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// ikzeLimitResponse is the body of GET /api/accounts/{id}/ikze-limit.
type ikzeLimitResponse struct {
	// CurrentYear is the calendar year of today in Polish time.
	CurrentYear int `json:"currentYear"`
	// Rows has one row for each year from the first IKZE Contribution to the current year, newest first.
	// It is empty if there is no IKZE Contribution.
	Rows []ikzeLimitRow `json:"rows"`
}

// ikzeLimitRow is one year of the IKZE Limit. Amounts are decimal text.
// Limit, Unused and OverLimit are nil for a year with no IKZE Limit in the tool.
type ikzeLimitRow struct {
	Year          int     `json:"year"`
	Contributions string  `json:"contributions"`
	Limit         *string `json:"limit"`
	Unused        *string `json:"unused"`
	OverLimit     *string `json:"overLimit"`
}

// accountIKZELimit returns the IKZE Contributions of each year against the IKZE Limit.
// It fails with 400 for an Account that is not of the type IKZE.
func (a *api) accountIKZELimit(w http.ResponseWriter, r *http.Request) {
	id, ok := accountID(w, r)
	if !ok {
		return
	}
	acc, err := a.store.Account(r.Context(), id)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	if acc.Type != importer.TypeIKZE {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("account %d is of the type %s, not IKZE", acc.ID, acc.Type))
		return
	}
	// The IKZE Limits are in PLN. The import accepts an IKZE Account in each currency.
	if acc.Currency != ledger.PLN {
		writeError(w, http.StatusBadRequest,
			fmt.Sprintf("account %d has the currency %s. The IKZE Limit needs an Account in PLN.", acc.ID, acc.Currency))
		return
	}
	l, err := a.store.Ledger(r.Context(), acc.ID)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	// The IKZE year is the calendar year in Polish time.
	today := exchange.StartOfDayInWarsaw(a.now())
	years := ikze.Years(l.CashOperations, ikze.BusinessLimit, today)
	resp := ikzeLimitResponse{CurrentYear: today.Year(), Rows: make([]ikzeLimitRow, 0, len(years))}
	for _, y := range years {
		row := ikzeLimitRow{Year: y.Year, Contributions: formatAmount(y.Contributions)}
		if y.HasLimit {
			row.Limit = new(formatAmount(y.Limit))
			row.Unused = new(formatAmount(y.Unused))
			row.OverLimit = new(formatAmount(y.OverLimit))
		}
		resp.Rows = append(resp.Rows, row)
	}
	writeJSON(w, http.StatusOK, resp)
}
