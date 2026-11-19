# Manual test checklist: tables

Spec: #48. Later tickets add more sections.

Warning: use a separate test database, for example `make demo`. Never use the files in `exports/`.

## 1. Sort

Open the Holdings page of an Account with several Holdings. Use the wide layout.

- [ ] With no sort, the rows are in ticker order. No header shows an arrow.
- [ ] Click "Value". The rows go from the smallest to the largest value. The header shows an up arrow and the number 1. `aria-sort` of the header is "ascending".
- [ ] Click "Value" again. The order reverses. The header shows a down arrow and the number 1.
- [ ] Click "Value" a third time. The arrow disappears. The rows are in ticker order.
- [ ] Click "Cost". Click "Value" with Shift. "Cost" shows 1 and "Value" shows 2. The rows are sorted by cost, and equal costs by value.
- [ ] Click "Volume" with Shift. "Volume" replaces "Value" as sort 2. "Cost" stays sort 1.
- [ ] Click "Volume" with Shift two times. The first click reverses it, the second click removes it. "Cost" stays sort 1.
- [ ] Click "Instrument" without Shift. Only "Instrument" is sorted. The other arrows disappear.
- [ ] Click "Unrealized Profit". The order follows the amount, not the percentage.
- [ ] Click "Price". The order follows the price, not the price date.
- [ ] The header "Price source" is not a button and has no sort.
- [ ] A Holding with no price or no value is last in both directions.
- [ ] A reload gives the default order.
- [ ] Use the keyboard: Tab to a header, press Enter. The sort changes.

Narrow layout (390 px wide):

- [ ] A select control with the label "Sort by" shows above the cards. It has "Default order" and the sortable columns.
- [ ] Select "Value". The cards are in value order. Press the direction button. The order reverses.
- [ ] Select "Default order". The cards are in ticker order. The direction button is disabled.

## 2. Holdings total row

- [ ] The last row of the table has the label "Total" in the first column. It stays last for each sort.
- [ ] "Cost", "Value" and "Unrealized Profit" show the sums of the rows with a known value. Check them with a calculator.
- [ ] Under the total Unrealized Profit, the percentage is the profit divided by the cost.
- [ ] The total profit has the sign and the colour of the cell above it.
- [ ] All other cells of the row are empty.
- [ ] With a Holding with no known value, the total row shows "Without 1 Holding with no known value". For two or more, it shows "Without N Holdings with no known value".
- [ ] The Holding with no known value is not in the sums, and its cost is not in the cost sum.
- [ ] On the narrow layout, the cards end with a card "Total" with the cost, the value, the profit and the percentage.
- [ ] Check the light theme and the dark theme, at 1440 px and at 390 px. The page does not scroll sideways at 390 px.

## 3. Column order and the stored view

Spec: #48, ticket #50. Use the wide layout. The key of the Holdings table in `localStorage` is `profit-tracker:table:holdings`.

- [ ] Each column header except "Instrument" has a small grip handle. "Instrument" has none.
- [ ] Drag "Value" in front of "Volume". Move the pointer by more than 8 px. The header, all body cells and the "Total" row show the new order.
- [ ] While you drag, the header moves only to the left and to the right.
- [ ] Try to drag a column in front of "Instrument". The order does not change.
- [ ] Try to drag "Instrument". It does not move.
- [ ] Click "Cost". The rows sort and no drag starts. Click "Value" with Shift. "Value" is sort 2.
- [ ] Press down the pointer on a header, move it less than 8 px and release. The click sorts and the order does not change.
- [ ] Click "Instrument". The rows sort by ticker.
- [ ] Reload. The column order and the sort are the same.
- [ ] Use the keyboard: Tab to the handle of "Cost" (its name is "Move column Cost"). Press Space, press the left arrow key, press Space. The column moves one place to the left. The screen reader announces the move.
- [ ] Press Space on a handle and press Escape. The order does not change.
- [ ] "Reset columns" is disabled while the order is the default and there is no sort.
- [ ] Move a column. "Reset columns" turns on. Click it. The default order and the default row order come back, the key is gone from `localStorage` and the button is disabled.
- [ ] Put the text `{broken` in the key and reload. The default view shows and the console has no error.
- [ ] Put `{"version":1,"columnOrder":["instrument","unknown"],"sorting":[]}` in the key and reload. The default view shows and the console has no error.
- [ ] Put a valid sort and an unknown column order in the key. The sort stays and the column order is the default.
- [ ] Put `{"version":2,"columnOrder":[],"sorting":[]}` in the key. The default view shows.
- [ ] Block `localStorage` in the browser settings. The table shows and sorts. Moving a column works until the reload.
- [ ] Open the Holdings page of another Account. It shows the same column order, because the key is for the table.

