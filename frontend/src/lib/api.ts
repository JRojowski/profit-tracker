export type Health = {
  status: string
}

export const accountTypes = ["regular", "IKE", "IKZE"] as const

export type AccountType = (typeof accountTypes)[number]

export type Account = {
  id: number
  accountNumber: string
  name: string
  type: AccountType
  currency: string
}

export type NewAccount = {
  name: string
  type: AccountType
  currency: string
}

// ImportState "needs-review" tells that the Import has Import Conflicts.
export type ImportState = "preview" | "needs-review"

// ConflictDecision "keep-existing" and "use-new" are for a "changed-data" conflict.
// "accept" is for a "check-mismatch" conflict. It saves the Import with the difference.
export type ConflictDecision = "keep-existing" | "use-new" | "accept"

// CashOperationData is the data of a Cash Operation that an Import Conflict compares.
// The time is RFC 3339 text in UTC. The amount is decimal text in the Account currency.
// The ticker and the position ID are "" for an operation with no Instrument or no Lot.
// The volume and the price are decimal text, or null for an operation with no volume.
export type CashOperationData = {
  type: string
  time: string
  amount: string
  comment: string
  ticker: string
  positionId: string
  volume: string | null
  price: string | null
}

// ProfitCheck compares the Realized Profit of a Closed Position with the Profit/Loss in Closed Positions.
// The values are decimal text in the Account currency.
export type ProfitCheck = {
  calculated: string
  closedPositions: string
}

// ImportConflict is a Cash Operation that needs a decision. The decision is null until the user selects one.
// A "changed-data" conflict is a known XTB ID with different data.
// A "check-mismatch" conflict is a Closed Position with a Realized Profit that differs from Closed Positions by more than 0.01.
export type ImportConflict =
  | {
      id: number
      kind: "changed-data"
      xtbId: string
      existing: CashOperationData
      new: CashOperationData
      check: null
      decision: ConflictDecision | null
    }
  | {
      id: number
      kind: "check-mismatch"
      xtbId: string
      // existing is null, because the check is for a new Closed Position. new holds the data of the Closed Position.
      existing: null
      new: CashOperationData
      check: ProfitCheck
      decision: ConflictDecision | null
    }

// ImportWarning tells the user about a possible problem. It does not stop the Import.
// The kind "transfer-without-other-side" is a Transfer with no other side in the other Accounts (ADR 0003).
export type ImportWarning = {
  kind: "transfer-without-other-side"
  direction: "in" | "out"
  xtbId: string
  time: string
  amount: string
}

// ImportPreview is the result of an upload. The backend saves no Cash Operations before the confirmation.
// The account is null when the account number is new.
export type ImportPreview = {
  id: number
  fileName: string
  accountNumber: string
  account: Account | null
  suggestedCurrency: string
  newCashOperations: number
  skippedCashOperations: number
  state: ImportState
  conflicts: ImportConflict[]
  warnings: ImportWarning[]
}

export type ImportSummary = {
  id: number
  fileName: string
  importedAt: string
  accountId: number
  accountNumber: string
  accountName: string
  newCashOperations: number
  skippedCashOperations: number
}

// ApiError holds the error message that the backend returns. The code is the machine-readable code of the
// error, when the backend gives one, for example "rate_limited".
export class ApiError extends Error {
  readonly status: number
  readonly code: string | null

  constructor(status: number, message: string, code: string | null = null) {
    super(message)
    this.name = "ApiError"
    this.status = status
    this.code = code
  }
}

// errorMessage gives the text of a caught error.
export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `HTTP ${response.status}`
    let code: string | null = null
    try {
      const body = (await response.json()) as { error?: string; code?: string }
      if (body.error) message = body.error
      if (body.code) code = body.code
    } catch {
      // The body is not JSON. Keep the status message.
    }
    throw new ApiError(response.status, message, code)
  }
  return (await response.json()) as T
}

export async function fetchHealth(signal?: AbortSignal): Promise<Health> {
  const response = await fetch("/api/health", { signal })
  if (!response.ok) {
    throw new Error(`health check failed: HTTP ${response.status}`)
  }
  return (await response.json()) as Health
}

