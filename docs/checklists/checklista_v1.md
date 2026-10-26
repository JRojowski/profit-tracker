# Manual test checklist: Profit Tracker v1

Warning: use a separate test database. Changed copies of an export can put unwanted data into the database.

## 1. Start

- [x] Run `make build`.
- [x] Run `./bin/profit-tracker -db ~/pt-test.db`.
- [x] Open http://127.0.0.1:8088. The top right corner shows "Backend: ok".
- [x] Open the Dashboard tab. It tells you that there are no Accounts yet.

## 2. First Import (USD)

- [x] Open the Import tab. Upload `exports/USD_…xlsx`.
- [x] Check the preview: the suggested Currency is USD, the number of new Cash Operations is more than 0, and Known is 0.
- [x] Type a name, select the type "regular" and click Confirm.
- [x] Open the Dashboard. The Account is in the list.

## 3. Import the same file again

- [x] Upload the same file again.
- [x] Check the preview: New is 0, Known is the number from step 2, and there are no Import Conflicts. The new-Account form does not show.
- [x] Click Confirm. The values on the Dashboard do not change.

## 4. Cancel an Import

- [x] Upload an export. In the preview, click Cancel.
- [x] Open the Imports tab. The cancelled Import is not in the list. The Dashboard does not change.

## 5. Import errors

- [x] Upload a file that is not an XTB export, for example another xlsx file or a PDF. A clear "The Import failed" error shows.
- [x] Make a copy of an export. In one Cash Operations row, change the Type to "Bonus". Upload the copy. The error names the type and the row number.
- [x] Make a copy of an export. Change one Amount and keep the Total row. Upload the copy. The error tells that the sum does not match the Total row.

## 6. Import Conflict

- [x] Make a copy of an imported export. In one Deposit row, change only the Comment. Keep the amount.
- [x] Upload the copy. The Import Conflicts card shows the stored data, the new data and the badge "No decision".
- [x] Try to click Confirm with no decision. The tool does not save the Import.
- [x] Select "Keep existing" and click Confirm. The old Comment stays.

## 7. PLN, IKZE and Transfers

- [x] Import the IKZE export with the type IKZE and the currency PLN. The preview shows a warning about a Transfer with no other side. The Import continues.
- [x] Import the PLN export. The warning about this pair of Transfers does not show.
- [x] On the Dashboard, select each of the 3 Accounts. Each Account shows its own currency and its own values.

## 8. Dashboard (compare with XTB)

- [x] The Cash and Net Deposits tiles show the same values as XTB.
- [x] Holdings: the volume and the average cost of each Instrument are the same as in Open Positions in XTB.
- [x] Sales: the Realized Profit of each sale is the same as the Profit/Loss in Closed Positions.
- [x] The Total result is Realized Profit + Unrealized Profit + Net dividends + Net interest + Fees.
- [x] PLN Account: the USD stocks show no value and a note about the other currency.

## 9. Prices

- [x] Above the tiles, the date of the prices shows ("Prices from …").
- [x] Click "Refresh prices". The prices and the Unrealized Profit change to the new values.
- [x] Turn off Wi-Fi and click "Refresh prices". A warning shows. The Dashboard still shows the last stored prices.
- [x] Start the program again on the next day. The tool gets the prices at start with no action.

## 10. Price source (ticket #10)

- [x] USD Account: click the pencil icon of CSPX.UK. Set Quote currency to `USD` and click Save. The Holding gets a value and the badge "manual".
- [x] Do the same for VWRA.UK.
- [x] On a `.US` Instrument, set the Price ticker to `AMZN.MX`, which is a wrong ticker. Click Save. A currency error shows and the Holding has no value.
- [x] Clear the Price ticker field and click Save. The default ticker and the price come back.
- [x] Import the USD export again. The manual values stay.

## 11. Other

- [x] Rename an Account. The Account list stays in name order.
- [x] Open the Imports tab. Each Import shows the date, the file name and the New and Known counts.
- [x] Stop the program. Copy `~/pt-test.db` and start the program with the copy. The data is the same.
