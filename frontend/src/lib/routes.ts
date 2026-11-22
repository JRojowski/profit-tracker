import type { Account } from "./api"

// accountPattern is the route pattern of the pages of one Account.
export const accountPattern = "/accounts/:id"

// paths holds the URL of each view of the tool.
export const paths = {
  root: "/",
  allAccounts: "/accounts/all",
  account: (id: number) => `/accounts/${id}`,
  holdings: (id: number) => `/accounts/${id}/holdings`,
  closedPositions: (id: number) => `/accounts/${id}/closed-positions`,
  benchmarks: (id: number) => `/accounts/${id}/benchmarks`,
  ikze: (id: number) => `/accounts/${id}/ikze`,
  import: "/import",
  imports: "/imports",
} as const

// parseAccountId gives the Account id of a URL segment, or null when the segment is not an id.
export function parseAccountId(segment: string | undefined): number | null {
  if (segment === undefined || !/^[1-9][0-9]*$/.test(segment)) return null
  return Number(segment)
}

// ViewTarget is a view of the view switch: all Accounts, or one Account with the data that decides its pages.
export type ViewTarget =
  { kind: "all" } | { kind: "account"; account: Pick<Account, "id" | "type"> }

// switchViewPath gives the URL that opens when the user changes the view. The same page type opens when the new
// view has it, otherwise the overview opens. The query string is dropped, because filters and the Instrument
// Overview name data of the old view.
export function switchViewPath(pathname: string, target: ViewTarget): string {
  if (target.kind === "all") return paths.allAccounts
  const { id, type } = target.account
  const parts = pathname.replace(/\/+$/, "").split("/")
  const onAccountPage =
    parts[1] === "accounts" && parseAccountId(parts[2]) !== null
  const page = onAccountPage ? parts[3] : undefined
  if (page === "holdings") return paths.holdings(id)
  if (page === "closed-positions") return paths.closedPositions(id)
  if (page === "benchmarks") return paths.benchmarks(id)
  if (page === "ikze" && type === "IKZE") return paths.ikze(id)
  return paths.account(id)
}
