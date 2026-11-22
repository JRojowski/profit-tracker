// Package ledger calculates the state of one Account from its Ledger, the prices and the Exchange Rates.
// The state has the cash balance, the Net Deposits and the Holdings with their value and Unrealized Profit.
// It also has the Closed Positions with their Realized Profit, the total result with its parts, the Simple Return and the Annual Return.
// It is a pure calculation with no I/O.
// The tool calculates this state on each request and does not store it.
package ledger

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/returns"
)

// The Cash Operation types that the calculator uses, as the broker export writes them.
const (
	TypeDeposit        = "Deposit"
	TypeWithdrawal     = "Withdrawal"
	TypeStockSell      = "Stock sell"
	TypeDividend       = "Dividend"
	TypeWithholdingTax = "Withholding tax"
	TypeInterest       = "Free funds interest"
	TypeInterestTax    = "Free funds interest tax"
	TypeSECFee         = "SEC fee"
	TypeCorrection     = "Correction"
	// TypeTransfer is a Transfer between two Accounts. See ADR 0003.
	TypeTransfer = "IKZE deposit"
	// TypeCryptoPurchase is a purchase of a crypto asset on an Account with no Lots, for example a Kraken Account.
	// Its Holding uses the Average Cost (ADR 0009). The type is not an XTB type, so no XTB Account has it.
	TypeCryptoPurchase = "Crypto purchase"
	// TypeReward is a Reward: an amount of a crypto asset that a Provider pays for a balance. Its amount is zero.
	// It adds its volume to the Holding at its value on its day (ADR 0009). The type is not an XTB type.
	TypeReward = "Reward"
)

// Errors of Calculate. Each error that Calculate returns wraps one of them.
var (
	// ErrUnknownLot tells that a Closed Position closes a Lot that the Ledger does not have.
	ErrUnknownLot = errors.New("the closed position closes a lot that the ledger does not have")
	// ErrLotOverclosed tells that the Closed Positions of a Lot close more than the Lot volume.
	ErrLotOverclosed = errors.New("the closed positions close more than the lot volume")
)

// PLN is the currency of the Exchange Rates. Each rate is the PLN amount for 1 unit of a currency.
const PLN = "PLN"

// Ledger is the input of Calculate: all Cash Operations of one Account, its Lots, the prices and the Exchange Rates.
type Ledger struct {
	// Currency is the Account currency, for example "USD".
	Currency string
	// CashOperations are all Cash Operations of the Account, in time order.
	CashOperations []CashOperation
	// Lots are the Lots of the Account, each with its Fills.
	Lots []Lot
	// QuoteCurrencies are the Quote Currencies of the Instruments, by broker ticker.
	// An Instrument that is not in the map has an unknown Quote Currency.
	QuoteCurrencies map[string]string
	// Prices are the close prices in the Quote Currency of each Instrument, by broker ticker.
	// A Holding with no price has no value and no Unrealized Profit.
	Prices map[string]decimal.Decimal
	// Closes are the stored closes of the daily history in the Quote Currency, by broker ticker, in date order.
	// Calculate uses them only for the cost of a Reward: the close on or before the day of the Reward (ADR 0009).
	// The caller gives the closes of each Instrument with a Reward. A Reward with no such close has no known value.
	Closes map[string][]Close
	// ExchangeRates are the latest Exchange Rates, by currency.
	ExchangeRates map[string]ExchangeRate
	// Today is the start of today in Polish time. The Annual Return counts days in its location.
	// This is an exception to "keep times in UTC": ADR 0004 takes the day of a flow in Polish time.
	// A zero Today is before each flow, so the Annual Return is not available.
	Today time.Time
}

// ExchangeRate is one Exchange Rate of a currency against PLN.
type ExchangeRate struct {
	// Rate is the PLN amount for 1 unit of the currency.
	Rate decimal.Decimal
	// Date is the NBP table date.
	Date time.Time
}

// Close is one close of the daily history of an Instrument.
type Close struct {
	// Date is the day of the close at midnight UTC.
	Date time.Time
	// Close is the close price in the Quote Currency.
	Close decimal.Decimal
}

// NoValueReason tells why a Holding has no value.
type NoValueReason int

