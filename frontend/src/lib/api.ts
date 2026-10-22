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
}

// Holding holds all open Lots of one Instrument. Volumes and amounts are decimal text.
export type Holding = {
  ticker: string
  name: string
  volume: string
  cost: string
  averageCost: string
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
