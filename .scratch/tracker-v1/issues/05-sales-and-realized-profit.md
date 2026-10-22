# 05: Sales and Realized Profit

**What to build:** The tool imports `Stock sell` Cash Operations, links each sale to its Lot by the Position ID, and calculates the Realized Profit by ADR 0001. The dashboard shows a list of sales with the date, the Instrument, the volume, the sale amount, the cost and the Realized Profit. Holdings show only the open part of each Lot. On each Import, the tool compares each Realized Profit with the Closed Positions sheet.

**Blocked by:** 03 (Account dashboard), 04 (Re-import and Import Conflicts).

**Status:** ready-for-agent

- [ ] The parser reads sale Comments with and without the order volume (`CLOSE BUY 9/10 @ 10.26` and `CLOSE BUY 18 @ 26.90`). The closed volume is the number before the slash.
- [ ] The calculator applies ADR 0001: cost of the closed part = Lot purchase amount × closed volume / Lot volume.
- [ ] `GET /api/accounts/{id}/sales` returns the sales with the Realized Profit.
- [ ] The Holdings table and the average cost use only the open part of each Lot.
- [ ] A difference of more than 0.01 between the Realized Profit and the Closed Positions Profit/Loss is an Import Conflict of kind "check mismatch".
- [ ] API tests reproduce ADR 0001 Example 1 (profits −6.12 and −0.66, Lot closed) and Example 2 (profit 57.08, open volume 0.1134, open cost 79.66).
- [ ] An API test checks that a wrong Closed Positions value gives a "check mismatch" conflict.
