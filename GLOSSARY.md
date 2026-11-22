# Profit Tracker

A personal tool that tracks investments held at a broker or a crypto exchange. The user imports the exports of these providers, and the tool shows the state and the results of the investments.

## Language

### Accounts and records

**Provider**:
The company that holds an Account, for example XTB (a broker) or Kraken (a crypto exchange).
_Avoid_: Broker (for Kraken), exchange, platform

**Account**:
One account at a Provider with one base currency, for example the XTB USD account, the XTB IKZE account or the Kraken account (base currency EUR).
_Avoid_: Wallet, portfolio, konto

**Cash Operation**:
One entry from the broker's record of cash movements on an Account, for example a deposit or a stock purchase. Cash Operations are the source of truth for an Account.
_Avoid_: Transaction, trade, record

**Ledger**:
All Cash Operations of one Account, in time order.
_Avoid_: History, journal

**Transfer**:
A move of money between two Accounts of the user. On the Account that sends the money it is a Transfer Out. On the Account that gets the money it is a Transfer In.
_Avoid_: IKZE deposit, internal deposit

**Net Deposits**:
The money that the user put into an Account: deposits and Transfers In, minus withdrawals and Transfers Out.
_Avoid_: Contributions, invested capital

### Holdings

**Instrument**:
One security or crypto asset that a Provider sells, for example a stock, an ETF or Bitcoin, identified by the Provider's ticker or asset code.
_Avoid_: Asset, symbol, ticker, coin

**Fill**:
One purchase Cash Operation that executes a part of a buy order. An order for a fractional volume has two or more Fills.
_Avoid_: Execution, partial

**Lot**:
One or more Fills of an Instrument that the broker tracks as one unit, identified by the broker's position ID. A Lot can be closed fully or in parts, in any order.
_Avoid_: Position, pozycja, trade

**Closed Position**:
One closed part of one Lot. The name is the name that the XTB export uses for the same record. The word "Position" alone stays a word to avoid.
_Avoid_: Sale, sell transaction, trade

**Holding**:
All open units of one Instrument on one Account, taken together. On an XTB Account these units are the open Lots. A Kraken Account has no Lots.
_Avoid_: Position, pozycja, asset

**Average Cost**:
The cost of one unit of a Holding on an Account with no Lots: the cost of all units that the user bought, divided by their quantity, weighted by quantity. A sale does not change it.
_Avoid_: Cost basis, break-even price, średnia

**Quote Currency**:
The currency in which the price source gives the price of an Instrument. It can be different from the Account currency.
_Avoid_: Instrument currency, price currency

**Price Warning**:
A sign that the price ticker or the Quote Currency of an Instrument can be wrong. The kinds are: the price source has no data, the price source gives a different currency, and the price source gives a price that does not agree with the broker prices.
_Avoid_: Alert, error, problem

**Allocation**:
The share of each part in the value of one Account or of all Accounts. The parts are the cash and the Holdings with a known value. One Instrument on two Accounts is one part. A Holding with no known value is not a part.
_Avoid_: Portfolio share, breakdown, udział

**Instrument Overview**:
The view of one Instrument on one Account. It shows the price history of the Instrument with each Fill and each Closed Position marked on it, the Holding, the open Lots and the Closed Positions.
_Avoid_: Position overview, details, pop-up

### Currencies

**Exchange Rate**:
The average rate of the National Bank of Poland (NBP) for one currency against PLN on one day.
_Avoid_: FX rate, kurs

**Currency Result**:
The part of the Total Result in PLN that comes from the change of the Exchange Rate between the day of each Net Deposits flow and today.
_Avoid_: FX gain, kursówka, currency profit

### Results

**Realized Profit**:
The sale amount of a closed part of a Lot minus the cost of that part, in the Account currency. It can be negative.
_Avoid_: P/L, gain, return

**Unrealized Profit**:
The current value of the open part of a Lot minus the cost of that part, in the Account currency.
_Avoid_: Paper profit, open P/L

**Total Result**:
The current value of an Account (cash and Holdings) minus its Net Deposits. It is equal to the sum of Realized Profit, Unrealized Profit, net dividends, net interest, Rewards and fees. The Total Result of a period is the change of the Total Result from the end of the day before the period to the end of its last day.
_Avoid_: Total P/L, return, wynik

**Reward**:
An amount of a crypto asset that a Provider pays to the user for a balance that the user holds, for example the weekly Kraken Auto Earn payment in BTC. It is income, like a dividend.
_Avoid_: Staking, interest, yield, earn

**Simple Return**:
The Total Result divided by the Net Deposits, as a percentage.
_Avoid_: ROI, yield

**Annual Return**:
The yearly rate of return that gives the current value from the Net Deposits flows, with the date of each flow taken into account.
_Avoid_: XIRR, IRR, CAGR

**Value History**:
The value of an Account, or of all Accounts, at the end of each calendar day.
_Avoid_: Chart, timeline, equity curve

**Time-Weighted Return**:
The rate of return in a period that the Net Deposits flows do not change. It measures the result of the investments, not the effect of the time of each deposit.
_Avoid_: TWR, performance

**Benchmark**:
A market index, a commodity or a crypto asset whose return in a period the tool compares with the Time-Weighted Return of the user. An accumulating ETF or ETC represents each market index and each commodity. The price of the asset itself represents a crypto asset (ADR 0008). A Benchmark is not an Instrument, also when the user holds the same asset as an Instrument. A Benchmark has no Provider ticker, no Lots, no Fills and no Price Warning.
_Avoid_: Index, reference, comparison

**Benchmark Return**:
The change of the value of a Benchmark in a period, in the currency of the comparison, as a percentage.
_Avoid_: Index return, market return

### IKZE

**IKZE Contribution**:
A deposit or a Transfer In on an IKZE Account. It counts towards the IKZE Limit of its calendar year in Polish time.
_Avoid_: IKZE deposit, wpłata

**IKZE Limit**:
The highest sum of IKZE Contributions in one calendar year that the law allows for the user.
_Avoid_: Cap, contribution cap

### Assistant

**Assistant**:
The part of the tool that answers the questions of the user in plain language. It uses only the results that the tool calculates. It describes and analyses the investments. It does not recommend a purchase or a sale.
_Avoid_: AI, chatbot, bot, agent

### Import

**Import**:
One upload of one Provider export file into one Account.
_Avoid_: Sync, upload

**Import Conflict**:
A Cash Operation in an Import that the tool cannot accept without a decision from the user, for example an operation with a known ID but different data.
_Avoid_: Error, duplicate
