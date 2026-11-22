package demo

import (
	"fmt"
	"slices"
	"time"

	"github.com/shopspring/decimal"

	"github.com/jrojowski/profit-tracker/backend/internal/kraken/krakentest"
)

// feeCurrency tells on which row of a purchase the fee is.
type feeCurrency int

const (
	// feeInBTC is a fee on the BTC row. It decreases the BTC volume.
	feeInBTC feeCurrency = iota
	// feeInEUR is a fee on the EUR row. It decreases the cash.
	feeInEUR
)

// krakenPurchase is one synthetic Bitcoin purchase. The price is a round number near the BTC-EUR close of the day,
// so the price check shows no Price Warning.
type krakenPurchase struct {
	at            time.Time
	eur, btc, fee string
	feeIn         feeCurrency
}

// krakenPurchases are monthly purchases of about 200 EUR. The fee is 0.4 %.
var krakenPurchases = []krakenPurchase{
	{day(2025, 1, 10), "-199.64", "0.00217000", "0.00000868", feeInBTC},
	{day(2025, 2, 10), "-200.45", "0.00211000", "0.00000844", feeInBTC},
	{day(2025, 3, 10), "-200.16", "0.00278000", "0.00001112", feeInBTC},
	{day(2025, 4, 10), "-200.22", "0.00282000", "0.00001128", feeInBTC},
	{day(2025, 5, 10), "-199.95", "0.00215000", "0.80", feeInEUR},
	{day(2025, 6, 10), "-199.68", "0.00208000", "0.00000832", feeInBTC},
	{day(2025, 7, 10), "-199.98", "0.00202000", "0.00000808", feeInBTC},
	{day(2025, 8, 10), "-199.92", "0.00196000", "0.00000784", feeInBTC},
	{day(2025, 9, 10), "-199.82", "0.00206000", "0.00000824", feeInBTC},
	{day(2025, 10, 10), "-199.82", "0.00206000", "0.80", feeInEUR},
	{day(2025, 11, 10), "-199.64", "0.00217000", "0.00000868", feeInBTC},
	{day(2025, 12, 10), "-199.87", "0.00253000", "0.00001012", feeInBTC},
	{day(2026, 1, 10), "-199.68", "0.00256000", "0.00001024", feeInBTC},
	{day(2026, 2, 10), "-200.10", "0.00345000", "0.00001380", feeInBTC},
	{day(2026, 3, 10), "-199.80", "0.00333000", "0.80", feeInEUR},
	{day(2026, 4, 10), "-200.26", "0.00323000", "0.00001292", feeInBTC},
	{day(2026, 5, 10), "-200.20", "0.00286000", "0.00001144", feeInBTC},
	{day(2026, 6, 10), "-199.81", "0.00377000", "0.00001508", feeInBTC},
	{day(2026, 7, 10), "-199.92", "0.00357000", "0.00001428", feeInBTC},
	{day(2026, 8, 10), "-200.20", "0.00364000", "0.80", feeInEUR},
	{day(2026, 9, 10), "-199.98", "0.00303000", "0.00001212", feeInBTC},
}

// krakenDeposits are the days of the quarterly EUR deposits of 700.00.
var krakenDeposits = []time.Time{
	day(2025, 1, 6), day(2025, 4, 7), day(2025, 7, 7), day(2025, 10, 6), day(2026, 1, 5), day(2026, 4, 6), day(2026, 7, 6),
}

// The weekly Rewards start on the Monday after the first purchase and stop at the last Monday before the end.
// Each Reward is 0.00000002 BTC larger than the Reward before it.
var (
	firstReward = time.Date(2025, 1, 13, 2, 0, 0, 0, time.UTC)
	lastReward  = time.Date(2026, 10, 19, 2, 0, 0, 0, time.UTC)
	rewardStart = decimal.RequireFromString("0.00000020")
	rewardStep  = decimal.RequireFromString("0.00000002")
)

// Kraken returns the synthetic Kraken ledger export of the demo Kraken Account:
// quarterly EUR deposits, monthly Bitcoin purchases and weekly Rewards. Some purchases have the fee in EUR.
func Kraken() krakentest.Export {
	var entries []krakentest.Entry
	for i, d := range krakenDeposits {
		entries = append(entries, krakentest.Deposit(fmt.Sprintf("DEMO-DEP-%02d", i+1), d, "700.00"))
	}
	for i, p := range krakenPurchases {
		refID := fmt.Sprintf("DEMO-TRD-%02d", i+1)
		at := p.at
		if p.feeIn == feeInBTC {
			entries = append(entries, krakentest.Purchase(refID, at, p.eur, "BTC", p.btc, p.fee)...)
			continue
		}
		entries = append(entries,
			krakentest.Entry{RefID: refID, Time: at, Type: "trade", Subtype: "tradespot", Asset: "EUR", Amount: p.eur, Fee: p.fee},
			krakentest.Entry{RefID: refID, Time: at, Type: "trade", Subtype: "tradespot", Asset: "BTC", Amount: p.btc},
		)
	}
	amount := rewardStart
	for i, at := 0, firstReward; !at.After(lastReward); i, at = i+1, at.AddDate(0, 0, 7) {
		entries = append(entries, krakentest.Entry{
			RefID: fmt.Sprintf("DEMO-ERN-%03d", i+1), Time: at, Type: "earn", Subtype: "reward",
			Asset: "BTC", Amount: amount.StringFixed(8),
		})
		amount = amount.Add(rewardStep)
	}
	// A real export puts the oldest entry first.
	slices.SortStableFunc(entries, func(a, b krakentest.Entry) int { return a.Time.Compare(b.Time) })
	return krakentest.Export{FileName: "kraken_spot_ledgers_2025-01-01-2026-10-31.csv", Entries: entries}
}