const (
	// ReasonNone tells that the Holding has a value.
	ReasonNone NoValueReason = iota
	// ReasonUnknownQuoteCurrency tells that the Quote Currency of the Instrument is not known.
	ReasonUnknownQuoteCurrency
	// ReasonCrossRate tells that the Quote Currency and the Account currency are both not PLN and not equal.
	// The tool has no rate between two such currencies.
	ReasonCrossRate
	// ReasonNoPrice tells that the Ledger has no price for the Instrument.
	ReasonNoPrice
	// ReasonNoExchangeRate tells that the Ledger has no Exchange Rate for the conversion to the Account currency.
	ReasonNoExchangeRate
	// ReasonNoRewardClose tells that a Reward of the Holding has no known value, so the cost is not known.
	// The Ledger has no close in the Account currency on or before the day of the Reward.
	ReasonNoRewardClose
)

// RateCurrency returns the currency whose Exchange Rate converts an amount in the Quote Currency quote
// to the Account currency account. It is the currency of the two that is not PLN.
// It returns "" and true if quote equals account, because then no conversion is necessary.
// It returns false if quote and account are both not PLN and not equal.
func RateCurrency(quote, account string) (string, bool) {
	switch {
	case quote == account:
		return "", true
	case account == PLN:
		return quote, true
	case quote == PLN:
		return account, true
	default:
		return "", false
	}
}

// CashOperation is one entry of the Ledger.
type CashOperation struct {
	// Type is the type as the broker export writes it, for example "Deposit" or "Stock purchase".
	Type string
	// Amount is in the Account currency. A purchase has a negative amount.
	Amount decimal.Decimal
	// XTBID is the XTB ID of the operation.
	XTBID string
	Time  time.Time
	// Ticker is the broker ticker of the Instrument that the broker ties to the operation.
	// It is empty for an operation with no Instrument, for example a Deposit or the free funds interest.
	Ticker string
	// PositionID is the Lot ID. It is empty for an operation with no Lot.
	PositionID string
	// Volume is the parsed volume. For a Stock sell, it is the closed volume of the Lot.
	// For a crypto purchase and a Reward, it is the volume after the fee. It is zero for an operation with no volume.
	Volume decimal.Decimal
	// Price is the stored price of one unit. It is not valid for an operation with no price.
	Price decimal.NullDecimal
	// Fee is the fee of a crypto purchase in FeeCurrency. It is zero for each other operation.
	Fee decimal.Decimal
	// FeeCurrency is the currency of Fee: the Account currency or the crypto asset. An empty value with a fee
	// tells the crypto asset. ADR 0009: a fee in the Account currency is a part of Amount, so it is in the cost.
	// A fee in the crypto asset decreases Volume.
	FeeCurrency string
}

// Instrument is one security, identified by its broker ticker.
type Instrument struct {
	Ticker string
	Name   string
}

// Lot is one or more Fills of an Instrument that the broker tracks as one unit.
type Lot struct {
	// PositionID is the Lot ID, the XTB position ID.
	PositionID string
	Instrument Instrument
	Fills      []Fill
}

// Fill is one purchase Cash Operation of a Lot.
type Fill struct {
	// XTBID is the XTB ID of the purchase.
	XTBID string
	// Time is the time of the purchase.
	Time   time.Time
	Volume decimal.Decimal
	// Price is the stored price of one unit. It is not valid for a purchase with no price.
	Price decimal.NullDecimal
	// Amount is the purchase amount in the Account currency. It is negative.
	Amount decimal.Decimal
}

