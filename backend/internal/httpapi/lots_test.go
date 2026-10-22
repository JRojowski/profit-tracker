package httpapi_test

import (
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// orderTime is the time of the first Fill of a buy order in the ADR 0002 examples.
var orderTime = time.Date(2026, 2, 3, 9, 30, 0, 0, time.UTC)

// adr0002ExampleA is ADR 0002 Example A: two Fills with one Position ID.
//
//	One Lot 2860602916, volume 0.27 + 1 = 1.27, purchase amount 10.63 + 39.24 = 49.87.
func adr0002ExampleA() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "56000002",
		Currency:      "USD",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock purchase", Ticker: "FPS.US", Time: orderTime.Add(2 * time.Second), Amount: "-39.24", ID: "503", Comment: "OPEN BUY FPS.US 1/1.27 @ 39.24", PositionID: "2860602916"},
			{Type: "Stock purchase", Ticker: "FPS.US", Time: orderTime, Amount: "-10.63", ID: "502", Comment: "OPEN BUY FPS.US 0.27/1.27 @ 39.37", PositionID: "2860602916"},
			{Type: "Deposit", Time: opTime(1), Amount: "100.00", ID: "501", Comment: "Blik deposit"},
		},
		OpenLots: []xtbtest.OpenLot{
			{Ticker: "FPS.US", PositionID: "2860602916", Volume: "1.27"},
		},
	}
}

// adr0002ExampleB is ADR 0002 Example B: two Fills with different IDs, which the broker merges into one Lot.
//
//	Observed volume of 1851988998: 0.4396 closed + 1.1773 open = 1.6169, the full order.
//	Observed volume of 1851989133: 0.
//	One Lot 1851988998, volume 1.6169, purchase amount 126.35 + 204.59 = 330.94.
//	Sale: cost = 330.94 × 0.4396 / 1.6169 ≈ 89.98, profit = 198.43 − 89.98 ≈ 108.45.
//	Open: volume 1.1773, cost ≈ 240.96.
func adr0002ExampleB() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "50000003",
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "LSMC.DE", Time: saleTime(10), Amount: "198.43", ID: "604", Comment: "CLOSE BUY 0.4396 @ 107.040", PositionID: "1851988998"},
			{Type: "Stock purchase", Ticker: "LSMC.DE", Time: orderTime.Add(2 * time.Second), Amount: "-204.59", ID: "603", Comment: "OPEN BUY 1/1.6169 @ 47.705", PositionID: "1851989133"},
			{Type: "Stock purchase", Ticker: "LSMC.DE", Time: orderTime, Amount: "-126.35", ID: "602", Comment: "OPEN BUY 0.6169/1.6169 @ 47.755", PositionID: "1851988998"},
			{Type: "Deposit", Time: opTime(1), Amount: "400.00", ID: "601", Comment: "Blik deposit"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "LSMC.DE", Volume: "0.4396", OpenTime: orderTime, CloseTime: saleTime(10), ProfitLoss: "108.45", PositionID: "1851988998"},
		},
		OpenLots: []xtbtest.OpenLot{
			{Ticker: "LSMC.DE", PositionID: "1851988998", Volume: "1.1773"},
		},
	}
}

// adr0002ExampleC is ADR 0002 Example C: two Fills with different IDs, which the broker keeps as two Lots.
//
//	Observed volume of 1672382331: 0.4265, the volume of its own Fill.
//	Observed volume of 1672382360: 1, the volume of its own Fill.
//	Lot 1672382331: profit = 56.92 − 56.74 = 0.18. Lot 1672382360: profit = 133.47 − 133.03 = 0.44.
func adr0002ExampleC() xtbtest.Export {
	return xtbtest.Export{
		AccountNumber: "50000004",
		Currency:      "PLN",
		CashOperations: []xtbtest.CashOperation{
			{Type: "Stock sell", Ticker: "EXUS.DE", Time: saleTime(10).Add(time.Second), Amount: "133.47", ID: "706", Comment: "CLOSE BUY 1/16.9887 @ 31.675", PositionID: "1672382360"},
			{Type: "Stock sell", Ticker: "EXUS.DE", Time: saleTime(10), Amount: "56.92", ID: "705", Comment: "CLOSE BUY 0.4265/16.9887 @ 31.675", PositionID: "1672382331"},
			{Type: "Stock purchase", Ticker: "EXUS.DE", Time: orderTime.Add(time.Second), Amount: "-133.03", ID: "703", Comment: "OPEN BUY 1/1.4265 @ 32.005", PositionID: "1672382360"},
			{Type: "Stock purchase", Ticker: "EXUS.DE", Time: orderTime, Amount: "-56.74", ID: "702", Comment: "OPEN BUY 0.4265/1.4265 @ 32.005", PositionID: "1672382331"},
			{Type: "Deposit", Time: opTime(1), Amount: "200.00", ID: "701", Comment: "Blik deposit"},
		},
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "EXUS.DE", Volume: "1", OpenTime: orderTime, CloseTime: saleTime(10).Add(time.Second), ProfitLoss: "0.44", PositionID: "1672382360"},
			{Ticker: "EXUS.DE", Volume: "0.4265", OpenTime: orderTime, CloseTime: saleTime(10), ProfitLoss: "0.18", PositionID: "1672382331"},
		},
	}
}

