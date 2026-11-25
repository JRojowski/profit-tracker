package httpapi

import (
	"cmp"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/store"
	"github.com/jrojowski/profit-tracker/backend/internal/tier"
)

// tiersResponse is the body of GET /api/tiers and of a successful PUT /api/tiers. Target Shares are decimal text
// that is a percentage: "12.5" is 12.5 %.
type tiersResponse struct {
	// Tiers are in the order of the order numbers.
	Tiers []tierResponse `json:"tiers"`
	// Instruments is the list for assignment: each Instrument with one or more Fills on a Tier Account,
	// in ticker order.
	Instruments []tierInstrumentResponse `json:"instruments"`
}

// tierResponse is one Tier.
type tierResponse struct {
	Order       int    `json:"order"`
	Name        string `json:"name"`
	TargetShare string `json:"targetShare"`
	// SharedPart has the name that the user gave, or an empty name, and the rest of the Tier to 100.
	SharedPart sharedPartResponse `json:"sharedPart"`
	// InstrumentTargets are the Instruments with their own Target Share, in ticker order.
	InstrumentTargets []instrumentTargetResponse `json:"instrumentTargets"`
	// SharedInstruments are the Instruments of the Shared Part, in ticker order.
	SharedInstruments []sharedInstrumentResponse `json:"sharedInstruments"`
}

// sharedPartResponse is the Shared Part of a Tier.
type sharedPartResponse struct {
	Name        string `json:"name"`
	TargetShare string `json:"targetShare"`
}

// instrumentTargetResponse is an Instrument Target of a Tier.
type instrumentTargetResponse struct {
	Ticker      string `json:"ticker"`
	Name        string `json:"name"`
	TargetShare string `json:"targetShare"`
}

// sharedInstrumentResponse is an Instrument of the Shared Part of a Tier.
type sharedInstrumentResponse struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
}

// tierInstrumentResponse is one Instrument of the list for assignment.
type tierInstrumentResponse struct {
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	// TierOrder is the order number of the Tier of the Instrument. It is nil if the Instrument has no Tier.
	TierOrder *int `json:"tierOrder"`
	// TargetShare is the Target Share of the Instrument Target. It is nil if the Instrument has no Tier or is in the
	// Shared Part.
	TargetShare *string `json:"targetShare"`
	// HasOpenHolding tells that the Instrument has a Holding on a Tier Account.
	HasOpenHolding bool `json:"hasOpenHolding"`
}

// tiersRequest is the body of PUT /api/tiers. It is the full configuration.
type tiersRequest struct {
	Tiers []tierRequest `json:"tiers"`
}

// tierRequest is one Tier of the request. The numbers are raw JSON, a number or a text, so that a bad number is a
// mistake of its Tier (422) and not a JSON error (400).
type tierRequest struct {
	Order          json.RawMessage `json:"order"`
	Name           string          `json:"name"`
	TargetShare    json.RawMessage `json:"targetShare"`
	SharedPartName string          `json:"sharedPartName"`
	// Instruments are the Instruments of the Tier.
	Instruments []assignmentRequest `json:"instruments"`
}

// assignmentRequest puts an Instrument in a Tier. A null or missing Target Share puts it in the Shared Part.
type assignmentRequest struct {
	Ticker      string          `json:"ticker"`
	TargetShare json.RawMessage `json:"targetShare"`
}

// rawText returns the text of a raw JSON number or string, with no quotes. A missing value and null give ok false.
func rawText(raw json.RawMessage) (text string, ok bool) {
	text = strings.TrimSpace(string(raw))
	if text == "" || text == "null" {
		return "", false
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return strings.TrimSpace(s), true
	}
	return text, true
}

