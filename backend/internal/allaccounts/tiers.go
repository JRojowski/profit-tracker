package allaccounts

import (
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
)

// Results are the Holdings and the Closed Positions of one Account in PLN, by ADR 0010.
type Results struct {
	// Available tells if the tool has each Exchange Rate that the conversion needs.
	// If it is false, Holdings and ClosedPositions are nil.
	Available bool
	// NoLatestRate tells that the tool has no latest rate of the Account currency.
	NoLatestRate bool
	// MissingDays are the times of the Fills and the sales with no rate on their day.
	MissingDays     []time.Time
	Holdings        []ledger.Holding
	ClosedPositions []ledger.ClosedPosition
}

// ResultsInPLN converts the Holdings and the Closed Positions of the Account to PLN, by ADR 0010.
// lots are the Lots of the Ledger of the Account, also the closed Lots.
//
// It does not convert the amounts of a PLN Account. For an Account in another currency:
//   - The value of a Holding uses the latest rate.
//   - The cost of a Lot, or of a part of a Lot, is its cost × the average rate of the Fills of the Lot.
//     The Fill amounts are the weights. Each Fill uses the rate of its day in Polish time.
//   - The sale amount of a Closed Position uses the rate of the sale day in Polish time.
//   - The Unrealized Profit and the Realized Profit are the differences of the amounts in PLN.
func ResultsInPLN(acc Account, lots []ledger.Lot, rates Rates) Results {
	s := acc.State
	if acc.Currency == ledger.PLN {
		return Results{Available: true, Holdings: s.Holdings, ClosedPositions: s.ClosedPositions}
	}
	// Only the value of a Holding uses the latest rate.
	latest, ok := rates.Latest(acc.Currency)
	if !ok && len(s.Holdings) > 0 {
		return Results{NoLatestRate: true}
	}

	c := converter{currency: acc.Currency, rates: rates, factors: map[string]costFactor{}}
	for _, l := range lots {
		c.factors[l.PositionID] = c.factorOf(l.Fills)
	}

	openCost := map[string]decimal.Decimal{}
	for _, l := range s.OpenLots {
		openCost[l.Instrument.Ticker] = openCost[l.Instrument.Ticker].Add(c.costInPLN(l.PositionID, l.Cost))
	}
	holdings := make([]ledger.Holding, 0, len(s.Holdings))
	for _, h := range s.Holdings {
		h.Cost = openCost[h.Instrument.Ticker]
		if h.HasValue() {
			// ADR 0010, rules 2 and 6.
			h.Value = h.Value.Mul(latest)
			h.UnrealizedProfit = h.Value.Sub(h.Cost)
		}
		holdings = append(holdings, h)
	}

	closed := make([]ledger.ClosedPosition, 0, len(s.ClosedPositions))
	for _, p := range s.ClosedPositions {
		// ADR 0010, rules 5 and 7.
		p.Amount = p.Amount.Mul(c.rateOn(p.Time))
		p.Cost = c.costInPLN(p.PositionID, p.Cost)
		p.RealizedProfit = p.Amount.Sub(p.Cost)
		closed = append(closed, p)
	}

	if len(c.missing) > 0 {
		return Results{MissingDays: c.missing}
	}
	return Results{Available: true, Holdings: holdings, ClosedPositions: closed}
}

// costFactor is the average rate of the Fills of a Lot, weighted by the Fill amount, as a fraction:
// the sum of the Fill amounts × their rates, and the sum of the Fill amounts.
type costFactor struct {
	weighted, amount decimal.Decimal
}

// converter converts the amounts of one Account and collects the days with no rate.
type converter struct {
	currency string
	rates    Rates
	// factors are the cost factors of the Lots by Lot ID.
	factors map[string]costFactor
	missing []time.Time
}

// rateOn returns the rate of the day of t. If the rate is missing, it records t and returns 0.
func (c *converter) rateOn(t time.Time) decimal.Decimal {
	rate, ok := c.rates.On(c.currency, t)
	if !ok {
		c.missing = append(c.missing, t)
		return decimal.Zero
	}
	return rate
}

// factorOf returns the cost factor of the Fills. ADR 0010, rules 3 and 4.
func (c *converter) factorOf(fills []ledger.Fill) costFactor {
	var f costFactor
	for _, fill := range fills {
		amount := fill.Amount.Abs()
		f.weighted = f.weighted.Add(amount.Mul(c.rateOn(fill.Time)))
		f.amount = f.amount.Add(amount)
	}
	return f
}

// costInPLN returns the cost × the average rate of the Fills of the Lot. The multiplication comes first,
// so an exact result stays exact. A Lot with no Fill amount has no cost.
func (c *converter) costInPLN(positionID string, cost decimal.Decimal) decimal.Decimal {
	f := c.factors[positionID]
	if f.amount.IsZero() {
		return decimal.Zero
	}
	return cost.Mul(f.weighted).Div(f.amount)
}
