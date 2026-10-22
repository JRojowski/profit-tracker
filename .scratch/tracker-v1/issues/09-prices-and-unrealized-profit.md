# 09: Prices and Unrealized Profit

**What to build:** The tool gets current prices for the Instruments in Holdings from Yahoo Finance, through a price provider interface. It stores one close price per Instrument per day. It gets prices at start when the stored prices are older than today, and when the user clicks "refresh prices". The dashboard shows the current value, the Unrealized Profit (amount and percentage) and the price date. The total result now includes the Unrealized Profit.

**Blocked by:** 03 (Account dashboard).

**Status:** ready-for-agent

- [ ] A price provider interface exists. The Yahoo implementation uses the chart endpoint. Tests use a fake implementation, and no automatic test calls Yahoo.
- [ ] Default ticker mapping: `.US` → no suffix, `.UK` → `.L`, `.PL` → `.WA`, `.DE` → `.DE`.
- [ ] The service rejects a price whose currency is not the Instrument's quote currency.
- [ ] `POST /api/prices/refresh` gets new prices. The tool also gets prices at start when the stored prices are older than today.
- [ ] The summary and Holdings return the value, the Unrealized Profit and the price date.
- [ ] When the provider fails, the dashboard uses the last stored prices and shows a stale-price warning.
- [ ] A Holding in a quote currency other than the Account currency shows no current value and a note.
- [ ] API tests with the fake provider check the value, the Unrealized Profit, the stale warning and the currency rejection.