export async function uploadImport(file: File): Promise<ImportPreview> {
  const body = new FormData()
  body.append("file", file)
  const response = await fetch("/api/imports", { method: "POST", body })
  return readJSON<ImportPreview>(response)
}

// confirmImport saves the Import. Send account only for a new account number.
export async function confirmImport(
  id: number,
  account?: NewAccount
): Promise<ImportSummary> {
  const response = await fetch(`/api/imports/${id}/confirm`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(account ? { account } : {}),
  })
  return readJSON<ImportSummary>(response)
}

// decideConflict records the decision for one Import Conflict and returns the updated conflict.
export async function decideConflict(
  importId: number,
  conflictId: number,
  decision: ConflictDecision
): Promise<ImportConflict> {
  const response = await fetch(
    `/api/imports/${importId}/conflicts/${conflictId}`,
    {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ decision }),
    }
  )
  return readJSON<ImportConflict>(response)
}

// cancelImport cancels an Import in preview. Nothing changes in the Ledger.
export async function cancelImport(id: number): Promise<void> {
  const response = await fetch(`/api/imports/${id}/cancel`, { method: "POST" })
  if (!response.ok) {
    await readJSON<never>(response)
  }
}

export async function fetchImports(
  signal?: AbortSignal
): Promise<ImportSummary[]> {
  const response = await fetch("/api/imports", { signal })
  return readJSON<ImportSummary[]>(response)
}

// AccountSummary holds the values that the backend calculates from the Ledger.
// Amounts are decimal text in the Account currency.
export type AccountSummary = {
  accountId: number
  currency: string
  cash: string
  netDeposits: string
  // value is the cash plus the value of the Holdings that have a price.
  value: string
  // totalResult is the sum of the parts below.
  totalResult: string
  realizedProfit: string
  unrealizedProfit: string
  netDividends: string
  netInterest: string
  fees: string
  // priceDate is the oldest price date of the priced Holdings, as YYYY-MM-DD. It is null when no Holding has a price.
  priceDate: string | null
  // stalePrices tells that a Holding with a known Quote Currency has no price, or that its price is not from today.
  stalePrices: boolean
  // exchangeRateDate is the oldest date of the Exchange Rates that the Holdings use, as YYYY-MM-DD.
  // It is null when no Holding uses an Exchange Rate.
  exchangeRateDate: string | null
  // staleRates tells that a Holding needs an Exchange Rate that is missing or that the tool did not fetch today.
  staleRates: boolean
  // unpricedHoldings is the count of Holdings with no value.
  unpricedHoldings: number
} & Returns

// Returns holds the Simple Return and the Annual Return as decimal percentages, for example "8.85" for 8.85 %.
// A return is null when the tool cannot calculate it. Then its note tells the user why.
export type Returns = {
  simpleReturn: string | null
  simpleReturnNote: string | null
  annualReturn: string | null
  annualReturnNote: string | null
}

// Holding holds all open Lots of one Instrument. Volumes and amounts are decimal text.
// The cost, the value and the Unrealized Profit are in the Account currency. The price is in the Quote Currency.
// The price fields are null when the Holding has no value. Then note tells the user why.
export type Holding = {
  ticker: string
  name: string
  volume: string
  cost: string
  averageCost: string
  price: string | null
  // priceDate is YYYY-MM-DD.
  priceDate: string | null
  // exchangeRate is the NBP rate in PLN for 1 unit of the currency that is not PLN.
  // It is null when the value needs no conversion.
  exchangeRate: string | null
  // exchangeRateCurrency is the currency of exchangeRate: the one of the Quote Currency and the Account currency
  // that is not PLN. It is null when exchangeRate is null.
  exchangeRateCurrency: string | null
  // exchangeRateDate is the NBP table date of exchangeRate, as YYYY-MM-DD.
  exchangeRateDate: string | null
  value: string | null
  unrealizedProfit: string | null
  // unrealizedProfitPercent is the Unrealized Profit as a percentage of the cost, for example "8.85" for 8.85 %.
  unrealizedProfitPercent: string | null
  note: string | null
  // priceWarnings are the Price Warnings of the Instrument. The list is empty when the Instrument has none.
  priceWarnings: PriceWarning[]
} & PriceSource

