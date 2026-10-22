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

### Results

**Realized Profit**:
The sale amount of a closed part of a Lot minus the cost of that part, in the Account currency. It can be negative.
_Avoid_: P/L, gain, return

**Unrealized Profit**:
The current value of the open part of a Lot minus the cost of that part, in the Account currency.
_Avoid_: Paper profit, open P/L

### Import

**Import**:
One upload of one broker export file into one Account.
_Avoid_: Sync, upload

**Import Conflict**:
A Cash Operation in an Import that the tool cannot accept without a decision from the user, for example an operation with a known ID but different data.
_Avoid_: Error, duplicate
