# Manual test checklist: Profit Tracker v3

Warning: use a separate test database. Changed copies of an export can put unwanted data into the database.

v3 changes only the user interface and adds the Allocation. Section 12 compares the v3 numbers with the v2 numbers.

## 1. Start

- [ ] Stop any other Profit Tracker. It uses the same port.
- [ ] Run `make build`.
- [ ] Run `./bin/profit-tracker -db ~/pt-v3-test.db`.
- [ ] Open http://127.0.0.1:8088. The Overview of All Accounts shows. The page tells you that there are no Accounts yet and shows the link "Import an export".
- [ ] The top bar shows no backend status badge.

## 2. Import

- [ ] Click "Import" in the sidebar. The URL is `/import`. The page shows the file drop zone.
- [ ] Upload `exports/USD_…xlsx` and click Preview. The figures show. Warnings show only when the export has them.
- [ ] The new-Account form shows. Type a name, select the type "regular" and click "Confirm Import".
- [ ] The Imports page opens. The sidebar shows the new Account with the badge "USD".
- [ ] Upload the same file again and click Preview. New is 0, and there are no Import Conflicts. The page shows the known Account and no new-Account form. Click "Confirm Import".
- [ ] Upload an export, click Preview and click Cancel. The Imports page does not list this Import.
- [ ] Upload a file that is not an XTB export and click Preview. A clear "The Import failed" error shows.
- [ ] Find the IKZE export by its account number. Import it with the type IKZE. The preview shows a warning about a Transfer with no other side.
- [ ] Import the other PLN export with the type "regular". The sidebar shows 3 Accounts. The IKZE Account shows the tag "IKZE".

## 3. Import Conflict

- [ ] Make a copy of an imported export. In one Deposit row, change only the Comment. Keep the amount.
- [ ] Upload the copy and click Preview. Each Import Conflict shows the stored data and the new data side by side. The changed field is highlighted. The badge "No decision" shows.
- [ ] "Confirm Import" is blocked until each Import Conflict has a decision.
- [ ] Select "Keep existing" and click "Confirm Import".
- [ ] Upload the copy again and click Preview. "Stored data" shows the old Comment. Click Cancel.

## 4. Imports

- [ ] Click "Imports" in the sidebar. The URL is `/imports`.
- [ ] The table lists each Import with the date, the file name, the Account and the New and Known counts.
- [ ] The table fits in its card. A long file name does not break the layout.

## 5. Navigation and URLs

- [ ] Click each item in the sidebar. The selected item is highlighted. The top bar shows the title of the view.
- [ ] Open an Account. The tabs Overview, Holdings and Sales show under the title. Only the IKZE Account also has the tab "IKZE".
- [ ] Open each tab. The URL changes to `/accounts/{id}`, `/accounts/{id}/holdings`, `/accounts/{id}/sales` or `/accounts/{id}/ikze`.
- [ ] Reload the page on each tab. The same view opens.
- [ ] Click the browser back button. The previous view opens.
- [ ] Open `/accounts/999`. The message "The tool has no such Account" shows with the link "Open All Accounts".
- [ ] Open `/accounts/{id}/ikze` for an Account that is not IKZE. The URL changes to `/accounts/{id}`. The Overview shows.
- [ ] Click the pencil icon next to the Account title. Type a new name and click Save. The sidebar shows the new name.

## 6. Top bar and themes

- [ ] The top bar shows the date of the prices and the date of the Exchange Rates.
- [ ] Click "Refresh prices". The button shows "Refreshing…". Then the result of the refresh shows.
- [ ] Turn off Wi-Fi and click "Refresh prices". The alert "Some Instruments got no new price" shows. The values stay on the screen. Turn on Wi-Fi.
- [ ] Run `make frontend-dev`. Stop the program. Open the Vite URL. The badge "Backend unavailable" shows.
- [ ] Start the program again and reload the page. The badge goes away. Close the Vite URL.
- [ ] Open the tool in a private window. The dark theme shows.
- [ ] Click the theme switch. The light theme shows. Profits stay green and losses stay red. Reload the page. The light theme stays.
- [ ] Do the sections 7 to 11 in the dark theme and in the light theme.

## 7. Overview of an Account