// PriceFailure is one Instrument that got no new price. The ticker is the broker ticker.
export type PriceFailure = {
  ticker: string
  error: string
}

// RateFailure is one currency that got no new Exchange Rate.
export type RateFailure = {
  currency: string
  error: string
}

// PriceRefresh is the result of a refresh of the prices and the Exchange Rates.
export type PriceRefresh = {
  refreshed: number
  failures: PriceFailure[]
  rateFailures: RateFailure[]
  // historyStored is the number of daily prices that the refresh stored.
  historyStored: number
  // historyFailures are the Instruments that got no new daily prices.
  historyFailures: PriceFailure[]
  // tooManyRequests tells that the price source stopped the refresh with HTTP 429.
  tooManyRequests: boolean
  // stopMessage tells why the refresh stopped early, or is null.
  stopMessage: string | null
  // priceWarnings are the Price Warnings of each Instrument that an Account ever held, after the refresh.
  priceWarnings: PriceWarning[]
}

// RefreshProgress is the progress of a running price refresh. total is 0 during the step of the current prices.
export type RefreshProgress = {
  running: boolean
  done: number
  total: number
  // ticker is the broker ticker that the refresh gets now, or null.
  ticker: string | null
}

// ClosedPosition is one Closed Position with its Realized Profit. The time is RFC 3339 text in UTC.
// Volumes and amounts are decimal text in the Account currency.
// openTime is the time of the earliest Fill of the Lot. openPrice and closePrice are decimal text, or null when a price is absent.
// The prices are in the currency of the broker price, which the response does not state.
export type ClosedPosition = {
  xtbId: string
  time: string
  ticker: string
  name: string
  positionId: string
  volume: string
  amount: string
  cost: string
  realizedProfit: string
  openTime: string
  openPrice: string | null
  closePrice: string | null
}

export async function fetchAccounts(signal?: AbortSignal): Promise<Account[]> {
  const response = await fetch("/api/accounts", { signal })
  return readJSON<Account[]>(response)
}

export async function renameAccount(
  id: number,
  name: string
): Promise<Account> {
  const response = await fetch(`/api/accounts/${id}`, {
    method: "PATCH",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ name }),
  })
  return readJSON<Account>(response)
}

export async function fetchAccountSummary(
  id: number,
  signal?: AbortSignal
): Promise<AccountSummary> {
  const response = await fetch(`/api/accounts/${id}/summary`, { signal })
  return readJSON<AccountSummary>(response)
}

export async function fetchHoldings(
  id: number,
  signal?: AbortSignal
): Promise<Holding[]> {
  const response = await fetch(`/api/accounts/${id}/holdings`, { signal })
  return readJSON<Holding[]>(response)
}

// OverviewPrice is one daily close of the price series of an Instrument Overview. date is YYYY-MM-DD.
export type OverviewPrice = { date: string; close: string }

// OverviewFill is one Fill of the Instrument. price is null when the Fill has no price.
// It is in the currency of the broker price, which can differ from the currency of the price series.
export type OverviewFill = {
  xtbId: string
  // time is an RFC 3339 time in UTC.
  time: string
  volume: string
  price: string | null
  positionId: string
}

// OverviewLot is one open Lot of the Instrument. volume and cost are the open part of the Lot.
export type OverviewLot = {
  positionId: string
  volume: string
  cost: string
  // openTime is an RFC 3339 time in UTC: the time of the first Fill.
  openTime: string
  // openPrice is the average Fill price of the whole Lot, or null when a Fill has no price.
  openPrice: string | null
  fills: OverviewFill[]
}

// InstrumentOverview is the Instrument Overview of one Instrument on one Account.
// The series is in ascending date order and can have gaps. currency is null exactly when the series is empty.
export type InstrumentOverview = NamedPriceSource & {
  prices: { currency: string | null; series: OverviewPrice[] }
  // fills are in ascending time order.
  fills: OverviewFill[]
  // holding is null when the Account has no open volume of the Instrument.
  holding: Holding | null
  // openLots is empty exactly when holding is null.
  openLots: OverviewLot[]
  // realizedProfitSum is the sum of the Realized Profit of the closedPositions, in the Account currency.
  // The backend adds the amounts. It is null exactly when closedPositions is empty.
  realizedProfitSum: string | null
  // closedPositions are in close time order, oldest first.
  closedPositions: ClosedPosition[]
}

