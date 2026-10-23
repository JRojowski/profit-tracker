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
  // totalResult is the sum of the parts below. Unrealized Profit joins it in a later version.
  totalResult: string
  realizedProfit: string
  netDividends: string
  netInterest: string
  fees: string
}

// Holding holds all open Lots of one Instrument. Volumes and amounts are decimal text.
export type Holding = {
  ticker: string
  name: string
  volume: string
  cost: string
  averageCost: string
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