Narrow layout (390 px wide):

- [ ] The cards, the "Sort by" control and the total card are unchanged. No handle and no "Reset columns" button show.
- [ ] The sort that you set on the cards is in the wide table after you widen the window.

Check the light theme and the dark theme with a moved column.

## 4. Closed Positions table

Spec: #48, ticket #51. Open the Closed Positions page of the Account "XTB USD" of `make demo`. Use the wide layout. The key in `localStorage` is `profit-tracker:table:closed-positions`.

- [ ] With no sort, the newest Closed Position is first. No header shows an arrow.
- [ ] The columns are in this order: Close date, Instrument, Volume, Open date, Realized Profit. The table has no Open price, Close price, Sale amount or Cost column.
- [ ] Click "Close date". The oldest row is first. Click it again. The newest row is first. Click it a third time. The arrow disappears.
- [ ] Click "Instrument". The rows sort by ticker.
- [ ] Click "Realized Profit". The rows sort by the amount. Click "Volume" with Shift. "Realized Profit" shows 1 and "Volume" shows 2.
- [ ] "Volume" sorts by the amount, not by the text.
- [ ] "Close date" and "Instrument" have no grip handle. Each other column has one.
- [ ] Drag "Realized Profit" in front of "Volume". The header, the body cells and the "Total" row show the new order.
- [ ] Try to drag a column in front of "Close date" or "Instrument". The order does not change.
- [ ] The "Total" row is the last row for each sort. It shows the sum of the Realized Profit under "Realized Profit", also after you move that column.
- [ ] Reload. The column order and the sort are the same.
- [ ] Open the Closed Positions page of another Account. The column order and the sort are the same, because the key is for the table.
- [ ] Select a year. The rows, the total row, and the two summary cards follow the year. The view does not change.
- [ ] "Reset columns" in the card header is disabled for the default view. After a move or a sort it turns on. Click it. The default view comes back and the key is gone.
- [ ] The Holdings table keeps its own view. A change in one table does not change the key of the other table.
- [ ] Put the text `{broken` in the key and reload. The default view shows and the console has no error.
- [ ] Use the keyboard: Tab to a header, press Enter. The sort changes. Tab to the handle "Move column Cost", press Space, press the left arrow key, press Space. The column moves.

Narrow layout (390 px wide):

- [ ] Each row shows the ticker, the name, the open date and the close date, and the volume in the first column. It shows the Realized Profit in the second column. The "Total" row shows.
- [ ] A select control with the label "Sort by" shows above the rows. It has "Default order" and the sortable columns. Select "Realized Profit". The rows are in profit order. Press the direction button. The order reverses.
- [ ] No handle and no "Reset columns" button show. The page does not scroll sideways.
- [ ] The sort that you set here is in the wide table after you widen the window.

Check the light theme and the dark theme, at 1440 px and at 390 px.

## 5. Closed Positions filters

Spec: #48, ticket #52. Open the Closed Positions page of the Account "XTB USD" of `make demo`. The filters are the query parameters `instrument` (repeated), `from` and `to` (both `YYYY-MM-DD`, both inclusive).