// fetchInstrumentOverview loads the Instrument Overview of one Instrument on one Account.
// It gives null when the Account never held the Instrument.
export async function fetchInstrumentOverview(
  id: number,
  ticker: string,
  signal?: AbortSignal
): Promise<InstrumentOverview | null> {
  const response = await fetch(
    `/api/accounts/${id}/instruments/${encodeURIComponent(ticker)}/overview`,
    { signal }
  )
  if (response.status === 404) return null
  return readJSON<InstrumentOverview>(response)
}

export async function fetchClosedPositions(
  id: number,
  signal?: AbortSignal
): Promise<ClosedPosition[]> {
  const response = await fetch(`/api/accounts/${id}/closed-positions`, {
    signal,
  })
  return readJSON<ClosedPosition[]>(response)
}

// refreshPrices gets new prices for the Instruments in the Holdings of all Accounts and new Exchange Rates.
// It also gets the missing daily prices of each Instrument that an Account ever held.
// The first refresh can take some minutes.
export async function refreshPrices(): Promise<PriceRefresh> {
  const response = await fetch("/api/prices/refresh", { method: "POST" })
  return readJSON<PriceRefresh>(response)
}

// fetchRefreshProgress gets the progress of the running price refresh.
export async function fetchRefreshProgress(
  signal?: AbortSignal
): Promise<RefreshProgress> {
  const response = await fetch("/api/prices/refresh/progress", { signal })
  return readJSON<RefreshProgress>(response)
}

// ValueHistoryPoint is the value and the Net Deposits at the end of one calendar day, as decimal text.
export type ValueHistoryPoint = {
  // date is the calendar day, as YYYY-MM-DD.
  date: string
  value: string
  netDeposits: string
}

// historyPeriods are the periods of the Value History, from the shortest to the longest.
export const historyPeriods = ["1M", "3M", "YTD", "1Y", "All"] as const

// HistoryPeriod is one period of the Value History.
export type HistoryPeriod = (typeof historyPeriods)[number]

// ValuedInstrument is an Instrument that the Value History values at the cost of its Lots on some days.
export type ValuedInstrument = {
  ticker: string
  name: string
}

// ValueHistory is the Value History of one Account or of all Accounts, with one point for each calendar day.
export type ValueHistory = {
  currency: string
  points: ValueHistoryPoint[]
  // timeWeightedReturn is the cumulative Time-Weighted Return of each period, as a percentage, or null.
  timeWeightedReturn: Record<HistoryPeriod, string | null>
  // periodStarts is the date of the first point that the chart shows for each period, as YYYY-MM-DD.
  // It is null only when the Value History has no points.
  periodStarts: Record<HistoryPeriod, string | null>
  valuedAtCost: ValuedInstrument[]
  // priceWarnings are the Price Warnings of the Instruments of the history, also of the sold ones.
  priceWarnings: PriceWarning[]
}

// AccountValueHistory is the Value History of one Account, in the Account currency.
export type AccountValueHistory = ValueHistory & { accountId: number }

// fetchAccountValueHistory gets the Value History and the Time-Weighted Return of one Account.
export async function fetchAccountValueHistory(
  id: number,
  signal?: AbortSignal
): Promise<AccountValueHistory> {
  const response = await fetch(`/api/accounts/${id}/value-history`, {
    signal,
  })
  return readJSON<AccountValueHistory>(response)
}

// AllAccountsValueHistory is the Value History of all Accounts, in PLN by ADR 0004.
// When available is false, points is empty, each period is null and the warnings tell why.
export type AllAccountsValueHistory = ValueHistory & {
  available: boolean
  // warnings are sentences for the user about missing Exchange Rates.
  warnings: string[]
}

// fetchAllAccountsValueHistory gets the Value History and the Time-Weighted Return of all Accounts.
export async function fetchAllAccountsValueHistory(
  signal?: AbortSignal
): Promise<AllAccountsValueHistory> {
  const response = await fetch("/api/all-accounts/value-history", { signal })
  return readJSON<AllAccountsValueHistory>(response)
}

