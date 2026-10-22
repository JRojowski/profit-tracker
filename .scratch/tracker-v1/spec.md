# Spec: Profit Tracker v1 (XTB import and Account dashboard)

Status: ready-for-agent

Read `GLOSSARY.md` and ADRs 0001, 0002 and 0003 in `docs/adr/` before you start. This spec uses the glossary terms. The ADRs define the calculation rules. This spec does not repeat them.

## Problem Statement

The user invests through several XTB Accounts: USD, PLN and IKZE. XTB shows the state of each Account, but it does not show a clear total result. The result includes realized and unrealized profit, dividends, interest and fees. Paid tools such as myfund and fundstat do this, but the user wants a free tool that they control. The user can export the full history of an Account from XTB as an xlsx file. There is no tool that reads this file, keeps the history in one place and shows the result.

## Solution

A local, single-user web application. The user uploads an XTB xlsx export of one Account. The tool shows a preview of the Import, and the user confirms it. The tool keeps all Cash Operations in a local database. It calculates Holdings, cash, Net Deposits, realized and unrealized profit from the Ledger, and shows them on a dashboard for one Account at a time. Current prices come from a price source on the internet, and the tool keeps a local copy of them.

v1 targets the XTB USD Account. The model supports many Accounts in different currencies, and the importer accepts all Cash Operation types that the user's three exports contain.

## User Stories

### Accounts

1. As the user, I want the tool to create an Account on the first Import of a new account number, so that I do not have to set up Accounts by hand before I import.
2. As the user, I want to give the new Account a name, a type (regular, IKE or IKZE) and a currency, so that I can tell my Accounts apart.
3. As the user, I want the tool to suggest the currency from the file name prefix (for example `USD_`), so that I do not have to type it.
4. As the user, I want the tool to send each later Import to the correct Account by the account number in the file, so that I cannot import a file into the wrong Account.
5. As the user, I want to see a list of my Accounts, so that I can select one.
6. As the user, I want to change the name of an Account, so that I can fix a mistake.

### Import

7. As the user, I want to upload an XTB xlsx export through the web interface, so that I do not need a command line.
8. As the user, I want to see a preview before the tool saves anything, so that I can check what the Import will change.
9. As the user, I want the preview to show the number of new Cash Operations and the number of known Cash Operations that the tool skips, so that I know the effect of the Import.
10. As the user, I want to import the same file two or more times with no change to my data, so that I do not have to remember which files I imported.
11. As the user, I want to import a newer full-history export that overlaps older Imports, so that I can update my data with one file.
12. As the user, I want the tool to show an Import Conflict when a Cash Operation has a known ID but different data, so that the tool never overwrites my data without my decision.
13. As the user, I want to select "keep existing" or "use new" for each Import Conflict, so that I decide which data is correct.
14. As the user, I want the tool to save the Import only after I resolve all Import Conflicts, so that my Ledger is never partly updated.
15. As the user, I want to cancel an Import in preview, so that nothing changes when I am not sure.
16. As the user, I want the Import to stop with a clear error when the file has an unknown Cash Operation type, so that the tool never calculates with data that it does not understand.
17. As the user, I want the error to name the unknown type and the row, so that the developer can add support for it.
18. As the user, I want the Import to stop with a clear error when the file is not a valid XTB export, so that I know that I selected the wrong file.
19. As the user, I want the tool to check the sum of the imported Cash Operations against the Total row of the export, so that I know the parser read every row.
20. As the user, I want the tool to check its Realized Profit for each sale against the Profit/Loss in the Closed Positions sheet, so that I can trust the calculation.
21. As the user, I want a difference of more than 0.01 in that check to be an Import Conflict, so that I see each disagreement with the broker.
22. As the user, I want the tool to build Lots from Fills by the rule of ADR 0002, so that orders with fractional volumes give the same Lots as the broker shows.
23. As the user, I want the Import to stop with an error when the Fills of an order do not match any case of ADR 0002, so that the tool never guesses the composition of a Lot.
24. As the user, I want a warning when a Transfer has no other side in my other Accounts, so that I know that the total for all Accounts can be wrong.
25. As the user, I want the Transfer warning not to stop the Import, so that I can import my Accounts in any order.
26. As the user, I want to see a list of my past Imports with the date, the file name and the counts, so that I know when I last updated each Account.

### Cash Operation types

27. As the user, I want the tool to accept deposits and withdrawals, so that Net Deposits are correct.
28. As the user, I want the tool to accept stock purchases and stock sales, so that Holdings and Realized Profit are correct.
29. As the user, I want the tool to accept dividends and withholding tax, so that my income from dividends is correct.
30. As the user, I want the tool to accept free funds interest and its tax, so that my income from interest is correct.
31. As the user, I want the tool to accept SEC fees and corrections, so that my costs are correct.
32. As the user, I want the tool to accept `IKZE deposit` operations as Transfers In or Transfers Out by their sign, so that money moved between my Accounts counts correctly.

### Dashboard

