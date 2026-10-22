# 11: Real-export tests

**What to build:** A local test suite imports the user's real exports from the `exports/` folder and checks the results against the broker's own sheets. Git ignores the folder, so the tests skip when the files are not there.

**Blocked by:** 05 (Sales and Realized Profit), 06 (Lots from multiple Fills), 07 (Income, costs and the total result), 08 (Transfers between Accounts).

**Status:** ready-for-agent

- [ ] The tests find each xlsx file in `exports/` and skip with a message when there is none.
- [ ] Each export imports with no error.
- [ ] Each sale matches the Closed Positions Profit/Loss within 0.01.
- [ ] Each open Lot volume matches the Open Positions volume.
- [ ] The tests never print personal data beyond the IDs and amounts of a failed check.
- [ ] Warning: never copy the content of the real exports into the repository.
