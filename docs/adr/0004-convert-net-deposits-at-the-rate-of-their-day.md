# Convert Net Deposits to PLN at the Exchange Rate of their day

The view of all Accounts shows values in PLN. The tool converts the current value of an Account in another currency at the latest Exchange Rate. It converts each Net Deposits flow at the Exchange Rate of the day of that flow. Thus the Total Result in PLN shows how much the user earned in PLN, and the effect of the Exchange Rate on the deposited money is visible as the Currency Result.

## Why

- The user gets USD from outside, from a contract in USD. The money does not come from the PLN Account. But the user measures the result in PLN.
- If the tool converts all values at the latest Exchange Rate, a fall of the USD rate is not visible. The Total Result in PLN is then too high.
- The NBP publishes old Exchange Rates, so the rate of each day is available.

## The rule

1. For an Account in PLN, the tool does not convert values. Its Currency Result is 0.
2. For an Account in another currency, the tool multiplies the current value (cash and Holdings) by the latest Exchange Rate.
3. The tool multiplies each Net Deposits flow (deposit, withdrawal, Transfer In, Transfer Out) by the Exchange Rate of the day of that flow, in Polish time.
4. The Total Result in PLN is the value in PLN minus the Net Deposits in PLN.
5. The tool multiplies each part of the Total Result (Realized Profit, Unrealized Profit, net dividends, net interest, fees) by the latest Exchange Rate.
6. The Currency Result is the Total Result in PLN minus the sum of the parts in PLN. It is equal to the sum, for each flow, of the flow amount multiplied by the difference between the latest Exchange Rate and the rate of the flow day.
7. The Simple Return and the Annual Return of all Accounts use the Net Deposits flows in PLN from step 3.

## Example

The user deposits 10000 USD into the USD Account. The Exchange Rate on that day is 4.00. Today the Exchange Rate is 3.60, and the Account is worth 11000 USD.

| Value | Calculation | PLN |
|---|---|---|
| Value | 11000 × 3.60 | 39600 |
| Net Deposits | 10000 × 4.00 | 40000 |
| Total Result | 39600 − 40000 | **−400** |
| Parts of the Total Result | (11000 − 10000) × 3.60 | 3600 |
| Currency Result | 10000 × (3.60 − 4.00) | **−4000** |

The parts and the Currency Result give the Total Result: 3600 − 4000 = −400. If the tool converts everything at the latest rate, the Total Result is +3600, and the loss of 4000 PLN on the rate is not visible.

## Not in this decision

- The dashboard of one Account. It shows values in the Account currency, as before.
- The spread of the broker. The tool does not know it, so it uses the NBP average rate.
- Transfers between Accounts with different currencies. The data has no example of this case.
