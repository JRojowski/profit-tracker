// Synthetic API responses for the screenshot tests. No value comes from a real export.
// The shapes follow the types in src/lib/api.ts. Amounts and percentages are decimal text.
import type {
  Account,
  AccountBenchmarks,
  AccountSummary,
  AccountValueHistory,
  AllAccountsRow,
  AllAccountsSummary,
  AllAccountsValueHistory,
  Allocation,
  BenchmarkReturn,
  Benchmarks,
  AllocationPart,
  BenchmarkPoint,
  ClosedPosition,
  Holding,
  HistoryPeriod,
  IkzeLimit,
  ImportPreview,
  ImportSummary,
  InstrumentOverview,
  OverviewFill,
  PriceWarning,
  Rebalancing,
  Tiers,
  UnsupportedEntry,
  ValueHistoryPoint,
} from "../src/lib/api"

// fixedNow is the fixed clock of the tests. It is a Monday, noon in UTC.
export const fixedNow = new Date("2026-06-15T12:00:00Z")

// today is the date of fixedNow, as YYYY-MM-DD. The prices and the Exchange Rates have this date.
const today = "2026-06-15"

// DataSet selects which data the fake backend answers with.
export type DataSet =
  | "populated"
  | "nearEqualEnd"
  | "benchmarkUnavailable"
  | "noAccounts"
  | "emptyAccounts"
  | "kraken"
  | "krakenRewardUnknown"

// The first Account is a regular Account. The second one is an IKZE Account.
export const mainAccount: Account = {
  id: 1,
  accountNumber: "10000001",
  name: "Main Account",
  type: "regular",
  currency: "PLN",
  provider: "xtb",
}

export const ikzeAccount: Account = {
  id: 2,
  accountNumber: "10000002",
  name: "Retirement",
  type: "IKZE",
  currency: "PLN",
  provider: "xtb",
}

// krakenAccount is the Account at Kraken. It has no Lots and uses the Average Cost. Only the data set "kraken" has it.
export const krakenAccount: Account = {
  id: 3,
  accountNumber: "kraken-spot",
  name: "Kraken",
  type: "regular",
  currency: "EUR",
  provider: "kraken",
}

const priceMismatch: PriceWarning = {
  ticker: "BETA.US",
  name: "Beta Software Inc.",
  priceTicker: "BETA",
  priceTickerManual: false,
  quoteCurrency: "USD",
  quoteCurrencyManual: false,
  priceConfirmed: false,
  kind: "priceMismatch",
  sourceCurrency: null,
  medianDifference: "6.42",
  maxDifference: "11.80",
  confirmable: true,
}

const noSource = {
  priceTickerManual: false,
  quoteCurrencyManual: false,
  priceConfirmed: false,
}

const alpha: Holding = {
  ticker: "ALPHA.PL",
  name: "Alpha Retail S.A.",
  volume: "120",
  cost: "14400.00",
  averageCost: "120.00",
  price: "158.40",
  priceDate: today,
  exchangeRate: null,
  exchangeRateCurrency: null,
  exchangeRateDate: null,
  value: "19008.00",
  unrealizedProfit: "4608.00",
  unrealizedProfitPercent: "32.00",
  note: null,
  priceWarnings: [],
  priceTicker: "ALR",
  quoteCurrency: "PLN",
  ...noSource,
}

const beta: Holding = {
  ticker: "BETA.US",
  name: "Beta Software Inc.",
  volume: "40",
  cost: "16000.00",
  averageCost: "400.00",
  price: "128.00",
  priceDate: today,
  exchangeRate: "4.1000",
  exchangeRateCurrency: "USD",
  exchangeRateDate: today,
  value: "20992.00",
  unrealizedProfit: "4992.00",
  unrealizedProfitPercent: "31.20",
  note: null,
  priceWarnings: [priceMismatch],
  priceTicker: "BETA",
  quoteCurrency: "USD",
  ...noSource,
}

const gamma: Holding = {
  ticker: "GAMMA.PL",
  name: "Gamma Energy S.A.",
  volume: "300",
  cost: "9000.00",
  averageCost: "30.00",
  price: "28.50",
  priceDate: today,
  exchangeRate: null,
  exchangeRateCurrency: null,
  exchangeRateDate: null,
  value: "8550.00",
  unrealizedProfit: "-450.00",
  unrealizedProfitPercent: "-5.00",
  note: null,
  priceWarnings: [],
  priceTicker: "GMA",
  quoteCurrency: "PLN",
  ...noSource,
}

const delta: Holding = {
  ticker: "DELTA.PL",
  name: "Delta Bond Fund",
  volume: "80",
  cost: "8000.00",
  averageCost: "100.00",
  price: "112.50",
  priceDate: today,
  exchangeRate: null,
  exchangeRateCurrency: null,
  exchangeRateDate: null,
  value: "9000.00",
  unrealizedProfit: "1000.00",
  unrealizedProfitPercent: "12.50",
  note: null,
  priceWarnings: [],
  priceTicker: "DLT",
  quoteCurrency: "PLN",
  ...noSource,
}

// bitcoin is the crypto Holding of the Kraken Account. The volume has 8 decimal places and includes the Rewards.
// The cost is 1999.74 EUR of purchases plus 36.52 EUR of Rewards at the closes of their days.
// The Average Cost has more decimal places than the screen shows. cost = volume × averageCost, rounded.
const bitcoin: Holding = {
  ticker: "BTC",
  name: "BTC",
  volume: "0.03448875",
  cost: "2036.26",
  averageCost: "59041.2815773259",
  price: "92450.00",
  priceDate: today,
  exchangeRate: "4.2600",
  exchangeRateCurrency: "EUR",
  exchangeRateDate: today,
  value: "3188.48",
  unrealizedProfit: "1152.22",
  unrealizedProfitPercent: "56.59",
  note: null,
  priceWarnings: [],
  priceTicker: "BTC-EUR",
  quoteCurrency: "EUR",
  ...noSource,
}

