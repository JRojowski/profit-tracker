package httpapi

import (
	"cmp"
	"context"
	"encoding/json"
	"slices"
	"strings"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/assistant"
	"github.com/jrojowski/profit-tracker/backend/internal/exchange"
	"github.com/jrojowski/profit-tracker/backend/internal/ledger"
	"github.com/jrojowski/profit-tracker/backend/internal/store"
)

const (
	// closedPositionsDefaultLimit is the number of rows of get_closed_positions when the model gives no limit.
	closedPositionsDefaultLimit = 20
	// closedPositionsMaxLimit is the largest number of rows of get_closed_positions. A larger limit becomes this number.
	closedPositionsMaxLimit = 100

	orderNewestFirst = "newest_first"
	orderProfitAsc   = "profit_asc"
	orderProfitDesc  = "profit_desc"
)

// closedPositionsParameters is the JSON Schema of the arguments of get_closed_positions.
const closedPositionsParameters = `{
  "type": "object",
  "properties": {
    "accountId": {"type": "integer", "description": "The ID of the Account. Omit it for all Accounts."},
    "instrument": {"type": "string", "description": "The broker ticker of the Instrument, for example \"NVDA.US\". Omit it for all Instruments."},
    "from": {"type": "string", "description": "The first day of the period, as YYYY-MM-DD. The day is included. The day of a Closed Position is its close day in Polish time."},
    "to": {"type": "string", "description": "The last day of the period, as YYYY-MM-DD. The day is included."},
    "order": {"type": "string", "enum": ["profit_asc", "profit_desc"], "description": "profit_desc gives the best Closed Positions first, profit_asc gives the worst first. Omit it for the newest first."},
    "limit": {"type": "integer", "description": "The largest number of rows. The default is 20 and the maximum is 100. The sums always cover all rows that agree with the filters."}
  },
  "additionalProperties": false
}`

// closedPositionsArguments are the arguments of get_closed_positions.
type closedPositionsArguments struct {
	AccountID  *int64 `json:"accountId"`
	Instrument string `json:"instrument"`
	From       string `json:"from"`
	To         string `json:"to"`
	Order      string `json:"order"`
	Limit      *int   `json:"limit"`
}

// closedPositionsResult is the result of get_closed_positions.
type closedPositionsResult struct {
	Scope string `json:"scope"`
	// Instrument, From and To repeat the filters. They are empty when the result has no such filter.
	Instrument string `json:"instrument,omitempty"`
	From       string `json:"from,omitempty"`
	To         string `json:"to,omitempty"`
	// Order is "newest_first", "profit_asc" or "profit_desc".
	Order string `json:"order"`
	// Limit is the row limit that the result used.
	Limit int `json:"limit"`
	// TotalRows is the number of Closed Positions that agree with the filters.
	TotalRows int `json:"totalRows"`
	// RowsLeftOut is TotalRows minus the number of rows in Rows.
	RowsLeftOut int `json:"rowsLeftOut"`
	// RealizedProfitSums holds the sum of each currency over all TotalRows rows, in currency order.
	RealizedProfitSums []realizedProfitSum          `json:"realizedProfitSums"`
	Rows               []assistantClosedPositionRow `json:"rows"`
}

// realizedProfitSum is the Realized Profit of the Closed Positions in one currency, as decimal text.
type realizedProfitSum struct {
	Currency string `json:"currency"`
	Sum      string `json:"sum"`
	// Rows is the number of Closed Positions in the sum.
	Rows int `json:"rows"`
}

// assistantClosedPositionRow is one Closed Position with its Account. The amounts are in the Account currency.
// It has no field for the broker IDs of the Closed Position: the model gets no identifier from the export.
type assistantClosedPositionRow struct {
	AccountID   int64  `json:"accountId"`
	AccountName string `json:"accountName"`
	Currency    string `json:"currency"`
	// Time is RFC 3339 text in UTC.
	Time   string `json:"time"`
	Ticker string `json:"ticker"`
	Name   string `json:"name"`
	// Volume is the closed volume of the Lot.
	Volume         string `json:"volume"`
	Amount         string `json:"amount"`
	Cost           string `json:"cost"`
	RealizedProfit string `json:"realizedProfit"`
	// OpenTime is the time of the first Fill of the Lot, RFC 3339 text in UTC.
	OpenTime string `json:"openTime"`
	// OpenPrice is null if a Fill has no price. ClosePrice is null if the sell operation has no price.
	OpenPrice  *string `json:"openPrice"`
	ClosePrice *string `json:"closePrice"`
}

