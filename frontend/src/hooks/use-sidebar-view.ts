import { useState } from "react"
import { useLocation, useMatch } from "react-router"

import type { Account } from "@/lib/api"
import { accountPattern, paths } from "@/lib/routes"

// SidebarView is the view that the sidebar shows. The sidebar has three states.
// "loading": the Accounts load. "view": All Accounts (account is null) or one Account.
// "unknown": the URL names an Account that does not exist.
export type SidebarView =
  | { state: "loading" }
  | { state: "view"; account: Account | null }
  | { state: "unknown" }

// useSidebarView gives the view that the sidebar shows for the URL.
// A page with no view (Import, Import history, a missing page) shows the last view of this session, or All Accounts.
export function useSidebarView(accounts: Account[] | null): SidebarView {
  const { pathname } = useLocation()
  const match = useMatch(`${accountPattern}/*`)
  // lastId is the id of the last Account view of this session, or null for All Accounts.
  const [lastId, setLastId] = useState<number | null>(null)

  if (accounts === null) return { state: "loading" }

  const path = pathname.replace(/\/+$/, "")
  if (
    path === "" ||
    path === paths.allAccounts ||
    path === paths.allBenchmarks
  ) {
    if (lastId !== null) setLastId(null)
    return { state: "view", account: null }
  }
  if (match) {
    const account = accounts.find((a) => String(a.id) === match.params.id)
    if (account === undefined) return { state: "unknown" }
    if (lastId !== account.id) setLastId(account.id)
    return { state: "view", account }
  }
  const last = accounts.find((a) => a.id === lastId) ?? null
  return { state: "view", account: last }
}