// bitcoinRewardUnknown is the Bitcoin Holding when a Reward has no stored close on or before its day.
// Then the cost, the Average Cost and the Unrealized Profit are not known. The value comes from the price.
const bitcoinRewardUnknown: Holding = {
  ...bitcoin,
  cost: null,
  averageCost: null,
  unrealizedProfit: null,
  unrealizedProfitPercent: null,
  note: "A Reward has no close of its day or an earlier day. Refresh the prices to get the price history.",
}

const mainHoldings = [alpha, beta, gamma]
const ikzeHoldings = [delta]

// The Closed Positions of the main Account: three gains and one loss. Their Realized Profit adds up to 2410.55.
function closed(
  xtbId: string,
  time: string,
  ticker: string,
  name: string,
  volume: string,
  cost: string,
  amount: string,
  realizedProfit: string,
  openTime: string,
  openPrice: string,
  closePrice: string
): ClosedPosition {
  return {
    xtbId,
    time,
    ticker,
    name,
    positionId: `9${xtbId}`,
    volume,
    amount,
    cost,
    realizedProfit,
    openTime,
    openPrice,
    closePrice,
  }
}

const mainClosed: ClosedPosition[] = [
  closed(
    "5001",
    "2026-02-10T10:15:00Z",
    "ALPHA.PL",
    "Alpha Retail S.A.",
    "30",
    "3600.00",
    "4440.00",
    "840.00",
    "2025-03-04T09:30:00Z",
    "120.00",
    "148.00"
  ),
  closed(
    "5002",
    "2026-03-18T14:40:00Z",
    "BETA.US",
    "Beta Software Inc.",
    "10",
    "4000.00",
    "4900.00",
    "900.00",
    "2025-05-12T13:45:00Z",
    "100.00",
    "119.50"
  ),
  closed(
    "5003",
    "2026-04-22T08:05:00Z",
    "OMEGA.PL",
    "Omega Mining S.A.",
    "200",
    "5000.00",
    "4350.00",
    "-650.00",
    "2025-09-01T08:10:00Z",
    "25.00",
    "21.75"
  ),
  closed(
    "5004",
    "2026-05-27T11:20:00Z",
    "ALPHA.PL",
    "Alpha Retail S.A.",
    "60",
    "7200.00",
    "8520.55",
    "1320.55",
    "2025-03-04T09:30:00Z",
    "120.00",
    "142.01"
  ),
]

// Totals of the main Account. value = cash + the value of the Holdings. totalResult is the sum of its parts.
const mainSummary: AccountSummary = {
  accountId: 1,
  currency: "PLN",
  cash: "3200.50",
  netDeposits: "39519.40",
  value: "51750.50",
  totalResult: "12231.10",
  realizedProfit: "2410.55",
  unrealizedProfit: "9150.00",
  netDividends: "820.40",
  netInterest: "35.15",
  fees: "-185.00",
  rewards: "0.00",
  feeSum: "0.00",
  priceDate: today,
  stalePrices: false,
  exchangeRateDate: today,
  staleRates: false,
  unpricedHoldings: 0,
  simpleReturn: "30.95",
  simpleReturnNote: null,
  annualReturn: "11.20",
  annualReturnNote: null,
}

const ikzeSummary: AccountSummary = {
  accountId: 2,
  currency: "PLN",
  cash: "450.00",
  netDeposits: "8800.00",
  value: "9450.00",
  totalResult: "650.00",
  realizedProfit: "0.00",
  unrealizedProfit: "1000.00",
  netDividends: "0.00",
  netInterest: "0.00",
  fees: "-350.00",
  rewards: "0.00",
  feeSum: "0.00",
  priceDate: today,
  stalePrices: false,
  exchangeRateDate: null,
  staleRates: false,
  unpricedHoldings: 0,
  simpleReturn: "7.39",
  simpleReturnNote: null,
  annualReturn: "3.10",
  annualReturnNote: null,
}

// Totals of the Kraken Account: 2500 EUR of Deposits, one Bitcoin Holding and the rest in cash.
// The fee is in the cost, so the fees part is zero. feeSum is information only.
const krakenSummary: AccountSummary = {
  accountId: 3,
  currency: "EUR",
  cash: "500.26",
  netDeposits: "2500.00",
  value: "3688.74",
  totalResult: "1188.74",
  realizedProfit: "0.00",
  unrealizedProfit: "1152.22",
  netDividends: "0.00",
  netInterest: "0.00",
  fees: "0.00",
  rewards: "36.52",
  feeSum: "8.97",
  priceDate: today,
  stalePrices: false,
  exchangeRateDate: today,
  staleRates: false,
  unpricedHoldings: 0,
  simpleReturn: "47.55",
  simpleReturnNote: null,
  annualReturn: "21.40",
  annualReturnNote: null,
}

// krakenRewardUnknownSummary is the Kraken summary when a Reward has no stored close (ADR 0009, rule 3).
// The Holding keeps its value from the price, so the value and the Total Result stay known.
// The tool cannot split the Total Result into the Unrealized Profit and the Rewards part.
const krakenRewardUnknownSummary: AccountSummary = {
  ...krakenSummary,
  unrealizedProfit: null,
  rewards: null,
}

// emptySummary is the summary of an Account with no Cash Operations.
function emptySummary(accountId: number): AccountSummary {
  return {
    accountId,
    currency: "PLN",
    cash: "0.00",
    netDeposits: "0.00",
    value: "0.00",
    totalResult: "0.00",
    realizedProfit: "0.00",
    unrealizedProfit: "0.00",
    netDividends: "0.00",
    netInterest: "0.00",
    fees: "0.00",
    rewards: "0.00",
    feeSum: "0.00",
    priceDate: null,
    stalePrices: false,
    exchangeRateDate: null,
    staleRates: false,
    unpricedHoldings: 0,
    simpleReturn: null,
    simpleReturnNote: "The Account has no Net Deposits.",
    annualReturn: null,
    annualReturnNote: "The Account has no Net Deposits.",
  }
}

