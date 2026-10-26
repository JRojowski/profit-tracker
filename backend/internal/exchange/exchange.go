// Package exchange gets the Exchange Rates of NBP table A from an Exchange Rate provider and stores them.
// The store keeps one rate per currency per table date, and the range of days that the tool asked the NBP for.
package exchange

import (
	"context"
	"time"

	// The binary must not depend on the zone files of the system for the Polish time.
	_ "time/tzdata"

	"github.com/shopspring/decimal"
)

// Rate is one Exchange Rate of NBP table A.
type Rate struct {
	// Date is the table date in Polish time, at midnight UTC.
	Date time.Time
	// Mid is the NBP average rate: the PLN amount for 1 unit of the currency.
	Mid decimal.Decimal
	// Table is the NBP table number, for example "204/A/NBP/2026".
	Table string
}

// Provider gets Exchange Rates from a rate source.
type Provider interface {
	// Rates returns the rates of the currency, for example "USD", with a table date from from to to.
	// from and to are days at midnight UTC, and the range includes both. A range with no table gives
	// an empty list and no error.
	Rates(ctx context.Context, currency string, from, to time.Time) ([]Rate, error)
}

// warsaw is the Polish time zone. The NBP publishes its tables in this time zone.
var warsaw = loadWarsaw()

// loadWarsaw returns the Polish time zone. The package embeds the zone data with time/tzdata,
// so an error tells that the build is broken.
func loadWarsaw() *time.Location {
	loc, err := time.LoadLocation("Europe/Warsaw")
	if err != nil {
		panic("load the Europe/Warsaw time zone: " + err.Error())
	}
	return loc
}

// DayInWarsaw returns the day of t in Polish time, at midnight UTC. A table date has the same form.
func DayInWarsaw(t time.Time) time.Time {
	y, m, d := t.In(warsaw).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// StartOfDayInWarsaw returns the start of the day of t in Polish time.
func StartOfDayInWarsaw(t time.Time) time.Time {
	y, m, d := t.In(warsaw).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, warsaw)
}