// State is the result of Calculate.
type State struct {
	// Cash is the cash balance: the sum of all Cash Operation amounts.
	Cash decimal.Decimal
	// NetDeposits is the money that the user put into the Account. See GLOSSARY.md, "Net Deposits".
	NetDeposits decimal.Decimal
	// Flows are the Net Deposits flows in the order of the Ledger. Their sum is NetDeposits.
	Flows []returns.Flow
	// Holdings has one Holding per Instrument with an open volume, in ticker order.
	Holdings []Holding
	// OpenLots are the Lots with an open volume, with the open part of each. The order is the order of Ledger.Lots.
	OpenLots []OpenLot
	// ClosedPositions are the Stock sell operations with their Realized Profit, in time order and then in XTB ID order.
	ClosedPositions []ClosedPosition
	// RealizedProfit is the sum of the Realized Profit of the Closed Positions.
	RealizedProfit decimal.Decimal
	// NetDividends is the sum of the dividends and the withholding tax. The tax is negative.
	NetDividends decimal.Decimal
	// NetInterest is the sum of the free funds interest and its tax. The tax is negative.
	NetInterest decimal.Decimal
	// Fees is the sum of the SEC fees and the corrections, with the sign of the export. It is usually negative.
	Fees decimal.Decimal
	// Rewards is the sum of the values of the Rewards on their days (ADR 0009). It is not valid if a Reward
	// has no known value. It is zero for an Account with no Rewards.
	Rewards decimal.NullDecimal
	// FeeSum is the sum of the crypto fees in the Account currency: for each crypto purchase, the fee × the price.
	// It is information only. The fee is in the cost of the Holding, so FeeSum is not a part of TotalResult.
	FeeSum decimal.Decimal
	// UnrealizedProfit is the sum of the Unrealized Profit of the Holdings that have a price.
	UnrealizedProfit decimal.Decimal
	// Value is the cash balance plus the value of the Holdings that have a price.
	Value decimal.Decimal
	// TotalResult is RealizedProfit + UnrealizedProfit + NetDividends + NetInterest + Rewards + Fees.
	// A Rewards part with no known value adds nothing.
	TotalResult decimal.Decimal
	// SimpleReturn is TotalResult × 100 / NetDeposits.
	SimpleReturn returns.Rate
	// AnnualReturn is the yearly rate that makes the Net Deposits flows grow to Value today.
	// It is not available if a Holding has no value, because then Value leaves the Holding out.
	AnnualReturn returns.Rate
}

// ClosedPosition is one Stock sell: the close of a part of one Lot. See ADR 0001.
type ClosedPosition struct {
	XTBID string
	Time  time.Time
	// Instrument is the Instrument of the Lot.
	Instrument Instrument
	// PositionID is the ID of the Lot that the Closed Position closes.
	PositionID string
	// Volume is the closed volume.
	Volume decimal.Decimal
	// Amount is the sale amount in the Account currency.
	Amount decimal.Decimal
	// Cost is the cost of the closed part: the Lot purchase amount × the closed volume / the Lot volume.
	Cost decimal.Decimal
	// RealizedProfit is Amount minus Cost. It can be negative.
	RealizedProfit decimal.Decimal
	// OpenTime is the time of the first Fill of the Lot.
	OpenTime time.Time
	// OpenPrice is the average of the Fill prices of the Lot with the Fill volumes as weights.
	// It is not valid if a Fill has no price. The division keeps decimal.DivisionPrecision (16) decimal places.
	OpenPrice decimal.NullDecimal
	// ClosePrice is the stored price of the sell Cash Operation. It is not valid if the operation has no price.
	ClosePrice decimal.NullDecimal
}

// OpenLot is the open part of one Lot. The Holding of its Instrument adds up the open Lots.
type OpenLot struct {
	// PositionID is the Lot ID.
	PositionID string
	Instrument Instrument
	// Volume is the open volume: the Lot volume minus the volume of its Closed Positions.
	Volume decimal.Decimal
	// Cost is the open cost in the Account currency. ADR 0001, rule 5.
	Cost decimal.Decimal
	// OpenTime is the time of the first Fill of the Lot.
	OpenTime time.Time
	// OpenPrice is the average of the Fill prices of the Lot, weighted by volume. It is not valid if a Fill has no price.
	OpenPrice decimal.NullDecimal
	// Fills are all Fills of the Lot, also the Fills that a Closed Position closed.
	Fills []Fill
}

// Holding is all open Lots of one Instrument, taken together.
type Holding struct {
	Instrument Instrument
	// Volume is the open volume.
	Volume decimal.Decimal
	// Cost is the open cost in the Account currency. It is positive. If UnknownCost is true,
	// it is only the cost of the parts with a known cost.
	Cost decimal.Decimal
	// UnknownCost tells that a Reward of the Holding has no known value, so the cost is not known.
	// Then the Holding has no value.
	UnknownCost bool
	// NoValueReason tells why the Holding has no value in the Account currency. It is ReasonNone for a Holding
	// with a value. For a Holding with no value, Price, ExchangeRate, Value and UnrealizedProfit are zero.
	NoValueReason NoValueReason
	// Price is the close price in the Quote Currency.
	Price decimal.Decimal
	// ExchangeRateCurrency is the currency whose Exchange Rate converts the value to the Account currency:
	// the currency of the two that is not PLN. It is empty if the value needs no conversion or no conversion is
	// possible. It is set also when the Ledger has no price or no rate.
	ExchangeRateCurrency string
	// ExchangeRate is the Exchange Rate that the value uses. It is zero if the value uses no rate.
	ExchangeRate ExchangeRate
	// Value is the open volume × the price in the Account currency.
	Value decimal.Decimal
	// UnrealizedProfit is Value minus Cost. It can be negative.
	UnrealizedProfit decimal.Decimal
}

