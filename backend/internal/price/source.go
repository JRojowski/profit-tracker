package price

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// maxPriceTickerLength is the maximum number of characters in a manual price ticker.
const maxPriceTickerLength = 32

var (
	// ErrInvalidPriceTicker tells that a manual price ticker has whitespace in it or is too long.
	ErrInvalidPriceTicker = errors.New("invalid price ticker")
	// ErrInvalidQuoteCurrency tells that a manual quote currency is not three letters A to Z.
	ErrInvalidQuoteCurrency = errors.New("invalid quote currency")
)

// SetPriceTicker sets the manual price ticker of the Instrument with the broker ticker and returns the Instrument.
// It removes the whitespace at the start and at the end. An empty value removes the manual price ticker,
// and the tool uses the default mapping again. It does not get a price.
//
// If the price ticker that the tool uses changes, SetPriceTicker deletes the stored prices of the Instrument.
// They came from the old price ticker, so the tool must not show them.
// It returns ErrInvalidPriceTicker for a value that is not valid, and store.ErrNotFound for an unknown Instrument.
func (s *Service) SetPriceTicker(ctx context.Context, brokerTicker, priceTicker string) (store.Instrument, error) {
	priceTicker = strings.TrimSpace(priceTicker)
	if err := checkPriceTicker(priceTicker); err != nil {
		return store.Instrument{}, err
	}
	// A refresh that runs reads the price ticker at its start and stores the price at its end.
	// The lock stops the refresh from storing a price of the old price ticker after the change.
	if err := s.acquire(ctx); err != nil {
		return store.Instrument{}, err
	}
	defer s.release()

	old, err := s.store.Instrument(ctx, brokerTicker)
	if err != nil {
		return store.Instrument{}, err
	}
	changed := EffectiveTicker(brokerTicker, old.PriceTicker) != EffectiveTicker(brokerTicker, priceTicker)
	return s.store.SetPriceTicker(ctx, brokerTicker, priceTicker, changed)
}

// checkPriceTicker returns ErrInvalidPriceTicker if the trimmed price ticker has whitespace in it or is too long.
func checkPriceTicker(priceTicker string) error {
	if n := utf8.RuneCountInString(priceTicker); n > maxPriceTickerLength {
		return fmt.Errorf("%w: %d characters, the maximum is %d", ErrInvalidPriceTicker, n, maxPriceTickerLength)
	}
	if strings.ContainsFunc(priceTicker, unicode.IsSpace) {
		return fmt.Errorf("%w: %q has whitespace in it", ErrInvalidPriceTicker, priceTicker)
	}
	return nil
}

// SetQuoteCurrency sets the manual quote currency of the Instrument with the broker ticker and returns the Instrument.
// It removes the whitespace at the start and at the end and changes the letters to upper case.
// An empty value removes the manual quote currency. Then the quote currency comes from the ticker suffix again,
// and it is not known for an unknown suffix.
//
// SetQuoteCurrency keeps the stored prices. The Holdings use only a price in the quote currency of the Instrument.
// It returns ErrInvalidQuoteCurrency for a value that is not valid, and store.ErrNotFound for an unknown Instrument.
func (s *Service) SetQuoteCurrency(ctx context.Context, brokerTicker, quoteCurrency string) (store.Instrument, error) {
	quoteCurrency = strings.ToUpper(strings.TrimSpace(quoteCurrency))
	if quoteCurrency == "" {
		return s.store.SetQuoteCurrency(ctx, brokerTicker, QuoteCurrency(brokerTicker), false)
	}
	if !isCurrencyCode(quoteCurrency) {
		return store.Instrument{}, fmt.Errorf("%w: %q is not three letters A to Z", ErrInvalidQuoteCurrency, quoteCurrency)
	}
	return s.store.SetQuoteCurrency(ctx, brokerTicker, quoteCurrency, true)
}

// isCurrencyCode tells if code has the form of an ISO currency code: three letters A to Z.
func isCurrencyCode(code string) bool {
	if len(code) != 3 {
		return false
	}
	for _, c := range []byte(code) {
		if c < 'A' || c > 'Z' {
			return false
		}
	}
	return true
}
