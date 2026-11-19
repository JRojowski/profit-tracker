package httpapi

import (
	"fmt"
	"net/http"
	"slices"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

// overviewResponse is the body of GET /api/accounts/{id}/instruments/{ticker}/overview.
// It is the Instrument Overview of one Instrument on one Account. Later tickets add the Holding,
// the open Lots and the Closed Positions as new fields.
type overviewResponse struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	// The price source state has the same fields and names as a row of the Holdings response.
	priceSourceResponse
	Prices overviewPrices `json:"prices"`
	// Fills are the Fills of the Instrument on the Account, in time order.
	Fills []overviewFill `json:"fills"`
	// Holding is the same row as the Holdings endpoint gives for the Instrument. It is nil when the Account has no open volume.
	Holding *holdingResponse `json:"holding"`
	// OpenLots are the Lots of the Instrument with an open volume. It is an empty array when there is none.
	OpenLots []overviewOpenLot `json:"openLots"`
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
	// PositionID is the ID of the Lot of the Fill.
	PositionID string `json:"positionId"`
}

// instrumentOverview returns the price series and the Fills of one Instrument on one Account.
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
	if len(lots) == 0 {
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
	writeJSON(w, http.StatusOK, overviewResponse{
		Ticker:              ticker,
		Name:                lots[0].Instrument.Name,
		priceSourceResponse: toPriceSourceResponse(inst),
		Prices:              toOverviewPrices(prices),
		Fills:               toOverviewFills(lots),
		Holding:             holding,
		OpenLots:            openLots,
	})
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
