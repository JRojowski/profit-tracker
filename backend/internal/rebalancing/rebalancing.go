// Package rebalancing calculates the Rebalancing table: the value, the Actual Share, the Target Value, the Gap and
// the Planned Purchase of each Tier, each Instrument Target and each Shared Part. See GLOSSARY.md, section
// "Rebalancing", and spec #103, section "Rebalancing API". It is a pure calculation with no I/O.
package rebalancing

import (
	"cmp"
	"slices"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/tier"
)

// The currencies of the Tier Accounts.
const (
	PLN = "PLN"
	USD = "USD"
)

// Account is one Tier Account.
type Account struct {
	ID       int64
	Currency string
	// Rate is the PLN amount for 1 unit of the Account currency: 1 for PLN, the latest Exchange Rate for USD.
	Rate decimal.Decimal
	// Cash is the cash for Rebalancing in the Account currency.
	Cash decimal.Decimal
	// Holdings are the Holdings of the Account.
	Holdings []Holding
}

// Holding is one Holding of a Tier Account.
type Holding struct {
	Ticker string
	// HasValue tells that the Holding has a price. A Holding with no price is not part of any value.
	HasValue bool
	// Value is in the Account currency.
	Value decimal.Decimal
}

// Instrument holds the data of an Instrument that the calculation uses.
type Instrument struct {
	Name          string
	QuoteCurrency string
}

// Input is the input of the calculation.
type Input struct {
	// Accounts are the Tier Accounts in the order of the Accounts list.
	Accounts []Account
	// Tiers are the Tiers in order number order.
	Tiers []tier.Tier
	// Instruments are the Instruments by broker ticker.
	Instruments map[string]Instrument
}

// Part holds the amounts of a Tier, an Instrument Target, a Shared Part or the "No Tier" group.
// The amounts are in PLN. The shares are percentages.
type Part struct {
	Value decimal.Decimal
	// ActualShare is Value × 100 / the value of the Tier, or of all priced Holdings for a Tier.
	ActualShare decimal.Decimal
	TargetShare decimal.Decimal
	TargetValue decimal.Decimal
	// Gap is TargetValue − Value. It can be negative.
	Gap decimal.Decimal
}

// Purchase is a Planned Purchase in the currency of its Account.
type Purchase struct {
	// HasAccount is false when the tool has no Tier Account in Currency. Then AccountID and Amount are zero.
	HasAccount bool
	AccountID  int64
	Currency   string
	Amount     decimal.Decimal
}

// Target is an Instrument Target.
type Target struct {
	Ticker, Name string
	// NoValue tells that the Instrument has Holdings, but none with a price.
	NoValue bool
	Part
	Purchase Purchase
}

// SharedPart is the Shared Part of a Tier.
type SharedPart struct {
	Name string
	Part
	Purchase Purchase
	// Instruments are the Instruments of the Shared Part with a Holding, in ticker order.
	Instruments []GroupHolding
}

// GroupHolding is a Holding in a Shared Part or in the "No Tier" group, with all its Accounts together.
type GroupHolding struct {
	Ticker, Name string
	// NoValue tells that the Instrument has no Holding with a price. Then Value and Share are zero.
	NoValue bool
	// Value is in PLN. Share is Value × 100 / the value of the group.
	Value, Share decimal.Decimal
}

// Tier is one Tier of the table.
type Tier struct {
	Order int
	Name  string
	Part
	// Purchases are the sums of the Planned Purchases of the Tier, one for each purchase Account, in the order of
	// the Accounts.
	Purchases         []Purchase
	InstrumentTargets []Target
	SharedPart        SharedPart
}

// NoTier is the group of the Holdings with no Tier. Its Target Share is 0.
type NoTier struct {
	Part
	// Holdings are in ticker order.
	Holdings []GroupHolding
}

// AccountResult is the cash for Rebalancing of one Tier Account and how the plan uses it.
type AccountResult struct {
	ID       int64
	Currency string
	Cash     decimal.Decimal
	// PlannedPurchases is the sum of the Planned Purchases of the Account. UnusedCash is Cash − PlannedPurchases.
	PlannedPurchases, UnusedCash decimal.Decimal
}

// Unpriced is a Holding with no price.
type Unpriced struct {
	AccountID int64
	Ticker    string
}

// Result is the Rebalancing table.
type Result struct {
	// Value is the value of all priced Holdings in PLN. Base is Value plus the cash for Rebalancing in PLN.
	Value, Base decimal.Decimal
	Accounts    []AccountResult
	Tiers       []Tier
	NoTier      NoTier
	// Unpriced are the Holdings with no price, in the order of the Accounts.
	Unpriced []Unpriced
}

