# Manual test checklist: Profit Tracker v4

Warning: use a separate test database. Changed copies of an export can put unwanted data into the database.

v4 adds the Value History, the Time-Weighted Return, the daily price history and the Price Warnings. Spec: #27.

## 0. Live test of the price history (result)

The live test gets the full history of all Instruments from the real exports from Yahoo. It is not in `make test`. Start it from the `backend` folder:

```bash
go test -tags live -count=1 -timeout 30m -run TestLiveHistory -v ./internal/httpapi/
```

Result on 2026-10-27:

| Refresh | Time | History requests | Daily closes stored | History failures | HTTP 429 |
|---|---|---|---|---|---|
| First | 2 min 3 s | 75 | 23,814 | 22 | 0 |
| Second | 5 s | 2 | 0 | 2 | 0 |

- The second refresh asked only for the 2 Instruments that failed in the first refresh.
- 2 failures are HTTP 404. These price tickers have no data at Yahoo.
- 20 failures are "the price is in USD, but the Instrument is quoted in GBP". These `.UK` lines are USD lines. The Price Warnings show them (section 6).
- With the real data, `GET /api/accounts/{id}/holdings` took about 35 ms. `GET /api/all-accounts/value-history` took about 70 ms. The limit is 500 ms.

## 1. Start

- [ ] Stop any other Profit Tracker. It uses the same port.
- [ ] Run `make build`.
- [ ] Run `./bin/profit-tracker -db ~/pt-v4-test.db`.
- [ ] Open http://127.0.0.1:8088.
- [ ] Import the three exports from `exports/` as in the v3 checklist, section 2.

## 2. First refresh of the prices

- [ ] Click "Refresh prices". The alert "Refreshing the prices" shows the progress of the history requests.
- [ ] The values stay on the screen during the refresh.
- [ ] The first refresh takes more than one minute. The result shows when the refresh ends.
- [ ] Click "Refresh prices" again. The second refresh takes some seconds.
- [ ] If Yahoo blocks the requests, the alert "The refresh stopped early" shows the message "Yahoo blocks the requests, try again later". Click "Refresh prices" later. The refresh continues from the stored prices.

## 3. Value History of an Account

- [ ] Open the Overview of an Account. The card "Value History" shows under the hero and over the waterfall.
- [ ] The chart shows two lines: the value and the Net Deposits.
- [ ] The chart starts on the day of the first Cash Operation of the Account. It has a point for each calendar day, also on weekends.
- [ ] Point at a day. The tooltip shows the date, the value, the Net Deposits and the Total Result of that day.
- [ ] The last point has the same value as the hero. If a Holding has no price, the difference is the cost of that Holding.
- [ ] All amounts are in the Account currency.

## 4. Period and Time-Weighted Return

- [ ] The period switch shows 1M, 3M, YTD, 1Y and All. All is selected first.
- [ ] Select each period. The chart shows only the days of the period. The header shows the Time-Weighted Return of the period.
- [ ] The Annual Return in the hero does not change.
- [ ] Find a large deposit in the chart. The Time-Weighted Return does not jump on that day.

## 5. Value History of all Accounts

- [ ] Open the Overview of All Accounts. The card "Value History" shows all amounts in PLN.
- [ ] The chart starts on the day of the first Cash Operation of any Account.
- [ ] A Transfer between two Accounts does not change the Net Deposits of All Accounts.

## 6. Price Warnings

- [ ] After the refresh, the result shows "N Instruments have a Price Warning" with the list of the Instruments.
- [ ] Each `.UK` line that Yahoo quotes in USD shows the warning "The price source gives the price of X in USD, but the Quote Currency is GBP".
- [ ] An Instrument with no data at Yahoo shows the warning "The price source has no data for the price ticker X".
- [ ] Open the Holdings page. Each Holding with a Price Warning shows the badge "Price Warning".
- [ ] Click the badge. The price source dialog opens and lists the Price Warnings of the Instrument.
- [ ] The card "Value History" shows the Price Warnings over the chart. The list includes sold Instruments. Each Instrument has the button "Edit the price source".
- [ ] The chart shows also when a Price Warning stays.
- [ ] Open the dialog of a `.UK` line with a different currency. Set the Quote Currency to USD and click Save. After the refresh, the warning about the currency is gone.
- [ ] A warning about no data or about a different currency has no confirmation button.

## 7. Confirmation of a price mismatch

The real data can have no price mismatch. In that case, make one: set the price ticker of a held Instrument to the price ticker of a different Instrument, and refresh the prices.

- [ ] The warning "The prices of the price source do not agree with the XTB prices" shows the median difference and the largest difference.
- [ ] Open the dialog. Click "The price ticker and the Quote Currency are correct". The badge and the price mismatch warning go away. The tool does not refresh the prices.
- [ ] Open the dialog again. Click "Remove the confirmation". The price mismatch warning shows again.
- [ ] Confirm again. Then change the Quote Currency or the price ticker. The confirmation is removed, and the tool checks the Instrument again.
- [ ] Set the correct price ticker again and refresh the prices.

## 8. Themes and narrow screen (375 px)

- [ ] Do the sections 3 to 7 in the dark theme and in the light theme.
- [ ] Set the browser width to 375 px. The card "Value History" has the full width. The period switch shows under the title.
- [ ] The Price Warnings over the chart and the badges on the Holdings fit the screen. No page needs horizontal scrolling.
- [ ] A tap on a Holding row with a badge opens the price source dialog.
