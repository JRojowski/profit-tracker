package httpapi

import (
	"cmp"
	"net/http"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/tier"
)

// noTierName is the name of the group of the Instruments with no Tier.
const noTierName = "No Tier"

var hundred = decimal.NewFromInt(100)

// tiersBreakdownResponse is the body of GET /api/accounts/{id}/tiers-breakdown. Amounts are decimal text.
type tiersBreakdownResponse struct {
	// Currency is the currency of all amounts.
	Currency string `json:"currency"`
	// Tiers are in the order of the order numbers. A Tier with no Instruments has zero sums.
	Tiers []breakdownGroupResponse `json:"tiers"`
	// NoTier holds the Instruments with no Tier. It is nil if there are none.
	NoTier *breakdownGroupResponse `json:"noTier"`
	// Total is the sum of all Tiers and the "No Tier" group.
	Total breakdownSumResponse `json:"total"`
}

// breakdownGroupResponse is a Tier, or the "No Tier" group.
type breakdownGroupResponse struct {
	// Order is the order number of the Tier. It is nil for the "No Tier" group.
	Order       *int                          `json:"order"`
	Name        string                        `json:"name"`
	Instruments []breakdownInstrumentResponse `json:"instruments"`
	Sum         breakdownSumResponse          `json:"sum"`
}

// breakdownInstrumentResponse is one Instrument. It has a Holding part, a Closed Positions part, or both.
type breakdownInstrumentResponse struct {
	Ticker          string                    `json:"ticker"`
	Name            string                    `json:"name"`
	Holding         *breakdownHoldingResponse `json:"holding"`
	ClosedPositions *breakdownClosedResponse  `json:"closedPositions"`
}

// breakdownHoldingResponse is the Holding part of an Instrument.
type breakdownHoldingResponse struct {
	// NoValue tells that the Holding has no price. Then Value, UnrealizedProfit and ReturnOnCost are nil.
	NoValue bool    `json:"noValue"`
	Value   *string `json:"value"`
	// Cost is the cost of the open Lots.
	Cost             string  `json:"cost"`
	UnrealizedProfit *string `json:"unrealizedProfit"`
	// ReturnOnCost is the Unrealized Profit × 100 / Cost, exact. It is nil if there is no profit or the cost is 0.
	ReturnOnCost *string `json:"returnOnCost"`
}

// breakdownClosedResponse is the Closed Positions part of an Instrument.
type breakdownClosedResponse struct {
	Count          int    `json:"count"`
	Cost           string `json:"cost"`
	SaleAmount     string `json:"saleAmount"`
	RealizedProfit string `json:"realizedProfit"`
	// ReturnOnCost is the Realized Profit × 100 / Cost, exact. It is nil if the cost is 0.
	ReturnOnCost *string `json:"returnOnCost"`
	// LastSaleDate is the date of the latest sale in UTC, as YYYY-MM-DD.
	LastSaleDate string `json:"lastSaleDate"`
	// Positions are the single Closed Positions, in time order.
	Positions []closedPositionResponse `json:"positions"`
}

// breakdownSumResponse is a sum row. A Holding with no price adds nothing to it. Dividends and interest are not in it.
type breakdownSumResponse struct {
	Value            string `json:"value"`
	UnrealizedProfit string `json:"unrealizedProfit"`
	RealizedProfit   string `json:"realizedProfit"`
	TotalProfit      string `json:"totalProfit"`
	// Cost is the cost of the open Lots with a price plus the cost of the Closed Positions.
	Cost string `json:"cost"`
	// ReturnOnCost is TotalProfit × 100 / Cost, exact. It is nil if Cost is 0.
	ReturnOnCost *string `json:"returnOnCost"`
}

// accountTiersBreakdown returns the result of each Instrument of a Tier Account, grouped by Tier.
func (a *api) accountTiersBreakdown(w http.ResponseWriter, r *http.Request) {
	view, ok := a.accountView(w, r)
	if !ok {
		return
	}
	acc := view.account
	if !tier.IsTierAccount(acc.Type, acc.Provider, acc.Currency) {
		writeError(w, http.StatusNotFound, "the account has no Tiers Breakdown: it is not a Tier Account")
		return
	}
	tiers, err := a.store.Tiers(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, tiersBreakdownOf(acc.Currency, tiers, view.state.Holdings, view.state.ClosedPositions))
}