// HasValue tells if the Holding has a value in the Account currency.
func (h Holding) HasValue() bool {
	return h.NoValueReason == ReasonNone
}

// UsesExchangeRate tells if the value of the Holding uses ExchangeRate.
func (h Holding) UsesExchangeRate() bool {
	return h.HasValue() && h.ExchangeRateCurrency != ""
}

// AverageCost returns the open cost per unit. The division keeps decimal.DivisionPrecision (16) decimal places.
// It does not round to the cent. The frontend rounds for display.
func (h Holding) AverageCost() decimal.Decimal {
	return h.Cost.Div(h.Volume)
}

// UnrealizedProfitPercent returns the Unrealized Profit × 100 / the open cost. The multiplication comes first,
// so an exact result stays exact. The division keeps decimal.DivisionPrecision (16) decimal places.
// It returns false if the Holding has no value or no cost.
func (h Holding) UnrealizedProfitPercent() (decimal.Decimal, bool) {
	if !h.HasValue() || h.Cost.IsZero() {
		return decimal.Zero, false
	}
	return h.UnrealizedProfit.Mul(decimal.NewFromInt(100)).Div(h.Cost), true
}

// Calculate returns the state of the Account from its Ledger. The values are exact. Calculate does not round.
// It returns an error that wraps ErrUnknownLot or ErrLotOverclosed if a Closed Position does not fit the Lots.
func Calculate(l Ledger) (State, error) {
	s := State{
		Cash: decimal.Zero, NetDeposits: decimal.Zero, RealizedProfit: decimal.Zero,
		NetDividends: decimal.Zero, NetInterest: decimal.Zero, Fees: decimal.Zero,
	}
	for _, op := range l.CashOperations {
		s.add(op)
	}
	// One rule for the income and the fees: the totals of the Account and the filtered sums use SumIncome.
	income := SumIncome(l.CashOperations, IncomeFilter{})
	s.NetDividends, s.NetInterest, s.Fees = income.NetDividends, income.NetInterest, income.Fees

	lots := make(map[string]*lotState, len(l.Lots))
	for _, lot := range l.Lots {
		lots[lot.PositionID] = newLotState(lot)
	}
	closedPositions, err := closeLots(l.CashOperations, lots)
	if err != nil {
		return State{}, err
	}
	s.ClosedPositions = closedPositions
	crypto := averageCostHoldings(l)
	s.Rewards, s.FeeSum = crypto.rewards, crypto.feeSum
	s.Holdings = holdings(l, crypto.holdings, lots)
	s.OpenLots = openLots(l, lots)
	s.RealizedProfit = SumRealizedProfit(closedPositions)
	s.UnrealizedProfit, s.Value = decimal.Zero, s.Cash
	for _, h := range s.Holdings {
		s.UnrealizedProfit = s.UnrealizedProfit.Add(h.UnrealizedProfit)
		s.Value = s.Value.Add(h.Value)
	}
	s.TotalResult = s.RealizedProfit.Add(s.UnrealizedProfit).Add(s.NetDividends).Add(s.NetInterest).
		Add(s.Rewards.Decimal).Add(s.Fees)
	s.Flows = flows(l.CashOperations)
	s.SimpleReturn = returns.Simple(s.TotalResult, s.NetDeposits)
	s.AnnualReturn = returns.Annual(s.Flows, s.Value, s.AllHoldingsHaveValue(), l.Today)
	return s, nil
}

// AllHoldingsHaveValue tells if each Holding has a price and, if necessary, an Exchange Rate.
// If it is false, Value does not include each Holding.
func (s *State) AllHoldingsHaveValue() bool {
	return !slices.ContainsFunc(s.Holdings, func(h Holding) bool { return !h.HasValue() })
}

