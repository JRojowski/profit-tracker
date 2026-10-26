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

// ProfitCheck compares the Realized Profit of a sale with the Profit/Loss in Closed Positions.
// The values are decimal text in the Account currency.
export type ProfitCheck = {
  calculated: string
  closedPositions: string
}

// ImportConflict is a Cash Operation that needs a decision. The decision is null until the user selects one.
// A "changed-data" conflict is a known XTB ID with different data.
// A "check-mismatch" conflict is a sale with a Realized Profit that differs from Closed Positions by more than 0.01.
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
      // existing is null, because the check is for a new sale. new holds the data of the sale.
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

// ApiError holds the error message that the backend returns.
export class ApiError extends Error {
  readonly status: number

  constructor(status: number, message: string) {
    super(message)
    this.name = "ApiError"
    this.status = status
  }
}

// errorMessage gives the text of a caught error.
export function errorMessage(err: unknown): string {
  return err instanceof Error ? err.message : String(err)
}

async function readJSON<T>(response: Response): Promise<T> {
  if (!response.ok) {
    let message = `HTTP ${response.status}`
    try {
      const body = (await response.json()) as { error?: string }
      if (body.error) message = body.error
    } catch {
      // The body is not JSON. Keep the status message.
    }
    throw new ApiError(response.status, message)
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
}

// Sale is one sale with its Realized Profit. The time is RFC 3339 text in UTC.
// Volumes and amounts are decimal text in the Account currency.
export type Sale = {
  xtbId: string
  time: string
  ticker: string
  name: string
  positionId: string
  volume: string
  amount: string
  cost: string
  realizedProfit: string
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

export async function fetchSales(
  id: number,
  signal?: AbortSignal
): Promise<Sale[]> {
  const response = await fetch(`/api/accounts/${id}/sales`, { signal })
  return readJSON<Sale[]>(response)
}

// refreshPrices gets new prices for the Instruments in the Holdings of all Accounts, and new Exchange Rates.
export async function refreshPrices(): Promise<PriceRefresh> {
  const response = await fetch("/api/prices/refresh", { method: "POST" })
  return readJSON<PriceRefresh>(response)
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
}

// Instrument is the price source of one Instrument. The ticker is the broker ticker.
export type Instrument = { ticker: string } & PriceSource

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
