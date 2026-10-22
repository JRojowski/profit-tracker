# 07: Income, costs and the total result

**What to build:** The tool imports dividends, withholding tax, free funds interest and its tax, SEC fees, corrections and withdrawals. The dashboard shows the total result with its parts: Realized Profit, net dividends, net interest and fees. Unrealized Profit joins the total in ticket 09.

**Blocked by:** 03 (Account dashboard).

**Status:** ready-for-agent

- [ ] The reader accepts `Dividend`, `Withholding tax`, `Free funds interest`, `Free funds interest tax`, `SEC fee`, `Correction` and `Withdrawal`.
- [ ] Net dividends = dividends + withholding tax (the tax is negative). Net interest = interest + interest tax.
- [ ] Fees = the sum of `SEC fee` and `Correction` amounts.
- [ ] A withdrawal decreases the cash balance and the Net Deposits.
- [ ] The summary returns the total result and each part. The dashboard shows them in the summary tiles.
- [ ] Dividends with an empty Position ID import with no error.
- [ ] API tests check each part and the total for a synthetic export that contains each type.
