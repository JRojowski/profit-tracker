package store

import (
	"context"
	"fmt"

	"github.com/jrojowski/profit-tracker/backend/internal/store/queries"
	"github.com/jrojowski/profit-tracker/backend/internal/tier"
)

// Tiers returns the Tiers in order number order, each with its Instruments in ticker order.
func (s *Store) Tiers(ctx context.Context) ([]tier.Tier, error) {
	q := queries.New(s.db)
	rows, err := q.Tiers(ctx)
	if err != nil {
		return nil, fmt.Errorf("read tiers: %w", err)
	}
	assigned, err := q.TierInstruments(ctx)
	if err != nil {
		return nil, fmt.Errorf("read tier instruments: %w", err)
	}

	tiers := make([]tier.Tier, 0, len(rows))
	index := make(map[int64]int, len(rows))
	for _, row := range rows {
		index[row.ID] = len(tiers)
		tiers = append(tiers, tier.Tier{
			Order: int(row.OrderNumber), Name: row.Name, TargetShare: row.TargetShare, SharedPartName: row.SharedPartName,
		})
	}
	for _, a := range assigned {
		i := index[a.TierID]
		tiers[i].Instruments = append(tiers[i].Instruments, tier.Assignment{Ticker: a.Ticker, TargetShare: a.TargetShare})
	}
	return tiers, nil
}

// ReplaceTiers replaces the Tiers and the assignments with tiers, in one transaction.
// The caller checks the configuration with tier.Validate. A ticker that is not an Instrument fails the call.
func (s *Store) ReplaceTiers(ctx context.Context, tiers []tier.Tier) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin tiers save: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	q := queries.New(tx)

	// The deletion of the Tiers deletes the assignments.
	if err := q.DeleteTiers(ctx); err != nil {
		return fmt.Errorf("delete tiers: %w", err)
	}
	for _, t := range tiers {
		id, err := q.InsertTier(ctx, queries.InsertTierParams{
			OrderNumber: int64(t.Order), Name: t.Name, TargetShare: t.TargetShare, SharedPartName: t.SharedPartName,
		})
		if err != nil {
			return fmt.Errorf("save tier %q: %w", t.Name, err)
		}
		for _, a := range t.Instruments {
			err := q.InsertTierInstrument(ctx, queries.InsertTierInstrumentParams{
				Ticker: a.Ticker, TierID: id, TargetShare: a.TargetShare,
			})
			if err != nil {
				return fmt.Errorf("assign %s to tier %q: %w", a.Ticker, t.Name, err)
			}
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit tiers save: %w", err)
	}
	return nil
}

// TierAccountInstrument is an Instrument with one or more Fills on a Tier Account.
type TierAccountInstrument struct {
	Ticker, Name string
}

// TierAccountInstruments returns the Instruments with one or more Fills on a Tier Account, in ticker order.
// A Tier Account is a regular XTB Account in PLN or USD.
func (s *Store) TierAccountInstruments(ctx context.Context) ([]TierAccountInstrument, error) {
	rows, err := queries.New(s.db).TierAccountInstruments(ctx)
	if err != nil {
		return nil, fmt.Errorf("read tier account instruments: %w", err)
	}
	list := make([]TierAccountInstrument, 0, len(rows))
	for _, row := range rows {
		list = append(list, TierAccountInstrument{Ticker: row.Ticker, Name: row.Name})
	}
	return list, nil
}
