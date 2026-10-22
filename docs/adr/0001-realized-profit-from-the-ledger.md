# Calculate realized profit per Lot from the Ledger

The tool calculates realized profit from the Ledger (the Cash Operations sheet). It does not copy the profit from the broker's Closed Positions sheet. XTB does not use FIFO. Each sale closes one specific Lot, fully or in part. Thus the tool links each sale to its Lot and calculates the profit for that Lot. The Closed Positions sheet is only a check of this calculation.

## Why

- The Ledger is the one source of truth for an Account. Holdings, cash and profit all come from the same data.
- If the tool copies the profit from Closed Positions, two sheets can disagree, and the tool cannot tell which one is correct.
- The calculation is simple and the broker data confirms it (see the examples).

## The rule

1. Each "Stock sell" Cash Operation has a `Position ID`. This ID identifies the Lot that the sale closes.
2. The Comment of the sale gives the closed volume. In `CLOSE BUY 9/10 @ 10.26`, the closed volume is 9. The number after the slash is the volume of the full sell order, which can close more than one Lot.
3. The cost of the closed part is the purchase amount of the Lot, multiplied by the closed volume, divided by the Lot volume.
4. The realized profit of the sale is the sale amount minus the cost of the closed part.
5. The rest of the Lot stays open. Its cost is the remaining part of the purchase amount.

All amounts are in the Account currency, as the Ledger shows them. The rule does not need exchange rates. The purchase amount and the sale amount already contain the rate that the broker used.

The tool calculates with exact decimal values. It rounds only for display.

## Example 1: one Lot, closed in two parts on the same day

Source: PLN Account, Lot `2636532207`.

| Cash Operation | Amount | Comment |
|---|---|---|
| Stock purchase | -109.40 | `OPEN BUY 10 @ 10.94` |
| Stock sell | 92.34 | `CLOSE BUY 9/10 @ 10.26` |
| Stock sell | 10.28 | `CLOSE BUY 1/10 @ 10.28` |

Calculation:

- Lot volume is 10. Purchase amount is 109.40.
- Sale 1: cost = 109.40 × 9 / 10 = 98.46. Profit = 92.34 − 98.46 = **−6.12**.
- Sale 2: cost = 109.40 × 1 / 10 = 10.94. Profit = 10.28 − 10.94 = **−0.66**.
- After both sales, the Lot volume is 0. The Lot is closed.

Check: Closed Positions shows two rows for this Lot, with Profit/Loss −6.12 and −0.66.

## Example 2: one Lot, closed in part, the rest stays open

Source: PLN Account, Lot `1349972184`. The Instrument is quoted in USD, and the Account is in PLN.

| Cash Operation | Amount | Comment |
|---|---|---|
| Stock purchase | -185.31 | `OPEN BUY 0.2638 @ 161.92` |
| Stock sell | 162.73 | `CLOSE BUY 0.1504/0.426 @ 256.60` |

Calculation:

- Lot volume is 0.2638. Purchase amount is 185.31 PLN.
- The sale closes 0.1504. The value 0.426 is the full sell order, which also closed other Lots.
- Cost = 185.31 × 0.1504 / 0.2638 = 105.65 PLN.
- Profit = 162.73 − 105.65 = **57.08 PLN**.
- The Lot stays open with volume 0.2638 − 0.1504 = 0.1134 and cost 185.31 − 105.65 = 79.66 PLN.

Check: Closed Positions shows Profit/Loss 57.08 for this Lot. Open Positions shows the Lot with volume 0.1134.

The profit includes the change of the USD/PLN rate. The broker bought at rate 4.3384 and sold at rate 4.2167. The rule does not need these rates, because the Ledger amounts already contain them.

## Example 3: what the Account sees in total

Take Example 2 at the moment after the sale:

- Realized profit: 57.08 PLN.
- Open cost of the Holding from this Lot: 79.66 PLN.
- Unrealized profit: current value of 0.1134 units minus 79.66 PLN. The current value comes from the price source, not from the Ledger.

## Check against Closed Positions

On each Import, the tool compares its realized profit per sale with the Profit/Loss in Closed Positions. A difference of more than 0.01 is an Import Conflict.

## Not in this decision

- Fees, dividends, withholding tax and interest are separate Cash Operations. This decision does not say how they count in the total result.
- A Lot can have more than one Fill. The broker data does not always give all Fills the same `Position ID`. ADR 0002 covers this case.
