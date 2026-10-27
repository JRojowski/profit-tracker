// Package valuehistory calculates the Value History of one Account.
// It is the value and the Net Deposits at the end of each calendar day, in the Account currency. See GLOSSARY.md, "Value History".
// It is a pure calculation with no I/O. The tool calculates the Value History on each request and does not store it.
package valuehistory

import (
	"cmp"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/returns"
)

// Input is the input of Calculate.
type Input struct {
	// Ledger is the input of the hero: the Cash Operations, the Lots, the Quote Currencies, the current prices,
	// the latest Exchange Rates and Today. Today is the start of today in Polish time. Calculate takes the day of
	// each Cash Operation in the location of Today, as the Annual Return does.
	Ledger ledger.Ledger
	// Closes are the stored prices of each Instrument in its Quote Currency, in date order, by broker ticker.
	// Give only the prices in the Quote Currency of the Instrument.
	Closes map[string][]Close
	// ExchangeRates are the stored Exchange Rates of each currency in date order, by currency.
	ExchangeRates map[string][]Rate
}

// Close is the close price of an Instrument on one day, in its Quote Currency.
type Close struct {
	// Date is the price date at midnight UTC.
	Date  time.Time
	Price decimal.Decimal
}

// Rate is one Exchange Rate of a currency against PLN.
type Rate struct {
	// Date is the NBP table date at midnight UTC.
	Date time.Time
	// Mid is the PLN amount for 1 unit of the currency.
	Mid decimal.Decimal
}

// Point is the state of the Account at the end of one calendar day.
type Point struct {
	// DayValue is the day, the value and the flow of the day. The value is the cash balance plus the value
	// of the Holdings, in the Account currency. It is the input of returns.TimeWeighted.
	returns.DayValue
	// NetDeposits are the Net Deposits at the end of the day.
	NetDeposits decimal.Decimal
}

// History is the result of Calculate.
type History struct {
	// Points has one point for each calendar day from the day of the first Cash Operation to today.
	// It is empty if the Ledger has no Cash Operation.
	Points []Point
	// ValuedAtCost are the Instruments that Calculate valued at the cost of their open Lots on one or more days,
	// in ticker order.
	ValuedAtCost []ledger.Instrument
}

// Calculate returns the Value History of the Account.
//
// It makes one pass over the days and the Cash Operations. It values each Holding of a past day at the last
// stored price on that day or before it. It converts the value at the last Exchange Rate on that day or before it.
// It values a Holding at the cost of its open Lots in these cases:
//   - It has no such price or no such rate.
//   - Its Quote Currency is not known.
//   - It needs a rate between two currencies that are not PLN.
//
// The point of today uses the state of the hero (ledger.Calculate) with the current prices and the latest
// Exchange Rates. Thus the value of today is equal to the value in the hero when each Holding has a value.
// A Holding with no value in the hero counts at its cost. The point of today also has the Cash Operations
// with a later time.
//
// It returns the error of ledger.Calculate if a sale does not fit the Lots.
func Calculate(in Input) (History, error) {
	l := in.Ledger
	state, err := ledger.Calculate(l)
	if err != nil {
		return History{}, err
	}
	if len(l.CashOperations) == 0 {
		return History{ValuedAtCost: []ledger.Instrument{}}, nil
	}

	c := newCalculator(in)
	today := dayOf(l.Today, l.Today.Location())
	// The data can have a Cash Operation that is later than the clock of the tool, for example in a test.
	first := dayOf(l.CashOperations[0].Time, l.Today.Location())
	if first.After(today) {
		first = today
	}
	points := make([]Point, 0, int(today.Sub(first)/(24*time.Hour))+1)
	next := 0
	for day := first; day.Before(today); day = day.AddDate(0, 0, 1) {
		flow := decimal.Zero
		for ; next < len(l.CashOperations) && !dayOf(l.CashOperations[next].Time, l.Today.Location()).After(day); next++ {
			flow = flow.Add(c.apply(l.CashOperations[next]))
		}
		points = append(points, Point{
			DayValue: returns.DayValue{Date: day, Value: c.value(day), Flow: flow}, NetDeposits: c.netDeposits,
		})
	}
	points = append(points, c.todayPoint(today, state, l.CashOperations[next:]))
	return History{Points: points, ValuedAtCost: c.valuedAtCost(l.Lots)}, nil
}