// tiersBreakdownOf groups the results of the Instruments by Tier. All amounts of the Holdings and the Closed Positions
// must be in currency. The function does not convert. The Tiers Breakdown of all Accounts calls it with Holdings and
// Closed Positions that it converted to PLN. Several Holdings of one Instrument, for example one on each Account,
// make one Holding part. The same holds for the Closed Positions.
func tiersBreakdownOf(currency string, tiers []tier.Tier, holdings []ledger.Holding, closed []ledger.ClosedPosition) tiersBreakdownResponse {
	tierOf := map[string]int{}
	for i, t := range tiers {
		for _, in := range t.Instruments {
			tierOf[in.Ticker] = i
		}
	}

	groups := make([]*groupBuilder, len(tiers))
	for i, t := range tiers {
		order := t.Order
		groups[i] = &groupBuilder{order: &order, name: t.Name}
	}
	noTier := &groupBuilder{name: noTierName}
	groupOf := func(ticker string) *groupBuilder {
		if i, ok := tierOf[ticker]; ok {
			return groups[i]
		}
		return noTier
	}

	for _, h := range holdings {
		groupOf(h.Instrument.Ticker).instrument(h.Instrument).addHolding(h)
	}
	for _, c := range closed {
		groupOf(c.Instrument.Ticker).instrument(c.Instrument).addClosed(c)
	}

	resp := tiersBreakdownResponse{Currency: currency, Tiers: make([]breakdownGroupResponse, 0, len(groups))}
	var total sumBuilder
	for _, g := range append(groups, noTier) {
		built, sum := g.build()
		total.add(sum)
		if g == noTier {
			if len(built.Instruments) > 0 {
				resp.NoTier = &built
			}
			continue
		}
		resp.Tiers = append(resp.Tiers, built)
	}
	resp.Total = total.response()
	return resp
}

// groupBuilder collects the Instruments of one Tier.
type groupBuilder struct {
	order       *int
	name        string
	instruments map[string]*instrumentBuilder
}

// instrument returns the builder of the Instrument. It makes one if needed.
func (g *groupBuilder) instrument(in ledger.Instrument) *instrumentBuilder {
	if g.instruments == nil {
		g.instruments = map[string]*instrumentBuilder{}
	}
	b, ok := g.instruments[in.Ticker]
	if !ok {
		b = &instrumentBuilder{instrument: in}
		g.instruments[in.Ticker] = b
	}
	return b
}

// build returns the response of the group, with its Instruments in ticker order, and its sum.
func (g *groupBuilder) build() (breakdownGroupResponse, sumBuilder) {
	builders := make([]*instrumentBuilder, 0, len(g.instruments))
	for _, b := range g.instruments {
		builders = append(builders, b)
	}
	slices.SortFunc(builders, func(a, b *instrumentBuilder) int { return cmp.Compare(a.instrument.Ticker, b.instrument.Ticker) })

	var sum sumBuilder
	resp := breakdownGroupResponse{Order: g.order, Name: g.name, Instruments: make([]breakdownInstrumentResponse, 0, len(builders))}
	for _, b := range builders {
		resp.Instruments = append(resp.Instruments, b.response())
		sum.add(b.sum())
	}
	resp.Sum = sum.response()
	return resp, sum
}

// instrumentBuilder collects the Holdings and the Closed Positions of one Instrument.
type instrumentBuilder struct {
	instrument ledger.Instrument
	holdings   []ledger.Holding
	closed     []ledger.ClosedPosition
}

func (b *instrumentBuilder) addHolding(h ledger.Holding)       { b.holdings = append(b.holdings, h) }
func (b *instrumentBuilder) addClosed(c ledger.ClosedPosition) { b.closed = append(b.closed, c) }