- [ ] The value shows in a very large font. The Total Result and the Annual Return show under it.
- [ ] A second row shows the Simple Return, the cash and the Net Deposits.
- [ ] All amounts are in the Account currency. Each profit has "+" and each loss has "−".
- [ ] The waterfall "Total Result breakdown" starts at zero. It shows the Realized Profit, the Unrealized Profit, the net dividends, the net interest and the fees. The last bar is the Total Result.
- [ ] Each bar shows its amount. A profit has "+" and a loss has "−". A row of boxes under the chart shows the exact amount of each part.
- [ ] A return that the tool cannot calculate shows "—" with the reason under it.

## 8. Overview of All Accounts

- [ ] The layout is the same as the Overview of an Account. All amounts are in PLN.
- [ ] The waterfall also shows the Currency Result.
- [ ] The table "Accounts" shows the value, the Total Result and the Annual Return of each Account in PLN, and a total row.
- [ ] Click a row of the table. The Overview of that Account opens.

## 9. Allocation

- [ ] Each Overview shows the donut "Allocation" with a legend. The legend shows the name, the share and the value of each part.
- [ ] The chart shows at most 8 Holdings, the cash and one part "Other". "Other" shows the number of its Holdings.
- [ ] Point at "Other". The tooltip lists its Holdings with their shares.
- [ ] The shares add up to 100%. Sold Instruments do not show.
- [ ] All Accounts: an Instrument on two Accounts is one part.
- [ ] A Holding with no value is not in the chart. A note tells how many Holdings are missing.

## 10. Holdings

- [ ] The donut "Allocation by Quote Currency" shows each currency. The cash counts in the Account currency.
- [ ] The chart "Unrealized Profit by Holding" goes from the largest profit to the largest loss. A profit and a loss go to opposite sides of the zero line.
- [ ] The table shows the price in the Quote Currency, the Exchange Rate, the value and the Unrealized Profit in the Account currency.
- [ ] Click the pencil icon of a Holding. Enter a correct Price ticker and Quote Currency. Click Save.
- [ ] Wait for the refresh of the prices. The price source shows "manual". The value changes.
- [ ] A note under the table lists the Holdings with no value.

## 11. Sales and IKZE

- [ ] Sales: the total Realized Profit and the number of sales show at the top.
- [ ] The table shows all sales, newest first, with a total row.
- [ ] The year filter shows "All years" and each year with a sale. Select a year. The total, the number and the table show only that year.
- [ ] IKZE: the IKZE Contributions of the current year show as a large figure with the IKZE Limit and a progress bar. The part that is not used shows next to it.
- [ ] The table of past years shows the contributions, the limit, the used part with a small bar, and the part that is not used.
- [ ] A year before 2025 shows "Limit unknown". Skip this step when the IKZE Account has no such year.

## 12. Compare with v2

Each program refreshes the prices at start. Two refreshes at different times give different prices. Keep Wi-Fi off in this section, so that both programs use the same stored prices.

- [ ] Stop the program. Copy `~/pt-v3-test.db` to `~/pt-v2-test.db`.
- [ ] Run `git worktree add ../profit-tracker-v2 9f4d9bb`. This is the last commit of v2.
- [ ] In `../profit-tracker-v2`, run `make build`.
- [ ] Turn off Wi-Fi. Do not click "Refresh prices" in this section.
- [ ] Run `../profit-tracker-v2/bin/profit-tracker -db ~/pt-v2-test.db -addr 127.0.0.1:8089`. Start v3 again with `~/pt-v3-test.db`.
- [ ] Open v2 at http://127.0.0.1:8089 and v3 at http://127.0.0.1:8088.
- [ ] For each Account, the value, the cash, the Net Deposits, the Total Result and its parts are the same in v2 and v3.
- [ ] For each Account, the Simple Return and the Annual Return are the same in v2 and v3.
- [ ] For All Accounts, the totals in PLN, the Currency Result and the returns are the same in v2 and v3.
- [ ] The Holdings, the sales and the IKZE years show the same numbers in v2 and v3. v2 shows the oldest sale first, v3 the newest.
- [ ] Stop v2. Run `git worktree remove ../profit-tracker-v2`. Turn on Wi-Fi.

## 13. Narrow screen (375 px)

- [ ] Set the browser width to 375 px.
- [ ] The sidebar is hidden. The menu button opens it. Select an item. The sidebar closes.
- [ ] The cards show in one column. No page needs horizontal scrolling.
- [ ] Open the IKZE Account. All four tabs are reachable.
- [ ] The Holdings show as a list of compact rows. A row opens the price source dialog.
- [ ] The charts show in a compact form with the same information.
- [ ] No number and its currency break into two lines.
- [ ] Open Import, Imports, Sales and IKZE. No page needs horizontal scrolling.