// flows returns the Net Deposits flows of the Cash Operations.
func flows(ops []CashOperation) []returns.Flow {
	var result []returns.Flow
	for _, op := range ops {
		if amount := NetDeposit(op); !amount.IsZero() {
			result = append(result, returns.Flow{Time: op.Time, Amount: amount})
		}
	}
	return result
}

// add adds the amount of op to the cash balance and to the parts of the state that op counts in.
// The amounts keep the sign of the export: a tax, a fee and a Withdrawal are negative.
func (s *State) add(op CashOperation) {
	s.Cash = s.Cash.Add(op.Amount)
	s.NetDeposits = s.NetDeposits.Add(NetDeposit(op))
}

// IncomeFilter limits the Cash Operations that SumIncome counts. The zero filter counts each Cash Operation.
type IncomeFilter struct {
	// Ticker is the broker ticker of an Instrument. If it is not empty, SumIncome counts only the Cash Operations
	// that the broker ties to that Instrument. A Cash Operation with no Instrument never matches.
	Ticker string
	// From is the first instant that SumIncome counts. The zero time means no lower limit.
	From time.Time
	// To is the first instant after the period, so SumIncome does not count it. The zero time means no upper limit.
	To time.Time
}

// matches tells if the filter counts op.
func (f IncomeFilter) matches(op CashOperation) bool {
	switch {
	case f.Ticker != "" && op.Ticker != f.Ticker:
		return false
	case !f.From.IsZero() && op.Time.Before(f.From):
		return false
	case !f.To.IsZero() && !op.Time.Before(f.To):
		return false
	}
	return true
}

// Income is the sum of the Cash Operations of the income and the fees.
type Income struct {
	// NetDividends is the sum of the dividends and the withholding tax. The tax is negative.
	NetDividends decimal.Decimal
	// NetInterest is the sum of the free funds interest and its tax. The tax is negative.
	NetInterest decimal.Decimal
	// Fees is the sum of the SEC fees and the corrections, with the sign of the export. It is usually negative.
	Fees decimal.Decimal
}

// SumRealizedProfit returns the sum of the Realized Profit of the Closed Positions. It is zero for no Closed Position.
// It is the one rule for this sum: the Account totals, the Instrument Overview and the Assistant use it.
// It does not join two currencies, so the caller gives it the Closed Positions of one currency.
func SumRealizedProfit(closedPositions []ClosedPosition) decimal.Decimal {
	sum := decimal.Zero
	for _, cp := range closedPositions {
		sum = sum.Add(cp.RealizedProfit)
	}
	return sum
}

// SumIncome returns the net dividends, the net interest and the fees of the Cash Operations that the filter counts.
// It is the one rule for which Cash Operation type is a dividend, a withholding tax, interest or a fee.
// The amounts keep the sign of the export. Calculate uses it with the zero filter for the totals of an Account.
func SumIncome(ops []CashOperation, f IncomeFilter) Income {
	sum := Income{NetDividends: decimal.Zero, NetInterest: decimal.Zero, Fees: decimal.Zero}
	for _, op := range ops {
		if !f.matches(op) {
			continue
		}
		switch op.Type {
		case TypeDividend, TypeWithholdingTax:
			sum.NetDividends = sum.NetDividends.Add(op.Amount)
		case TypeInterest, TypeInterestTax:
			sum.NetInterest = sum.NetInterest.Add(op.Amount)
		case TypeSECFee, TypeCorrection:
			sum.Fees = sum.Fees.Add(op.Amount)
		}
	}
	return sum
}

// netDepositsTypes are the Cash Operation types of the Net Deposits flows:
// a Deposit, a Withdrawal and a Transfer (ADR 0003).
var netDepositsTypes = []string{TypeDeposit, TypeWithdrawal, TypeTransfer}

// NetDepositsTypes returns a copy of the Cash Operation types of the Net Deposits flows:
// a Deposit, a Withdrawal and a Transfer (ADR 0003).
func NetDepositsTypes() []string {
	return slices.Clone(netDepositsTypes)
}