// share gives the exact percentage of part in total as decimal text with 10 decimal places.
function share(part: number, total: number): string {
  return ((part / total) * 100).toFixed(10)
}

// allocationOf builds the Allocation of Holdings and cash. The parts are in order of value.
function allocationOf(
  holdings: Holding[],
  cash: string,
  currency: string
): Allocation {
  const parts = holdings.map((holding) => ({
    ticker: holding.ticker,
    name: holding.name,
    value: Number(holding.value),
  }))
  parts.sort((a, b) => b.value - a.value)
  const cashValue = Number(cash)
  const total = parts.reduce((sum, part) => sum + part.value, cashValue)
  const quoteTotals = new Map<string, number>()
  for (const holding of holdings) {
    const currencyOfQuote = holding.quoteCurrency ?? currency
    quoteTotals.set(
      currencyOfQuote,
      (quoteTotals.get(currencyOfQuote) ?? 0) + Number(holding.value)
    )
  }
  const byQuoteCurrency = [...quoteTotals.entries()]
    .sort((a, b) => b[1] - a[1])
    .map(([quoteCurrency, value]) => ({
      currency: quoteCurrency,
      value: value.toFixed(2),
      share: share(value, total),
    }))
  const cashPart: AllocationPart = {
    ticker: "",
    name: "Cash",
    value: cash,
    share: share(cashValue, total),
  }
  return {
    available: true,
    warnings: [],
    currency,
    total: total.toFixed(2),
    byInstrument: {
      holdings: parts.map((part) => ({
        ticker: part.ticker,
        name: part.name,
        value: part.value.toFixed(2),
        share: share(part.value, total),
      })),
      cash: cashValue > 0 ? cashPart : null,
      other: null,
    },
    byQuoteCurrency: byQuoteCurrency,
    unpricedHoldings: 0,
  }
}

// emptyAllocation is the Allocation of an Account with no value.
function emptyAllocation(currency: string): Allocation {
  return {
    available: true,
    warnings: [],
    currency,
    total: "0.00",
    byInstrument: { holdings: [], cash: null, other: null },
    byQuoteCurrency: [],
    unpricedHoldings: 0,
  }
}

// historyDays are the calendar days of the Value History: three years, one point for each day.
function historyDays(): string[] {
  const days: string[] = []
  const end = Date.UTC(2026, 5, 15)
  for (let time = Date.UTC(2023, 5, 15); time <= end; time += 86_400_000) {
    days.push(new Date(time).toISOString().slice(0, 10))
  }
  return days
}

// historyPoints builds a smooth Value History that ends at the given Net Deposits and value.
// The Net Deposits rise in monthly steps. The value follows them with a slow wave on top of the gain.
function historyPoints(
  finalDeposits: number,
  finalValue: number,
  startDeposits: number,
  wave: number
): ValueHistoryPoint[] {
  const days = historyDays()
  const last = days.length - 1
  const steps = Math.floor(last / 30)
  const stepSize = (finalDeposits - startDeposits) / steps
  return days.map((date, index) => {
    const t = index / last
    const deposits =
      index === last
        ? finalDeposits
        : startDeposits + Math.floor(index / 30) * stepSize
    const gain = (finalValue - finalDeposits) * t ** 1.5
    const ripple = index === last ? 0 : wave * Math.sin(index / 25) * t
    return {
      date,
      value: (deposits + gain + ripple).toFixed(2),
      netDeposits: deposits.toFixed(2),
    }
  })
}

const periodStarts: Record<HistoryPeriod, string | null> = {
  "1M": "2026-05-15",
  "3M": "2026-03-15",
  YTD: "2026-01-01",
  "1Y": "2025-06-15",
  All: "2023-06-15",
}

const emptyPeriodStarts: Record<HistoryPeriod, string | null> = {
  "1M": null,
  "3M": null,
  YTD: null,
  "1Y": null,
  All: null,
}

const noReturns: Record<HistoryPeriod, string | null> = {
  "1M": null,
  "3M": null,
  YTD: null,
  "1Y": null,
  All: null,
}

const mainHistory: AccountValueHistory = {
  accountId: 1,
  currency: "PLN",
  points: historyPoints(39519.4, 51750.5, 5000, 1100),
  timeWeightedReturn: {
    "1M": "1.80",
    "3M": "4.10",
    YTD: "9.60",
    "1Y": "18.40",
    All: "31.20",
  },
  periodStarts,
  valuedAtCost: [],
  priceWarnings: [priceMismatch],
}

// nearEqualHistory ends with a value within 1 percent of the Net Deposits, so the two end tags of the chart overlap.
const nearEqualHistory: AccountValueHistory = {
  ...mainHistory,
  points: historyPoints(39519.4, 39700.0, 5000, 300),
}

const ikzeHistory: AccountValueHistory = {
  accountId: 2,
  currency: "PLN",
  points: historyPoints(8800, 9450, 1500, 160),
  timeWeightedReturn: {
    "1M": "0.40",
    "3M": "1.20",
    YTD: "2.80",
    "1Y": "5.10",
    All: "7.20",
  },
  periodStarts,
  valuedAtCost: [],
  priceWarnings: [],
}

const krakenHistory: AccountValueHistory = {
  accountId: 3,
  currency: "EUR",
  points: historyPoints(2500, 3688.74, 500, 140),
  timeWeightedReturn: {
    "1M": "3.20",
    "3M": "11.40",
    YTD: "24.80",
    "1Y": "38.10",
    All: "47.90",
  },
  periodStarts,
  valuedAtCost: [],
  priceWarnings: [],
}

function emptyHistory(accountId: number): AccountValueHistory {
  return {
    accountId,
    currency: "PLN",
    points: [],
    timeWeightedReturn: noReturns,
    periodStarts: emptyPeriodStarts,
    valuedAtCost: [],
    priceWarnings: [],
  }
}

