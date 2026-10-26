// Package nbp is an exchange.Provider that uses the API of the National Bank of Poland (NBP) for table A.
// All knowledge of the NBP API stays in this package.
package nbp

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
)

// DefaultBaseURL is the address of the NBP API.
const DefaultBaseURL = "https://api.nbp.pl"

// userAgent names the tool in each request. The NBP answers 403 to the default User-Agent of Go.
const userAgent = "profit-tracker"

// maxDays is the largest number of days in one request. The API answers 400 for a longer range.
const maxDays = 367

// maxBody is the largest response body that the provider reads. A range of 367 days is about 20 kilobytes.
const maxBody = 1 << 20

// Provider gets the Exchange Rates of NBP table A.
type Provider struct {
	// Client sends the requests. It must have a timeout.
	Client *http.Client
	// BaseURL is the address of the API, with no final slash.
	BaseURL string
}

// New returns a Provider for the NBP API with an HTTP client that stops a request after 15 seconds.
func New() *Provider {
	return &Provider{Client: &http.Client{Timeout: 15 * time.Second}, BaseURL: DefaultBaseURL}
}

// ratesResponse is the part of the answer that the provider reads.
type ratesResponse struct {
	Code  string `json:"code"`
	Rates []struct {
		// No is the table number, for example "204/A/NBP/2026".
		No string `json:"no"`
		// EffectiveDate is the table date as YYYY-MM-DD.
		EffectiveDate string `json:"effectiveDate"`
		// Mid is a JSON number. json.Number keeps its text, so no float64 rounding occurs.
		Mid json.Number `json:"mid"`
	} `json:"rates"`
}

// Rates returns the rates of table A for the currency with a table date from from to to, in date order.
// from and to are days at midnight UTC. A range with no table gives an empty list and no error.
// The provider splits a range into parts of at most 367 days.
func (p *Provider) Rates(ctx context.Context, currency string, from, to time.Time) ([]exchange.Rate, error) {
	var rates []exchange.Rate
	for start := from; !start.After(to); {
		end := start.AddDate(0, 0, maxDays-1)
		if end.After(to) {
			end = to
		}
		part, err := p.rates(ctx, currency, start, end)
		if err != nil {
			return nil, fmt.Errorf("nbp rates of %s from %s to %s: %w",
				currency, start.Format(time.DateOnly), end.Format(time.DateOnly), err)
		}
		rates = append(rates, part...)
		start = end.AddDate(0, 0, 1)
	}
	return rates, nil
}

// rates sends one request for a range of at most 367 days and reads the rates.
func (p *Provider) rates(ctx context.Context, currency string, from, to time.Time) ([]exchange.Rate, error) {
	u := p.BaseURL + "/api/exchangerates/rates/a/" + url.PathEscape(strings.ToLower(currency)) + "/" +
		from.Format(time.DateOnly) + "/" + to.Format(time.DateOnly) + "/?format=json"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("make request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "application/json")

	resp, err := p.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer resp.Body.Close()

	// The API answers 404 for a range with no table, for example a weekend.
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("status %d: %s", resp.StatusCode, errorText(resp.Body))
	}
	var body ratesResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&body); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if !strings.EqualFold(body.Code, currency) {
		return nil, fmt.Errorf("the response has the currency %q", body.Code)
	}
	return toRates(body)
}

// toRates reads the rates of the response.
func toRates(body ratesResponse) ([]exchange.Rate, error) {
	rates := make([]exchange.Rate, 0, len(body.Rates))
	for _, r := range body.Rates {
		date, err := time.Parse(time.DateOnly, r.EffectiveDate)
		if err != nil {
			return nil, fmt.Errorf("parse table date %q: %w", r.EffectiveDate, err)
		}
		if r.Mid == "" {
			return nil, fmt.Errorf("table %s has no rate", r.No)
		}
		mid, err := decimal.NewFromString(r.Mid.String())
		if err != nil {
			return nil, fmt.Errorf("table %s: parse rate %q: %w", r.No, r.Mid, err)
		}
		rates = append(rates, exchange.Rate{Date: date, Mid: mid, Table: r.No})
	}
	return rates, nil
}

// errorText returns the start of an error body, for example "400 BadRequest - Limit of 367 days has been exceeded".
func errorText(body io.Reader) string {
	data, err := io.ReadAll(io.LimitReader(body, 200))
	if err != nil || len(data) == 0 {
		return "no error text"
	}
	return strings.TrimSpace(string(data))
}
