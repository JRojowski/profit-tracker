package httpapi

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/returns"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/valuehistory"
)

// valueHistoryResponse is the body of GET /api/accounts/{id}/value-history.
type valueHistoryResponse struct {
	AccountID int64 `json:"accountId"`
	valueHistoryBody
}

// valueHistoryBody is the Value History of an Account in the Account currency, or of all Accounts in PLN.
// Amounts are decimal text. The frontend does no calculation. It shows the points from the date in PeriodStarts
// of the selected period.
type valueHistoryBody struct {
	Currency string `json:"currency"`
	// Points has one point for each calendar day from the day of the first Cash Operation to today, in date order.
	Points []valuePointResponse `json:"points"`
	// TimeWeightedReturn is the cumulative Time-Weighted Return of each period. See returns.TimeWeightedReturns.
	TimeWeightedReturn timeWeightedResponse `json:"timeWeightedReturn"`
	// PeriodStarts is the first date that the chart shows for each period. It is the date of a point.
	PeriodStarts periodStartsResponse `json:"periodStarts"`
	// ValuedAtCost are the Instruments that the tool valued at cost on one or more days, in ticker order.
	ValuedAtCost []valuedAtCostResponse `json:"valuedAtCost"`
	// PriceWarnings are the Price Warnings of the Instruments, also of the sold ones. The points do not change
	// when a warning exists.
	PriceWarnings []priceWarningResponse `json:"priceWarnings"`
}

// valuePointResponse is the state of the Account at the end of one calendar day.
type valuePointResponse struct {
	// Date is the day as YYYY-MM-DD.
	Date        string `json:"date"`
	Value       string `json:"value"`
	NetDeposits string `json:"netDeposits"`
}

// timeWeightedResponse is the Time-Weighted Return of each period, as a percentage in decimal text with at most
// 8 decimal places. A period is nil if no day of the period has a value on the day before it.
type timeWeightedResponse struct {
	OneMonth    *string `json:"1M"`
	ThreeMonths *string `json:"3M"`
	YearToDate  *string `json:"YTD"`
	OneYear     *string `json:"1Y"`
	All         *string `json:"All"`
}

// periodStartsResponse is the first date of each period as YYYY-MM-DD. It is the base day of the
// Time-Weighted Return. See returns.PeriodStarts. A base day before the first point is the first point.
// A period is nil if there are no points.
type periodStartsResponse struct {
	OneMonth    *string `json:"1M"`
	ThreeMonths *string `json:"3M"`
	YearToDate  *string `json:"YTD"`
	OneYear     *string `json:"1Y"`
	All         *string `json:"All"`
}

// valuedAtCostResponse names one Instrument that the tool valued at cost.
type valuedAtCostResponse struct {
	// Ticker is the broker ticker.
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
}

// accountValueHistory returns the Value History and the Time-Weighted Returns of the Account.
// It calculates them on each request and stores nothing.
func (a *api) accountValueHistory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id, ok := accountID(w, r)
	if !ok {
		return
	}
	acc, err := a.store.Account(ctx, id)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	in, err := a.readHeroInput(ctx, acc)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	prices, err := a.store.PriceHistory(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	rates, err := a.store.ExchangeRateSeries(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	warnings, err := a.readPriceWarnings(ctx, in.instruments, prices)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}

	history, err := valuehistory.Calculate(valuehistory.Input{
		Ledger:        in.ledger,
		Closes:        quoteCloses(in.instruments, prices),
		ExchangeRates: rateSeries(rates),
	})
	if err != nil {
		a.writeInternalError(w, r, fmt.Errorf("calculate the value history of account %d: %w", acc.ID, err))
		return
	}

	body := newValueHistoryBody(acc.Currency, history.Points, history.ValuedAtCost)
	body.PriceWarnings = warnings.forLedger(in.ledger)
	writeJSON(w, http.StatusOK, valueHistoryResponse{AccountID: acc.ID, valueHistoryBody: body})
}

// newValueHistoryBody returns the body of the points, with the Time-Weighted Return of each period.
func newValueHistoryBody(currency string, points []valuehistory.Point, valuedAtCost []ledger.Instrument) valueHistoryBody {
	body := valueHistoryBody{
		Currency:      currency,
		Points:        make([]valuePointResponse, 0, len(points)),
		ValuedAtCost:  make([]valuedAtCostResponse, 0, len(valuedAtCost)),
		PriceWarnings: []priceWarningResponse{},
	}
	days := make([]returns.DayValue, 0, len(points))
	for _, p := range points {
		body.Points = append(body.Points, valuePointResponse{
			Date: p.Date.Format(time.DateOnly), Value: formatAmount(p.Value), NetDeposits: formatAmount(p.NetDeposits),
		})
		days = append(days, p.DayValue)
	}
	twr := returns.TimeWeighted(days)
	body.TimeWeightedReturn = timeWeightedResponse{
		OneMonth: percent(twr.OneMonth), ThreeMonths: percent(twr.ThreeMonths), YearToDate: percent(twr.YearToDate),
		OneYear: percent(twr.OneYear), All: percent(twr.All),
	}
	if len(days) > 0 {
		st := twr.Starts
		body.PeriodStarts = periodStartsResponse{
			OneMonth: dateText(st.OneMonth), ThreeMonths: dateText(st.ThreeMonths), YearToDate: dateText(st.YearToDate),
			OneYear: dateText(st.OneYear), All: dateText(st.All),
		}
	}
	for _, inst := range valuedAtCost {
		body.ValuedAtCost = append(body.ValuedAtCost, valuedAtCostResponse{Ticker: inst.Ticker, Name: inst.Name})
	}
	return body
}

// dateText returns the day as YYYY-MM-DD.
func dateText(day time.Time) *string {
	return new(day.Format(time.DateOnly))
}

// quoteCloses returns the stored prices of each Instrument in its Quote Currency, in date order.
// As quotePrices for the current price, it ignores a stored price in another currency.
func quoteCloses(instruments map[string]store.Instrument, history map[string][]store.Price) map[string][]valuehistory.Close {
	closes := make(map[string][]valuehistory.Close, len(history))
	for ticker, prices := range history {
		qc := instruments[ticker].QuoteCurrency
		if qc == "" {
			continue
		}
		list := make([]valuehistory.Close, 0, len(prices))
		for _, p := range prices {
			if p.Currency == qc {
				list = append(list, valuehistory.Close{Date: p.Date, Price: p.Close})
			}
		}
		closes[ticker] = list
	}
	return closes
}

// rateSeries returns the stored Exchange Rates of each currency, in date order.
func rateSeries(stored map[string][]store.ExchangeRate) map[string][]valuehistory.Rate {
	series := make(map[string][]valuehistory.Rate, len(stored))
	for currency, rates := range stored {
		list := make([]valuehistory.Rate, 0, len(rates))
		for _, r := range rates {
			list = append(list, valuehistory.Rate{Date: r.Date, Mid: r.Rate})
		}
		series[currency] = list
	}
	return series
}