// closedPositionEntry is a Closed Position of the Ledger with the Account it belongs to.
type closedPositionEntry struct {
	row assistantClosedPositionRow
	// position is the Closed Position of the row. Its time and its XTB ID order the rows, and its Realized
	// Profit goes into the sums. The row has no XTB ID.
	position ledger.ClosedPosition
}

// toAssistantClosedPositionRow returns the row of the Closed Position of the Account.
func toAssistantClosedPositionRow(acc store.Account, cp ledger.ClosedPosition) assistantClosedPositionRow {
	return assistantClosedPositionRow{
		AccountID: acc.ID, AccountName: acc.Name, Currency: acc.Currency,
		Time: cp.Time.UTC().Format(time.RFC3339), Ticker: cp.Instrument.Ticker, Name: cp.Instrument.Name,
		Volume: cp.Volume.String(), Amount: formatRoundedAmount(cp.Amount), Cost: formatRoundedAmount(cp.Cost),
		RealizedProfit: formatRoundedAmount(cp.RealizedProfit), OpenTime: cp.OpenTime.UTC().Format(time.RFC3339),
		OpenPrice: nullPrice(cp.OpenPrice), ClosePrice: nullPrice(cp.ClosePrice),
	}
}

// closedPositionsFunction returns get_closed_positions: the Closed Positions with their Realized Profit sum.
func (a *api) closedPositionsFunction() assistant.Function {
	return assistant.Function{
		Definition: assistant.FunctionDefinition{
			Name: "get_closed_positions",
			Description: "Returns the Closed Positions of one Account or of all Accounts, optionally of one Instrument " +
				"and in a period. Use it for the Realized Profit and for the best and the worst Closed Positions. " +
				"The result holds the rows, the number of rows that it left out because of the row limit, " +
				"and the Realized Profit sum for each currency over all rows that agree with the filters.",
			Parameters: json.RawMessage(closedPositionsParameters),
		},
		Run: a.runClosedPositions,
	}
}

// runClosedPositions returns the result of get_closed_positions. It uses the same calculation as
// GET /api/accounts/{id}/closed-positions.
func (a *api) runClosedPositions(ctx context.Context, arguments json.RawMessage) (any, error) {
	var args closedPositionsArguments
	if err := assistant.DecodeArguments(arguments, &args); err != nil {
		return nil, err
	}
	first, last, err := closedPositionsPeriod(args.From, args.To)
	if err != nil {
		return nil, err
	}
	order, err := closedPositionsOrder(args.Order)
	if err != nil {
		return nil, err
	}
	limit, err := closedPositionsLimit(args.Limit)
	if err != nil {
		return nil, err
	}
	accounts, scope, err := a.assistantScope(ctx, args.AccountID)
	if err != nil {
		return nil, err
	}
	ticker := ""
	if args.Instrument != "" {
		if ticker, err = a.assistantInstrument(ctx, args.Instrument); err != nil {
			return nil, err
		}
	}

	var entries []closedPositionEntry
	for _, acc := range accounts {
		view, err := a.readAccountView(ctx, acc)
		if err != nil {
			return nil, err
		}
		for _, cp := range view.state.ClosedPositions {
			if closedPositionAgrees(cp, ticker, first, last) {
				entries = append(entries, closedPositionEntry{
					row: toAssistantClosedPositionRow(acc, cp), position: cp,
				})
			}
		}
	}

	sums := sumRealizedProfit(entries)
	slices.SortStableFunc(entries, closedPositionOrder(order))
	rows := make([]assistantClosedPositionRow, 0, min(limit, len(entries)))
	for _, e := range entries[:min(limit, len(entries))] {
		rows = append(rows, e.row)
	}
	return closedPositionsResult{
		Scope: scope, Instrument: ticker, From: args.From, To: args.To, Order: order, Limit: limit,
		TotalRows: len(entries), RowsLeftOut: len(entries) - len(rows), RealizedProfitSums: sums, Rows: rows,
	}, nil
}