33. As the user, I want to select one Account on the dashboard, so that I see its state.
34. As the user, I want to see the current value of the Account, so that I know what my investments are worth now.
35. As the user, I want to see the cash balance of the Account, so that I know how much money I can invest.
36. As the user, I want to see the Net Deposits of the Account, so that I know how much money I put in.
37. As the user, I want to see the total result of the Account, so that I know if I make or lose money.
38. As the user, I want to see the total result split into Realized Profit, Unrealized Profit, net dividends, net interest and fees, so that I know where the result comes from.
39. As the user, I want a table of Holdings with the Instrument, the volume, the average cost, the current value and the Unrealized Profit as an amount and as a percentage, so that I can compare my Holdings.
40. As the user, I want a list of sales with the date, the Instrument, the volume, the sale amount, the cost and the Realized Profit, so that I can see the result of each sale.
41. As the user, I want all amounts in the Account currency, so that I do not have to convert them.
42. As the user, I want the dashboard to show the date of the prices it uses, so that I know how current the values are.

### Prices

43. As the user, I want the tool to get current prices for my Holdings from a price source, so that the tool can calculate Unrealized Profit.
44. As the user, I want the tool to get prices automatically at start when the stored prices are older than today, so that I usually see current values with no action.
45. As the user, I want a "refresh prices" button, so that I can get new prices when I want.
46. As the user, I want the dashboard to show the last stored prices with a warning when the price source does not respond, so that the dashboard still works.
47. As the user, I want the tool to map XTB tickers to the tickers of the price source, so that I do not have to enter them.
48. As the user, I want to set a manual ticker mapping for an Instrument, so that I can fix a ticker that the default mapping gets wrong.
49. As the user, I want the tool to reject a price in a currency other than the Instrument's currency, so that a wrong price line never changes my values.

### Running the tool

50. As the user, I want to start the tool as one program and open it in a browser on localhost, so that I do not have to run a separate server and frontend.
51. As the user, I want my data in one local database file, so that I can back it up with a file copy.

## Implementation Decisions

### Architecture

- Monorepo with a Go backend and a React frontend. The frontend uses TypeScript, Vite, Tailwind and shadcn/ui.
- In production, the Go program embeds the built frontend and serves it, together with a JSON HTTP API under `/api`. The user runs one binary.
- In development, the Vite dev server sends `/api` requests to the Go program.
- The database is SQLite in one local file. The path comes from configuration, with a default in the user's data directory.
- All money and volume values use exact decimal arithmetic. The database stores them as decimal text, not as floating-point numbers. Values are rounded only for display.
- All times are stored in UTC, as the export gives them.

### Modules (backend)

- **XTB export reader.** Input: the bytes of one xlsx file. Output: the account number, the currency prefix of the file name, the Cash Operations, the Closed Positions rows, the Open Positions Lot rows (ID and volume only) and the Total row. The reader knows the sheet layout and the header rows. It knows nothing about the database.
  - The reader parses the Comment of purchases and sales. It accepts both forms, with and without the ticker: `OPEN BUY 0.6169/1.6169 @ 47.755` and `OPEN BUY FPS.US 1/1.27 @ 39.24`. It also accepts sales with and without the order volume: `CLOSE BUY 9/10 @ 10.26` and `CLOSE BUY 18 @ 26.90`.
  - The reader rejects an unknown Cash Operation type with an error that names the type and the row.
  - Known types: `Deposit`, `Withdrawal`, `Stock purchase`, `Stock sell`, `Dividend`, `Withholding tax`, `Free funds interest`, `Free funds interest tax`, `SEC fee`, `Correction`, `IKZE deposit`.
- **Lot builder.** Input: the purchase Cash Operations, plus the Lot volumes from Closed Positions and Open Positions. Output: the Lots with their Fills. It applies ADR 0002 and returns an error for a group that matches no case.
- **Ledger calculator.** A pure function. Input: the Ledger of one Account, the Lots, and the prices. Output: the Holdings, the sales with Realized Profit (ADR 0001), the cash balance, the Net Deposits (ADR 0003), the net dividends, the net interest, the fees, the Unrealized Profit and the total result. Nothing is stored. The tool calculates the state on each request.
- **Import service.** It runs the reader and the Lot builder, compares the new Cash Operations with the stored ones by XTB ID, runs the checks and makes an Import preview. It saves the Import in one database transaction when the user confirms it and all Import Conflicts have a decision.
- **Price service.** It uses a price provider interface. The first implementation uses the Yahoo Finance chart endpoint. The interface is the second test seam, with a fake implementation in tests.
  - Default ticker mapping: `.US` → no suffix, `.UK` → `.L`, `.PL` → `.WA`, `.DE` → `.DE`. A manual mapping per Instrument overrides the default.
  - The service checks that the currency in the response is the Instrument's currency. If not, it rejects the price.
  - The service stores one close price per Instrument per day, with the price date.
- **HTTP API.** A thin layer that maps requests to the services.

### Data model

Tables (names are a direction, not a contract):