func TestLots_ADR0002ExampleA_FillsWithOneIDGiveOneLot(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, adr0002ExampleA())
	require.Empty(t, preview.Conflicts)
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB USD", "regular", "USD")).AccountID

	got := srv.holdings(t, accountID)

	require.Len(t, got, 1)
	assert.Equal(t, "FPS.US", got[0].Ticker)
	assert.Equal(t, "1.27", got[0].Volume)
	assert.Equal(t, "49.87", cents(t, got[0].Cost))
}

func TestLots_ADR0002ExampleB_FillsWithDifferentIDsGiveOneMergedLot(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, adr0002ExampleB())
	require.Empty(t, preview.Conflicts, "the Realized Profit agrees with Closed Positions")
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	sales := srv.sales(t, accountID)
	holdings := srv.holdings(t, accountID)

	require.Len(t, sales, 1)
	assert.Equal(t, "1851988998", sales[0].PositionID)
	assert.Equal(t, "89.98", cents(t, sales[0].Cost))
	assert.Equal(t, "108.45", cents(t, sales[0].RealizedProfit))
	require.Len(t, holdings, 1)
	assert.Equal(t, "LSMC.DE", holdings[0].Ticker)
	assert.Equal(t, "1.1773", holdings[0].Volume)
	assert.Equal(t, "240.96", cents(t, holdings[0].Cost))
}

func TestLots_ADR0002ExampleC_FillsWithDifferentIDsGiveOneLotPerID(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	preview := srv.preview(t, adr0002ExampleC())
	require.Empty(t, preview.Conflicts, "the Realized Profit agrees with Closed Positions")
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	sales := srv.sales(t, accountID)

	require.Len(t, sales, 2)
	assert.Equal(t, "1672382331", sales[0].PositionID)
	assert.Equal(t, "0.18", cents(t, sales[0].RealizedProfit))
	assert.Equal(t, "1672382360", sales[1].PositionID)
	assert.Equal(t, "0.44", cents(t, sales[1].RealizedProfit))
	assert.Empty(t, srv.holdings(t, accountID), "both Lots are closed")
}

func TestLots_FillsThatMatchNoCase_RejectTheImport(t *testing.T) {
	t.Parallel()

	// Observed volume of 1851988998: 0.4396 + 1 = 1.4396. This is not the order volume 1.6169,
	// and not the volume of its own Fill 0.6169.
	noCase := adr0002ExampleB()
	noCase.OpenLots[0].Volume = "1"
	// The second Fill of the order is missing. The Fill volumes never add up to the order volume.
	incomplete := adr0002ExampleB()
	incomplete.CashOperations = append(incomplete.CashOperations[:1], incomplete.CashOperations[2:]...)
	// The second Fill comes 11 seconds after the first Fill. It is not a part of the same order.
	late := adr0002ExampleB()
	late.CashOperations[1].Time = orderTime.Add(11 * time.Second)
	// The Fill volumes add up to more than the order volume.
	tooMuch := adr0002ExampleB()
	tooMuch.CashOperations[1].Comment = "OPEN BUY 1.1/1.6169 @ 47.705"

	tests := []struct {
		name      string
		export    xtbtest.Export
		wantError []string
	}{
		{
			name:      "observed volumes fit no case",
			export:    noCase,
			wantError: []string{"LSMC.DE", "1.6169", "2026-02-03T09:30:00Z", "1851988998", "1.4396", "1851989133"},
		},
		{name: "order with a missing Fill", export: incomplete, wantError: []string{"LSMC.DE", "1.6169", "2026-02-03T09:30:00Z"}},
		{name: "Fill more than 10 seconds later", export: late, wantError: []string{"LSMC.DE", "1.6169", "2026-02-03T09:30:00Z"}},
		{name: "Fills with more than the order volume", export: tooMuch, wantError: []string{"LSMC.DE", "1.6169", "2026-02-03T09:30:00Z"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			srv := startServer(t)

			status, body := srv.upload(t, tc.export.FileName(), xtbtest.Build(t, tc.export))

			assert.Equal(t, http.StatusBadRequest, status)
			var got errorBody
			decode(t, body, &got)
			for _, want := range tc.wantError {
				assert.Contains(t, got.Error, want)
			}
			assert.Empty(t, srv.imports(t))
		})
	}
}