- [ ] The toolbar above the table shows, in this order: the year buttons, "From", "To", "Instrument" and "Reset filters". "Reset filters" is disabled. "All years" is active.
- [ ] Click a year button. The URL gets `from=YYYY-01-01` and `to=YYYY-12-31`. The rows, the total row and the two summary cards follow. The caption of each card shows the year. The button is active.
- [ ] Click "All years". `from` and `to` leave the URL.
- [ ] Open "From", pick a day. The button shows the day in the local format and the URL gets `from`. Open "To", pick a day. The URL gets `to`. No year button is active for a range that is not a year.
- [ ] Set only "From". Then clear it and set only "To". Each open range works. "Clear From date" and "Clear To date" remove the date.
- [ ] A day in the range with no Closed Position shows "No Closed Positions match the filters." with a "Reset filters" button. The summary cards show zero. The button brings the rows back.
- [ ] Open "Instrument". Type part of a ticker or of a name. The list shrinks. Select one Instrument. The trigger shows the ticker and the URL has one `instrument`. Select a second one. The trigger shows "2 Instruments". "Clear selection" removes both.
- [ ] The list holds only Instruments that have a Closed Position on the Account, also when a date range hides some rows.
- [ ] Reload the page with filters set. The filters, the rows and the cards are the same. Open the same URL in a new tab. The view is the same.
- [ ] Open each of these URLs. The page shows all rows, no error, and "Reset filters" is disabled: `?from=abc`, `?from=2025-02-30`, `?from=2026-01-01&to=2025-01-01`, `?instrument=NOPE`. The URL stays as typed.
- [ ] Add another parameter, for example `?x=1`, and change a filter. The parameter `x` stays.
- [ ] Change some filters, then click the browser Back button. The browser leaves the page. It does not step through the filter changes.
- [ ] Sort a column and drag a column. The URL filters do not change. Change a filter. The key `profit-tracker:table:closed-positions` does not change. "Reset columns" does not touch the filters.
- [ ] Near midnight on 31 December in Polish time: use a Closed Position that closed at 2025-12-31T23:30:00Z. In Polish time it is on 2026-01-01. With `to=2025-12-31` the row is not in the result. With `from=2026-01-01&to=2026-12-31` it is in the result. The "Close date" column shows 01/01/2026. Check this also with the time zone of the browser set to a zone far from Poland, for example Los Angeles.
- [ ] A Closed Position at 2025-12-31T22:30:00Z is on 2025-12-31 in Polish time. The year 2025 includes it.

Narrow layout (390 px wide):

- [ ] The toolbar wraps to more rows. The popovers fit the screen. The page does not scroll sideways.
- [ ] The filters select the stacked rows, and the cards and the total row follow.

Check the light theme and the dark theme, at 1440 px and at 390 px, also with a popover open.

## 6. Open date and the removed price columns

Spec: #48, ticket #53, changed by #58. Open the Closed Positions page of the Account "XTB USD" of `make demo`. Use the wide layout.

- [ ] The column "Open date" shows after "Volume". The first column is named "Close date".
- [ ] "Open date" shows the date of the first Fill of the Lot, in the same format as "Close date".
- [ ] The table has no "Open price", "Close price", "Sale amount" or "Cost" column. No control shows them again. The Instrument Overview shows them.
- [ ] Click "Open date". It sorts by its date, not by its text.
- [ ] Drag "Open date" to another place. Reload. The place is the same. "Reset columns" brings the default order back.
- [ ] Put an old stored view in the key `profit-tracker:table:closed-positions`, for example `{"version":1,"columnOrder":["time","instrument","volume","openTime","openPrice","closePrice","amount","cost","realizedProfit"],"sorting":[{"id":"cost","desc":true}]}`. Reload. The default view of five columns shows, with no sort. The console has no error.
- [ ] The "Realized Profit" column and the "Total" row do not change.
- [ ] Put `?sort=cost` in the URL. The page shows all rows and no error.

Narrow layout (390 px wide):

- [ ] The first column shows "open → close date" under the name. The second column shows only the Realized Profit. The headers name them.
- [ ] The "Sort by" control lists "Close date", "Instrument", "Volume", "Open date" and "Realized Profit".

Check the light theme and the dark theme, at 1440 px and at 390 px.

## 7. Instrument Overview

Spec: #58, tickets #59, #60 and #61. Use the Account "XTB USD" of `make demo`. The panel state is in the URL: `overview=<ticker>` and `closed=<xtbId>`.

Open from the Holdings table:

