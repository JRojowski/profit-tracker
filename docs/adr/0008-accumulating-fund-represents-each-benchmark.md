# An accumulating fund represents each Benchmark, with split-adjusted closes

The tool compares the Time-Weighted Return of the user with the Benchmark Return of each Benchmark. A Benchmark is a market index or a commodity. The price source does not give a total return index. Thus an accumulating ETF or ETC that follows the index represents each Benchmark. The tool stores the closes of the fund as the price source gives them, adjusted for splits.

## Why

- An accumulating fund keeps the dividends in its price. The Total Result of the user includes the dividends. A price index with no dividends makes the comparison unfair.
- The price source has daily closes of the funds. It has no daily data of a total return index.
- A Benchmark has no Lots and no raw volumes. The raw-price conversion of ADR 0005 exists only because the Lot volumes are raw. The Benchmark Return needs a continuous series, and a split-adjusted close gives one.
- The user made the decision to use an accumulating fund. It closes the open question about the dividends of a Benchmark.

## The rule

1. One accumulating ETF or ETC represents each Benchmark. The fund pays no dividends.
2. The Quote Currency of the fund is a full unit. A London listing in pence (GBp) is not acceptable.
3. The tool gets the closes through the existing price provider, with `AdjustedDailyCloses`. It does not apply the conversion of ADR 0005.
4. The tool stores the closes of a Benchmark apart from the prices of the Instruments, with their own record of the stored date range.
5. The tool does not store a close in a currency that is not the Quote Currency of the Benchmark. It does not guess a conversion.
6. The list of the Benchmarks is in one place in the backend: `backend/internal/benchmark`.
7. A crypto asset Benchmark uses the price of the asset itself, not a fund. Rule 1 does not apply to it. See "Amendment: crypto asset Benchmarks".

## The list

The ticker check on 2026-11-22 used the chart endpoint of Yahoo Finance. Each fund has daily closes, no dividend events and a currency in a full unit. No candidate needed a replacement.

| Benchmark | Fund | Price ticker | Quote Currency | First close |
|---|---|---|---|---|
| S&P 500 | iShares Core S&P 500 UCITS ETF USD (Acc) | `CSPX.L` | USD | 2010-09-15 |
| WIG20TR | Beta ETF WIG20TR Portfelowy FIZ | `ETFBW20TR.WA` | PLN | 2019-01-07 |
| mWIG40TR | Beta ETF mWIG40TR Portfelowy FIZ | `ETFBM40TR.WA` | PLN | 2019-09-05 |
| World equities | iShares Core MSCI World UCITS ETF USD (Acc) | `IWDA.L` | USD | 2009-09-25 |
| Gold | iShares Physical Gold ETC | `IGLN.L` | USD | 2011-04-08 |

mWIG40TR has the shortest history. For a period that starts before its first close, it has no Benchmark Return. The tool does not shorten the period for one Benchmark.

## Amendment: crypto asset Benchmarks

The user added Bitcoin as a Benchmark on 2026-11-22. The Bitcoin Benchmark shows the result of a plan that puts all deposits into Bitcoin.

A crypto asset pays no dividends. Thus its price is equal to its total return, and the reason for rule 1 does not apply. The price source gives a daily price of the asset itself. A fund adds a yearly cost and has a shorter history. Thus the price of the asset represents a crypto asset Benchmark.

| Benchmark | Price ticker | Quote Currency | First close |
|---|---|---|---|
| Bitcoin | `BTC-USD` | USD | 2014-09-17 |

The ticker check on 2026-11-22 used the chart endpoint of Yahoo Finance. The ticker has no dividend events and a currency in a full unit.

The Bitcoin Benchmark is not the Bitcoin Instrument on the Kraken Account. The two have different price tickers and different stored prices.

A crypto asset has a price on each day of the week. Rules 2 to 5 apply with no change.

We rejected a Bitcoin ETP, for example a physical Bitcoin fund on a European exchange. It has a yearly cost and a shorter history, and it gives no advantage.

## Considered options

- **A price index with no dividends, for example `^GSPC`.** The price source has it with a long history. We rejected it because it ignores the dividends.
- **A distributing fund with the dividends added by the tool.** It needs the dividend events and a reinvestment rule. We rejected it because an accumulating fund gives the same result with no extra rule.
- **The raw closes of ADR 0005.** We rejected them because a split in the period gives a false drop of the Benchmark Return.

## Consequences

- The Benchmark Return is lower than the return of the index by the cost of the fund and by its tracking error. The page shows the name of the fund, so that the user knows what the tool compares.
- A later split changes the stored closes before the split at the price source. The tool does not get them again. For an accumulating UCITS fund a split is rare.
- To add a Benchmark, a developer adds one entry to the list and checks its price ticker as above.
