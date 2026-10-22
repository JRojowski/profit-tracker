# 03: Account dashboard with cash, Net Deposits and Holdings

**What to build:** The user opens the dashboard and selects one Account. The dashboard shows the cash balance, the Net Deposits and a table of Holdings with the Instrument, the volume and the average cost. All amounts are in the Account currency. The tool calculates all values from the Ledger on each request and does not store them.

**Blocked by:** 02 (Import an export into a new Account).

**Status:** done

- [x] `GET /api/accounts` lists the Accounts. `PATCH /api/accounts/{id}` changes the name.
- [x] `GET /api/accounts/{id}/summary` returns the cash balance and the Net Deposits.
- [x] `GET /api/accounts/{id}/holdings` returns one row per Instrument with the open volume and the average cost.
- [x] A pure Ledger calculator produces these values. No table stores Holdings or balances.
- [x] The dashboard has an Account selector, the summary tiles and the Holdings table.
- [x] API tests check the values for a synthetic export with deposits and purchases of two Instruments.