var hundred = decimal.NewFromInt(100)

// Calculate returns the Rebalancing table of the input.
//
// One Instrument on two Accounts counts as one Holding. A Holding with no price is not part of any value, share or
// Gap. An Instrument Target with a Quote Currency other than USD uses the first PLN Account for purchases. All other
// parts use the first USD Account.
func Calculate(in Input) Result {
	result := Result{Accounts: make([]AccountResult, 0, len(in.Accounts)), Unpriced: []Unpriced{}}

	values := map[string]decimal.Decimal{}
	priced := map[string]bool{}
	held := map[string]bool{}
	cash := decimal.Zero
	buyers := map[string]Account{}
	for _, acc := range in.Accounts {
		if _, ok := buyers[acc.Currency]; !ok {
			buyers[acc.Currency] = acc
		}
		cash = cash.Add(acc.Cash.Mul(acc.Rate))
		for _, h := range acc.Holdings {
			held[h.Ticker] = true
			if !h.HasValue {
				result.Unpriced = append(result.Unpriced, Unpriced{AccountID: acc.ID, Ticker: h.Ticker})
				continue
			}
			priced[h.Ticker] = true
			values[h.Ticker] = values[h.Ticker].Add(h.Value.Mul(acc.Rate))
		}
	}
	for _, v := range values {
		result.Value = result.Value.Add(v)
	}
	result.Base = result.Value.Add(cash)

	purchaseOf := func(currency string) Purchase {
		p := Purchase{Currency: currency}
		if acc, ok := buyers[currency]; ok {
			p.HasAccount, p.AccountID = true, acc.ID
		}
		return p
	}

	assigned := map[string]bool{}
	result.Tiers = make([]Tier, 0, len(in.Tiers))
	for _, t := range in.Tiers {
		row := Tier{Order: t.Order, Name: t.Name, InstrumentTargets: []Target{}}
		row.TargetShare = t.TargetShare
		row.TargetValue = percentOf(result.Base, t.TargetShare)
		var shared []string
		for _, a := range t.Instruments {
			assigned[a.Ticker] = true
			row.Value = row.Value.Add(values[a.Ticker])
			if !a.TargetShare.Valid {
				shared = append(shared, a.Ticker)
			}
		}
		row.Gap = row.TargetValue.Sub(row.Value)
		row.ActualShare = share(row.Value, result.Value)

		for _, a := range t.Instruments {
			if !a.TargetShare.Valid {
				continue
			}
			inst := in.Instruments[a.Ticker]
			target := Target{Ticker: a.Ticker, Name: inst.Name, NoValue: held[a.Ticker] && !priced[a.Ticker]}
			target.Part = partOf(values[a.Ticker], row.Value, row.TargetValue, a.TargetShare.Decimal)
			// The account rule: an Instrument Target with a Quote Currency other than USD uses the PLN Account.
			currency := USD
			if inst.QuoteCurrency != USD {
				currency = PLN
			}
			target.Purchase = purchaseOf(currency)
			row.InstrumentTargets = append(row.InstrumentTargets, target)
		}

		sp := SharedPart{Name: t.SharedPartName, Instruments: []GroupHolding{}}
		sharedValue := decimal.Zero
		for _, ticker := range shared {
			sharedValue = sharedValue.Add(values[ticker])
		}
		sp.Part = partOf(sharedValue, row.Value, row.TargetValue, t.SharedPartShare())
		sp.Purchase = purchaseOf(USD)
		sp.Instruments = instrumentsOf(shared, held, priced, values, sharedValue, in.Instruments)
		row.SharedPart = sp
		result.Tiers = append(result.Tiers, row)
	}

	planned := plan(result.Tiers, buyers)
	for i := range result.Tiers {
		row := &result.Tiers[i]
		row.Purchases = []Purchase{}
		for _, acc := range in.Accounts {
			if buyers[acc.Currency].ID != acc.ID {
				continue
			}
			sum := Purchase{HasAccount: true, AccountID: acc.ID, Currency: acc.Currency}
			eachPart(row, func(_ decimal.Decimal, p *Purchase) {
				if p.HasAccount && p.AccountID == acc.ID {
					sum.Amount = sum.Amount.Add(p.Amount)
				}
			})
			row.Purchases = append(row.Purchases, sum)
		}
	}

	for _, acc := range in.Accounts {
		result.Accounts = append(result.Accounts, AccountResult{
			ID: acc.ID, Currency: acc.Currency, Cash: acc.Cash,
			PlannedPurchases: planned[acc.ID], UnusedCash: acc.Cash.Sub(planned[acc.ID]),
		})
	}

	var unassigned []string
	for ticker := range held {
		if !assigned[ticker] {
			unassigned = append(unassigned, ticker)
		}
	}
	slices.Sort(unassigned)
	noTierValue := decimal.Zero
	for _, ticker := range unassigned {
		noTierValue = noTierValue.Add(values[ticker])
	}
	result.NoTier = NoTier{
		Part: Part{
			Value: noTierValue, ActualShare: share(noTierValue, result.Value), TargetShare: decimal.Zero,
			TargetValue: decimal.Zero, Gap: noTierValue.Neg(),
		},
		Holdings: instrumentsOf(unassigned, held, priced, values, noTierValue, in.Instruments),
	}
	return result
}

