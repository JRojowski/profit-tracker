import { useMatch } from "react-router"

import type { Account } from "@/lib/api"
import { accountPattern, parseAccountId } from "@/lib/routes"

// useRouteAccount gives the Account of the URL, or null when the URL has no known Account.
export function useRouteAccount(accounts: Account[] | null): Account | null {
  const match = useMatch(`${accountPattern}/*`)
  const id = parseAccountId(match?.params.id)
  if (id === null || accounts === null) return null
  return accounts.find((account) => account.id === id) ?? null
}
