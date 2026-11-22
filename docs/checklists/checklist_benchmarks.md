# Manual test checklist: Benchmarks

Spec: #81. Ticket: #86.

The automatic tests use a fake price source. This checklist is the only check of the real prices, the real layout and the browser behavior of the Benchmarks page.

Warning: the Benchmark prices come from the price source. Start the tool with network access. Never use the files in `exports/` for this checklist. Use the demo data.

## 0. Setup

- [ ] Run `make demo` in the repository root. Wait until the command prints the URL.
- [ ] Open the URL that `make demo` prints.
- [ ] Check that the sidebar shows the item "Benchmarks".
- [ ] Note the Account currency of the demo Accounts in the list of Accounts.

## 1. Sidebar and views

- [ ] Open an Account. Click "Benchmarks" in the sidebar. Check that the Benchmarks page opens.
- [ ] Check that the page shows the Account that you opened.
- [ ] Click "All Accounts". Click "Benchmarks" in the sidebar. Check that the page shows all Accounts.
- [ ] Check that the sidebar marks "Benchmarks" as the selected item.
- [ ] On the Benchmarks page of one Account, click the view switch to All Accounts. Check that the Benchmarks page stays open.
- [ ] Select the period 3M. Click the view switch. Check that the period 3M stays selected.
- [ ] Check that the URL shows the period that is selected.

## 2. Period selector

- [ ] Click "1M". Check that the chart and the table change.
- [ ] Click "3M". Check that the chart and the table change.
- [ ] Click "YTD". Check that the chart and the table change.
- [ ] Click "1Y". Check that the chart and the table change.
- [ ] Click "All". Check that the chart and the table change.
- [ ] Click "All". Check that the chart starts on the first day of the Value History.
- [ ] Click "1Y". Compare the last value of the line of the user with the return of the user in the table. Check that both numbers are equal.

## 3. Period after reload and back button

- [ ] Select the period 3M. Reload the page. Check that 3M stays selected.
- [ ] Select the period 3M. Open another page. Click the browser back button. Check that 3M stays selected.
- [ ] Reload the page. Check that a loading state shows before the chart and the table.

## 4. Toggles and stored selection

- [ ] Open the Benchmarks page in a new private window. Check that all five Benchmark lines show.
- [ ] Note the numbers in the table. Turn off the toggle of "S&P 500". Check that its line disappears from the chart.
- [ ] Check that no number in the table changes after the toggle change.
- [ ] Check that the table still shows the row of "S&P 500".
- [ ] Reload the page. Check that "S&P 500" stays hidden.
- [ ] Click "All Accounts". Check that "S&P 500" stays hidden.
- [ ] Turn on the toggle of "S&P 500" in All Accounts. Click the view switch to one Account. Check that "S&P 500" shows.
- [ ] Turn off all five Benchmark toggles. Check that the line of the user stays on the chart.

## 5. Chart

- [ ] Select the period 1Y. Check that each visible line starts at 0 % on the first day.
- [ ] Check that each visible line has a value tag at its last point.
- [ ] Check that the value tag of each line shows the final return of that line.
- [ ] Check that the line of the user uses the primary color.
- [ ] Hover over a day in the middle of the period. Check that the tooltip shows the return of the user.
- [ ] Hover over a day in the middle of the period. Check that the tooltip shows the return of each visible Benchmark.
- [ ] Hover over a day in the middle of the period. Check that the tooltip shows no return for a hidden Benchmark.
- [ ] Hover over the last day. Check that the return of the user in the tooltip equals the return of the user in the table.
- [ ] Check that the lines have no gap on a weekend or on a holiday.

## 6. Table

- [ ] Check that the table has one row for the user.
- [ ] Check that the table has one row for each Benchmark: "S&P 500", "WIG20TR", "mWIG40TR", "World equities" and "Gold".
- [ ] Check that each Benchmark row shows the name of its ETF or ETC.
- [ ] Check that each difference shows the unit percentage points (pp).
- [ ] Check that a positive difference and a negative difference show different looks.
- [ ] Check that the look of a positive or negative difference matches the result badge of the other pages.
- [ ] Take one Benchmark row. Subtract its return from the return of the user. Check that the result equals the difference in the row.
- [ ] Hide two Benchmarks with the toggles. Check that the table still shows all five Benchmark rows.

## 7. Unavailable Benchmark

If all five Benchmarks are available, mark this section as "not shown" in the result table.

- [ ] Find the row of the unavailable Benchmark in the table. Check that the row keeps its place.
- [ ] Check that the row shows the reason in place of the numbers.
- [ ] Check that the reason says "No price for the full period" or "The price source failed".
- [ ] Click the toggle of the unavailable Benchmark. Check that the toggle is disabled and no line appears.
- [ ] Check that the other Benchmarks still show their numbers.

## 8. Empty state

If the demo data has no Account without Cash Operations, mark this section as "not shown" in the result table.

- [ ] Open an Account with no Cash Operations. Open its Benchmarks page.
- [ ] Check that the page shows the empty state.
- [ ] Check that the page shows no broken chart.

## 9. Errors

- [ ] Stop the tool. Reload the Benchmarks page. Check that the error state shows.
- [ ] Start the tool again. Reload the Benchmarks page. Check that the chart and the table show again.

## 10. Theme

- [ ] Set the light theme. Open the Benchmarks page. Check that the text, the lines, the table and the badges are readable.
- [ ] Set the dark theme. Open the Benchmarks page. Check that the text, the lines, the table and the badges are readable.
- [ ] In the dark theme, check that no light box or light text stays from the light theme.

## 11. Phone width

- [ ] Set the browser window width to 375 px. Open the Benchmarks page.
- [ ] Check that the page has no horizontal scroll bar.
- [ ] Check that the period selector, the chart, the table and the toggles fit the screen.

## 12. Currency

- [ ] Open an Account. Check that the currency label of the Benchmarks page is the Account currency.
- [ ] Open "All Accounts". Check that the currency label of the Benchmarks page is PLN.
- [ ] If the demo has an Account in a foreign currency, check that its currency label is the Account currency. Otherwise, mark this item as "not shown" in the result table.

## 13. Result of the run

Copy this table below for each run.

| Date | Commit | Items that failed | Notes |
|---|---|---|---|
| | | | |