// plan sets the Planned Purchase of each Instrument Target and each Shared Part. It returns the sum of the Planned
// Purchases of each Account. buyers are the Accounts for purchases by currency.
//
// For each Account, the factor is its cash divided by the sum of its positive Gaps, or 1 when the sum is not more
// than the cash. The Planned Purchase is the positive Gap × the factor, in the Account currency.
func plan(tiers []Tier, buyers map[string]Account) map[int64]decimal.Decimal {
	positive := map[int64]decimal.Decimal{}
	for i := range tiers {
		eachPart(&tiers[i], func(gap decimal.Decimal, p *Purchase) {
			if p.HasAccount && gap.IsPositive() {
				positive[p.AccountID] = positive[p.AccountID].Add(gap.Div(buyers[p.Currency].Rate))
			}
		})
	}
	planned := map[int64]decimal.Decimal{}
	for i := range tiers {
		eachPart(&tiers[i], func(gap decimal.Decimal, p *Purchase) {
			if !p.HasAccount || !gap.IsPositive() {
				return
			}
			acc, sum := buyers[p.Currency], positive[p.AccountID]
			p.Amount = gap.Div(acc.Rate)
			if sum.GreaterThan(acc.Cash) {
				p.Amount = p.Amount.Mul(decimal.Max(acc.Cash, decimal.Zero)).Div(sum)
			}
			planned[p.AccountID] = planned[p.AccountID].Add(p.Amount)
		})
	}
	return planned
}

// eachPart calls fn with the Gap and the Planned Purchase of each Instrument Target and of the Shared Part of the Tier.
func eachPart(t *Tier, fn func(gap decimal.Decimal, p *Purchase)) {
	for i := range t.InstrumentTargets {
		fn(t.InstrumentTargets[i].Gap, &t.InstrumentTargets[i].Purchase)
	}
	fn(t.SharedPart.Gap, &t.SharedPart.Purchase)
}

// partOf returns the amounts of an Instrument Target or a Shared Part in a Tier.
func partOf(value, tierValue, tierTargetValue, targetShare decimal.Decimal) Part {
	p := Part{Value: value, ActualShare: share(value, tierValue), TargetShare: targetShare}
	p.TargetValue = percentOf(tierTargetValue, targetShare)
	p.Gap = p.TargetValue.Sub(value)
	return p
}

// instrumentsOf returns the tickers with a Holding as a list in ticker order, with the share of the group value.
func instrumentsOf(tickers []string, held, priced map[string]bool, values map[string]decimal.Decimal,
	groupValue decimal.Decimal, instruments map[string]Instrument,
) []GroupHolding {
	list := []GroupHolding{}
	for _, ticker := range tickers {
		if !held[ticker] {
			continue
		}
		item := GroupHolding{Ticker: ticker, Name: instruments[ticker].Name, NoValue: !priced[ticker]}
		if priced[ticker] {
			item.Value = values[ticker]
			item.Share = share(item.Value, groupValue)
		}
		list = append(list, item)
	}
	slices.SortFunc(list, func(a, b GroupHolding) int { return cmp.Compare(a.Ticker, b.Ticker) })
	return list
}

// percentOf returns amount × percentage / 100.
func percentOf(amount, percentage decimal.Decimal) decimal.Decimal {
	return amount.Mul(percentage).Div(hundred)
}

// share returns value × 100 / total, or zero if total is zero.
func share(value, total decimal.Decimal) decimal.Decimal {
	if total.IsZero() {
		return decimal.Zero
	}
	return value.Mul(hundred).Div(total)
}