- `accounts`: id, account number (unique), name, type (regular / IKE / IKZE), currency.
- `instruments`: broker ticker (unique), name, category (STOCK / ETF), quote currency, optional manual price ticker.
- `cash_operations`: XTB ID (unique per Account), Account, type, Instrument (optional), time, amount, comment, XTB position ID (optional), parsed volume and price (optional), Import that added it.
- `lots`: Account, Lot ID (the XTB position ID), Instrument.
- `lot_fills`: Lot, purchase Cash Operation.
- `imports`: id, Account, file name, time, state (preview / needs-review / saved / cancelled), counts.
- `import_conflicts`: Import, kind (changed data / check mismatch), XTB ID, the stored data, the new data, decision.
- `prices`: Instrument, date, close price, currency, source.

The calculator never stores Holdings or profit.

### API contract (direction)

- `POST /api/imports`: upload a file. It returns the preview: the Account (known, or new with suggested values), the counts, the Import Conflicts and the warnings. It returns an error for an invalid file, an unknown type or an ADR 0002 mismatch.
- `POST /api/imports/{id}/conflicts/{conflictId}`: record a decision.
- `POST /api/imports/{id}/confirm`: save the Import. For a new Account, the body contains the name, the type and the currency. The request fails if a conflict has no decision.
- `POST /api/imports/{id}/cancel`.
- `GET /api/imports`: the list of past Imports.
- `GET /api/accounts`, `PATCH /api/accounts/{id}`.
- `GET /api/accounts/{id}/summary`: value, cash, Net Deposits, the total result and its parts, the price date, the stale-price warning.
- `GET /api/accounts/{id}/holdings`.
- `GET /api/accounts/{id}/sales`.
- `POST /api/prices/refresh`.
- `PUT /api/instruments/{ticker}/price-ticker`: set the manual mapping.

### Import rules

- A Cash Operation is known when its XTB ID exists on the Account.
- A known Cash Operation with the same data is skipped.
- A known Cash Operation with different data is an Import Conflict.
- The Total row check: the sum of all Cash Operation amounts in the file must equal the Total row. A mismatch is an error, because it means that the reader lost rows.
- The Realized Profit check: for each sale, a difference of more than 0.01 from Closed Positions is an Import Conflict.
- The Import is all or nothing. Nothing is saved until the user confirms it.
- An Import that waits for decisions has the state needs-review.

### Frontend

- Pages: the Import page (upload, preview, conflicts, confirm), the dashboard (Account selector, summary tiles, Holdings table, sales list), and the Imports list.
- The dashboard shows one Account at a time.
- The frontend follows the Protopia frontend standard (shadcn/ui, Tailwind semantic tokens).

## Testing Decisions

- A good test checks external behaviour through the seam. It does not call internal functions and does not read database tables directly.
- **Seam 1: the HTTP API.** Each test starts the backend with a new temporary SQLite file. The test uploads an xlsx file, confirms the Import, and then reads the summary, Holdings, sales, conflicts and warnings through the API.
- **Seam 2: the price provider interface.** Tests use a fake provider with fixed prices. No automatic test calls Yahoo.
- **Synthetic fixtures** are committed in the repository. Tests build them as small xlsx files in the XTB layout. Each ADR example is one fixture with the expected values from the ADR:
  - ADR 0001 Example 1: one Lot closed in two parts (profits −6.12 and −0.66).
  - ADR 0001 Example 2: partial close with the rest open (profit 57.08, open volume 0.1134, open cost 79.66).
  - ADR 0002 Examples A, B and C (Lot volumes 1.27 / 1.6169 / two Lots, profits 108.45 / 0.18 and 0.44).
  - ADR 0003 Example: Net Deposits 0 / 10000 / 10000, and the warning when one side is missing.
  - Idempotency: the same file twice gives zero new Cash Operations.
  - Import Conflict: a known ID with a different amount.
  - Unknown type: the Import stops with an error that names the type.
  - Total row mismatch: the Import stops with an error.
- **Real-export tests** read the files in the `exports/` folder, which git ignores. The tests skip when the files are not there. They import each export and check that every sale matches Closed Positions within 0.01, and that every open Lot volume matches Open Positions.
- Frontend: typecheck and build only in v1.
- There is no prior art. The repository is empty.

## Out of Scope

- The investment agent: analysis, recommendations and help with the review of Imports.
- A view of all Accounts together, and all currency conversion.
- Unrealized Profit for Instruments quoted in a currency other than the Account currency, for example USD stocks on the PLN Account. The tool shows these Holdings without a current value and with a note.
- A chart of value over time and historical price series.
- The yearly IKZE contribution limit.
- Brokers other than XTB.
- Users, login and hosting.
- Short positions (`SELL` Lots) and instruments with margin (CFD).

## Further Notes

- The user's three exports (USD, PLN, IKZE) are in `exports/`. They contain personal data. Never commit them and never copy their content into fixtures. Fixtures use invented values, except for the ADR examples, which only show amounts and Position IDs.
- The Yahoo endpoint is unofficial. It can change without notice. The adapter keeps the change local, and EODHD is the planned paid fallback.
- The backend is in Go by the user's choice. The Protopia Python standard does not apply to it.
