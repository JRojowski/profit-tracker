# The Average Cost gives the cost of a Holding on an Account with no Lots

The Kraken export has no Lots and no position IDs. Thus the tool cannot use the Lot composition of ADR 0002 for a Kraken Account. The tool keeps one Average Cost for each Holding on such an Account. The cost includes the fee of each purchase and the value of each Reward on its day.

## Why

- The Kraken ledger has one entry for each purchase, with no link between a purchase and a later sale. A Lot from the Provider does not exist.
- The user made the decision to use a weighted Average Cost. It is the most simple rule, and the user does not need a cost for each purchase.
- The Polish tax on crypto assets uses the yearly sum of the income and the yearly sum of the costs. It does not match sold units to purchases. Thus FIFO gives no tax advantage to the user.

## The rule

1. Each purchase adds its quantity and its cost to the Holding. The cost is the full EUR amount that the user paid. Kraken takes the fee of a purchase in the crypto asset or in EUR. The fee rate is the same in both currencies.
   - A fee in the crypto asset decreases the quantity. The quantity is the crypto amount after the fee. The cost is the EUR amount.
   - A fee in EUR decreases the EUR cash. The quantity is the full crypto amount. The cost is the EUR amount plus the EUR fee.
   - A purchase with a fee on both rows, or with a fee in a different currency, stops the Import.
2. The fee is part of the cost. The Total Result of a Kraken Account has no separate fee part. The tool shows the sum of the fees in EUR as information. A fee in EUR adds its amount. A fee in the crypto asset adds the fee times the purchase price.
3. A Reward adds its quantity to the Holding. Its cost is its value in EUR on the day that the user got it. The tool uses the `<ASSET>-EUR` close of that day.
4. The value of each Reward on its day is the Rewards part of the Total Result. A later change of the price of these units is part of the Unrealized Profit.
5. The Average Cost is the sum of the costs divided by the sum of the quantities.
6. The tool does not calculate a sale yet. An Import with a sale, a withdrawal or another unknown entry stops with a list of these entries. The user made the decision to design a sale when a real export contains one.

## Considered options

- **FIFO.** It gives a cost for each sold unit. We rejected it because the user does not need this detail, and the Polish tax does not use it.
- **One synthetic Lot for each purchase.** It reuses the Lot views of XTB. We rejected it because the Lots are not real, and the user cannot close one of them.
- **A Reward with a cost of zero.** We rejected it because the Total Result then counts the value of the Reward two times: one time as a Reward and one time as Unrealized Profit.
- **A separate fee part in the Total Result.** We rejected it because a fee in the crypto asset decreases the quantity. Its value in EUR is only an estimate on the day of the purchase. One rule for both fee currencies is more simple.

## Consequences

- The Instrument Overview of a crypto asset shows the Average Cost and a list of purchases. It shows no Lots and no Closed Positions.
- To add a sale later, a developer adds a rule for the Realized Profit: the EUR amount of the sale minus the Average Cost times the sold quantity.
