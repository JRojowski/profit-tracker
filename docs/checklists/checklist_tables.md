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