// PriceSource tells where the tool gets the price of an Instrument.
export type PriceSource = {
  // priceTicker is the ticker at the price source: the manual ticker, or the default mapping.
  priceTicker: string
  // priceTickerManual tells that the user set the price ticker.
  priceTickerManual: boolean
  // quoteCurrency is null when the tool does not know the quote currency of the Instrument.
  quoteCurrency: string | null
  // quoteCurrencyManual tells that the user set the quote currency.
  quoteCurrencyManual: boolean
  // priceConfirmed tells that the user confirmed that the price ticker and the Quote Currency are correct.
  // A change of the price ticker or of the Quote Currency removes the confirmation.
  priceConfirmed: boolean
}

// Instrument is the price source of one Instrument. The ticker is the broker ticker.
export type Instrument = { ticker: string } & PriceSource

// NamedPriceSource is the price source of one Instrument with its broker ticker and its name.
// A Holding and a Price Warning both have these fields.
export type NamedPriceSource = Instrument & { name: string }

// PriceWarningKind is the kind of a Price Warning: the price source has no data, gives a different currency,
// or gives prices that do not agree with the XTB prices.
export type PriceWarningKind = "noData" | "differentCurrency" | "priceMismatch"

// PriceWarning tells that the price ticker or the Quote Currency of an Instrument can be wrong.
// It holds the price source of the Instrument, so that the price source dialog can open also for a sold Instrument.
export type PriceWarning = NamedPriceSource & {
  kind: PriceWarningKind
  // sourceCurrency is the currency of the price source. It is set only for "differentCurrency".
  sourceCurrency: string | null
  // medianDifference and maxDifference are exact percentages as decimal text. The frontend rounds them for display.
  // They are set only for "priceMismatch".
  medianDifference: string | null
  maxDifference: string | null
  // confirmable tells that the user can confirm the price source to remove the warning. It is true only for "priceMismatch".
  confirmable: boolean
}

// setPriceConfirmation sets or removes the confirmation that the price ticker and the Quote Currency of the
// Instrument are correct. The confirmation removes only the Price Warning about a price mismatch.
export async function setPriceConfirmation(
  ticker: string,
  confirmed: boolean
): Promise<Instrument> {
  const response = await fetch(
    `/api/instruments/${encodeURIComponent(ticker)}/price-confirmation`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ confirmed }),
    }
  )
  return readJSON<Instrument>(response)
}

// setPriceTicker sets the manual price ticker of the Instrument. An empty value removes it.
export async function setPriceTicker(
  ticker: string,
  priceTicker: string
): Promise<Instrument> {
  const response = await fetch(
    `/api/instruments/${encodeURIComponent(ticker)}/price-ticker`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ priceTicker }),
    }
  )
  return readJSON<Instrument>(response)
}

// setQuoteCurrency sets the manual quote currency of the Instrument. An empty value removes it.
export async function setQuoteCurrency(
  ticker: string,
  quoteCurrency: string
): Promise<Instrument> {
  const response = await fetch(
    `/api/instruments/${encodeURIComponent(ticker)}/quote-currency`,
    {
      method: "PUT",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ quoteCurrency }),
    }
  )
  return readJSON<Instrument>(response)
}

// IkzeLimitYear is one calendar year of an IKZE Account, in Polish time. Amounts are decimal text in PLN.
// limit, unused and overLimit are null when the tool has no IKZE Limit for the year.
export type IkzeLimitYear = {
  year: number
  contributions: string
  limit: string | null
  // unused is the part of the limit that the IKZE Contributions do not use.
  unused: string | null
  // overLimit is the amount of the IKZE Contributions above the limit.
  overLimit: string | null
}

// IkzeLimit holds one row for each year from the first IKZE Contribution to currentYear, newest first.
export type IkzeLimit = {
  currentYear: number
  rows: IkzeLimitYear[]
}

// fetchIkzeLimit fails for an Account that is not of the type IKZE.
export async function fetchIkzeLimit(
  id: number,
  signal?: AbortSignal
): Promise<IkzeLimit> {
  const response = await fetch(`/api/accounts/${id}/ikze-limit`, { signal })
  return readJSON<IkzeLimit>(response)
}