// NetDeposit returns the part of op that counts in the Net Deposits.
// A Withdrawal and a Transfer Out have a negative amount in the export, so they decrease the Net Deposits.
// ADR 0003: a Transfer In counts as a deposit and a Transfer Out counts as a withdrawal.
func NetDeposit(op CashOperation) decimal.Decimal {
	if slices.Contains(netDepositsTypes, op.Type) {
		return op.Amount
	}
	return decimal.Zero
}

// lotState is one Lot with the volume and the cost that its Closed Positions closed.
type lotState struct {
	instrument Instrument
	// volume is the Lot volume: the sum of the Fill volumes.
	volume decimal.Decimal
	// purchase is the purchase amount of the Lot. It is positive.
	purchase decimal.Decimal
	// closedVolume and closedCost are the sums over the Closed Positions of the Lot.
	closedVolume decimal.Decimal
	closedCost   decimal.Decimal
	// openTime and openPrice come from all Fills of the Lot, like the purchase amount.
	openTime  time.Time
	openPrice decimal.NullDecimal
}

func newLotState(lot Lot) *lotState {
	st := &lotState{instrument: lot.Instrument, volume: decimal.Zero, purchase: decimal.Zero, closedVolume: decimal.Zero, closedCost: decimal.Zero}
	weighted, priced := decimal.Zero, true
	for _, f := range lot.Fills {
		st.volume = st.volume.Add(f.Volume)
		st.purchase = st.purchase.Sub(f.Amount)
		if st.openTime.IsZero() || f.Time.Before(st.openTime) {
			st.openTime = f.Time
		}
		if !f.Price.Valid {
			priced = false
			continue
		}
		weighted = weighted.Add(f.Price.Decimal.Mul(f.Volume))
	}
	// One Fill gives its exact price. The multiplication comes first, so an exact result stays exact.
	if priced && st.volume.IsPositive() {
		st.openPrice = decimal.NewNullDecimal(weighted.Div(st.volume))
	}
	return st
}

// openVolume is the volume that no Closed Position closed.
func (st *lotState) openVolume() decimal.Decimal {
	return st.volume.Sub(st.closedVolume)
}

// openCost is the remaining part of the purchase amount. ADR 0001, rule 5.
// A closed Lot has no open cost, also when the cost of its parts has a rest from the division.
func (st *lotState) openCost() decimal.Decimal {
	if st.openVolume().IsZero() {
		return decimal.Zero
	}
	return st.purchase.Sub(st.closedCost)
}

// closeLots calculates the Realized Profit of each Stock sell and records the closed part in its Lot.
// It returns the Closed Positions in time order and then in XTB ID order.
func closeLots(ops []CashOperation, lots map[string]*lotState) ([]ClosedPosition, error) {
	var closedPositions []ClosedPosition
	for _, op := range ops {
		if op.Type != TypeStockSell {
			continue
		}
		st, ok := lots[op.PositionID]
		if !ok {
			return nil, fmt.Errorf("closed position %s closes lot %s: %w", op.XTBID, op.PositionID, ErrUnknownLot)
		}
		st.closedVolume = st.closedVolume.Add(op.Volume)
		if !st.volume.IsPositive() || st.openVolume().IsNegative() {
			return nil, fmt.Errorf("closed position %s closes %s of lot %s, but the lot volume is %s: %w",
				op.XTBID, op.Volume, op.PositionID, st.volume, ErrLotOverclosed)
		}
		// ADR 0001, rule 3. The multiplication comes first, so an exact result stays exact.
		cost := st.purchase.Mul(op.Volume).Div(st.volume)
		st.closedCost = st.closedCost.Add(cost)
		closedPositions = append(closedPositions, ClosedPosition{
			XTBID:          op.XTBID,
			Time:           op.Time,
			Instrument:     st.instrument,
			PositionID:     op.PositionID,
			Volume:         op.Volume,
			Amount:         op.Amount,
			Cost:           cost,
			RealizedProfit: op.Amount.Sub(cost),
			OpenTime:       st.openTime,
			OpenPrice:      st.openPrice,
			ClosePrice:     op.Price,
		})
	}
	slices.SortStableFunc(closedPositions, func(a, b ClosedPosition) int {
		return cmp.Or(a.Time.Compare(b.Time), cmp.Compare(a.XTBID, b.XTBID))
	})
	return closedPositions, nil
}

