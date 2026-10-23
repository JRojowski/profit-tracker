// Package price gets the current prices of the Instruments in Holdings from a price provider and stores them.
// The store keeps one close price per Instrument per price date.
package price

import (
	"context"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// Quote is the last close price of one price ticker.
type Quote struct {
	// Close is the close price in Currency.
	Close decimal.Decimal
	// Currency is the ISO code of the currency of Close, for example "USD".
	Currency string
	// Date is the price date in the exchange time, at midnight UTC.
	Date time.Time
	// Source is the name of the provider, for example "yahoo".
	Source string
}

// Provider gets prices from a price source.
type Provider interface {
	// LastClose returns the last close price of the price ticker, for example "AAPL" or "VUSA.L".
	LastClose(ctx context.Context, ticker string) (Quote, error)
}

// market is what the suffix of a broker ticker tells about the Instrument.
type market struct {
	// quoteCurrency is the currency of the price at the exchange.
	quoteCurrency string
	// priceSuffix replaces the broker suffix in the price ticker.
	priceSuffix string
}

// markets maps the suffix of a broker ticker to its market.
var markets = map[string]market{
	".US": {quoteCurrency: "USD", priceSuffix: ""},
	".UK": {quoteCurrency: "GBP", priceSuffix: ".L"},
	".PL": {quoteCurrency: "PLN", priceSuffix: ".WA"},
	".DE": {quoteCurrency: "EUR", priceSuffix: ".DE"},
}

// QuoteCurrency returns the quote currency of the Instrument from the suffix of its broker ticker.
// It returns an empty string for an unknown suffix.
func QuoteCurrency(brokerTicker string) string {
	m, _, ok := marketOf(brokerTicker)
	if !ok {
		return ""
	}
	return m.quoteCurrency
}

// Ticker returns the price ticker of the broker ticker by the default mapping,
// for example "AAPL" for "AAPL.US" and "VUSA.L" for "VUSA.UK".
// It returns the broker ticker for an unknown suffix.
func Ticker(brokerTicker string) string {
	m, base, ok := marketOf(brokerTicker)
	if !ok {
		return brokerTicker
	}
	return base + m.priceSuffix
}

// marketOf returns the market of the broker ticker and the ticker with no suffix.
func marketOf(brokerTicker string) (market, string, bool) {
	dot := strings.LastIndexByte(brokerTicker, '.')
	if dot <= 0 {
		return market{}, "", false
	}
	m, ok := markets[brokerTicker[dot:]]
	return m, brokerTicker[:dot], ok
}