func TestLots_ADR0002ExampleB_SameFileAgain_GivesNoNewOperationsAndNoConflicts(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	accountID := srv.importAccount(t, adr0002ExampleB(), "XTB PLN")

	again := srv.preview(t, adr0002ExampleB())

	assert.Equal(t, 0, again.NewCashOperations)
	assert.Equal(t, 4, again.SkippedCashOperations)
	assert.Empty(t, again.Conflicts)
	srv.confirm(t, again.ID, nil)
	holdings := srv.holdings(t, accountID)
	require.Len(t, holdings, 1)
	assert.Equal(t, "1.1773", holdings[0].Volume, "the merged Lot stays")
	assert.Equal(t, "240.96", cents(t, holdings[0].Cost))
}

func TestLots_OneFillOrderBetweenFillsOfOtherOrder_GivesTwoOrders(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	export := adr0002ExampleB()
	// A separate one-Fill order of the same Instrument and the same volume, 1 second after the first Fill of Example B.
	export.CashOperations = append(export.CashOperations, xtbtest.CashOperation{
		Type: "Stock purchase", Ticker: "LSMC.DE", Time: orderTime.Add(time.Second), Amount: "-200.00", ID: "605",
		Comment: "OPEN BUY 1.6169 @ 123.69", PositionID: "1851989200",
	})
	export.OpenLots = append(export.OpenLots, xtbtest.OpenLot{Ticker: "LSMC.DE", PositionID: "1851989200", Volume: "1.6169"})
	preview := srv.preview(t, export)
	require.Empty(t, preview.Conflicts, "the Realized Profit agrees with Closed Positions")
	accountID := srv.confirm(t, preview.ID, newAccountBody("XTB PLN", "regular", "PLN")).AccountID

	sales := srv.sales(t, accountID)
	holdings := srv.holdings(t, accountID)

	require.Len(t, sales, 1)
	assert.Equal(t, "108.45", cents(t, sales[0].RealizedProfit))
	require.Len(t, holdings, 1)
	assert.Equal(t, "2.7942", holdings[0].Volume, "1.1773 of Example B plus 1.6169 of the one-Fill order")
	assert.Equal(t, "440.96", cents(t, holdings[0].Cost))
}

func TestLots_ReimportMovesKnownFillToLotOfADR0002(t *testing.T) {
	t.Parallel()
	srv := startServer(t)
	// The first file has the two purchases of Example B and no sale. Its Open Positions show
	// one Lot per Fill (case C). Thus the Account has two Lots, as the code before ADR 0002 saved them.
	first := adr0002ExampleB()
	first.CashOperations = first.CashOperations[1:]
	first.ClosedPositions = nil
	first.OpenLots = []xtbtest.OpenLot{
		{Ticker: "LSMC.DE", PositionID: "1851988998", Volume: "0.6169"},
		{Ticker: "LSMC.DE", PositionID: "1851989133", Volume: "1"},
	}
	accountID := srv.importAccount(t, first, "XTB PLN")

	again := srv.preview(t, adr0002ExampleB())

	require.Empty(t, again.Conflicts, "the check uses the merged Lot of Example B")
	assert.Equal(t, 1, again.NewCashOperations)
	assert.Equal(t, 3, again.SkippedCashOperations)
	saved := srv.confirm(t, again.ID, nil)
	assert.Equal(t, 1, saved.NewCashOperations)
	assert.Equal(t, 3, saved.SkippedCashOperations, "a moved Fill counts as skipped")
	sales := srv.sales(t, accountID)
	require.Len(t, sales, 1)
	assert.Equal(t, "1851988998", sales[0].PositionID)
	assert.Equal(t, "108.45", cents(t, sales[0].RealizedProfit))
	holdings := srv.holdings(t, accountID)
	require.Len(t, holdings, 1)
	assert.Equal(t, "1.1773", holdings[0].Volume)
	assert.Equal(t, "240.96", cents(t, holdings[0].Cost))
}