const allHistory: AllAccountsValueHistory = {
  currency: "PLN",
  available: true,
  warnings: [],
  points: mainHistory.points.map((point, index) => ({
    date: point.date,
    value: (
      Number(point.value) + Number(ikzeHistory.points[index].value)
    ).toFixed(2),
    netDeposits: (
      Number(point.netDeposits) + Number(ikzeHistory.points[index].netDeposits)
    ).toFixed(2),
  })),
  timeWeightedReturn: {
    "1M": "1.60",
    "3M": "3.70",
    YTD: "8.70",
    "1Y": "16.50",
    All: "28.90",
  },
  periodStarts,
  valuedAtCost: [],
  priceWarnings: [priceMismatch],
}

// returnSeries gives a smooth daily series of cumulative returns from "0" to the final return, one point for each day.
// The last point has the exact text of the final return. The wave makes the lines differ.
function returnSeries(
  from: string,
  to: string,
  final: string,
  wave: number
): BenchmarkPoint[] {
  const days: string[] = []
  const end = Date.parse(to)
  for (let time = Date.parse(from); time <= end; time += 86_400_000) {
    days.push(new Date(time).toISOString().slice(0, 10))
  }
  const last = days.length - 1
  return days.map((date, index) => {
    const t = index / last
    const value =
      index === 0
        ? "0"
        : index === last
          ? final
          : (
              Number(final) * t ** 1.3 +
              wave * Math.sin(index / 30) * Math.sin(Math.PI * t)
            ).toFixed(2)
    return { date, return: value }
  })
}

// benchmarkRows gives the six Benchmarks for a user return. Gold has no price for the whole period.
function benchmarkRows(
  user: number,
  from: string,
  to: string
): BenchmarkReturn[] {
  const rows: [string, string, string, string, string, number | null][] = [
    [
      "sp500",
      "S&P 500",
      "iShares Core S&P 500 UCITS ETF USD (Acc)",
      "CSPX.L",
      "USD",
      45.1,
    ],
    [
      "wig20tr",
      "WIG20TR",
      "Beta ETF WIG20TR Portfelowy FIZ",
      "ETFBW20TR.WA",
      "PLN",
      22.35,
    ],
    [
      "mwig40tr",
      "mWIG40TR",
      "Beta ETF mWIG40TR Portfelowy FIZ",
      "ETFBM40TR.WA",
      "PLN",
      18.9,
    ],
    [
      "world",
      "World equities",
      "iShares Core MSCI World UCITS ETF USD (Acc)",
      "IWDA.L",
      "USD",
      38.7,
    ],
    ["gold", "Gold", "iShares Physical Gold ETC", "IGLN.L", "USD", null],
    ["bitcoin", "Bitcoin", "Bitcoin spot price", "BTC-USD", "USD", 112.4],
  ]
  return rows.map(([id, name, fund, ticker, quoteCurrency, value], index) => ({
    id,
    name,
    fund,
    ticker,
    quoteCurrency,
    status: value === null ? "noFullPeriod" : "available",
    return: value === null ? null : value.toFixed(2),
    difference: value === null ? null : (user - value).toFixed(2),
    series:
      value === null
        ? []
        : returnSeries(from, to, value.toFixed(2), 2 + index * 1.5),
  }))
}

// accountBenchmarks gives the Benchmarks of an Account for the period All, from the Time-Weighted Return of its Value History.
function accountBenchmarks(
  history: AccountValueHistory,
  failed = false
): AccountBenchmarks {
  const user = history.timeWeightedReturn.All
  const from = history.periodStarts.All ?? ""
  const to = history.points.at(-1)?.date ?? ""
  return {
    accountId: history.accountId,
    period: "All",
    currency: history.currency,
    available: true,
    warnings: [],
    baseDay: history.periodStarts.All,
    lastDay: history.points.at(-1)?.date ?? null,
    user: {
      return: user,
      series: user === null ? [] : returnSeries(from, to, user, 3),
    },
    benchmarks: failed
      ? failedBenchmarkRows(Number(user), from, to)
      : benchmarkRows(Number(user), from, to),
  }
}

// failedBenchmarkRows is benchmarkRows with a failed price source for WIG20TR. Gold has no price for the whole period.
function failedBenchmarkRows(
  user: number,
  from: string,
  to: string
): BenchmarkReturn[] {
  return benchmarkRows(user, from, to).map((row) =>
    row.id === "wig20tr"
      ? {
          ...row,
          status: "sourceFailure",
          return: null,
          difference: null,
          series: [],
        }
      : row
  )
}

// emptyBenchmarks is the answer for an Account with no Value History.
function emptyBenchmarks(accountId: number): AccountBenchmarks {
  return {
    accountId,
    period: "All",
    currency: "PLN",
    available: false,
    warnings: [],
    baseDay: null,
    lastDay: null,
    user: { return: null, series: [] },
    benchmarks: [],
  }
}

// allAccountsBenchmarks gives the Benchmarks of all Accounts in PLN for the period All.
const allAccountsBenchmarks: Benchmarks = {
  period: "All",
  currency: "PLN",
  available: true,
  warnings: [],
  baseDay: allHistory.periodStarts.All,
  lastDay: allHistory.points.at(-1)?.date ?? null,
  user: {
    return: allHistory.timeWeightedReturn.All,
    series: returnSeries(
      allHistory.periodStarts.All ?? "",
      allHistory.points.at(-1)?.date ?? "",
      allHistory.timeWeightedReturn.All ?? "0",
      3
    ),
  },
  benchmarks: benchmarkRows(
    Number(allHistory.timeWeightedReturn.All),
    allHistory.periodStarts.All ?? "",
    allHistory.points.at(-1)?.date ?? ""
  ),
}

// emptyAllBenchmarks is the answer for all Accounts when they have no Value History.
const emptyAllBenchmarks: Benchmarks = {
  period: "All",
  currency: "PLN",
  available: false,
  warnings: [],
  baseDay: null,
  lastDay: null,
  user: { return: null, series: [] },
  benchmarks: [],
}

