// Package ikze calculates the IKZE Contributions of each year against the IKZE Limit. See GLOSSARY.md.
// It is a pure calculation with no I/O.
package ikze

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// businessLimits are the IKZE Limits for people who run a business, in PLN, by calendar year.
// The government publishes the limit of the next year in November. Add it here each year.
// A year that is not in the table has no known limit.
//
// Sources: the MRPiPS notice of 10 November 2025, summarized at
// https://czasopismo.legeartis.org/2025/11/limit-wplat-ikze-2026-oipe-suma-skladek-dodatkowych-ppe-obwieszczenie-mrpips/
// and https://strefainwestorow.pl/emerytura/limit-ike-ikze-2026.
var businessLimits = map[int]decimal.Decimal{
	2025: decimal.RequireFromString("15611.40"),
	2026: decimal.RequireFromString("16956.00"),
}

// BusinessLimit returns the IKZE Limit in PLN of the calendar year for people who run a business.
// It returns false if the tool does not know the limit of the year.
func BusinessLimit(year int) (decimal.Decimal, bool) {
	limit, ok := businessLimits[year]
	return limit, ok
}

// Year is the IKZE Contributions of one calendar year against the IKZE Limit of that year.
type Year struct {
	Year int
	// Contributions is the sum of the IKZE Contributions of the year.
	Contributions decimal.Decimal
	// HasLimit tells if the table has the IKZE Limit of the year.
	// If it is false, Limit, Unused and OverLimit are zero.
	HasLimit bool
	Limit    decimal.Decimal
	// Unused is the part of the limit that the contributions do not use: max(Limit − Contributions, 0).
	Unused decimal.Decimal
	// OverLimit is the amount of the contributions above the limit: max(Contributions − Limit, 0).
	OverLimit decimal.Decimal
}

// Years returns one Year for each calendar year from the year of the first IKZE Contribution to the current year,
// newest first. It returns no Year if there is no IKZE Contribution.
//
// An IKZE Contribution is a Deposit or a Transfer In: a Cash Operation of one of these types with a positive amount.
// A withdrawal and a Transfer Out do not reduce the contributions.
// Years takes the calendar year in the location of today. Give today in Polish time.
// A contribution after the current year gets no row.
// limit returns the IKZE Limit of a year, or false if the limit of the year is not known. See BusinessLimit.
func Years(ops []ledger.CashOperation, limit func(year int) (decimal.Decimal, bool), today time.Time) []Year {
	byYear := map[int]decimal.Decimal{}
	first, current := 0, today.Year()
	for _, op := range ops {
		if !isContribution(op) {
			continue
		}
		year := op.Time.In(today.Location()).Year()
		byYear[year] = byYear[year].Add(op.Amount)
		if first == 0 || year < first {
			first = year
		}
	}
	if first == 0 {
		return []Year{}
	}

	// If each contribution is after the current year, there is no row.
	years := make([]Year, 0, max(current-first+1, 0))
	for year := current; year >= first; year-- {
		y := Year{Year: year, Contributions: byYear[year]}
		y.Limit, y.HasLimit = limit(year)
		if y.HasLimit {
			y.Unused = decimal.Max(y.Limit.Sub(y.Contributions), decimal.Zero)
			y.OverLimit = decimal.Max(y.Contributions.Sub(y.Limit), decimal.Zero)
		}
		years = append(years, y)
	}
	return years
}

// isContribution tells if op is an IKZE Contribution: a Deposit or a Transfer In.
// ADR 0003: a Transfer with a positive amount is a Transfer In.
func isContribution(op ledger.CashOperation) bool {
	return (op.Type == ledger.TypeDeposit || op.Type == ledger.TypeTransfer) && op.Amount.IsPositive()
}
