# 08: Transfers between Accounts

**What to build:** The tool imports `IKZE deposit` Cash Operations as a Transfer In (positive amount) or a Transfer Out (negative amount), by ADR 0003. Transfers count in the Net Deposits of their Account. The Import preview shows a warning when a Transfer has no other side in the other Accounts. The warning does not stop the Import.

**Blocked by:** 03 (Account dashboard).

**Status:** ready-for-agent

- [ ] The reader accepts `IKZE deposit` and records the direction from the sign.
- [ ] Net Deposits = deposits + Transfers In − withdrawals − Transfers Out.
- [ ] The tool looks for the other side: another Account, the opposite amount, a time within a few seconds.
- [ ] A missing other side gives a warning in the preview. The user can still confirm the Import.
- [ ] API tests reproduce ADR 0003: Net Deposits 0 on PLN and 10000 on IKZE, and the warning when only one Account is imported.
