# Count Transfers per Account and do not link the two sides

The user funds the IKZE Account through the PLN Account. The user deposits money into the PLN Account and then moves it to the IKZE Account. The broker records this move as two Cash Operations, one on each Account. The tool records each side as a Transfer on its own Account and does not link the two sides. The sums stay correct without a link, so a link adds work with no benefit in v1.

## Facts from the broker data

Both sides have the type `IKZE deposit`. The sign of the amount gives the direction.

| Account | Amount | Time (UTC) | Comment |
|---|---|---|---|
| PLN | -10000.00 | 2026-02-06 16:00:43.157 | `Transfer out operation on account with id 50670442` |
| IKZE | 10000.00 | 2026-02-06 16:00:43.172 | `Transfer in operation on account with id 54155203` |

- The Comment gives the number of the Account that holds the operation, not the number of the other Account.
- The two sides have different IDs. The amounts are opposite, and the times differ by milliseconds.

## The rule

1. An `IKZE deposit` with a positive amount is a Transfer In. With a negative amount, it is a Transfer Out.
2. On its Account, a Transfer In counts as a deposit and a Transfer Out counts as a withdrawal.
3. Net Deposits of an Account are deposits plus Transfers In, minus withdrawals and Transfers Out.
4. Net Deposits of all Accounts together are the sum of the Net Deposits of each Account.
5. On each Import, the tool looks for the other side of each Transfer in the other Accounts. The other side has the opposite amount and a time within a few seconds. If the tool does not find it, it shows a warning. The warning does not stop the Import.

## Example

The user deposits 10000 PLN into the PLN Account and moves it to the IKZE Account.

| Account | Cash Operation | Amount |
|---|---|---|
| PLN | Deposit | 10000 |
| PLN | Transfer Out | -10000 |
| IKZE | Transfer In | 10000 |

Net Deposits:

- PLN Account: 10000 − 10000 = **0**. This is correct, because the money does not work on the PLN Account now.
- IKZE Account: **10000**. This is correct, because the money works on the IKZE Account now.
- All Accounts: 0 + 10000 = **10000**. This is correct, because the user put 10000 into the broker from outside.

The two Transfers cancel each other in the total. A link is not necessary for this result.

## When the rule gives a wrong total

If the user imports only one of the two Accounts, the total for all Accounts is wrong. The warning in step 5 shows this case.

## Not in this decision

- The yearly IKZE contribution limit. A later version can calculate it from the Transfers In on the IKZE Account.
- Transfers between Accounts with different currencies. The data has no example of this case.