// accountRow is the row of one Account in the summary of all Accounts.
function accountRow(account: Account, summary: AccountSummary): AllAccountsRow {
  return {
    accountId: account.id,
    name: account.name,
    type: account.type,
    currency: account.currency,
    available: true,
    exchangeRate: null,
    unpricedHoldings: 0,
    note: null,
    value: summary.value,
    cash: summary.cash,
    netDeposits: summary.netDeposits,
    totalResult: summary.totalResult,
    realizedProfit: summary.realizedProfit,
    unrealizedProfit: summary.unrealizedProfit,
    netDividends: summary.netDividends,
    netInterest: summary.netInterest,
    fees: summary.fees,
    rewards: "0.00",
    currencyResult: "0.00",
    annualReturn: summary.annualReturn,
    annualReturnNote: null,
  }
}

const allSummary: AllAccountsSummary = {
  available: true,
  stalePrices: false,
  staleRates: false,
  priceDate: today,
  exchangeRateDate: today,
  unpricedHoldings: 0,
  warnings: [],
  value: "61200.50",
  cash: "3650.50",
  netDeposits: "48319.40",
  totalResult: "12881.10",
  realizedProfit: "2410.55",
  unrealizedProfit: "10150.00",
  netDividends: "820.40",
  netInterest: "35.15",
  fees: "-535.00",
  rewards: "0.00",
  currencyResult: "0.00",
  simpleReturn: "26.66",
  simpleReturnNote: null,
  annualReturn: "9.80",
  annualReturnNote: null,
  accounts: [
    accountRow(mainAccount, mainSummary),
    accountRow(ikzeAccount, ikzeSummary),
  ],
}

const emptyAllSummary: AllAccountsSummary = {
  ...allSummary,
  priceDate: null,
  exchangeRateDate: null,
  value: "0.00",
  cash: "0.00",
  netDeposits: "0.00",
  totalResult: "0.00",
  realizedProfit: "0.00",
  unrealizedProfit: "0.00",
  netDividends: "0.00",
  netInterest: "0.00",
  fees: "0.00",
  simpleReturn: null,
  simpleReturnNote: "The Accounts have no Net Deposits.",
  annualReturn: null,
  annualReturnNote: "The Accounts have no Net Deposits.",
  accounts: [],
}

const mainAllocation = allocationOf(mainHoldings, "3200.50", "PLN")
const ikzeAllocation = allocationOf(ikzeHoldings, "450.00", "PLN")
const allAllocation = allocationOf(
  [...mainHoldings, ...ikzeHoldings],
  "3650.50",
  "PLN"
)

const krakenAllocation = allocationOf([bitcoin], "500.26", "EUR")

const ikzeLimit: IkzeLimit = {
  currentYear: 2026,
  rows: [
    {
      year: 2026,
      contributions: "4400.00",
      limit: "11304.60",
      unused: "6904.60",
      overLimit: null,
    },
    {
      year: 2025,
      contributions: "4400.00",
      limit: "10407.60",
      unused: "6007.60",
      overLimit: null,
    },
  ],
}

const imports: ImportSummary[] = [
  {
    id: 2,
    fileName: "account_10000002_2026.xlsx",
    importedAt: "2026-06-10T08:30:00Z",
    accountId: 2,
    accountNumber: "10000002",
    accountName: "Retirement",
    newCashOperations: 18,
    skippedCashOperations: 0,
  },
  {
    id: 1,
    fileName: "account_10000001_2026.xlsx",
    importedAt: "2026-06-09T19:05:00Z",
    accountId: 1,
    accountNumber: "10000001",
    accountName: "Main Account",
    newCashOperations: 142,
    skippedCashOperations: 6,
  },
]

// overviewPrices builds the weekday closes of BETA.US in USD for the 150 days before the fixed date.
function overviewPrices(): { date: string; close: string }[] {
  const series: { date: string; close: string }[] = []
  const end = Date.UTC(2026, 5, 15)
  const days = 150
  for (let back = days; back >= 0; back--) {
    const time = end - back * 86_400_000
    const weekday = new Date(time).getUTCDay()
    if (weekday === 0 || weekday === 6) continue
    const step = days - back
    const close = 100 + step * 0.19 + 6 * Math.sin(step / 9)
    series.push({
      date: new Date(time).toISOString().slice(0, 10),
      close: close.toFixed(2),
    })
  }
  series[series.length - 1].close = "128.00"
  return series
}

const betaOverview: InstrumentOverview = {
  ticker: "BETA.US",
  name: "Beta Software Inc.",
  priceTicker: "BETA",
  priceTickerManual: false,
  quoteCurrency: "USD",
  quoteCurrencyManual: false,
  priceConfirmed: false,
  prices: { currency: "USD", series: overviewPrices() },
  fills: [
    {
      xtbId: "4101",
      time: "2025-05-12T13:45:00Z",
      volume: "10",
      price: "100.00",
      positionId: "94101",
      cost: null,
      fee: null,
      feeCurrency: null,
    },
    {
      xtbId: "4102",
      time: "2025-09-23T14:10:00Z",
      volume: "30",
      price: "122.50",
      positionId: "94102",
      cost: null,
      fee: null,
      feeCurrency: null,
    },
    {
      xtbId: "4103",
      time: "2026-02-04T15:25:00Z",
      volume: "10",
      price: "134.00",
      positionId: "94103",
      cost: null,
      fee: null,
      feeCurrency: null,
    },
  ],
  holding: beta,
  openLots: [
    {
      positionId: "94102",
      volume: "30",
      cost: "12000.00",
      openTime: "2025-09-23T14:10:00Z",
      openPrice: "122.50",
      fills: [
        {
          xtbId: "4102",
          time: "2025-09-23T14:10:00Z",
          volume: "30",
          price: "122.50",
          positionId: "94102",
          cost: null,
          fee: null,
          feeCurrency: null,
        },
      ],
    },
    {
      positionId: "94103",
      volume: "10",
      cost: "4000.00",
      openTime: "2026-02-04T15:25:00Z",
      openPrice: "134.00",
      fills: [
        {
          xtbId: "4103",
          time: "2026-02-04T15:25:00Z",
          volume: "10",
          price: "134.00",
          positionId: "94103",
          cost: null,
          fee: null,
          feeCurrency: null,
        },
      ],
    },
  ],
  realizedProfitSum: "900.00",
  closedPositions: [mainClosed[1]],
  rewards: [],
  feeSum: null,
}