// openLots returns the Lots with an open volume, with their open volume and open cost.
func openLots(l Ledger, states map[string]*lotState) []OpenLot {
	result := []OpenLot{}
	for _, lot := range l.Lots {
		st := states[lot.PositionID]
		if !st.openVolume().IsPositive() {
			continue
		}
		result = append(result, OpenLot{
			PositionID: lot.PositionID, Instrument: lot.Instrument, Volume: st.openVolume(), Cost: st.openCost(),
			OpenTime: st.openTime, OpenPrice: st.openPrice, Fills: lot.Fills,
		})
	}
	return result
}

// holdings adds the open part of the Lots per Instrument. A fully closed Lot adds nothing.
// It also adds each crypto purchase to the Holding of its Instrument by the Average Cost rule (ADR 0009).
// A Holding with a value gets its Unrealized Profit.
// byTicker has the Holdings at the Average Cost. holdings adds the Lots to it.
func holdings(l Ledger, byTicker map[string]*Holding, states map[string]*lotState) []Holding {
	for _, lot := range l.Lots {
		st := states[lot.PositionID]
		if !st.openVolume().IsPositive() {
			continue
		}
		h, ok := byTicker[lot.Instrument.Ticker]
		if !ok {
			h = &Holding{Instrument: lot.Instrument, Volume: decimal.Zero, Cost: decimal.Zero}
			byTicker[lot.Instrument.Ticker] = h
		}
		h.Volume = h.Volume.Add(st.openVolume())
		h.Cost = h.Cost.Add(st.openCost())
	}

	result := make([]Holding, 0, len(byTicker))
	for _, h := range byTicker {
		h.Price, h.Value, h.UnrealizedProfit = decimal.Zero, decimal.Zero, decimal.Zero
		h.setValue(l)
		result = append(result, *h)
	}
	slices.SortFunc(result, func(a, b Holding) int { return cmp.Compare(a.Instrument.Ticker, b.Instrument.Ticker) })
	return result
}

// averageCost is the result of averageCostHoldings.
type averageCost struct {
	// holdings are the Holdings at the Average Cost, by ticker.
	holdings map[string]*Holding
	// rewards is the sum of the values of the Rewards. It is not valid if a Reward has no known value.
	rewards decimal.NullDecimal
	// feeSum is the sum of the crypto fees × the purchase prices.
	feeSum decimal.Decimal
}

// averageCostHoldings returns the Holdings of the crypto purchases and the Rewards, by ticker (ADR 0009).
// Each purchase adds its volume after the fee and its full EUR amount, so the cost includes the fee.
// Each Reward adds its volume and its value on its day. The Average Cost is Holding.AverageCost:
// the sum of the costs divided by the sum of the volumes.
func averageCostHoldings(l Ledger) averageCost {
	result := averageCost{
		holdings: map[string]*Holding{}, rewards: decimal.NewNullDecimal(decimal.Zero), feeSum: decimal.Zero,
	}
	for _, op := range l.CashOperations {
		if op.Type != TypeCryptoPurchase && op.Type != TypeReward {
			continue
		}
		h, ok := result.holdings[op.Ticker]
		if !ok {
			// The asset code is also the name of a crypto Instrument.
			h = &Holding{Instrument: Instrument{Ticker: op.Ticker, Name: op.Ticker}, Volume: decimal.Zero, Cost: decimal.Zero}
			result.holdings[op.Ticker] = h
		}
		h.Volume = h.Volume.Add(op.Volume)
		if op.Type == TypeCryptoPurchase {
			h.Cost = h.Cost.Sub(op.Amount)
			result.feeSum = result.feeSum.Add(l.feeValue(op))
			continue
		}
		value, ok := l.RewardValue(op)
		if !ok {
			h.UnknownCost = true
			result.rewards = decimal.NullDecimal{}
			continue
		}
		h.Cost = h.Cost.Add(value)
		if result.rewards.Valid {
			result.rewards.Decimal = result.rewards.Decimal.Add(value)
		}
	}
	return result
}

// feeValue returns the fee of the crypto purchase in the Account currency. ADR 0009, rule 2.
// A fee in the Account currency counts as it is. A fee in the crypto asset counts at the purchase price.
func (l Ledger) feeValue(op CashOperation) decimal.Decimal {
	if op.FeeCurrency == l.Currency {
		return op.Fee
	}
	return op.Fee.Mul(op.Price.Decimal)
}

