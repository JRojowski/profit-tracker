# Store raw prices that the tool calculates from split events

The Value History needs the close price of each Instrument on each day. Yahoo gives a `close` that it adjusts for all splits, also for the splits after that day. The tool does not store this adjusted price. It multiplies the price by the ratio of each later split and stores the result. The stored price is then the raw price of that day, in the same units as the volumes and prices in the broker export.

## Why

- The Ledger has raw volumes and raw prices from XTB. A split-adjusted price with a raw volume gives a wrong value. For a 4:1 split, the value before the split is 4 times too low.
- Yahoo changes all earlier `close` values after each new split. An adjusted price that the tool stored before the split is then wrong, and the tool cannot see this.
- After a ticker is delisted, Yahoo deletes all its history. The tool cannot get the history again. A raw price never changes, so the stored prices stay correct without a new download.
- The price check compares the Yahoo price with the price of each XTB transaction. This check needs raw prices.

## The rule

1. The tool asks Yahoo for the daily prices with `events=split`.
2. For each day, the tool multiplies the `close` by the ratio (numerator / denominator) of each split with a date after that day.
3. The tool stores the result. It never changes a stored price later, because a later split does not change a raw price.
4. The tool deletes the stored prices of an Instrument only when its price ticker changes.

## Example

Yahoo gives a `close` of 124.81 for AAPL on 2020-08-28. On 2020-08-31 AAPL had a 4:1 split. The tool stores 124.81 × 4 = 499.24. This is the price of that day in the export.

## Not in this decision

- A split in the Ledger. The XTB import does not know a split operation and stops with an error. The volumes in the Ledger stay raw.
- The dividend adjustment. The tool uses `close`, not `adjclose`, because the Value History counts the dividends as cash.
- The errors in the Yahoo data, for example a split that Yahoo did not apply. The price check finds most of these errors.
