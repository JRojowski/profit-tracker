// Package lot builds the Lots of an Account from the Fills of its buy orders. See ADR 0002.
//
// It is a pure calculation with no I/O. It uses the Lot volumes of the Closed Positions and Open Positions
// sheets only to select the case of each order.
package lot

import (
	"cmp"
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb"
)

// Errors of Build. Each error that Build returns wraps one of them.
var (
	// ErrIncompleteOrder tells that the Fill volumes of an order do not add up to the order volume.
	ErrIncompleteOrder = errors.New("the fill volumes of the order do not add up to the order volume")
	// ErrNoCase tells that the Lot volumes of an order match no case of ADR 0002.
	ErrNoCase = errors.New("the lot volumes of the order match no case of ADR 0002")
)

// Build returns the Lot ID of each purchase, by the XTB ID of the purchase.
// purchases are the Stock purchase Cash Operations of the export. Other Cash Operations are ignored.
// closed and open are the rows of the Closed Positions and Open Positions sheets of the same export.
// Build returns an error that wraps ErrIncompleteOrder or ErrNoCase for an order that does not fit ADR 0002.
// The error names the Instrument, the order volume and the time of the order.
func Build(purchases []xtb.CashOperation, closed []xtb.ClosedPosition, open []xtb.OpenLot) (map[string]string, error) {
	orders, err := groupOrders(purchases)
	if err != nil {
		return nil, err
	}
	observed := observedVolumes(closed, open)

	lots := make(map[string]string, len(purchases))
	for _, o := range orders {
		lotID, err := o.lotIDs(observed)
		if err != nil {
			return nil, err
		}
		for _, f := range o.fills {
			lots[f.ID] = lotID(f)
		}
	}
	return lots, nil
}

// order is the Fills of one buy order.
type order struct {
	fills  []xtb.CashOperation
	volume decimal.Decimal
}

func (o order) ticker() string      { return o.fills[0].Ticker }
func (o order) orderVolume() string { return o.fills[0].Purchase.OrderVolume.String() }
func (o order) time() string        { return o.fills[0].Time.UTC().Format(time.RFC3339) }

// describe names the order so that the user can find it in the export.
func (o order) describe() string {
	return fmt.Sprintf("the order of %s for volume %s at %s", o.ticker(), o.orderVolume(), o.time())
}

func (o order) complete() bool { return o.volume.Equal(o.fills[0].Purchase.OrderVolume) }

// groupOrders groups the purchases into orders. ADR 0002, rule 1: the Fills of one order have the same
// Instrument and the same order volume. groupOrders takes them in time order until their volumes add up to
// the order volume. The time between the Fills has no limit.
func groupOrders(purchases []xtb.CashOperation) ([]order, error) {
	fills := make([]xtb.CashOperation, 0, len(purchases))
	for _, p := range purchases {
		if p.Purchase != nil {
			fills = append(fills, p)
		}
	}
	// The export puts the newest operation first. The time order and then the row order give a stable result.
	slices.SortStableFunc(fills, func(a, b xtb.CashOperation) int {
		return cmp.Or(a.Time.Compare(b.Time), cmp.Compare(b.Row, a.Row))
	})

	type key struct{ ticker, orderVolume string }
	var orders []order
	pending := map[key]*order{}
	for _, f := range fills {
		// A Fill with the full order volume is a one-Fill order. It never joins a pending order.
		if f.Purchase.Volume.Equal(f.Purchase.OrderVolume) {
			orders = append(orders, order{fills: []xtb.CashOperation{f}, volume: f.Purchase.Volume})
			continue
		}
		k := key{f.Ticker, f.Purchase.OrderVolume.String()}
		o := pending[k]
		if o == nil {
			o = &order{volume: decimal.Zero}
		}
		o.fills = append(o.fills, f)
		o.volume = o.volume.Add(f.Purchase.Volume)
		switch {
		case o.complete():
			orders = append(orders, *o)
			delete(pending, k)
		case o.volume.GreaterThan(f.Purchase.OrderVolume):
			return nil, fmt.Errorf("%s: the fill volumes add up to %s: %w", o.describe(), o.volume, ErrIncompleteOrder)
		default:
			pending[k] = o
		}
	}
	if len(pending) > 0 {
		left := make([]*order, 0, len(pending))
		for _, o := range pending {
			left = append(left, o)
		}
		// Report the first order in time order, so that the error is the same on each run.
		first := slices.MinFunc(left, func(a, b *order) int { return a.fills[0].Time.Compare(b.fills[0].Time) })
		return nil, fmt.Errorf("%s: the fill volumes add up to %s: %w", first.describe(), first.volume, ErrIncompleteOrder)
	}
	return orders, nil
}

// observedVolumes returns the observed volume of each Lot ID: the sum of its Closed Positions volumes
// plus its Open Positions volume. ADR 0002: this sum does not change with the time of the report.
func observedVolumes(closed []xtb.ClosedPosition, open []xtb.OpenLot) map[string]decimal.Decimal {
	observed := map[string]decimal.Decimal{}
	for _, c := range closed {
		observed[c.PositionID] = observed[c.PositionID].Add(c.Volume)
	}
	for _, o := range open {
		observed[o.PositionID] = observed[o.PositionID].Add(o.Volume)
	}
	return observed
}

// lotIDs selects the case of ADR 0002 for the order and returns the function that gives the Lot ID of a Fill.
func (o order) lotIDs(observed map[string]decimal.Decimal) (func(xtb.CashOperation) string, error) {
	// The volume of the own Fills of each Position ID, in the order of the first Fill.
	var ids []string
	own := map[string]decimal.Decimal{}
	for _, f := range o.fills {
		if _, ok := own[f.PositionID]; !ok {
			ids = append(ids, f.PositionID)
		}
		own[f.PositionID] = own[f.PositionID].Add(f.Purchase.Volume)
	}
	ownLot := func(f xtb.CashOperation) string { return f.PositionID }

	// Case A: all Fills have one ID.
	if len(ids) == 1 {
		return ownLot, nil
	}

	// Case B: one ID has the observed volume of the full order, and all other IDs have 0.
	merged, full, zero := "", 0, 0
	for _, id := range ids {
		switch v := observed[id]; {
		case v.Equal(o.volume):
			merged = id
			full++
		case v.IsZero():
			zero++
		}
	}
	if full == 1 && zero == len(ids)-1 {
		return func(xtb.CashOperation) string { return merged }, nil
	}

	// Case C: each ID has the observed volume of its own Fills.
	separate := true
	for _, id := range ids {
		if !observed[id].Equal(own[id]) {
			separate = false
			break
		}
	}
	if separate {
		return ownLot, nil
	}

	volumes := make([]string, 0, len(ids))
	for _, id := range ids {
		volumes = append(volumes, fmt.Sprintf("%s has %s (own fills %s)", id, observed[id].String(), own[id].String()))
	}
	return nil, fmt.Errorf("%s: observed volumes in Closed Positions and Open Positions: %s: %w",
		o.describe(), strings.Join(volumes, ", "), ErrNoCase)
}