// RewardValue returns the value of the Reward in the Account currency: the volume × the close on or before
// the day of the Reward in UTC. ADR 0009, rule 3. It returns false if the Ledger has no such close,
// or if the Quote Currency is not the Account currency. The Ledger has no Exchange Rates of past days.
func (l Ledger) RewardValue(op CashOperation) (decimal.Decimal, bool) {
	if quote, ok := l.QuoteCurrencies[op.Ticker]; !ok || quote != l.Currency {
		return decimal.Zero, false
	}
	t := op.Time.UTC()
	day := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
	closes := l.Closes[op.Ticker]
	// The closes are in date order. i is the number of closes on or before the day.
	i, _ := slices.BinarySearchFunc(closes, day, func(c Close, d time.Time) int {
		if c.Date.After(d) {
			return 1
		}
		return -1
	})
	if i == 0 {
		return decimal.Zero, false
	}
	return op.Volume.Mul(closes[i-1].Close), true
}

// setValue sets the value of the Holding in the Account currency from its price and, if necessary,
// the Exchange Rate. Otherwise it sets the reason why the Holding has no value.
func (h *Holding) setValue(l Ledger) {
	quote, ok := l.QuoteCurrencies[h.Instrument.Ticker]
	if !ok || quote == "" {
		h.NoValueReason = ReasonUnknownQuoteCurrency
		return
	}
	currency, ok := RateCurrency(quote, l.Currency)
	if !ok {
		h.NoValueReason = ReasonCrossRate
		return
	}
	if h.UnknownCost {
		h.NoValueReason = ReasonNoRewardClose
		return
	}
	h.ExchangeRateCurrency = currency
	m := l.marketValue(h.Instrument.Ticker, quote, currency, h.Volume)
	if m.reason != ReasonNone {
		h.NoValueReason = m.reason
		return
	}
	h.Price, h.Value, h.ExchangeRate = m.price, m.value, m.rate
	h.UnrealizedProfit = m.value.Sub(h.Cost)
}

// MarketValue returns the value of the volume of the Instrument in the Account currency, from the current price
// and, if necessary, the latest Exchange Rate. It uses the same arithmetic as the value of a Holding, but it does not
// need the cost. It returns false if the Instrument has no such value.
func (l Ledger) MarketValue(ticker string, volume decimal.Decimal) (decimal.Decimal, bool) {
	quote, ok := l.QuoteCurrencies[ticker]
	if !ok || quote == "" {
		return decimal.Zero, false
	}
	currency, ok := RateCurrency(quote, l.Currency)
	if !ok {
		return decimal.Zero, false
	}
	m := l.marketValue(ticker, quote, currency, volume)
	return m.value, m.reason == ReasonNone
}

// market is the result of marketValue.
type market struct {
	price, value decimal.Decimal
	// rate is the Exchange Rate that value uses. It is zero if value uses no rate.
	rate ExchangeRate
	// reason tells why there is no value. It is ReasonNone if there is a value.
	reason NoValueReason
}

// marketValue returns the value of the volume in the Account currency. quote is the Quote Currency of the Instrument.
// currency is the currency of the Exchange Rate from RateCurrency, or "" if no conversion is necessary.
func (l Ledger) marketValue(ticker, quote, currency string, volume decimal.Decimal) market {
	closePrice, ok := l.Prices[ticker]
	if !ok {
		return market{reason: ReasonNoPrice}
	}
	m := market{price: closePrice, value: volume.Mul(closePrice)}
	if currency == "" {
		return m
	}
	rate, ok := l.ExchangeRates[currency]
	// A rate of zero or less is not valid. It would give a wrong value or a division by zero.
	if !ok || !rate.Rate.IsPositive() {
		return market{reason: ReasonNoExchangeRate}
	}
	m.value, m.rate = Convert(m.value, quote, rate.Rate), rate
	return m
}

// Convert converts the amount in the currency from to the other currency of the rate.
// The rate is the PLN amount for 1 unit of the currency that is not PLN.
// The division keeps decimal.DivisionPrecision (16) decimal places.
func Convert(amount decimal.Decimal, from string, rate decimal.Decimal) decimal.Decimal {
	if from == PLN {
		return amount.Div(rate)
	}
	return amount.Mul(rate)
}
