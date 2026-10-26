// accountPattern is the route pattern of the pages of one Account.
export const accountPattern = "/accounts/:id"

// paths holds the URL of each view of the tool.
export const paths = {
  root: "/",
  allAccounts: "/accounts/all",
  account: (id: number) => `/accounts/${id}`,
  holdings: (id: number) => `/accounts/${id}/holdings`,
  sales: (id: number) => `/accounts/${id}/sales`,
  ikze: (id: number) => `/accounts/${id}/ikze`,
  import: "/import",
  imports: "/imports",
} as const

// parseAccountId gives the Account id of a URL segment, or null when the segment is not an id.
export function parseAccountId(segment: string | undefined): number | null {
  if (segment === undefined || !/^[1-9][0-9]*$/.test(segment)) return null
  return Number(segment)
}
