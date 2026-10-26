// Package demo holds synthetic XTB exports for a demo database.
//
// The data is made up. The three Accounts cover the cases that the pages show:
// two currencies, a Holding quoted in another currency, a Holding with no price,
// the same Instrument on two Accounts, sales, dividends and IKZE Contributions.
package demo

import (
	"fmt"
	"strconv"
	"time"

	"github.com/jrojowski/profit-tracker/backend/internal/xtb/xtbtest"
)

// Account describes one demo Account and its export.
type Account struct {
	// Name, Type and Currency are the values to give at the confirm of the Import.
	Name, Type, Currency string
	// Export is the content of the export file of the Account.
	Export xtbtest.Export
}

// Accounts returns the demo Accounts: "XTB USD", "XTB PLN" and "XTB IKZE".
func Accounts() []Account {
	return []Account{
		{Name: "XTB USD", Type: "regular", Currency: "USD", Export: usdExport()},
		{Name: "XTB PLN", Type: "regular", Currency: "PLN", Export: plnExport()},
		{Name: "XTB IKZE", Type: "IKZE", Currency: "PLN", Export: ikzeExport()},
	}
}

func day(y, m, d int) time.Time { return time.Date(y, time.Month(m), d, 10, 0, 0, 0, time.UTC) }

type buy struct {
	ticker, name, volume, price, amount string
	at                                  time.Time
}

func buys(start int, list []buy) []xtbtest.CashOperation {
	ops := make([]xtbtest.CashOperation, 0, len(list))
	for i, b := range list {
		ops = append(ops, xtbtest.CashOperation{
			Type: "Stock purchase", Ticker: b.ticker, Instrument: b.name, Time: b.at,
			Amount: b.amount, ID: strconv.Itoa(start + i), Comment: fmt.Sprintf("OPEN BUY %s @ %s", b.volume, b.price),
			PositionID: strconv.Itoa(900000 + start + i),
		})
	}
	return ops
}

func usdExport() xtbtest.Export {
	usd := buys(1000, []buy{
		{"AAPL.US", "Apple", "10", "180.00", "-1800.00", day(2024, 2, 1)},
		{"MSFT.US", "Microsoft", "5", "400.00", "-2000.00", day(2024, 2, 2)},
		{"NVDA.US", "NVIDIA", "12", "90.00", "-1080.00", day(2024, 3, 1)},
		{"AMZN.US", "Amazon", "6", "170.00", "-1020.00", day(2024, 3, 2)},
		{"GOOGL.US", "Alphabet", "6", "150.00", "-900.00", day(2024, 4, 1)},
		{"META.US", "Meta", "2", "480.00", "-960.00", day(2024, 4, 2)},
		{"TSLA.US", "Tesla", "3", "200.00", "-600.00", day(2024, 5, 1)},
		{"KO.US", "Coca-Cola", "8", "60.00", "-480.00", day(2024, 5, 2)},
		{"PEP.US", "PepsiCo", "3", "170.00", "-510.00", day(2024, 6, 1)},
		{"VWCE.DE", "Vanguard FTSE All-World", "5", "110.00", "-600.00", day(2024, 6, 2)},
		{"ZZQX.US", "Unknown Corp", "4", "25.00", "-100.00", day(2024, 6, 3)},
		{"INTC.US", "Intel", "20", "40.00", "-800.00", day(2024, 1, 10)},
	})
	ops := []xtbtest.CashOperation{
		{Type: "Stock sell", Ticker: "INTC.US", Instrument: "Intel", Time: day(2025, 8, 5), Amount: "420.00", ID: "1100", Comment: "CLOSE BUY 20/20 @ 21.00", PositionID: "901011"},
		{Type: "Stock sell", Ticker: "PEP.US", Instrument: "PepsiCo", Time: day(2026, 2, 5), Amount: "160.00", ID: "1101", Comment: "CLOSE BUY 1/3 @ 160.00", PositionID: "901008"},
		{Type: "Dividend", Ticker: "AAPL.US", Instrument: "Apple", Time: day(2025, 5, 15), Amount: "2.50", ID: "1102", Comment: "AAPL.US USD 0.2500/ SHR"},
		{Type: "Withholding tax", Ticker: "AAPL.US", Instrument: "Apple", Time: day(2025, 5, 15), Amount: "-0.38", ID: "1103", Comment: "AAPL.US USD WHT 15%"},
		{Type: "Free funds interest", Time: day(2025, 6, 1), Amount: "4.12", ID: "1104", Comment: "Free-funds Interest"},
		{Type: "SEC fee", Time: day(2026, 2, 5), Amount: "-0.05", ID: "1105", Comment: "SEC fee"},
	}
	ops = append(ops, usd...)
	ops = append(ops,
		xtbtest.CashOperation{Type: "Deposit", Time: day(2024, 1, 5), Amount: "10000.00", ID: "999", Comment: "Deposit"},
		xtbtest.CashOperation{Type: "Deposit", Time: day(2024, 5, 20), Amount: "2000.00", ID: "998", Comment: "Deposit"},
	)
	return xtbtest.Export{
		AccountNumber: "56100001", Currency: "USD", CashOperations: ops,
		ClosedPositions: []xtbtest.ClosedPosition{
			{Ticker: "INTC.US", Instrument: "Intel", Volume: "20", OpenTime: day(2024, 1, 10), CloseTime: day(2025, 8, 5), ProfitLoss: "-380.00", PositionID: "901011"},
			{Ticker: "PEP.US", Instrument: "PepsiCo", Volume: "1", OpenTime: day(2024, 6, 1), CloseTime: day(2026, 2, 5), ProfitLoss: "-10.00", PositionID: "901008"},
		},
	}
}

