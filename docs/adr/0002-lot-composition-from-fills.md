# Build Lots from Fills with the broker's Lot volumes

A buy order for a fractional volume executes as two or more Fills. The broker does not give these Fills a consistent `Position ID`. Sometimes all Fills share one ID. Sometimes each Fill has its own ID, and the broker merges them into one Lot. Sometimes each Fill has its own ID, and the broker keeps separate Lots. The Ledger alone cannot tell the last two cases apart. Thus the tool reads the Lot volumes from the Closed Positions and Open Positions sheets of the same export, and builds the Lots from these volumes.

This decision changes one point of ADR 0001's context. The tool now reads the Open Positions sheet, but only the Lot IDs and the volumes. It does not read prices from it.

## Facts from the broker data

The data comes from three exports: USD, PLN and IKZE, 86 orders with more than one Fill in total.

- The broker usually executes the fractional part first and the whole units 1 to 2 seconds later. But the order and the time between the Fills can change. In the PLN export, the whole units come first in 2 orders. In one of them, the Fills are 3 minutes and 21 seconds apart, and purchases of other Instruments are between them. The probable cause: the broker sends the whole units and the fractional part as two separate orders. Each order executes when the market has a seller at its price. Thus one Fill can wait for minutes, and the two Fills can have different prices.
- Each Fill has its own purchase Cash Operation. The Comment shows the Fill volume and the order volume: `OPEN BUY 0.6169/1.6169 @ 47.755`.
- Older Comments omit the ticker (`OPEN BUY 1/1.4265 @ 32.005`). Newer Comments include it (`OPEN BUY FPS.US 1/1.27 @ 39.24`). The parser accepts both forms.

| Case | Accounts | Orders |
|---|---|---|
| A. All Fills share one `Position ID` | IKZE from 2026-03, USD | 33 |
| B. Different IDs, the broker merges them into one Lot | PLN, IKZE before 2026-03 | 35 |
| C. Different IDs, the broker keeps one Lot per ID | PLN | 35 |

The case does not depend on a date or on an Account in a way that the tool can predict.

## Why the volumes from Open Positions are safe to use

The Open Positions sheet shows the state at the time of the report. Prices and open volumes change after that time. But for one Lot ID, the closed volume plus the open volume is always the full Lot volume. This sum does not change with the time of the report.

## Why the Fills have no time limit

A time limit gives no safety that the other checks do not give. The Fill volumes must add up to the order volume exactly. The observed volumes must then match case A, B or C. A wrong group fails one of these checks. A time limit only stops correct orders with slow Fills, as in the PLN example above.

## The rule

1. Find the Fills of one order. The Fills have the same Instrument and the same order volume. Take them in time order until their volumes add up to the order volume. The time between the Fills has no limit. A sum over the order volume, or an order that does not become complete, stops the Import with an error.
2. For each `Position ID` in the group, calculate the observed volume: the sum of its volumes in Closed Positions plus its volume in Open Positions.
3. Select the case:
   - All Fills have one ID: make one Lot (case A).
   - One ID has the observed volume of the full order and all other IDs have 0: merge all Fills into one Lot with that ID (case B).
   - Each ID has the observed volume of its own Fills: make one Lot per ID (case C).
   - Any other result: stop the Import with an error.
4. The volume of the Lot is the sum of the Fill volumes. The purchase amount of the Lot is the sum of the Fill amounts.

After this rule, ADR 0001 applies without change.

## Example A: one ID for all Fills

Source: USD Account, order FPS.US, volume 1.27.

| Cash Operation | Amount | Comment | Position ID |
|---|---|---|---|
| Stock purchase | -10.63 | `OPEN BUY FPS.US 0.27/1.27 @ 39.37` | 2860602916 |
| Stock purchase | -39.24 | `OPEN BUY FPS.US 1/1.27 @ 39.24` | 2860602916 |

Result: one Lot `2860602916`, volume 1.27, purchase amount 49.87.

Check: Open Positions shows Lot `2860602916` with volume 1.27.

## Example B: different IDs, one merged Lot

Source: PLN Account, order LSMC.DE, volume 1.6169.

| Cash Operation | Amount | Comment | Position ID |
|---|---|---|---|
| Stock purchase | -126.35 | `OPEN BUY 0.6169/1.6169 @ 47.755` | 1851988998 |
| Stock purchase | -204.59 | `OPEN BUY 1/1.6169 @ 47.705` | 1851989133 |
| Stock sell | 198.43 | `CLOSE BUY 0.4396 @ 107.040` | 1851988998 |

Observed volumes:

- `1851988998`: 0.4396 in Closed Positions plus 1.1773 in Open Positions gives 1.6169. This is the full order.
- `1851989133`: 0 (the ID is not in any sheet).

Result: one Lot `1851988998`, volume 1.6169, purchase amount 126.35 + 204.59 = 330.94.

Realized profit of the sale (ADR 0001):

- Cost = 330.94 × 0.4396 / 1.6169 = 89.98.
- Profit = 198.43 − 89.98 = **108.45**.
- The Lot stays open with volume 1.1773 and cost 240.96.

Check: Closed Positions shows Profit/Loss 108.45.

Counter-example: if the tool uses only the first Fill as the Lot, the cost is 126.35 × 0.4396 / 0.6169 = 90.04. The profit is then 108.39, and the check against Closed Positions fails.

## Example C: different IDs, separate Lots

Source: PLN Account, order EXUS.DE, volume 1.4265.

| Cash Operation | Amount | Comment | Position ID |
|---|---|---|---|
| Stock purchase | -56.74 | `OPEN BUY 0.4265/1.4265 @ 32.005` | 1672382331 |
| Stock purchase | -133.03 | `OPEN BUY 1/1.4265 @ 32.005` | 1672382360 |
| Stock sell | 56.92 | `CLOSE BUY 0.4265/16.9887 @ 31.675` | 1672382331 |
| Stock sell | 133.47 | `CLOSE BUY 1/16.9887 @ 31.675` | 1672382360 |

Observed volumes:

- `1672382331`: 0.4265. This is the volume of its own Fill.
- `1672382360`: 1. This is the volume of its own Fill.

Result: two Lots.

- Lot `1672382331`: profit = 56.92 − 56.74 = **0.18**.
- Lot `1672382360`: profit = 133.47 − 133.03 = **0.44**.

Check: Closed Positions shows 0.18 and 0.44.
