package tier

import (
	"fmt"
	"strings"

	"github.com/shopspring/decimal"
)

// NoTier is the TierIndex of an Error that belongs to no single Tier.
const NoTier = -1

// Error is one mistake in a configuration of Tiers.
type Error struct {
	// TierIndex is the index of the Tier with the mistake in the configuration, or NoTier.
	TierIndex int
	// Ticker is the ticker of the Instrument with the mistake. It is empty if the mistake is not about one Instrument.
	Ticker string
	// Message is a sentence for the user. It names the Tier.
	Message string
}

// Validate checks a full configuration of Tiers and returns each mistake. It returns nil for a valid configuration.
// An empty configuration is valid. Otherwise the Target Shares of the Tiers must add up to 100.
// The rules:
//   - A Tier has a name. The names are unique. The order numbers are unique.
//   - Each Target Share is from 0 to 100.
//   - The Instrument Targets of one Tier add up to 100 or less, so that the Shared Part keeps a share of 0 or more.
//   - Each ticker is known (known returns true) and is in one Tier only.
//
// The caller trims the names.
func Validate(tiers []Tier, known func(ticker string) bool) []Error {
	var errs []Error
	orderOwner := map[int]int{}
	nameOwner := map[string]int{}
	tickerOwner := map[string]int{}
	sum := decimal.Zero

	for i, t := range tiers {
		add := func(ticker, format string, args ...any) {
			errs = append(errs, Error{TierIndex: i, Ticker: ticker, Message: fmt.Sprintf(format, args...)})
		}
		label := fmt.Sprintf("Tier %q", t.Name)
		sum = sum.Add(t.TargetShare)

		if strings.TrimSpace(t.Name) == "" {
			add("", "Tier %d: the name is empty.", t.Order)
		} else if first, ok := nameOwner[t.Name]; ok {
			add("", "%s: the name is also the name of Tier %d.", label, tiers[first].Order)
		} else {
			nameOwner[t.Name] = i
		}
		if first, ok := orderOwner[t.Order]; ok {
			add("", "%s: the order number %d is also the order number of Tier %q.", label, t.Order, tiers[first].Name)
		} else {
			orderOwner[t.Order] = i
		}
		if !inRange(t.TargetShare) {
			add("", "%s: the Target Share %s is not from 0 to 100.", label, t.TargetShare)
		}

		targets := decimal.Zero
		for _, a := range t.Instruments {
			if !known(a.Ticker) {
				add(a.Ticker, "%s: %s is not a known Instrument.", label, a.Ticker)
			}
			if first, ok := tickerOwner[a.Ticker]; ok {
				add(a.Ticker, "%s: %s is already in Tier %q.", label, a.Ticker, tiers[first].Name)
			} else {
				tickerOwner[a.Ticker] = i
			}
			if a.TargetShare.Valid {
				targets = targets.Add(a.TargetShare.Decimal)
				if !inRange(a.TargetShare.Decimal) {
					add(a.Ticker, "%s: the Target Share %s of %s is not from 0 to 100.", label, a.TargetShare.Decimal, a.Ticker)
				}
			}
		}
		if targets.GreaterThan(hundred) {
			add("", "%s: the Instrument Targets add up to %s, more than 100.", label, targets)
		}
	}

	if len(tiers) > 0 && !sum.Equal(hundred) {
		errs = append(errs, Error{TierIndex: NoTier, Message: fmt.Sprintf("The Target Shares of the Tiers add up to %s, not 100.", sum)})
	}
	return errs
}

// inRange tells if the Target Share is from 0 to 100.
func inRange(share decimal.Decimal) bool {
	return !share.IsNegative() && !share.GreaterThan(hundred)
}
