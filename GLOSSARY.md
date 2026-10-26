# Profit Tracker

A personal tool that tracks investments held at a broker. The user imports broker exports, and the tool shows the state and the results of the investments.

## Language

### Accounts and records

**Account**:
One broker account with one base currency, for example the XTB USD account or the XTB IKZE account.
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
One security that the broker sells, for example a stock or an ETF, identified by its broker ticker.
_Avoid_: Asset, symbol, ticker

**Fill**:
One purchase Cash Operation that executes a part of a buy order. An order for a fractional volume has two or more Fills.
_Avoid_: Execution, partial

**Lot**:
One or more Fills of an Instrument that the broker tracks as one unit, identified by the broker's position ID. A Lot can be closed fully or in parts, in any order.
_Avoid_: Position, pozycja, trade

**Holding**:
All open Lots of one Instrument on one Account, taken together.
_Avoid_: Position, pozycja, asset

**Quote Currency**:
The currency in which the price source gives the price of an Instrument. It can be different from the Account currency.
_Avoid_: Instrument currency, price currency

**Allocation**:
The share of each part in the value of one Account or of all Accounts. The parts are the cash and the Holdings with a known value. One Instrument on two Accounts is one part. A Holding with no known value is not a part.
_Avoid_: Portfolio share, breakdown, udział

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
The current value of an Account (cash and Holdings) minus its Net Deposits. It is equal to the sum of Realized Profit, Unrealized Profit, net dividends, net interest and fees.
_Avoid_: Total P/L, return, wynik

**Simple Return**:
The Total Result divided by the Net Deposits, as a percentage.
_Avoid_: ROI, yield

**Annual Return**:
The yearly rate of return that gives the current value from the Net Deposits flows, with the date of each flow taken into account.
_Avoid_: XIRR, IRR, CAGR

### IKZE

**IKZE Contribution**:
A deposit or a Transfer In on an IKZE Account. It counts towards the IKZE Limit of its calendar year in Polish time.
_Avoid_: IKZE deposit, wpłata

**IKZE Limit**:
The highest sum of IKZE Contributions in one calendar year that the law allows for the user.
_Avoid_: Cap, contribution cap

### Import

**Import**:
One upload of one broker export file into one Account.
_Avoid_: Sync, upload

**Import Conflict**:
A Cash Operation in an Import that the tool cannot accept without a decision from the user, for example an operation with a known ID but different data.
_Avoid_: Error, duplicate
