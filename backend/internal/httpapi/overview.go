package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// overviewResponse is the body of GET /api/accounts/{id}/instruments/{ticker}/overview.
// It is the Instrument Overview of one Instrument on one Account. It has the identity and the price source state,
// the daily price series, the Fills, the Holding (nil when the Account has no open volume), the open Lots
// and the Closed Positions. For a crypto Instrument on an Account with no Lots (ADR 0009), it also has the Rewards
// and the sum of the fees. The Holding gives the Average Cost. The open Lots and the Closed Positions are empty.
type overviewResponse struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	// The price source state has the same fields and names as a row of the Holdings response.
	priceSourceResponse
	Prices overviewPrices `json:"prices"`
	// Fills are the Fills of the Instrument on the Account, in time order. For a crypto Instrument,
	// they are the crypto purchases.
	Fills []overviewFill `json:"fills"`
	// Holding is the same row as the Holdings endpoint gives for the Instrument. It is nil when the Account has no open volume.
	Holding *holdingResponse `json:"holding"`
	// OpenLots are the Lots of the Instrument with an open volume. It is an empty array when there is none.
	OpenLots []overviewOpenLot `json:"openLots"`
	// ClosedPositions are the items of the Closed Positions endpoint for the Instrument, in the same order.
	// It is an empty array when there is none.
	ClosedPositions []closedPositionResponse `json:"closedPositions"`
	// RealizedProfitSum is the sum of the Realized Profit of the Closed Positions above, in the Account currency, as decimal text.
	// It is nil when the Instrument has no Closed Position on the Account.
	RealizedProfitSum *string `json:"realizedProfitSum"`
	// Rewards are the Rewards of the Instrument on the Account, in time order (ADR 0009).
	// It is an empty array when there is none, for example for an Instrument with Lots.
	Rewards []overviewReward `json:"rewards"`
	// FeeSum is the sum of the fees of the crypto purchases of the Instrument in the Account currency, as decimal text.
	// It is information only (ADR 0009, rule 2). It is nil for an Instrument with Lots.
	FeeSum *string `json:"feeSum"`
}

// overviewReward is one Reward in the Instrument Overview.
type overviewReward struct {
	// XTBID is the ID of the Reward at the Provider, for example the Kraken refid.
	XTBID string `json:"xtbId"`
	// Time is RFC 3339 text in UTC.
	Time string `json:"time"`
	// Volume is the received quantity as decimal text.
	Volume string `json:"volume"`
	// Value is the value of the Reward on its day in the Account currency, as decimal text (ADR 0009, rule 3).
	// It is nil when the tool has no stored close on or before the day of the Reward.
	Value *string `json:"value"`
}

// overviewOpenLot is the open part of one Lot.
type overviewOpenLot struct {
	PositionID string `json:"positionId"`
	// Volume is the open volume. Cost is the open cost in the Account currency. Both are decimal text.
	Volume string `json:"volume"`
	Cost   string `json:"cost"`
	// OpenTime is the time of the first Fill, RFC 3339 text in UTC.
	OpenTime string `json:"openTime"`
	// OpenPrice is the average of the Fill prices weighted by volume. It is nil if a Fill has no price.
	OpenPrice *string `json:"openPrice"`
	// Fills are all Fills of the Lot, in time order.
	Fills []overviewFill `json:"fills"`
}

// overviewPrices is the daily price series of an Instrument. The prices are raw (ADR 0005).
type overviewPrices struct {
	// Currency is the currency of the series. It is nil when the series is empty.
	Currency *string `json:"currency"`
	// Series is the closes of the daily history in date order. It is an empty array when there is none.
	Series []overviewPrice `json:"series"`
}

// overviewPrice is one close of the daily history.
type overviewPrice struct {
	// Date is the price date as YYYY-MM-DD.
	Date string `json:"date"`
	// Close is decimal text.
	Close string `json:"close"`
}

