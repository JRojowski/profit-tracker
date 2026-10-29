// Package yahoo is a price.Provider that uses the chart endpoint of Yahoo Finance.
// The endpoint is not official and can change with no notice. All knowledge of Yahoo stays in this package.
package yahoo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/price"
)

// Source is the name of this provider in the stored prices.
const Source = "yahoo"

// DefaultBaseURL is the address of the Yahoo Finance API.
const DefaultBaseURL = "https://query1.finance.yahoo.com"

// userAgent is the User-Agent of each request. Yahoo refuses the default User-Agent of Go.
// Since 2026-10, Yahoo also answers 429 to some full browser User-Agent strings. "Mozilla/5.0" gets 200.
const userAgent = "Mozilla/5.0"

// maxBody is the largest response body that the provider reads. A chart for 5 days is a few kilobytes.
// The daily chart of 20 years is about 1 MB.
const maxBody = 16 << 20

// Provider gets the last close price from the Yahoo Finance chart endpoint.
type Provider struct {
	// Client sends the requests. It must have a timeout.
	Client *http.Client
	// BaseURL is the address of the API, with no final slash.
	BaseURL string
	// Now is the clock that tells "today" for the daily closes. If it is nil, the provider uses time.Now.
	Now func() time.Time
}

// New returns a Provider for the Yahoo Finance API with an HTTP client that stops a request after 15 seconds.
func New() *Provider {
	return &Provider{Client: &http.Client{Timeout: 15 * time.Second}, BaseURL: DefaultBaseURL}
}