// sum returns the amounts of the Instrument for the sum rows.
func (b *instrumentBuilder) sum() sumBuilder {
	var s sumBuilder
	for _, h := range b.holdings {
		if !h.HasValue() {
			continue
		}
		s.value = s.value.Add(h.Value)
		s.unrealized = s.unrealized.Add(h.UnrealizedProfit)
		s.cost = s.cost.Add(h.Cost)
	}
	for _, c := range b.closed {
		s.realized = s.realized.Add(c.RealizedProfit)
		s.cost = s.cost.Add(c.Cost)
	}
	return s
}

func (b *instrumentBuilder) response() breakdownInstrumentResponse {
	resp := breakdownInstrumentResponse{Ticker: b.instrument.Ticker, Name: b.instrument.Name}
	if len(b.holdings) > 0 {
		resp.Holding = b.holdingPart()
	}
	if len(b.closed) > 0 {
		resp.ClosedPositions = b.closedPart()
	}
	return resp
}

// holdingPart returns the Holding part. It counts only the Holdings with a price, so it agrees with the sums.
// If no Holding has a price, it shows the cost of all and the "no value" flag.
func (b *instrumentBuilder) holdingPart() *breakdownHoldingResponse {
	var value, unrealized, cost, unpricedCost decimal.Decimal
	priced := false
	for _, h := range b.holdings {
		if !h.HasValue() {
			unpricedCost = unpricedCost.Add(h.Cost)
			continue
		}
		priced = true
		value = value.Add(h.Value)
		unrealized = unrealized.Add(h.UnrealizedProfit)
		cost = cost.Add(h.Cost)
	}
	if !priced {
		return &breakdownHoldingResponse{NoValue: true, Cost: formatAmount(unpricedCost)}
	}
	return &breakdownHoldingResponse{
		Value: new(formatAmount(value)), Cost: formatAmount(cost),
		UnrealizedProfit: new(formatAmount(unrealized)), ReturnOnCost: returnOnCost(unrealized, cost),
	}
}

// closedPart returns the Closed Positions part, with the Closed Positions in time order and then in XTB ID order.
func (b *instrumentBuilder) closedPart() *breakdownClosedResponse {
	closed := slices.Clone(b.closed)
	slices.SortStableFunc(closed, func(x, y ledger.ClosedPosition) int {
		return cmp.Or(x.Time.Compare(y.Time), cmp.Compare(x.XTBID, y.XTBID))
	})
	var cost, amount, profit decimal.Decimal
	var last time.Time
	positions := make([]closedPositionResponse, 0, len(closed))
	for _, c := range closed {
		cost = cost.Add(c.Cost)
		amount = amount.Add(c.Amount)
		profit = profit.Add(c.RealizedProfit)
		if c.Time.After(last) {
			last = c.Time
		}
		positions = append(positions, toClosedPositionResponse(c))
	}
	return &breakdownClosedResponse{
		Count: len(closed), Cost: formatAmount(cost), SaleAmount: formatAmount(amount),
		RealizedProfit: formatAmount(profit), ReturnOnCost: returnOnCost(profit, cost),
		LastSaleDate: last.UTC().Format(time.DateOnly), Positions: positions,
	}
}

// sumBuilder adds up the amounts of a sum row.
type sumBuilder struct {
	value, unrealized, realized, cost decimal.Decimal
}

func (s *sumBuilder) add(o sumBuilder) {
	s.value = s.value.Add(o.value)
	s.unrealized = s.unrealized.Add(o.unrealized)
	s.realized = s.realized.Add(o.realized)
	s.cost = s.cost.Add(o.cost)
}

func (s sumBuilder) response() breakdownSumResponse {
	total := s.unrealized.Add(s.realized)
	return breakdownSumResponse{
		Value: formatAmount(s.value), UnrealizedProfit: formatAmount(s.unrealized), RealizedProfit: formatAmount(s.realized),
		TotalProfit: formatAmount(total), Cost: formatAmount(s.cost), ReturnOnCost: returnOnCost(total, s.cost),
	}
}

// returnOnCost returns the profit × 100 / cost as decimal text, or nil if the cost is 0. The multiplication comes
// first, so an exact result stays exact.
func returnOnCost(profit, cost decimal.Decimal) *string {
	if cost.IsZero() {
		return nil
	}
	return new(profit.Mul(hundred).Div(cost).String())
}