// bitcoinPrices builds the daily closes of BTC-EUR for the 150 days before the fixed date. Crypto has weekend closes.
function bitcoinPrices(): { date: string; close: string }[] {
  const series: { date: string; close: string }[] = []
  const end = Date.UTC(2026, 5, 15)
  const days = 150
  for (let back = days; back >= 0; back--) {
    const time = end - back * 86_400_000
    const step = days - back
    const close = 52000 + step * 260 + 2600 * Math.sin(step / 11)
    series.push({
      date: new Date(time).toISOString().slice(0, 10),
      close: close.toFixed(2),
    })
  }
  series[series.length - 1].close = bitcoin.price ?? "0"
  return series
}

// cryptoPurchase builds one Kraken purchase as a Fill. A crypto purchase has no Lot, so positionId is empty.
function cryptoPurchase(
  xtbId: string,
  time: string,
  volume: string,
  price: string,
  cost: string,
  fee: string,
  feeCurrency: string
): OverviewFill {
  return { xtbId, time, volume, price, positionId: "", cost, fee, feeCurrency }
}

// bitcoinOverview is the Instrument Overview of Bitcoin on the Kraken Account.
// Two purchases have the fee in BTC and one has the fee in EUR. The fees add to the feeSum of the summary.
// The purchases cost 1999.74 EUR, and the Rewards have 36.52 EUR of value at the closes of their days.
const bitcoinOverview: InstrumentOverview = {
  ticker: "BTC",
  name: "BTC",
  priceTicker: "BTC-EUR",
  priceTickerManual: false,
  quoteCurrency: "EUR",
  quoteCurrencyManual: false,
  priceConfirmed: false,
  prices: { currency: "EUR", series: bitcoinPrices() },
  fills: [
    cryptoPurchase(
      "TQ7GA2-K1",
      "2026-01-20T09:14:00Z",
      "0.01294663",
      "53960.00",
      "700.00",
      "0.00002594",
      "BTC"
    ),
    cryptoPurchase(
      "TQ7GA2-K2",
      "2026-03-10T10:02:00Z",
      "0.01024529",
      "63190.00",
      "650.00",
      "2.60",
      "EUR"
    ),
    cryptoPurchase(
      "TQ7GA2-K3",
      "2026-05-05T08:47:00Z",
      "0.00814823",
      "79130.00",
      "649.74",
      "0.00006281",
      "BTC"
    ),
  ],
  holding: bitcoin,
  openLots: [],
  realizedProfitSum: null,
  closedPositions: [],
  rewards: [
    {
      xtbId: "ST3RW1-R1",
      time: "2026-03-02T01:05:00Z",
      volume: "0.00012000",
      value: "7.39",
    },
    {
      xtbId: "ST3RW1-R2",
      time: "2026-04-06T01:05:00Z",
      volume: "0.00012000",
      value: "9.00",
    },
    {
      xtbId: "ST3RW1-R3",
      time: "2026-05-04T01:05:00Z",
      volume: "0.00011990",
      value: "9.48",
    },
    {
      xtbId: "ST3RW1-R4",
      time: "2026-06-08T01:05:00Z",
      volume: "0.00011800",
      value: "10.65",
    },
  ],
  feeSum: krakenSummary.feeSum,
}

// populatedTiers is the configuration of two Tiers with an Instrument Target, a Shared Part and a sold Instrument.
const populatedTiers: Tiers = {
  tiers: [
    {
      order: 1,
      name: "Global Market Core",
      targetShare: "60",
      sharedPart: { name: "Blue Chips", targetShare: "40" },
      instrumentTargets: [
        { ticker: "ALPHA.PL", name: "Alpha Retail S.A.", targetShare: "60" },
      ],
      sharedInstruments: [
        { ticker: "BETA.US", name: "Beta Software Inc." },
        { ticker: "DELTA.PL", name: "Delta Bond Fund" },
      ],
    },
    {
      order: 2,
      name: "Asymmetric Bets",
      targetShare: "40",
      sharedPart: { name: "", targetShare: "100" },
      instrumentTargets: [],
      sharedInstruments: [],
    },
  ],
  instruments: [
    {
      ticker: "ALPHA.PL",
      name: "Alpha Retail S.A.",
      tierOrder: 1,
      targetShare: "60",
      hasOpenHolding: true,
    },
    {
      ticker: "BETA.US",
      name: "Beta Software Inc.",
      tierOrder: 1,
      targetShare: null,
      hasOpenHolding: true,
    },
    {
      ticker: "DELTA.PL",
      name: "Delta Bond Fund",
      tierOrder: 1,
      targetShare: null,
      hasOpenHolding: false,
    },
    {
      ticker: "GAMMA.PL",
      name: "Gamma Energy S.A.",
      tierOrder: null,
      targetShare: null,
      hasOpenHolding: true,
    },
  ],
}

// emptyTiers is the configuration before the user creates a Tier.
const emptyTiers: Tiers = {
  tiers: [],
  instruments: populatedTiers.instruments.map((i) => ({
    ...i,
    tierOrder: null,
    targetShare: null,
  })),
}