// overviewFill is one Fill in the Instrument Overview.
type overviewFill struct {
	XTBID string `json:"xtbId"`
	// Time is RFC 3339 text in UTC.
	Time string `json:"time"`
	// Volume is decimal text.
	Volume string `json:"volume"`
	// Price is the XTB price of one unit as decimal text, or nil if the Fill has no price.
	Price *string `json:"price"`
	// PositionID is the ID of the Lot of the Fill. It is empty for a crypto purchase, because it has no Lot.
	PositionID string `json:"positionId"`
	// Cost is the full amount of a crypto purchase in the Account currency, with a fee in the Account currency,
	// as decimal text (ADR 0009, rule 1). It is nil for a Fill of a Lot.
	Cost *string `json:"cost"`
	// Fee is the fee of a crypto purchase in FeeCurrency, as decimal text. It is nil for a Fill of a Lot.
	Fee *string `json:"fee"`
	// FeeCurrency is the Account currency or the crypto asset, for example "EUR" or "BTC". It is nil for a Fill of a Lot.
	FeeCurrency *string `json:"feeCurrency"`
}

// instrumentOverview returns the Instrument Overview of one Instrument on one Account.
// It reads only stored data and starts no price refresh.
func (a *api) instrumentOverview(w http.ResponseWriter, r *http.Request) {
	id, ok := accountID(w, r)
	if !ok {
		return
	}
	ctx := r.Context()
	acc, err := a.store.Account(ctx, id)
	if err != nil {
		a.writeAccountError(w, r, err)
		return
	}
	ticker := r.PathValue("ticker")
	in, err := a.readHeroInput(ctx, acc)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	// The Ledger has a Lot for each Instrument that the Account ever held, also for a sold one.
	var lots []ledger.Lot
	for _, lot := range in.ledger.Lots {
		if lot.Instrument.Ticker == ticker {
			lots = append(lots, lot)
		}
	}
	// An Instrument on an Account with no Lots has crypto purchases and Rewards (ADR 0009).
	var purchases, rewards []ledger.CashOperation
	for _, op := range in.ledger.CashOperations {
		if op.Ticker != ticker {
			continue
		}
		switch op.Type {
		case ledger.TypeCryptoPurchase:
			purchases = append(purchases, op)
		case ledger.TypeReward:
			rewards = append(rewards, op)
		}
	}
	if len(lots) == 0 && len(purchases) == 0 && len(rewards) == 0 {
		writeError(w, http.StatusNotFound, fmt.Sprintf("account %d never held instrument %q", id, ticker))
		return
	}
	inst, err := a.store.Instrument(ctx, ticker)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	prices, err := a.store.DailyPrices(ctx, ticker)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	// The Holding and the open Lots come from the Ledger calculator, like the Holdings endpoint.
	view, err := viewOf(acc, in)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	warnings, err := a.readAllPriceWarnings(ctx)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	var holding *holdingResponse
	for _, h := range view.state.Holdings {
		if h.Instrument.Ticker == ticker {
			row := view.holdingRow(h, warnings)
			holding = &row
		}
	}
	openLots := []overviewOpenLot{}
	for _, lot := range view.state.OpenLots {
		if lot.Instrument.Ticker != ticker {
			continue
		}
		openLots = append(openLots, overviewOpenLot{
			PositionID: lot.PositionID,
			Volume:     lot.Volume.String(),
			Cost:       formatAmount(lot.Cost),
			OpenTime:   lot.OpenTime.UTC().Format(time.RFC3339),
			OpenPrice:  nullPrice(lot.OpenPrice),
			Fills:      toOverviewFills([]ledger.Lot{{PositionID: lot.PositionID, Fills: lot.Fills}}),
		})
	}
	closedPositions := []closedPositionResponse{}
	var instrumentClosed []ledger.ClosedPosition
	for _, c := range view.state.ClosedPositions {
		if c.Instrument.Ticker == ticker {
			closedPositions = append(closedPositions, toClosedPositionResponse(c))
			instrumentClosed = append(instrumentClosed, c)
		}
	}
	// The sum is null when the Instrument has no Closed Position.
	var realizedProfitSum *string
	if len(instrumentClosed) > 0 {
		realizedProfitSum = new(formatAmount(ledger.SumRealizedProfit(instrumentClosed)))
	}
	resp := overviewResponse{
		Ticker:              ticker,
		priceSourceResponse: toPriceSourceResponse(inst),
		Prices:              toOverviewPrices(prices),
		Holding:             holding,
		OpenLots:            openLots,
		ClosedPositions:     closedPositions,
		RealizedProfitSum:   realizedProfitSum,
		Rewards:             toOverviewRewards(in.ledger, rewards),
	}
	if len(lots) > 0 {
		resp.Name, resp.Fills = lots[0].Instrument.Name, toOverviewFills(lots)
	} else {
		// The asset code is also the name of a crypto Instrument, as in the Holding.
		resp.Name, resp.Fills = ticker, toCryptoFills(purchases)
		feeSum := decimal.Zero
		for _, op := range purchases {
			feeSum = feeSum.Add(in.ledger.FeeValue(op))
		}
		resp.FeeSum = new(formatAmount(feeSum))
	}
	writeJSON(w, http.StatusOK, resp)
}

