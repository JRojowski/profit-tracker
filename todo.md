# To do

This file holds the feature ideas that are not yet GitHub Issues.
Each item becomes a GitHub Issue when the work starts. Then this file only links to the issue.

Last update: 2026-11-28.

## Order of work

1. Annual summary and PIT-38 draft.
2. Daily result.

Open issues: only the v5 Azure deployment (#31, #33 to #36). This work is on hold.

## Kraken: waits for an event (no decision yet)

The Kraken Import exists (#88, merged in #97). The Import stops on a sale and on a withdrawal (ADR 0009).

- [ ] Sale of a crypto asset: add it when the first sale is in the Kraken export.
- [ ] Withdrawal to a hardware wallet (Ledger): decide how the tool shows the BTC after the withdrawal.

## 1. Annual summary and PIT-38 draft (decided: needed)

Show the tax data for one calendar year.

- [ ] Annual summary: the realized result, the costs, the income, the dividends and the tax withheld abroad.
- [ ] PIT-38 draft: the values with the field numbers of the form.
- [ ] Loss from earlier years: let the user enter it and subtract it.
- [ ] Apply this feature only to taxable accounts. IKE and IKZE accounts are exempt.

Warning: a wrong tax value has a real cost. Write a full spec before the implementation. Test each rule against a known PIT-8C or a calculation made by hand.

Open questions:

- Which exchange rate applies to each event? The usual rule is the NBP average rate of the last working day before the event.
- Does the XTB export hold the tax withheld abroad for each dividend?

## 2. Daily result (decided: optional, low priority)

Show what moved the portfolio today.

- [ ] Table: the change of each holding today, in PLN and in percent, and its share of the total change.
- [ ] Total: the change of the portfolio today.

Data refresh: a permanent automatic refresh is not necessary for the first version.

- [ ] Refresh the prices when the user opens the page, if the prices are older than a set age (for example 15 minutes).
- [ ] Show the time of the last refresh and a manual refresh button.
- [ ] Optional: refresh on an interval while the page is open and the market is open.

Open question: Yahoo prices have a delay of about 15 minutes for some exchanges. Decide if the page must show this delay.

## Ideas backlog

These ideas are not sorted by priority and have no decision yet.
Discuss each group before it becomes a GitHub Issue.

### A. Personal investment analyst

The goal is an analyst that knows the user and helps to understand the portfolio.
The Assistant exists (#63). It reads the context of the current page (#65).

- [ ] Give the Assistant a personal context: the goals, the time horizon and the risk tolerance of the user.
- [ ] Explain the numbers in the application in plain words.
- [ ] Make the Assistant explain the data on the current screen first, before general answers.
- [ ] Show if the investments reach the goals of the user.
- [ ] Suggest how to improve the investments.
- [ ] Decide between the in-app Assistant and an MCP server that gives the portfolio data to an external assistant.

Note: the Assistant does not recommend a purchase or a sale (`GLOSSARY.md`, ADR 0007). The idea "suggest how to improve" needs a decision on this rule.

### B. Portfolio over time and composition

The Value History chart, the Time-Weighted Return, the Annual Return and the Allocation by Holding exist.

- [ ] Charts over time: the Total Result and the Annual Return.
- [ ] Maximum drawdown chart: show the chart on each Account page and on the All Accounts page.
- [ ] Composition by industry, sector, country and currency.

### C. Holdings

The Tiers, the Rebalancing page and the Tiers Breakdown exist (#109).

- [ ] Add an average buy price column to the Holdings table of an XTB Account. Today only the Kraken table has the Average Cost column.

### D. Transactions

- [ ] Add a list of all Cash Operations of an Account, similar to a spreadsheet. The Import History already shows the Cash Operations of one Import.
- [ ] Add a table of buys.

### E. Tax

- [ ] Tax optimization: a calculator that simulates the tax effect of a sale. It is related to section 1.

### F. New platforms and assets

- [ ] Mobile application. Note: PWA is in "Rejected for now".
- [ ] Other crypto assets on the Kraken Account. The Import accepts each asset code, but only Bitcoin has a check with a real export.

### G. Import preview and Import history

The goal: the user sees which operations an Import adds before the user confirms it.

- [ ] Import preview: show the list of the operations that the Import adds.
- [ ] Import preview: put the list in a section that the user can expand.
- [ ] Import preview: split the cash operations into new cash operations and known cash operations.

## Done

- Visual refresh: spec #73, tickets #74 to #79, merged in #80.
- Benchmarks page: spec #81, tickets #82 to #86, merged in #87.
- Total Result of a period in the Results function of the Assistant: #69.
- Kraken Import, crypto Holdings at the Average Cost and the Bitcoin Benchmark: spec #88, tickets #89 to #96, merged in #97.
- Nasdaq-100 Benchmark: merged in #110.
- Tiers, Rebalancing and the Tiers Breakdown: tickets #104 to #106, merged in #109.
- Tiers Breakdown, Accounts table and Import History fixes: hide the closed rows, a name on each Instrument row, sortable Accounts, the Period and the Cash Operations of each Import: #112.

## Rejected for now

- Dividends view: the portfolio is not a dividend portfolio.
- Import from other brokers (except Kraken), public portfolios, PWA: no value for one user.