// chartResponse is the part of the chart response that the provider reads.
type chartResponse struct {
	Chart struct {
		Result []struct {
			Meta struct {
				Currency string `json:"currency"`
				// RegularMarketPrice is a JSON number. json.Number keeps its text, so no float64 rounding occurs.
				RegularMarketPrice json.Number `json:"regularMarketPrice"`
				// RegularMarketTime is the Unix time of the price.
				RegularMarketTime int64 `json:"regularMarketTime"`
				// GMTOffset is the offset of the exchange time from UTC, in seconds.
				GMTOffset int64 `json:"gmtoffset"`
				// DataGranularity is the interval of the rows, for example "1d" or "1mo".
				DataGranularity string `json:"dataGranularity"`
			} `json:"meta"`
			// Timestamp is the Unix time of each row: the start of the session, not midnight.
			Timestamp []int64 `json:"timestamp"`
			Events    struct {
				// Splits are the splits by the text of their Unix time.
				Splits map[string]split `json:"splits"`
			} `json:"events"`
			Indicators struct {
				Quote []struct {
					// Close has one value for each timestamp. A null value is a row with no close.
					Close []*json.Number `json:"close"`
				} `json:"quote"`
			} `json:"indicators"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
}

// split is one split event. A 4:1 split has the numerator 4 and the denominator 1.
type split struct {
	// Date is the Unix time of the split.
	Date        int64       `json:"date"`
	Numerator   json.Number `json:"numerator"`
	Denominator json.Number `json:"denominator"`
}

// LastClose returns the last price of the Yahoo symbol, its currency and its date in the exchange time.
// A London price in pence (GBp) comes back in pounds (GBP).
func (p *Provider) LastClose(ctx context.Context, ticker string) (price.Quote, error) {
	q, err := p.lastClose(ctx, ticker)
	if err != nil {
		return price.Quote{}, fmt.Errorf("yahoo price of %s: %w", ticker, err)
	}
	return q, nil
}

func (p *Provider) lastClose(ctx context.Context, ticker string) (price.Quote, error) {
	chart, err := p.chart(ctx, ticker, "range=5d&interval=1d")
	if err != nil {
		return price.Quote{}, err
	}
	if len(chart.Chart.Result) == 0 {
		return price.Quote{}, errNoResult
	}
	meta := chart.Chart.Result[0].Meta
	if meta.RegularMarketPrice == "" {
		return price.Quote{}, errors.New("the response has no price")
	}
	if meta.Currency == "" {
		return price.Quote{}, errors.New("the response has no currency")
	}
	if meta.RegularMarketTime == 0 {
		return price.Quote{}, errors.New("the response has no market time")
	}
	closePrice, err := decimal.NewFromString(meta.RegularMarketPrice.String())
	if err != nil {
		return price.Quote{}, fmt.Errorf("parse price %q: %w", meta.RegularMarketPrice, err)
	}

	return price.Quote{
		Close:    inUnit(closePrice, meta.Currency),
		Currency: unitCurrency(meta.Currency),
		Date:     exchangeDate(meta.RegularMarketTime, meta.GMTOffset),
		Source:   Source,
	}, nil
}

// errNoResult tells that the response has no result. Yahoo has no data for the symbol.
var errNoResult = fmt.Errorf("the response has no result: %w", price.ErrNoData)

// pence is the Yahoo currency of a London price in pence. The provider gives such a price in pounds (GBP).
const pence = "GBp"

// unitCurrency returns the currency of the prices that the provider gives for a Yahoo currency.
func unitCurrency(currency string) string {
	if currency == pence {
		return "GBP"
	}
	return currency
}

// inUnit returns the Yahoo price in the currency of unitCurrency: a price in pence divided by 100.
// It returns other prices as they are.
func inUnit(amount decimal.Decimal, currency string) decimal.Decimal {
	if currency == pence {
		return amount.Div(decimal.NewFromInt(100))
	}
	return amount
}

// exchangeDate returns the date of the Unix time in the exchange time, at midnight UTC.
// The exchange time is UTC plus the offset in seconds.
func exchangeDate(unix, gmtOffset int64) time.Time {
	y, m, d := time.Unix(unix+gmtOffset, 0).UTC().Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// DailyCloses returns the raw daily closes of the Yahoo symbol from the day from to the day to, and their currency.
// It skips a row with no close and the row of today in UTC, because that row is a live price and not a close.
// A London price in pence (GBp) comes back in pounds (GBP).
func (p *Provider) DailyCloses(ctx context.Context, ticker string, from, to time.Time) (price.History, error) {
	h, err := p.dailyCloses(ctx, ticker, from, to)
	if err != nil {
		return price.History{}, fmt.Errorf("yahoo daily prices of %s: %w", ticker, err)
	}
	return h, nil
}

func (p *Provider) dailyCloses(ctx context.Context, ticker string, from, to time.Time) (price.History, error) {
	now := p.now()
	// range=max gives monthly rows, so the request always has period1 and period2.
	// period1 is one day early, because a session that opens before midnight UTC has a timestamp of the day before.
	// period2 is now and not the last day, because the response has only the splits in the period.
	// ADR 0005: the raw close needs each split after the day.
	query := url.Values{
		"period1":  {strconv.FormatInt(from.AddDate(0, 0, -1).Unix(), 10)},
		"period2":  {strconv.FormatInt(now.Unix(), 10)},
		"interval": {"1d"},
		"events":   {"split"},
	}
	chart, err := p.chart(ctx, ticker, query.Encode())
	if err != nil {
		return price.History{}, err
	}
	if len(chart.Chart.Result) == 0 {
		return price.History{}, errNoResult
	}
	result := chart.Chart.Result[0]
	if g := result.Meta.DataGranularity; g != "1d" {
		return price.History{}, fmt.Errorf("the response has rows of %q, not of one day", g)
	}
	splits, err := splitDays(result.Events.Splits, result.Meta.GMTOffset)
	if err != nil {
		return price.History{}, err
	}
	var quoteCloses []*json.Number
	if len(result.Indicators.Quote) > 0 {
		quoteCloses = result.Indicators.Quote[0].Close
	}
	if len(quoteCloses) != len(result.Timestamp) {
		return price.History{}, fmt.Errorf("the response has %d timestamps and %d closes", len(result.Timestamp), len(quoteCloses))
	}

	today := price.StartOfDay(now)
	h := price.History{Currency: unitCurrency(result.Meta.Currency), Source: Source}
	for i, ts := range result.Timestamp {
		date := exchangeDate(ts, result.Meta.GMTOffset)
		if quoteCloses[i] == nil || date.Before(from) || date.After(to) || !date.Before(today) {
			continue
		}
		closePrice, err := decimal.NewFromString(quoteCloses[i].String())
		if err != nil {
			return price.History{}, fmt.Errorf("parse close %q of %s: %w", quoteCloses[i], date.Format(time.DateOnly), err)
		}
		// ADR 0005: multiply by the ratio of each split after the day.
		for _, s := range splits {
			if s.date.After(date) {
				closePrice = closePrice.Mul(s.ratio)
			}
		}
		h.Closes = append(h.Closes, price.DailyClose{Date: date, Close: inUnit(closePrice, result.Meta.Currency)})
	}
	if len(h.Closes) > 0 && h.Currency == "" {
		return price.History{}, errors.New("the response has no currency")
	}
	return h, nil
}

// splitDay is one split: its date in the exchange time and its ratio, numerator / denominator.
type splitDay struct {
	date  time.Time
	ratio decimal.Decimal
}

// splitDays returns the date and the ratio of each split.
func splitDays(splits map[string]split, gmtOffset int64) ([]splitDay, error) {
	days := make([]splitDay, 0, len(splits))
	for _, s := range splits {
		num, err := decimal.NewFromString(s.Numerator.String())
		if err != nil {
			return nil, fmt.Errorf("parse split numerator %q: %w", s.Numerator, err)
		}
		den, err := decimal.NewFromString(s.Denominator.String())
		if err != nil {
			return nil, fmt.Errorf("parse split denominator %q: %w", s.Denominator, err)
		}
		if !num.IsPositive() || !den.IsPositive() {
			return nil, fmt.Errorf("the split of %s has the ratio %s:%s", exchangeDate(s.Date, gmtOffset).Format(time.DateOnly), num, den)
		}
		days = append(days, splitDay{date: exchangeDate(s.Date, gmtOffset), ratio: num.Div(den)})
	}
	return days, nil
}

// now returns the time of the clock of the provider.
func (p *Provider) now() time.Time {
	if p.Now == nil {
		return time.Now()
	}
	return p.Now()
}

// chart sends the chart request for the symbol with the encoded query and decodes the response.
// It returns an error that wraps price.ErrTooManyRequests for HTTP 429,
// and an error that wraps price.ErrNoData for HTTP 404. Yahoo answers 404 for an unknown or a delisted symbol.
func (p *Provider) chart(ctx context.Context, ticker, rawQuery string) (chartResponse, error) {
	u := p.BaseURL + "/v8/finance/chart/" + url.PathEscape(ticker) + "?" + rawQuery
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return chartResponse{}, fmt.Errorf("make request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.Client.Do(req)
	if err != nil {
		return chartResponse{}, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		return chartResponse{}, fmt.Errorf("status %d: %w", resp.StatusCode, price.ErrTooManyRequests)
	}

	var chart chartResponse
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&chart)
	var statusErr error
	if e := chart.Chart.Error; decodeErr == nil && e != nil {
		statusErr = fmt.Errorf("status %d: %s: %s", resp.StatusCode, e.Code, e.Description)
	} else if resp.StatusCode != http.StatusOK {
		statusErr = fmt.Errorf("status %d", resp.StatusCode)
	}
	if statusErr != nil && resp.StatusCode == http.StatusNotFound {
		return chartResponse{}, fmt.Errorf("%w: %w", price.ErrNoData, statusErr)
	}
	if statusErr != nil {
		return chartResponse{}, statusErr
	}
	if decodeErr != nil {
		return chartResponse{}, fmt.Errorf("decode response: %w", decodeErr)
	}
	return chart, nil
}