// toCryptoFills returns the crypto purchases as Fills with no Lot, in the order of the Ledger.
// The Ledger gives the Cash Operations in time order.
func toCryptoFills(purchases []ledger.CashOperation) []overviewFill {
	resp := make([]overviewFill, 0, len(purchases))
	for _, op := range purchases {
		// An empty fee currency with a fee tells the crypto asset (ADR 0009).
		feeCurrency := op.FeeCurrency
		if feeCurrency == "" {
			feeCurrency = op.Ticker
		}
		resp = append(resp, overviewFill{
			XTBID:       op.XTBID,
			Time:        op.Time.UTC().Format(time.RFC3339),
			Volume:      op.Volume.String(),
			Price:       nullPrice(op.Price),
			Cost:        new(formatAmount(op.Amount.Neg())),
			Fee:         new(op.Fee.String()),
			FeeCurrency: &feeCurrency,
		})
	}
	return resp
}

// toOverviewRewards returns the Rewards with their values on their days, in the order of the Ledger.
func toOverviewRewards(l ledger.Ledger, rewards []ledger.CashOperation) []overviewReward {
	resp := make([]overviewReward, 0, len(rewards))
	for _, op := range rewards {
		r := overviewReward{XTBID: op.XTBID, Time: op.Time.UTC().Format(time.RFC3339), Volume: op.Volume.String()}
		if value, ok := l.RewardValue(op); ok {
			r.Value = new(formatAmount(value))
		}
		resp = append(resp, r)
	}
	return resp
}

// toOverviewPrices returns the series of the closes in the currency of the newest close.
// A close in another currency is not on the same scale, so the series leaves it out.
func toOverviewPrices(prices []store.Price) overviewPrices {
	series := []overviewPrice{}
	if len(prices) == 0 {
		return overviewPrices{Series: series}
	}
	currency := prices[len(prices)-1].Currency
	for _, p := range prices {
		if p.Currency == currency {
			series = append(series, overviewPrice{Date: p.Date.Format(time.DateOnly), Close: p.Close.String()})
		}
	}
	return overviewPrices{Currency: &currency, Series: series}
}

// toOverviewFills returns the Fills of the Lots in time order. Fills with the same time keep the order of the Ledger.
func toOverviewFills(lots []ledger.Lot) []overviewFill {
	type fill struct {
		ledger.Fill
		positionID string
	}
	var fills []fill
	for _, lot := range lots {
		for _, f := range lot.Fills {
			fills = append(fills, fill{Fill: f, positionID: lot.PositionID})
		}
	}
	slices.SortStableFunc(fills, func(a, b fill) int { return a.Time.Compare(b.Time) })
	resp := make([]overviewFill, 0, len(fills))
	for _, f := range fills {
		resp = append(resp, overviewFill{
			XTBID:      f.XTBID,
			Time:       f.Time.UTC().Format(time.RFC3339),
			Volume:     f.Volume.String(),
			Price:      nullPrice(f.Price),
			PositionID: f.positionID,
		})
	}
	return resp
}
