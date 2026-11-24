// Package tier holds the Tiers of Rebalancing and the rules of their configuration.
// See GLOSSARY.md, section "Rebalancing". The package has no HTTP and no SQL.
package tier

import (
	"github.com/shopspring/decimal"
)

// Tier is a named part of the Holdings with a Target Share and an order number.
type Tier struct {
	// Order is the order number of the Tier. It is unique.
	Order int
	// Name is the name of the Tier. It is unique.
	Name string
	// TargetShare is the share of the Tier in the value of all Holdings, as a percentage from 0 to 100.
	TargetShare decimal.Decimal
	// SharedPartName is the name of the Shared Part of the Tier. It is empty if the user gave none.
	SharedPartName string
	// Instruments are the Instruments of the Tier, in ticker order.
	Instruments []Assignment
}

// Assignment puts an Instrument in a Tier.
type Assignment struct {
	Ticker string
	// TargetShare is the share of the Instrument in the Tier. It makes the Instrument an Instrument Target.
	// It is not valid for an Instrument in the Shared Part.
	TargetShare decimal.NullDecimal
}

// IsTierAccount tells if an Account is a Tier Account: a regular Account of the Provider XTB in PLN or USD.
// Rebalancing and the Tiers use only these Accounts.
func IsTierAccount(accountType, provider, currency string) bool {
	return accountType == "regular" && provider == "xtb" && (currency == "PLN" || currency == "USD")
}

var hundred = decimal.NewFromInt(100)

// SharedPartShare returns the Target Share of the Shared Part: 100 minus the Instrument Targets of the Tier.
func (t Tier) SharedPartShare() decimal.Decimal {
	share := hundred
	for _, a := range t.Instruments {
		if a.TargetShare.Valid {
			share = share.Sub(a.TargetShare.Decimal)
		}
	}
	return share
}