// tiersRejected is the 422 answer of a save with the Target Shares of the Tiers that add up to 90.
export const tiersRejected = {
  error: "the configuration of the Tiers has 2 error(s), nothing is saved",
  errors: [
    {
      tierIndex: null,
      ticker: "",
      message: "The Target Shares of the Tiers add up to 90, not to 100.",
    },
    {
      tierIndex: 0,
      ticker: "",
      message:
        "Tier 1. Global Market Core: the Instrument Targets add up to 110, more than 100.",
    },
  ],
}

// ApiFixture is the set of answers of the fake backend for one data set.
// populatedRebalancing is a Rebalancing table with a Planned Purchase on each Account, a negative Gap, a Holding with
// no Tier and a Holding with no price.
const populatedRebalancing: Rebalancing = {
  available: true,
  warnings: [],
  exchangeRate: { currency: "USD", rate: "3.6512", date: today },
  accounts: [
    {
      accountId: 1,
      name: "Main Account",
      currency: "PLN",
      freeCash: "4200.00",
      cash: "4200.00",
      plannedPurchases: "3000.00",
      unusedCash: "1200.00",
    },
    {
      accountId: 5,
      name: "Main Account USD",
      currency: "USD",
      freeCash: "1500.00",
      cash: "1500.00",
      plannedPurchases: "1500.00",
      unusedCash: "0.00",
    },
  ],
  missingAccounts: [],
  value: "48200.00",
  base: "58300.00",
  tiers: [
    {
      order: 1,
      name: "Global Market Core",
      value: "24100.4872",
      actualShare: "41.3366",
      targetShare: "60",
      targetValue: "34980.00",
      gap: "10879.51",
      plannedPurchases: [
        { accountId: 1, currency: "PLN", amount: "3000.00" },
        { accountId: 5, currency: "USD", amount: "1500.00" },
      ],
      instrumentTargets: [
        {
          ticker: "ALPHA.PL",
          name: "Alpha Retail S.A.",
          noValue: false,
          value: "9800.00",
          actualShare: "40.6624",
          targetShare: "60",
          targetValue: "20988.00",
          gap: "11188.00",
          plannedPurchase: { accountId: 1, currency: "PLN", amount: "3000.00" },
        },
      ],
      sharedPart: {
        name: "Blue Chips",
        value: "14300.4872",
        actualShare: "59.3376",
        targetShare: "40",
        targetValue: "13992.00",
        gap: "-308.49",
        plannedPurchase: { accountId: 5, currency: "USD", amount: "1500.00" },
        instruments: [
          {
            ticker: "BETA.US",
            name: "Beta Software Inc.",
            noValue: false,
            value: "14300.49",
            share: "100",
          },
        ],
      },
    },
    {
      order: 2,
      name: "Asymmetric Bets",
      value: "20100.00",
      actualShare: "34.4768",
      targetShare: "40",
      targetValue: "23320.00",
      gap: "3220.00",
      plannedPurchases: [{ accountId: 5, currency: "USD", amount: "0.00" }],
      instrumentTargets: [],
      sharedPart: {
        name: "",
        value: "20100.00",
        actualShare: "100",
        targetShare: "100",
        targetValue: "23320.00",
        gap: "3220.00",
        plannedPurchase: { accountId: 5, currency: "USD", amount: "0.00" },
        instruments: [],
      },
    },
  ],
  noTier: {
    value: "4000.00",
    actualShare: "6.8611",
    targetShare: "0",
    targetValue: "0.00",
    gap: "-4000.00",
    holdings: [
      {
        ticker: "GAMMA.PL",
        name: "Gamma Energy S.A.",
        noValue: false,
        value: "4000.00",
        share: "100",
      },
    ],
  },
  unpricedHoldings: [
    { accountId: 1, ticker: "DELTA.PL", name: "Delta Bond Fund" },
  ],
}

// emptyRebalancing is the table before the user creates a Tier.
const emptyRebalancing: Rebalancing = {
  available: true,
  warnings: [],
  exchangeRate: populatedRebalancing.exchangeRate,
  accounts: [],
  missingAccounts: ["PLN", "USD"],
  value: "0.00",
  base: "0.00",
  tiers: [],
  noTier: null,
  unpricedHoldings: [],
}

export type ApiFixture = {
  accounts: Account[]
  allSummary: AllAccountsSummary
  allAllocation: Allocation
  allHistory: AllAccountsValueHistory
  summary: Record<number, AccountSummary>
  allocation: Record<number, Allocation>
  history: Record<number, AccountValueHistory>
  benchmarks: Record<number, AccountBenchmarks>
  allBenchmarks: Benchmarks
  holdings: Record<number, Holding[]>
  closedPositions: Record<number, ClosedPosition[]>
  ikzeLimit: Record<number, IkzeLimit>
  overview: Record<string, InstrumentOverview>
  imports: ImportSummary[]
  tiers: Tiers
  rebalancing: Rebalancing
}

