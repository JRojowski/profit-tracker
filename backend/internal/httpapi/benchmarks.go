package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/benchmark"
	"github.com/jrojowski/profit-tracker/backend/internal/returns"
	"github.com/jrojowski/profit-tracker/backend/internal/valuehistory"
)

// benchmarksResponse is the body of GET /api/accounts/{id}/benchmarks.
type benchmarksResponse struct {
	AccountID int64 `json:"accountId"`
	benchmarksBody
}

// benchmarksBody compares the Time-Weighted Return of the user in one period with the Benchmark Return of each
// Benchmark, in the currency of the comparison. Percentages are decimal text. The frontend does no calculation.
type benchmarksBody struct {
	Period   string `json:"period"`
	Currency string `json:"currency"`
	// Available is false when the Time-Weighted Return of the period does not exist. Then the days and the return
	// of the user are nil, and Benchmarks is empty.
	Available bool `json:"available"`
	// Warnings are empty for one Account. For all Accounts they are the warnings of the Value History.
	Warnings []string `json:"warnings"`
	// BaseDay and LastDay are the days of the Time-Weighted Return of the period, as YYYY-MM-DD.
	BaseDay *string            `json:"baseDay"`
	LastDay *string            `json:"lastDay"`
	User    userReturnResponse `json:"user"`
	// Benchmarks has each Benchmark of the list, in the order of the list, when Available is true.
	Benchmarks []benchmarkResponse `json:"benchmarks"`
}

// userReturnResponse is the Time-Weighted Return of the user in the period. It is the value of the Value History.
type userReturnResponse struct {
	Return *string `json:"return"`
}

// benchmarkResponse is one Benchmark of the comparison.
type benchmarkResponse struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Fund          string `json:"fund"`
	Ticker        string `json:"ticker"`
	QuoteCurrency string `json:"quoteCurrency"`
	// Status is "available" or "noFullPeriod".
	Status string `json:"status"`
	// Return is the Benchmark Return as a percentage, or nil unless the status is "available".
	Return *string `json:"return"`
	// Difference is the return of the user minus Return, in percentage points, or nil unless the status is "available".
	Difference *string `json:"difference"`
}

// periods are the names of the periods of the Value History.
var periods = map[string]func(returns.TimeWeightedReturns) (returns.Rate, time.Time){
	"1M": func(r returns.TimeWeightedReturns) (returns.Rate, time.Time) { return r.OneMonth, r.Starts.OneMonth },
	"3M": func(r returns.TimeWeightedReturns) (returns.Rate, time.Time) {
		return r.ThreeMonths, r.Starts.ThreeMonths
	},
	"YTD": func(r returns.TimeWeightedReturns) (returns.Rate, time.Time) {
		return r.YearToDate, r.Starts.YearToDate
	},
	"1Y":  func(r returns.TimeWeightedReturns) (returns.Rate, time.Time) { return r.OneYear, r.Starts.OneYear },
	"All": func(r returns.TimeWeightedReturns) (returns.Rate, time.Time) { return r.All, r.Starts.All },
}

// accountBenchmarks returns the comparison of the Account with each Benchmark in the period of the query
// parameter "period", in the Account currency. A missing or unknown period is a client error.
func (a *api) accountBenchmarks(w http.ResponseWriter, r *http.Request) {
	id, ok := accountID(w, r)
	if !ok {
		return
	}
	period, ok := benchmarkPeriod(w, r)
	if !ok {
		return
	}
	acc, err := a.store.Account(r.Context(), id)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	history, err := a.accountValueHistoryBody(r.Context(), acc)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	body, err := a.benchmarksBody(r.Context(), acc.Currency, period, history.points)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, benchmarksResponse{AccountID: acc.ID, benchmarksBody: body})
}

// benchmarkPeriod returns the query parameter "period". A missing or unknown period is a client error.
// It writes the error and returns false in that case.
func benchmarkPeriod(w http.ResponseWriter, r *http.Request) (string, bool) {
	period := r.URL.Query().Get("period")
	if _, ok := periods[period]; !ok {
		writeError(w, http.StatusBadRequest, fmt.Sprintf("unknown period %q, use 1M, 3M, YTD, 1Y or All", period))
		return "", false
	}
	return period, true
}

// benchmarksBody returns the comparison of the Value History points with each Benchmark in the period.
// The period must be a key of periods.
func (a *api) benchmarksBody(ctx context.Context, currency, period string, points []valuehistory.Point) (benchmarksBody, error) {
	body := benchmarksBody{
		Period: period, Currency: currency, Warnings: []string{}, Benchmarks: []benchmarkResponse{},
	}
	days := make([]returns.DayValue, 0, len(points))
	for _, p := range points {
		days = append(days, p.DayValue)
	}
	userReturn, baseDay := periods[period](returns.TimeWeighted(days))
	if !userReturn.Available() {
		return body, nil
	}
	lastDay := days[len(days)-1].Date

	in := benchmark.Input{
		Currency: currency, BaseDay: baseDay, LastDay: lastDay, UserReturn: userReturn.Percent,
	}
	var err error
	if in.Closes, err = a.benchmarkCloses(ctx); err != nil {
		return benchmarksBody{}, err
	}
	if in.Rates, err = a.benchmarkRates(ctx); err != nil {
		return benchmarksBody{}, err
	}

	body.Available = true
	body.BaseDay, body.LastDay = dateText(baseDay), dateText(lastDay)
	body.User.Return = percent(userReturn)
	for _, c := range benchmark.Compare(in) {
		b := benchmarkResponse{
			ID: c.ID, Name: c.Name, Fund: c.Fund, Ticker: c.Ticker, QuoteCurrency: c.QuoteCurrency, Status: string(c.Status),
		}
		if c.Status == benchmark.StatusAvailable {
			b.Return, b.Difference = new(c.Return.String()), new(c.Difference.String())
		}
		body.Benchmarks = append(body.Benchmarks, b)
	}
	return body, nil
}

// benchmarkCloses returns the stored closes of each Benchmark in its Quote Currency, by Benchmark identifier.
func (a *api) benchmarkCloses(ctx context.Context) (map[string][]benchmark.Close, error) {
	stored, err := a.store.BenchmarkPrices(ctx)
	if err != nil {
		return nil, err
	}
	closes := map[string][]benchmark.Close{}
	for _, b := range benchmark.List() {
		for _, p := range stored[b.ID] {
			// The refresh stores no close in another currency. The check keeps the rule in one more place.
			if p.Currency == b.QuoteCurrency {
				closes[b.ID] = append(closes[b.ID], benchmark.Close{Date: p.Date, Price: p.Close})
			}
		}
	}
	return closes, nil
}

// benchmarkRates returns the stored Exchange Rates of each currency, in date order.
func (a *api) benchmarkRates(ctx context.Context) (map[string][]benchmark.Rate, error) {
	stored, err := a.store.ExchangeRateSeries(ctx)
	if err != nil {
		return nil, err
	}
	rates := make(map[string][]benchmark.Rate, len(stored))
	for currency, list := range stored {
		for _, r := range list {
			rates[currency] = append(rates[currency], benchmark.Rate{Date: r.Date, Mid: r.Rate})
		}
	}
	return rates, nil
}
