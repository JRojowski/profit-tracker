package allaccounts

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/returns"
	"github.com/jrojowski/profit-tracker/backend/internal/valuehistory"
)

// HistoryAccount is the input of one Account for ValueHistory.
type HistoryAccount struct {
	Currency string
	// Points are the points of valuehistory.Calculate in the Account currency. The last point is today.
	Points []valuehistory.Point
}

// MissingRate is an Exchange Rate that ValueHistory needs and does not have.
type MissingRate struct {
	Currency string
	// Day is the first day with no rate, at midnight UTC. It is the zero time if the latest rate is missing.
	Day time.Time
}

// History is the result of ValueHistory.
type History struct {
	// Available tells if each needed Exchange Rate is available. If it is false, Points is empty
	// and Missing tells why.
	Available bool
	// Points has one point in PLN for each calendar day from the first point of any Account to today, in date order.
	Points []valuehistory.Point
	// Missing has one entry for each Account currency with a missing rate, in the order of the input.
	Missing []MissingRate
}

// ValueHistory returns the Value History of all Accounts in PLN.
//
// rates are the lookups of the hero (Calculate). ExchangeRates are the stored Exchange Rates of each currency,
// in date order.
//
//   - The value of a past day is the value of the Account × the last stored rate on that day or before it.
//     Thus a day with no NBP table uses the rate of the day before. The lookup ignores the asked range.
//   - The value of today is the value of the Account today × the latest rate, as in the hero.
//   - The flow of a day is the flow of the Account on that day × the rate of that day, by ADR 0004, rule 3.
//     The Net Deposits are the sum of the flows so far. Thus the Net Deposits of today are the Net Deposits of the hero.
//   - An Account adds nothing before the day of its first point.
//
// ADR 0003: the tool does not link the two sides of a Transfer. The two sides are on the same day, so their flows
// cancel in the sum of the day. A Transfer between two Accounts is thus not a flow for all Accounts.
//
// The history is not available if a rate is missing. A PLN Account needs no rate.
func ValueHistory(accounts []HistoryAccount, rates Rates, exchangeRates map[string][]valuehistory.Rate) History {
	var first, last time.Time
	for _, acc := range accounts {
		if len(acc.Points) == 0 {
			continue
		}
		if d := acc.Points[0].Date; first.IsZero() || d.Before(first) {
			first = d
		}
		if d := acc.Points[len(acc.Points)-1].Date; d.After(last) {
			last = d
		}
	}
	if first.IsZero() {
		return History{Available: true, Points: []valuehistory.Point{}}
	}

	days := daysBetween(first, last) + 1
	values := make([]decimal.Decimal, days)
	flows := make([]decimal.Decimal, days)
	var missing []MissingRate
	for _, acc := range accounts {
		if len(acc.Points) == 0 {
			continue
		}
		accValues, accFlows, gap, ok := inPLN(acc, rates, exchangeRates[acc.Currency])
		if !ok {
			missing = append(missing, gap)
			continue
		}
		offset := daysBetween(first, acc.Points[0].Date)
		for i := range accValues {
			values[offset+i] = values[offset+i].Add(accValues[i])
			flows[offset+i] = flows[offset+i].Add(accFlows[i])
		}
	}
	if len(missing) > 0 {
		return History{Points: []valuehistory.Point{}, Missing: missing}
	}

	points := make([]valuehistory.Point, days)
	netDeposits := decimal.Zero
	for i := range points {
		netDeposits = netDeposits.Add(flows[i])
		points[i] = valuehistory.Point{
			DayValue:    returns.DayValue{Date: first.AddDate(0, 0, i), Value: values[i], Flow: flows[i]},
			NetDeposits: netDeposits,
		}
	}
	return History{Available: true, Points: points}
}

// inPLN returns the value and the flow of each point of the Account in PLN.
// It returns false and the missing rate if a rate is missing.
func inPLN(acc HistoryAccount, rates Rates, series []valuehistory.Rate) (values, flows []decimal.Decimal, gap MissingRate, ok bool) {
	values = make([]decimal.Decimal, len(acc.Points))
	flows = make([]decimal.Decimal, len(acc.Points))
	if acc.Currency == ledger.PLN {
		for i, p := range acc.Points {
			values[i], flows[i] = p.Value, p.Flow
		}
		return values, flows, MissingRate{}, true
	}

	latest, ok := rates.Latest(acc.Currency)
	if !ok {
		return nil, nil, MissingRate{Currency: acc.Currency}, false
	}
	today := len(acc.Points) - 1
	next := 0
	for i, p := range acc.Points {
		if !p.Flow.IsZero() {
			// rates.On takes the day of a time in Polish time. Midnight UTC is on the same day in Polish time.
			rate, ok := rates.On(acc.Currency, p.Date)
			if !ok {
				return nil, nil, MissingRate{Currency: acc.Currency, Day: p.Date}, false
			}
			flows[i] = p.Flow.Mul(rate)
		}
		if i == today {
			values[i] = p.Value.Mul(latest)
			continue
		}
		// The days only grow, so the search continues from the last rate.
		for next < len(series) && !series[next].Date.After(p.Date) {
			next++
		}
		// series[next-1] is the last stored rate on the day or before it. A rate of zero or less is not valid,
		// as in valuehistory.Calculate.
		if next == 0 || !series[next-1].Mid.IsPositive() {
			return nil, nil, MissingRate{Currency: acc.Currency, Day: p.Date}, false
		}
		values[i] = p.Value.Mul(series[next-1].Mid)
	}
	return values, flows, MissingRate{}, true
}

// daysBetween returns the number of calendar days from one day at midnight UTC to a later day at midnight UTC.
func daysBetween(from, to time.Time) int {
	return int(to.Sub(from) / (24 * time.Hour))
}