- [ ] Open the Holdings page. Click a row. A panel opens at the right. The URL gets `overview=<ticker>`. The table stays visible.
- [ ] Click the "Price source" button of a row. The price source dialog opens. The panel does not open.
- [ ] Press Tab until a row has focus. Press Enter. The panel opens.
- [ ] With the panel open, click another row. The panel shows the new Instrument. The Back button does not step through the rows.
- [ ] The panel shows the header, the price chart, the Holding, the open Lots and, when the Instrument has some, the Closed Positions.
- [ ] The Holdings table has no "Average cost" and no "Exchange Rate" column. The Holding section of the panel shows both.
- [ ] No Closed Position is highlighted, and the URL has no `closed`.

Open from the chart:

- [ ] Click a bar of "Unrealized Profit by Holding". The panel shows the same Instrument as the row. The URL has no `closed`.
- [ ] Press Tab to a ticker at the left of a bar. Press Enter. The panel opens.

Open from the Closed Positions table:

- [ ] Open the Closed Positions page. Click a row. The panel opens. The URL has `overview` and `closed`.
- [ ] The row of that Closed Position in the list has a ring. The list scrolls to it. Its sell marker and each Fill marker of its Lot are large. The other markers are dim.
- [ ] Click another row of the same Instrument. The highlight moves to the other Closed Position.
- [ ] Press Tab until a row has focus. Press Enter. The panel opens with the highlight.
- [ ] In the panel, click a card that is not highlighted. The card gets a ring. The markers of that Closed Position get large.
- [ ] After that click, the URL has `closed=<xtbId>` of the card. The list does not scroll.
- [ ] Click another card. The highlight moves. The Back button does not step back to the earlier card.
- [ ] Click the highlighted card. The highlight goes. `closed` leaves the URL. The other parameters stay.
- [ ] Do the same on the Holdings page and on the Closed Positions page. On the Closed Positions page, the filters stay in the URL.
- [ ] Press Tab until a card has focus. Press Enter. The card is highlighted. Press Space. The highlight goes.
- [ ] Move the pointer over a card. The pointer shows as a hand. The card changes its background.
- [ ] Reload the page after you select a card. The card is still highlighted and the list scrolls to it.
- [ ] Open `?overview=<ticker>&closed=NOPE`. The panel opens. No row is highlighted. The page shows no error.

Chart and lists:

- [ ] Buy markers are triangles that point up. Sell markers are diamonds. The legend names both, with their colours.
- [ ] Hover over a marker day. The tooltip lists each Fill or sell with its time, volume and price. A day with two or more operations shows the average price.
- [ ] Open an Instrument that you sold in full, for example INTC.US. The panel shows the header, the chart and the Closed Positions. It has no Holding and no open Lots.
- [ ] The price line continues after the last sell.
- [ ] Each Closed Position in the list is a card with two columns. The left column shows Open date, Open price, Volume and Cost.
- [ ] The right column of each card shows Close date, Close price, Sale amount and Realized Profit.
- [ ] Open an Instrument with no price data, for example ZZQX.US. The panel shows a message. The chart still shows the markers.

URL state and the Back button:

- [ ] Select two Instrument filters on the Closed Positions page. Click a row. The filters stay in the URL. The table keeps its rows.
- [ ] Reload the page. The panel, the highlight and the filters are the same.
- [ ] Click the Back button. The panel closes. The filters stay.
- [ ] Open the panel and click the close button of the panel. `overview` and `closed` leave the URL. The filters stay.
- [ ] Open the panel, then press Escape. The panel closes.
- [ ] Open the URL with an unknown ticker. The panel shows "No such Instrument". The page shows no error page.

Phone width (390 px wide):

- [ ] The panel uses the full width of the screen. The page does not scroll sideways.
- [ ] Tap a Holdings row. The panel opens. The price source dialog does not open.
- [ ] Tap "Change the price source" in the panel header. The price source dialog opens.
- [ ] Tap a Closed Positions row. The panel opens with the highlight.
- [ ] The Closed Positions table has no price, sale amount or cost field in its rows.

Check the light theme and the dark theme, at 1440 px and at 390 px.
