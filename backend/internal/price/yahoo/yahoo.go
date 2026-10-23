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
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/price"
)

// Source is the name of this provider in the stored prices.
const Source = "yahoo"

// DefaultBaseURL is the address of the Yahoo Finance API.
const DefaultBaseURL = "https://query1.finance.yahoo.com"

// userAgent is a browser User-Agent. Yahoo refuses the default User-Agent of Go.
const userAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/141.0 Safari/537.36"

// maxBody is the largest response body that the provider reads. A chart for 5 days is a few kilobytes.
const maxBody = 1 << 20

// Provider gets the last close price from the Yahoo Finance chart endpoint.
type Provider struct {
	// Client sends the requests. It must have a timeout.
	Client *http.Client
	// BaseURL is the address of the API, with no final slash.
	BaseURL string
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
			} `json:"meta"`
		} `json:"result"`
		Error *struct {
			Code        string `json:"code"`
			Description string `json:"description"`
		} `json:"error"`
	} `json:"chart"`
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
	chart, err := p.chart(ctx, ticker)
	if err != nil {
		return price.Quote{}, err
	}
	if len(chart.Chart.Result) == 0 {
		return price.Quote{}, errors.New("the response has no result")
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

	currency := meta.Currency
	if currency == "GBp" {
		closePrice, currency = closePrice.Div(decimal.NewFromInt(100)), "GBP"
	}
	// The exchange time is UTC plus the offset. Its date is the price date.
	y, m, d := time.Unix(meta.RegularMarketTime+meta.GMTOffset, 0).UTC().Date()
	return price.Quote{
		Close:    closePrice,
		Currency: currency,
		Date:     time.Date(y, m, d, 0, 0, 0, 0, time.UTC),
		Source:   Source,
	}, nil
}

// chart sends the chart request for the symbol and decodes the response.
func (p *Provider) chart(ctx context.Context, ticker string) (chartResponse, error) {
	u := p.BaseURL + "/v8/finance/chart/" + url.PathEscape(ticker) + "?range=5d&interval=1d"
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

	var chart chartResponse
	decodeErr := json.NewDecoder(io.LimitReader(resp.Body, maxBody)).Decode(&chart)
	if e := chart.Chart.Error; decodeErr == nil && e != nil {
		return chartResponse{}, fmt.Errorf("status %d: %s: %s", resp.StatusCode, e.Code, e.Description)
	}
	if resp.StatusCode != http.StatusOK {
		return chartResponse{}, fmt.Errorf("status %d", resp.StatusCode)
	}
	if decodeErr != nil {
		return chartResponse{}, fmt.Errorf("decode response: %w", decodeErr)
	}
	return chart, nil
}