// parseTiers converts the request to Tiers. A number that is not valid gives an error of its Tier, and the
// function returns no Tiers.
func parseTiers(req []tierRequest) ([]tier.Tier, []tier.Error) {
	var errs []tier.Error
	tiers := make([]tier.Tier, 0, len(req))
	for i, t := range req {
		name := strings.TrimSpace(t.Name)
		add := func(ticker, format string, args ...any) {
			errs = append(errs, tier.Error{TierIndex: i, Ticker: ticker, Message: fmt.Sprintf(format, args...)})
		}
		label := fmt.Sprintf("Tier %q", name)

		var order int
		if text, ok := rawText(t.Order); ok {
			n, err := strconv.Atoi(text)
			if err != nil {
				add("", "%s: the order number %q is not a whole number.", label, text)
			}
			order = n
		}
		var share decimal.Decimal
		if text, ok := rawText(t.TargetShare); ok {
			d, err := decimal.NewFromString(text)
			if err != nil {
				add("", "%s: the Target Share %q is not a number.", label, text)
			}
			share = d
		}
		assignments := make([]tier.Assignment, 0, len(t.Instruments))
		for _, in := range t.Instruments {
			a := tier.Assignment{Ticker: in.Ticker}
			if text, ok := rawText(in.TargetShare); ok {
				d, err := decimal.NewFromString(text)
				if err != nil {
					add(in.Ticker, "%s: the Target Share %q of %s is not a number.", label, text, in.Ticker)
				}
				a.TargetShare = decimal.NewNullDecimal(d)
			}
			assignments = append(assignments, a)
		}
		tiers = append(tiers, tier.Tier{
			Order: order, Name: name, TargetShare: share, SharedPartName: strings.TrimSpace(t.SharedPartName),
			Instruments: assignments,
		})
	}
	if len(errs) > 0 {
		return nil, errs
	}
	return tiers, nil
}

// tiersErrorResponse is the 422 body of PUT /api/tiers. Nothing is saved.
type tiersErrorResponse struct {
	// Error is a summary sentence.
	Error string `json:"error"`
	// Errors has one entry for each mistake. The frontend shows an entry next to its Tier.
	Errors []tierErrorResponse `json:"errors"`
}

// tierErrorResponse is one mistake of a configuration of Tiers.
type tierErrorResponse struct {
	// TierIndex is the index of the Tier in the request, from 0. It is nil if the mistake belongs to no single Tier,
	// for example when the Target Shares of the Tiers do not add up to 100.
	TierIndex *int `json:"tierIndex"`
	// Ticker is the ticker of the Instrument with the mistake. It is empty if the mistake is not about one Instrument.
	Ticker string `json:"ticker"`
	// Message is a sentence for the user. It names the Tier.
	Message string `json:"message"`
}

// toTiersErrorResponse returns the 422 body for the mistakes.
func toTiersErrorResponse(errs []tier.Error) tiersErrorResponse {
	resp := tiersErrorResponse{
		Error:  fmt.Sprintf("the configuration of the Tiers has %d error(s), nothing is saved", len(errs)),
		Errors: make([]tierErrorResponse, 0, len(errs)),
	}
	for _, e := range errs {
		entry := tierErrorResponse{Ticker: e.Ticker, Message: e.Message}
		if e.TierIndex != tier.NoTier {
			entry.TierIndex = new(e.TierIndex)
		}
		resp.Errors = append(resp.Errors, entry)
	}
	return resp
}

// getTiers returns the Tiers, their Instruments and the list of Instruments for assignment.
func (a *api) getTiers(w http.ResponseWriter, r *http.Request) {
	resp, err := a.readTiers(r)
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, resp)
}

// putTiers replaces the full configuration of the Tiers in one transaction.
func (a *api) putTiers(w http.ResponseWriter, r *http.Request) {
	var req tiersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "the body is not valid JSON: "+err.Error())
		return
	}
	tiers, errs := parseTiers(req.Tiers)
	if len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, toTiersErrorResponse(errs))
		return
	}
	// An Instrument is assignable when it has Fills on a Tier Account, or when a saved Tier holds it already.
	// The second case keeps an old configuration saveable.
	options, err := a.store.TierAccountInstruments(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	saved, err := a.store.Tiers(r.Context())
	if err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	assignable := map[string]bool{}
	for _, o := range options {
		assignable[o.Ticker] = true
	}
	for _, t := range saved {
		for _, in := range t.Instruments {
			assignable[in.Ticker] = true
		}
	}
	known := func(ticker string) bool { return assignable[ticker] }
	if errs := tier.Validate(tiers, known); len(errs) > 0 {
		writeJSON(w, http.StatusUnprocessableEntity, toTiersErrorResponse(errs))
		return
	}
	if err := a.store.ReplaceTiers(r.Context(), tiers); err != nil {
		a.writeInternalError(w, r, err)
		return
	}
	a.getTiers(w, r)
}