// AllAccountsAmounts are the values of one Account or of all Accounts in PLN, by ADR 0004.
// An amount is null when the tool cannot convert it.
export type AllAccountsAmounts = {
  value: string | null
  cash: string | null
  netDeposits: string | null
  totalResult: string | null
  realizedProfit: string | null
  unrealizedProfit: string | null
  netDividends: string | null
  netInterest: string | null
  fees: string | null
  // currencyResult is the part of the Total Result that comes from the change of the Exchange Rates.
  currencyResult: string | null
}

// AllAccountsRow is one Account in PLN. When available is false, the amounts are null and note tells why.
// The Annual Return uses the Net Deposits flows of the Account in PLN, by ADR 0004. It is null when the tool
// cannot calculate it. Then annualReturnNote tells the user why.
export type AllAccountsRow = AllAccountsAmounts & {
  annualReturn: string | null
  annualReturnNote: string | null
  accountId: number
  name: string
  type: AccountType
  currency: string
  available: boolean
  // exchangeRate is the latest Exchange Rate of the Account currency. It is null for an Account in PLN.
  exchangeRate: string | null
  unpricedHoldings: number
  note: string | null
}

// AllAccountsSummary holds the totals of all Accounts in PLN. When available is false, the totals are null
// and warnings tell why.
// The returns of all Accounts use the Net Deposits flows in PLN, by ADR 0004.
export type AllAccountsSummary = AllAccountsAmounts &
  Returns & {
    available: boolean
    // stalePrices and staleRates have the meaning of the AccountSummary fields, over all Accounts.
    stalePrices: boolean
    staleRates: boolean
    // priceDate and exchangeRateDate are YYYY-MM-DD, or null.
    priceDate: string | null
    exchangeRateDate: string | null
    unpricedHoldings: number
    warnings: string[]
    accounts: AllAccountsRow[]
  }

export async function fetchAllAccountsSummary(
  signal?: AbortSignal
): Promise<AllAccountsSummary> {
  const response = await fetch("/api/all-accounts/summary", { signal })
  return readJSON<AllAccountsSummary>(response)
}

// AllocationPart is one part of the Allocation. The value is decimal text in the currency of the Allocation.
// The share is the exact percentage of the total, for example "33.3333333333333333". The frontend rounds it for display.
// The cash part has the ticker "" and the name "Cash".
export type AllocationPart = {
  ticker: string
  name: string
  value: string
  share: string
}

// AllocationOther holds the Holdings after the 8 largest, taken together as one part.
export type AllocationOther = {
  count: number
  value: string
  share: string
  holdings: AllocationPart[]
}

// AllocationCurrencyPart is the value of one Quote Currency in the Allocation.
export type AllocationCurrencyPart = {
  currency: string
  value: string
  share: string
}

// Allocation is the share of each part in the value of one Account or of all Accounts.
// When available is false, total, cash and other are null, the lists are empty and warnings tell why.
export type Allocation = {
  available: boolean
  warnings: string[]
  // currency is the Account currency, or "PLN" for all Accounts.
  currency: string
  total: string | null
  byInstrument: {
    // holdings are at most 8 parts, largest value first.
    holdings: AllocationPart[]
    // cash is null when the cash is zero or less.
    cash: AllocationPart | null
    // other is null when there are 8 Holdings or fewer.
    other: AllocationOther | null
  }
  byQuoteCurrency: AllocationCurrencyPart[]
  // unpricedHoldings is the count of Holdings with no known value. They are not parts.
  unpricedHoldings: number
}

// fetchAccountAllocation loads the Allocation of one Account, in the Account currency.
export async function fetchAccountAllocation(
  id: number,
  signal?: AbortSignal
): Promise<Allocation> {
  const response = await fetch(`/api/accounts/${id}/allocation`, { signal })
  return readJSON<Allocation>(response)
}

// fetchAllAccountsAllocation loads the Allocation of all Accounts, in PLN by ADR 0004.
export async function fetchAllAccountsAllocation(
  signal?: AbortSignal
): Promise<Allocation> {
  const response = await fetch("/api/all-accounts/allocation", { signal })
  return readJSON<Allocation>(response)
}

