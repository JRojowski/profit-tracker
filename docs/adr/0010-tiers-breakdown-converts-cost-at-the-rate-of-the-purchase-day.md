# The Tiers Breakdown converts the cost at the Exchange Rate of the purchase day

The Tiers Breakdown of all Accounts shows the result of each Instrument and each Tier in PLN. The tool converts each USD amount at the Exchange Rate of the day of that amount. Thus the Unrealized Profit, the Realized Profit and the Return on Cost in PLN include the effect of the Exchange Rate.

This rule is different from rule 5 of ADR 0004. ADR 0004 converts the parts of the Total Result at the latest Exchange Rate and shows the effect of the rate as one Currency Result. The Tiers Breakdown has no Currency Result row. Each Instrument must show the full result in PLN by itself.

## Why

- The user measures the result in PLN. The user gets USD from a contract in USD (ADR 0004).
- The Tiers Breakdown answers the question "how much did this Instrument or this Tier give me". The answer must include the change of the USD rate.
- The same Instrument can be on the PLN Account and on the USD Account. The tool shows it as one row in PLN. The PLN Account pays in PLN at the purchase day, so the cost from the USD Account must use the rate of the purchase day too. Otherwise the two parts of one row use different rules.
- The NBP publishes old Exchange Rates, so the rate of each day is available.

## The rule

1. For an Account in PLN, the tool does not convert amounts.
2. The tool converts the value of a Holding at the latest Exchange Rate.
3. The tool converts the purchase amount of each Fill at the Exchange Rate of the day of the Fill, in Polish time.
4. The cost in PLN of a Lot, or of a part of a Lot, is its cost in USD multiplied by the average Exchange Rate of the Fills of the Lot. The tool weights the average by the Fill amount.
5. The tool converts the sale amount of each Closed Position at the Exchange Rate of the day of the sale.
6. The Unrealized Profit in PLN is the value in PLN minus the cost in PLN of the open Lots.
7. The Realized Profit in PLN is the sale amount in PLN minus the cost in PLN of the closed part.
8. The Return on Cost in PLN is the profit in PLN divided by the cost in PLN.

## Example

The user buys AMZN.US for 100 USD on the USD Account. The Exchange Rate on that day is 4.00.

| Case | Calculation | PLN |
|---|---|---|
| Cost | 100 × 4.00 | 400 |
| Open today: value 110 USD, rate 3.60 | 110 × 3.60 − 400 | Unrealized Profit **−4**, Return on Cost −1% |
| Sold for 110 USD, rate on the sale day 3.80 | 110 × 3.80 − 400 | Realized Profit **18**, Return on Cost 4.5% |

At the latest rate only, the open Holding shows +36 PLN and +10%. The loss on the rate is then not visible.

## Not in this decision

- The Tiers Breakdown of one Account. It shows the amounts in the Account currency, with no conversion.
- Rebalancing. It uses only the values of today, so it converts them at the latest Exchange Rate.
- The tax result for PIT-38. It needs its own rule for the day of the rate.
- The spread of the broker. The tool does not know it, so it uses the NBP average rate.