// closedPositionsPeriod parses the period. An empty text means no limit, and the result is the zero time.
// A first day after the last day is an ArgumentError.
func closedPositionsPeriod(from, to string) (first, last time.Time, err error) {
	if first, err = parseAssistantDay("from", from); err != nil {
		return first, last, err
	}
	if last, err = parseAssistantDay("to", to); err != nil {
		return first, last, err
	}
	if !first.IsZero() && !last.IsZero() && first.After(last) {
		return first, last, assistant.Argumentf("from %s is after to %s. The first day of the period must not be after its last day.", from, to)
	}
	return first, last, nil
}

// parseAssistantDay parses a day in the form YYYY-MM-DD. An empty text gives the zero time.
func parseAssistantDay(name, text string) (time.Time, error) {
	if text == "" {
		return time.Time{}, nil
	}
	day, err := time.Parse(time.DateOnly, text)
	if err != nil {
		return time.Time{}, assistant.Argumentf("%s %q is not a date in the form YYYY-MM-DD.", name, text)
	}
	return day, nil
}

// closedPositionsOrder returns the order of the rows. An empty text means newest first.
func closedPositionsOrder(order string) (string, error) {
	switch order {
	case "":
		return orderNewestFirst, nil
	case orderProfitAsc, orderProfitDesc:
		return order, nil
	}
	return "", assistant.Argumentf("unknown order %q. Use %q or %q, or omit order for the newest first.", order, orderProfitAsc, orderProfitDesc)
}

// closedPositionsLimit returns the row limit. A limit above the maximum becomes the maximum.
func closedPositionsLimit(limit *int) (int, error) {
	if limit == nil {
		return closedPositionsDefaultLimit, nil
	}
	if *limit < 1 {
		return 0, assistant.Argumentf("limit %d is not valid. The limit is at least 1.", *limit)
	}
	return min(*limit, closedPositionsMaxLimit), nil
}

// closedPositionAgrees tells if the Closed Position agrees with the filters. The day of a Closed Position is its
// close day in Polish time, as on the Closed Positions page. first and last are included. A zero time means no limit.
func closedPositionAgrees(cp ledger.ClosedPosition, ticker string, first, last time.Time) bool {
	if ticker != "" && cp.Instrument.Ticker != ticker {
		return false
	}
	day := exchange.DayInWarsaw(cp.Time)
	return (first.IsZero() || !day.Before(first)) && (last.IsZero() || !day.After(last))
}

// sumRealizedProfit adds the Realized Profit per currency. It never joins two currencies.
func sumRealizedProfit(entries []closedPositionEntry) []realizedProfitSum {
	byCurrency := map[string][]ledger.ClosedPosition{}
	for _, e := range entries {
		byCurrency[e.row.Currency] = append(byCurrency[e.row.Currency], e.position)
	}
	sums := make([]realizedProfitSum, 0, len(byCurrency))
	for cur, positions := range byCurrency {
		sums = append(sums, realizedProfitSum{
			Currency: cur, Sum: formatRoundedAmount(ledger.SumRealizedProfit(positions)), Rows: len(positions),
		})
	}
	slices.SortFunc(sums, func(x, y realizedProfitSum) int { return strings.Compare(x.Currency, y.Currency) })
	return sums
}

// closedPositionOrder returns the comparison of the rows for the order. A tie goes to the newer Closed Position.
func closedPositionOrder(order string) func(a, b closedPositionEntry) int {
	newest := func(a, b closedPositionEntry) int {
		return cmp.Or(
			b.position.Time.Compare(a.position.Time),
			strings.Compare(b.position.XTBID, a.position.XTBID),
			cmp.Compare(a.row.AccountID, b.row.AccountID),
		)
	}
	switch order {
	case orderProfitAsc:
		return func(a, b closedPositionEntry) int {
			return cmp.Or(a.position.RealizedProfit.Cmp(b.position.RealizedProfit), newest(a, b))
		}
	case orderProfitDesc:
		return func(a, b closedPositionEntry) int {
			return cmp.Or(b.position.RealizedProfit.Cmp(a.position.RealizedProfit), newest(a, b))
		}
	}
	return newest
}