// AssistantMessage is one message of a conversation with the Assistant.
export type AssistantMessage = {
  role: "user" | "assistant"
  content: string
}

// AssistantStep is one call of a function by the model, with its arguments and the result that the model got.
export type AssistantStep = {
  function: string
  arguments: Record<string, unknown>
  // result is the JSON value that the function gave to the model. An older backend does not send it.
  result?: unknown
}

// AssistantContext is the page that the user has open when the user asks. Each field is optional.
export type AssistantContext = {
  // accountId is the Account of the page.
  accountId?: number
  // instrument is the broker ticker of the open Instrument Overview.
  instrument?: string
}

// AssistantAnswer is the answer of the Assistant and the steps that the model used.
export type AssistantAnswer = {
  answer: string
  steps: AssistantStep[]
}

// fetchAssistantAvailable tells whether the Assistant is on. It is off when the backend has no API key.
export async function fetchAssistantAvailable(
  signal?: AbortSignal
): Promise<boolean> {
  const response = await fetch("/api/assistant", { signal })
  const body = await readJSON<{ available: boolean }>(response)
  return body.available
}

// askAssistant sends the conversation and the page context to the Assistant. The last message is the new question.
// A context without a field is left out of the request.
export async function askAssistant(
  messages: AssistantMessage[],
  context: AssistantContext,
  signal?: AbortSignal
): Promise<AssistantAnswer> {
  const hasContext =
    context.accountId !== undefined || context.instrument !== undefined
  const response = await fetch("/api/assistant/questions", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(hasContext ? { messages, context } : { messages }),
    signal,
  })
  return readJSON<AssistantAnswer>(response)
}

// BenchmarkStatus tells if the tool can compare a Benchmark with the user for a period.
// noFullPeriod means that the tool has no price of the Benchmark for the whole period.
export type BenchmarkStatus = "available" | "noFullPeriod" | "sourceFailure"

// BenchmarkPoint is the cumulative return of one day, as a percentage in decimal text. The first day of a series is "0".
export type BenchmarkPoint = { date: string; return: string }

// BenchmarkReturn is the Benchmark Return of one Benchmark for a period, and its difference to the user.
export type BenchmarkReturn = {
  id: string
  name: string
  // fund names what represents the Benchmark: an ETF or ETC, or the spot price of a crypto asset.
  fund: string
  ticker: string
  quoteCurrency: string
  status: BenchmarkStatus
  // return is a percentage as decimal text. It is null unless the status is "available".
  return: string | null
  // difference is the return of the user minus the Benchmark Return, in percentage points as decimal text.
  difference: string | null
  // series has one point for each day from the base day to the last day. It is empty unless the status is "available".
  series: BenchmarkPoint[]
}

// Benchmarks is the comparison of the user with the Benchmarks for one period.
// When available is false, the days, the return of the user and the Benchmarks are empty.
export type Benchmarks = {
  period: HistoryPeriod
  // currency is the currency of the comparison.
  currency: string
  available: boolean
  warnings: string[]
  baseDay: string | null
  lastDay: string | null
  user: { return: string | null; series: BenchmarkPoint[] }
  benchmarks: BenchmarkReturn[]
}

// AccountBenchmarks is the comparison of one Account with the Benchmarks, in the Account currency.
export type AccountBenchmarks = Benchmarks & { accountId: number }

// fetchAccountBenchmarks gets the Benchmark Returns of one Account for one period.
export async function fetchAccountBenchmarks(
  id: number,
  period: HistoryPeriod,
  signal?: AbortSignal
): Promise<AccountBenchmarks> {
  const response = await fetch(
    `/api/accounts/${id}/benchmarks?period=${period}`,
    { signal }
  )
  return readJSON<AccountBenchmarks>(response)
}

// fetchAllAccountsBenchmarks gets the Benchmark Returns of all Accounts in PLN for one period.
export async function fetchAllAccountsBenchmarks(
  period: HistoryPeriod,
  signal?: AbortSignal
): Promise<Benchmarks> {
  const response = await fetch(
    `/api/all-accounts/benchmarks?period=${period}`,
    {
      signal,
    }
  )
  return readJSON<Benchmarks>(response)
}