// readTiers reads the configuration and the list of Instruments for assignment.
func (a *api) readTiers(r *http.Request) (tiersResponse, error) {
	ctx := r.Context()
	tiers, err := a.store.Tiers(ctx)
	if err != nil {
		return tiersResponse{}, err
	}
	instruments, err := a.store.Instruments(ctx)
	if err != nil {
		return tiersResponse{}, err
	}

	options, err := a.store.TierAccountInstruments(ctx)
	if err != nil {
		return tiersResponse{}, err
	}
	open, err := a.openTierHoldings(ctx)
	if err != nil {
		return tiersResponse{}, err
	}

	resp := tiersResponse{
		Tiers:       make([]tierResponse, 0, len(tiers)),
		Instruments: make([]tierInstrumentResponse, 0, len(options)),
	}
	assigned := map[string]assignedInstrument{}
	for _, t := range tiers {
		resp.Tiers = append(resp.Tiers, toTierResponse(t, instruments))
		for _, in := range t.Instruments {
			assigned[in.Ticker] = assignedInstrument{tierOrder: t.Order, targetShare: in.TargetShare}
		}
	}
	// A saved Tier can hold an Instrument with no Fill on a Tier Account. The list keeps it, so the editor does not
	// drop it at the next save.
	listed := make(map[string]bool, len(options))
	for _, o := range options {
		listed[o.Ticker] = true
	}
	for ticker := range assigned {
		if !listed[ticker] {
			options = append(options, store.TierAccountInstrument{Ticker: ticker, Name: instruments[ticker].Name})
		}
	}
	slices.SortFunc(options, func(x, y store.TierAccountInstrument) int { return cmp.Compare(x.Ticker, y.Ticker) })
	for _, o := range options {
		item := tierInstrumentResponse{Ticker: o.Ticker, Name: o.Name, HasOpenHolding: open[o.Ticker]}
		if as, ok := assigned[o.Ticker]; ok {
			item.TierOrder = new(as.tierOrder)
			if as.targetShare.Valid {
				item.TargetShare = new(as.targetShare.Decimal.String())
			}
		}
		resp.Instruments = append(resp.Instruments, item)
	}
	return resp, nil
}

// assignedInstrument is the Tier of an Instrument, for the list for assignment.
type assignedInstrument struct {
	tierOrder   int
	targetShare decimal.NullDecimal
}

// openTierHoldings returns the tickers of the Holdings on the Tier Accounts.
func (a *api) openTierHoldings(ctx context.Context) (map[string]bool, error) {
	accounts, err := a.store.Accounts(ctx)
	if err != nil {
		return nil, err
	}
	open := map[string]bool{}
	for _, acc := range accounts {
		if !tier.IsTierAccount(acc.Type, acc.Provider, acc.Currency) {
			continue
		}
		view, err := a.readAccountView(ctx, acc)
		if err != nil {
			return nil, err
		}
		for _, h := range view.state.Holdings {
			open[h.Instrument.Ticker] = true
		}
	}
	return open, nil
}

// toTierResponse returns the response item of the Tier. The names come from the Instruments.
func toTierResponse(t tier.Tier, instruments map[string]store.Instrument) tierResponse {
	resp := tierResponse{
		Order: t.Order, Name: t.Name, TargetShare: t.TargetShare.String(),
		SharedPart:        sharedPartResponse{Name: t.SharedPartName, TargetShare: t.SharedPartShare().String()},
		InstrumentTargets: []instrumentTargetResponse{},
		SharedInstruments: []sharedInstrumentResponse{},
	}
	for _, in := range t.Instruments {
		name := instruments[in.Ticker].Name
		if in.TargetShare.Valid {
			resp.InstrumentTargets = append(resp.InstrumentTargets, instrumentTargetResponse{
				Ticker: in.Ticker, Name: name, TargetShare: in.TargetShare.Decimal.String(),
			})
			continue
		}
		resp.SharedInstruments = append(resp.SharedInstruments, sharedInstrumentResponse{Ticker: in.Ticker, Name: name})
	}
	return resp
}
