import type { Account } from "./api"
import { periodParam } from "./benchmark-period"
import { isTierAccount } from "./provider"

// accountPattern is the route pattern of the pages of one Account.
export const accountPattern = "/accounts/:id"

// paths holds the URL of each view of the tool.
export const paths = {
  root: "/",
  allAccounts: "/accounts/all",
  account: (id: number) => `/accounts/${id}`,
  holdings: (id: number) => `/accounts/${id}/holdings`,
  closedPositions: (id: number) => `/accounts/${id}/closed-positions`,
  tiersBreakdown: (id: number) => `/accounts/${id}/tiers-breakdown`,
  benchmarks: (id: number) => `/accounts/${id}/benchmarks`,
  allBenchmarks: "/accounts/all/benchmarks",
  allTiersBreakdown: "/accounts/all/tiers-breakdown",
  rebalancing: "/accounts/all/rebalancing",
  ikze: (id: number) => `/accounts/${id}/ikze`,
  import: "/import",
  imports: "/imports",
  // importCashOperations opens the Import history with the Cash Operations of one Import.
  importCashOperations: (id: number) => `/imports?import=${id}`,
} as const

// parseAccountId gives the Account id of a URL segment, or null when the segment is not an id.
export function parseAccountId(segment: string | undefined): number | null {
  if (segment === undefined || !/^[1-9][0-9]*$/.test(segment)) return null
  return Number(segment)
}

// allSegment is the URL segment that names the view of all Accounts, in place of an Account id.
const allSegment = "all"

// isAllAccountsPath tells whether a path is in the view of all Accounts, for example the overview or the Benchmarks.
export function isAllAccountsPath(pathname: string): boolean {
  const parts = pathname.replace(/\/+$/, "").split("/")
  return parts[1] === "accounts" && parts[2] === allSegment
}

// ViewTarget is a view of the view switch: all Accounts, or one Account with the data that decides its pages.
export type ViewTarget =
  | { kind: "all" }
  | {
      kind: "account"
      account: Pick<Account, "id" | "type" | "provider" | "currency">
    }

// switchViewPath gives the URL that opens when the user changes the view. The same page type opens when the new
// view has it, otherwise the overview opens. The query string is dropped, because filters and the Instrument
// Overview name data of the old view. The Benchmarks page keeps the period of the query string.
export function switchViewPath(
  pathname: string,
  target: ViewTarget,
  search = ""
): string {
  const parts = pathname.replace(/\/+$/, "").split("/")
  const period = new URLSearchParams(search).get(periodParam)
  const query =
    period === null ? "" : `?${new URLSearchParams({ [periodParam]: period })}`
  const onBenchmarks =
    parts[1] === "accounts" && parts[3] === "benchmarks" && parts.length === 4
  const onTiersBreakdown =
    parts[1] === "accounts" && parts[3] === "tiers-breakdown"
  if (target.kind === "all") {
    if (onBenchmarks) return `${paths.allBenchmarks}${query}`
    return onTiersBreakdown ? paths.allTiersBreakdown : paths.allAccounts
  }
  const { id, type } = target.account
  if (onBenchmarks) return `${paths.benchmarks(id)}${query}`
  const onAccountPage =
    parts[1] === "accounts" && parseAccountId(parts[2]) !== null
  const page = onAccountPage ? parts[3] : undefined
  if (onTiersBreakdown && isTierAccount(target.account))
    return paths.tiersBreakdown(id)
  if (page === "holdings") return paths.holdings(id)
  if (page === "closed-positions") return paths.closedPositions(id)
  if (page === "ikze" && type === "IKZE") return paths.ikze(id)
  return paths.account(id)
}