// dayOf returns the calendar day of t in the location loc, at midnight UTC.
func dayOf(t time.Time, loc *time.Location) time.Time {
	y, m, d := t.In(loc).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// calculator is the state of the pass over the days.
type calculator struct {
	in          Input
	cash        decimal.Decimal
	netDeposits decimal.Decimal
	// lots are the Lots by position ID. fills are the Lots of the purchases by XTB ID.
	lots  map[string]*lotState
	fills map[string]fill
	// holdings are the open volume and the open cost of each Instrument, by broker ticker.
	holdings map[string]*holding
	// tickers are the broker tickers of holdings in ticker order, so that the sum has a fixed order.
	tickers []string
	// closeIndex and rateIndex are the positions of the last price and the last rate that the pass used.
	closeIndex map[string]int
	rateIndex  map[string]int
	atCost     map[string]bool
}

// lotState is one Lot with the Fills and the sales of the Cash Operations so far.
type lotState struct {
	ticker string
	// volume and purchase are the volume and the purchase amount of all Fills. ADR 0001 uses them for the cost
	// of a sale.
	volume, purchase decimal.Decimal
	// filledVolume and filledPurchase are the sums of the Fills so far.
	filledVolume, filledPurchase decimal.Decimal
	closedVolume, closedCost     decimal.Decimal
}

// openVolume is the volume that the Fills so far add and that no sale so far closed.
func (st *lotState) openVolume() decimal.Decimal {
	return st.filledVolume.Sub(st.closedVolume)
}

// openCost is the remaining part of the purchase amount of the Fills so far. ADR 0001, rule 5.
// A closed Lot has no open cost, also when the cost of its parts has a rest from the division.
func (st *lotState) openCost() decimal.Decimal {
	if !st.openVolume().IsPositive() {
		return decimal.Zero
	}
	return st.filledPurchase.Sub(st.closedCost)
}

// fill is one purchase of a Lot.
type fill struct {
	positionID string
	volume     decimal.Decimal
	// purchase is the purchase amount. It is positive.
	purchase decimal.Decimal
}

// holding is the open volume and the open cost of one Instrument.
type holding struct {
	volume, cost decimal.Decimal
}

func newCalculator(in Input) *calculator {
	c := &calculator{
		in: in, cash: decimal.Zero, netDeposits: decimal.Zero,
		lots: map[string]*lotState{}, fills: map[string]fill{}, holdings: map[string]*holding{},
		closeIndex: map[string]int{}, rateIndex: map[string]int{}, atCost: map[string]bool{},
	}
	for _, lot := range in.Ledger.Lots {
		st := &lotState{
			ticker: lot.Instrument.Ticker, volume: decimal.Zero, purchase: decimal.Zero,
			filledVolume: decimal.Zero, filledPurchase: decimal.Zero, closedVolume: decimal.Zero, closedCost: decimal.Zero,
		}
		for _, f := range lot.Fills {
			st.volume = st.volume.Add(f.Volume)
			st.purchase = st.purchase.Sub(f.Amount)
			c.fills[f.XTBID] = fill{positionID: lot.PositionID, volume: f.Volume, purchase: f.Amount.Neg()}
		}
		c.lots[lot.PositionID] = st
		if _, ok := c.holdings[st.ticker]; !ok {
			c.holdings[st.ticker] = &holding{volume: decimal.Zero, cost: decimal.Zero}
			c.tickers = append(c.tickers, st.ticker)
		}
	}
	slices.Sort(c.tickers)
	return c
}

// apply adds the Cash Operation to the state and returns its Net Deposits flow.
func (c *calculator) apply(op ledger.CashOperation) decimal.Decimal {
	flow := ledger.NetDeposit(op)
	c.cash = c.cash.Add(op.Amount)
	c.netDeposits = c.netDeposits.Add(flow)
	if f, ok := c.fills[op.XTBID]; ok {
		c.changeLot(f.positionID, func(st *lotState) {
			st.filledVolume = st.filledVolume.Add(f.volume)
			st.filledPurchase = st.filledPurchase.Add(f.purchase)
		})
		return flow
	}
	// ledger.Calculate checked that each sale closes a known Lot within its volume.
	if op.Type == ledger.TypeStockSell {
		c.changeLot(op.PositionID, func(st *lotState) {
			st.closedVolume = st.closedVolume.Add(op.Volume)
			// ADR 0001, rule 3, as in ledger.Calculate.
			st.closedCost = st.closedCost.Add(st.purchase.Mul(op.Volume).Div(st.volume))
		})
	}
	return flow
}

// changeLot changes the Lot and moves the change of its open volume and open cost to its Instrument.
func (c *calculator) changeLot(positionID string, change func(*lotState)) {
	st, ok := c.lots[positionID]
	if !ok {
		return
	}
	h := c.holdings[st.ticker]
	h.volume, h.cost = h.volume.Sub(st.openVolume()), h.cost.Sub(st.openCost())
	change(st)
	h.volume, h.cost = h.volume.Add(st.openVolume()), h.cost.Add(st.openCost())
}

// value returns the cash balance plus the value of the Holdings at the end of a past day.
func (c *calculator) value(day time.Time) decimal.Decimal {
	value := c.cash
	for _, ticker := range c.tickers {
		h := c.holdings[ticker]
		if !h.volume.IsPositive() {
			continue
		}
		v, ok := c.marketValue(ticker, h.volume, day)
		if !ok {
			c.atCost[ticker] = true
			v = h.cost
		}
		value = value.Add(v)
	}
	return value
}

// marketValue returns the value of the volume of the Instrument on a past day in the Account currency,
// with the same arithmetic as the hero. It returns false if the day has no price or no Exchange Rate.
func (c *calculator) marketValue(ticker string, volume decimal.Decimal, day time.Time) (decimal.Decimal, bool) {
	quote := c.in.Ledger.QuoteCurrencies[ticker]
	if quote == "" {
		return decimal.Zero, false
	}
	currency, ok := ledger.RateCurrency(quote, c.in.Ledger.Currency)
	if !ok {
		return decimal.Zero, false
	}
	closes := c.in.Closes[ticker]
	i, ok := lastOnOrBefore(closes, c.closeIndex[ticker], day, func(cl Close) time.Time { return cl.Date })
	if !ok {
		return decimal.Zero, false
	}
	c.closeIndex[ticker] = i
	value := volume.Mul(closes[i].Price)
	if currency == "" {
		return value, true
	}
	rates := c.in.ExchangeRates[currency]
	j, ok := lastOnOrBefore(rates, c.rateIndex[currency], day, func(r Rate) time.Time { return r.Date })
	// A rate of zero or less is not valid, as in the hero.
	if !ok || !rates[j].Mid.IsPositive() {
		return decimal.Zero, false
	}
	c.rateIndex[currency] = j
	return ledger.Convert(value, quote, rates[j].Mid), true
}

// lastOnOrBefore returns the index of the last item with a date on the day or before it. The search starts at
// the index from, because the days of the pass only grow. It returns false if no item has such a date.
func lastOnOrBefore[T any](items []T, from int, day time.Time, date func(T) time.Time) (int, bool) {
	i := from
	for i < len(items) && !date(items[i]).After(day) {
		i++
	}
	// items[i-1] is the last item on the day or before it.
	if i == 0 {
		return 0, false
	}
	return i - 1, true
}

// todayPoint returns the point of today from the state of the hero. It applies the remaining Cash Operations,
// so that the flow of the point has them.
func (c *calculator) todayPoint(today time.Time, state ledger.State, rest []ledger.CashOperation) Point {
	flow := decimal.Zero
	for _, op := range rest {
		flow = flow.Add(c.apply(op))
	}
	value := state.Value
	for _, h := range state.Holdings {
		if !h.HasValue() {
			c.atCost[h.Instrument.Ticker] = true
			value = value.Add(h.Cost)
		}
	}
	return Point{DayValue: returns.DayValue{Date: today, Value: value, Flow: flow}, NetDeposits: state.NetDeposits}
}

// valuedAtCost returns the Instruments that the pass valued at cost, in ticker order.
func (c *calculator) valuedAtCost(lots []ledger.Lot) []ledger.Instrument {
	result := []ledger.Instrument{}
	seen := map[string]bool{}
	for _, lot := range lots {
		ticker := lot.Instrument.Ticker
		if c.atCost[ticker] && !seen[ticker] {
			seen[ticker] = true
			result = append(result, lot.Instrument)
		}
	}
	slices.SortFunc(result, func(a, b ledger.Instrument) int { return cmp.Compare(a.Ticker, b.Ticker) })
	return result
}