// fixtureFor gives the answers of the fake backend for a data set.
export function fixtureFor(dataSet: DataSet): ApiFixture {
  if (dataSet === "krakenRewardUnknown") {
    // The data set "krakenRewardUnknown" is the data set "kraken" with a Reward that has no stored close.
    const kraken = fixtureFor("kraken")
    return {
      ...kraken,
      // The Holding has a value, so the Allocation is the same as in the data set "kraken".
      summary: { ...kraken.summary, 3: krakenRewardUnknownSummary },
      holdings: { ...kraken.holdings, 3: [bitcoinRewardUnknown] },
    }
  }
  if (dataSet === "kraken") {
    // The data set "kraken" is the populated data set with the Kraken Account in addition.
    const populated = fixtureFor("populated")
    return {
      ...populated,
      accounts: [...populated.accounts, krakenAccount].sort((a, b) =>
        a.name.localeCompare(b.name)
      ),
      summary: { ...populated.summary, 3: krakenSummary },
      allocation: { ...populated.allocation, 3: krakenAllocation },
      history: { ...populated.history, 3: krakenHistory },
      benchmarks: {
        ...populated.benchmarks,
        3: accountBenchmarks(krakenHistory),
      },
      holdings: { ...populated.holdings, 3: [bitcoin] },
      closedPositions: { ...populated.closedPositions, 3: [] },
      overview: { ...populated.overview, "3:BTC": bitcoinOverview },
    }
  }
  if (
    dataSet === "populated" ||
    dataSet === "nearEqualEnd" ||
    dataSet === "benchmarkUnavailable"
  ) {
    return {
      accounts: [ikzeAccount, mainAccount].sort((a, b) =>
        a.name.localeCompare(b.name)
      ),
      allSummary,
      allAllocation,
      allHistory,
      summary: { 1: mainSummary, 2: ikzeSummary },
      allocation: { 1: mainAllocation, 2: ikzeAllocation },
      history: {
        1: dataSet === "nearEqualEnd" ? nearEqualHistory : mainHistory,
        2: ikzeHistory,
      },
      benchmarks: {
        1: accountBenchmarks(mainHistory, dataSet === "benchmarkUnavailable"),
        2: accountBenchmarks(ikzeHistory),
      },
      allBenchmarks: allAccountsBenchmarks,
      holdings: { 1: mainHoldings, 2: ikzeHoldings },
      closedPositions: { 1: mainClosed, 2: [] },
      ikzeLimit: { 2: ikzeLimit },
      overview: { "1:BETA.US": betaOverview },
      imports,
      tiers: populatedTiers,
      rebalancing: populatedRebalancing,
    }
  }
  const emptyAllHistory: AllAccountsValueHistory = {
    ...emptyHistory(0),
    available: true,
    warnings: [],
  }
  return {
    accounts:
      dataSet === "noAccounts"
        ? []
        : [ikzeAccount, mainAccount].sort((a, b) =>
            a.name.localeCompare(b.name)
          ),
    allSummary: emptyAllSummary,
    allAllocation: emptyAllocation("PLN"),
    allHistory: emptyAllHistory,
    summary: { 1: emptySummary(1), 2: emptySummary(2) },
    allocation: { 1: emptyAllocation("PLN"), 2: emptyAllocation("PLN") },
    history: { 1: emptyHistory(1), 2: emptyHistory(2) },
    benchmarks: { 1: emptyBenchmarks(1), 2: emptyBenchmarks(2) },
    allBenchmarks: emptyAllBenchmarks,
    holdings: { 1: [], 2: [] },
    closedPositions: { 1: [], 2: [] },
    ikzeLimit: { 2: { currentYear: 2026, rows: [] } },
    overview: {},
    imports: [],
    tiers: emptyTiers,
    rebalancing: emptyRebalancing,
  }
}

// previewOperation is a Cash Operation of the Import conflicts.
const previewOperation = {
  type: "Deposit",
  time: "2026-05-04T09:12:00Z",
  amount: "1500.00",
  comment: "Synthetic deposit",
  ticker: "",
  positionId: "",
  volume: null,
  price: null,
}

// importPreviews are the answers of an upload. "known" is a file of the Main Account with an Import Conflict and a warning.
// "new" is a file with a new account number. "kraken" is the first Kraken ledger export, with the suggested Account values.
export const importPreviews: Record<"known" | "new" | "kraken", ImportPreview> =
  {
    known: {
      id: 3,
      fileName: "account_10000001_2026.xlsx",
      accountNumber: "10000001",
      account: mainAccount,
      provider: "xtb",
      suggestedName: "",
      suggestedType: "",
      suggestedCurrency: "PLN",
      newCashOperations: 12,
      skippedCashOperations: 148,
      state: "needs-review",
      conflicts: [
        {
          id: 1,
          kind: "changed-data",
          xtbId: "900001",
          existing: previewOperation,
          new: { ...previewOperation, amount: "1550.00" },
          check: null,
          decision: null,
        },
      ],
      warnings: [
        {
          kind: "transfer-without-other-side",
          direction: "out",
          xtbId: "900002",
          time: "2026-05-20T10:00:00Z",
          amount: "-500.00",
        },
      ],
    },
    new: {
      id: 4,
      fileName: "account_10000003_2026.xlsx",
      accountNumber: "10000003",
      account: null,
      provider: "xtb",
      suggestedName: "",
      suggestedType: "",
      suggestedCurrency: "PLN",
      newCashOperations: 30,
      skippedCashOperations: 0,
      state: "preview",
      conflicts: [],
      warnings: [],
    },
    kraken: {
      id: 5,
      fileName: "kraken_spot_ledgers_2026-01-01-2026-06-01.csv",
      accountNumber: "kraken-spot",
      account: null,
      provider: "kraken",
      suggestedName: "Kraken",
      suggestedType: "regular",
      suggestedCurrency: "EUR",
      newCashOperations: 7,
      skippedCashOperations: 0,
      state: "preview",
      conflicts: [],
      warnings: [],
    },
  }

// unsupportedEntries are the entries of a Kraken export that stop an Import: a sale, a withdrawal and a staking move.
const unsupportedEntries: UnsupportedEntry[] = [
  {
    type: "trade",
    subtype: "tradespot",
    asset: "BTC",
    time: "2026-04-14T08:21:00Z",
    reason: "sale",
  },
  {
    type: "withdrawal",
    subtype: "",
    asset: "BTC",
    time: "2026-05-02T17:45:12Z",
    reason: "withdrawal",
  },
  {
    type: "earn",
    subtype: "allocation",
    asset: "BTC",
    time: "2026-05-20T06:00:00Z",
    reason: "unknown entry",
  },
]

// unsupportedEntriesError is the body of the HTTP 400 answer to an upload of a Kraken export with unsupported entries.
export const unsupportedEntriesError = {
  error:
    "the export has entries that the tool does not support: sale at 2026-04-14T08:21:00Z; withdrawal at 2026-05-02T17:45:12Z; unknown entry at 2026-05-20T06:00:00Z",
  code: "unsupported-entries",
  unsupportedEntries,
}