func plnExport() xtbtest.Export {
	pln := buys(2000, []buy{
		{"PKN.PL", "Orlen", "30", "60.00", "-1800.00", day(2024, 3, 1)},
		{"CDR.PL", "CD Projekt", "10", "120.00", "-1200.00", day(2024, 3, 2)},
		{"AAPL.US", "Apple", "2", "720.00", "-1440.00", day(2024, 3, 3)},
	})
	ops := []xtbtest.CashOperation{
		{Type: "Dividend", Ticker: "PKN.PL", Instrument: "Orlen", Time: day(2025, 8, 20), Amount: "126.00", ID: "2100", Comment: "PKN.PL PLN 4.2000/ SHR"},
		{Type: "Withholding tax", Ticker: "PKN.PL", Instrument: "Orlen", Time: day(2025, 8, 20), Amount: "-23.94", ID: "2101", Comment: "PKN.PL PLN WHT 19%"},
	}
	ops = append(ops, pln...)
	ops = append(ops, xtbtest.CashOperation{Type: "Deposit", Time: day(2024, 2, 20), Amount: "5000.00", ID: "1999", Comment: "Blik deposit"})
	return xtbtest.Export{AccountNumber: "56100002", Currency: "PLN", CashOperations: ops}
}

func ikzeExport() xtbtest.Export {
	ops := buys(3000, []buy{
		{"ETFBW20TR.PL", "Beta ETF WIG20TR", "40", "100.00", "-4000.00", day(2025, 2, 1)},
		{"PZU.PL", "PZU", "100", "50.00", "-5000.00", day(2026, 2, 1)},
	})
	ops = append(ops,
		xtbtest.CashOperation{Type: "IKZE deposit", Time: day(2026, 1, 20), Amount: "6000.00", ID: "2999", Comment: "IKZE deposit"},
		xtbtest.CashOperation{Type: "IKZE deposit", Time: day(2025, 1, 20), Amount: "4500.00", ID: "2998", Comment: "IKZE deposit"},
	)
	return xtbtest.Export{AccountNumber: "56100003", Currency: "PLN", CashOperations: ops}
}
