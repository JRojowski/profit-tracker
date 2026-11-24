import { Navigate, Outlet, useLocation, useOutletContext } from "react-router"

import { AccountNotFound } from "@/components/account-not-found"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { useRouteAccount } from "@/hooks/use-route-account"
import type { Account } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { isTierAccount } from "@/lib/provider"
import { paths } from "@/lib/routes"

// AccountLayout gives the Account of the URL to the page of the Account. The sidebar holds the navigation.
export function AccountLayout() {
  const { accounts, accountsError } = useAppState()
  const account = useRouteAccount(accounts)
  // A trailing slash opens the same page.
  const pathname = useLocation().pathname.replace(/\/+$/, "")

  if (accountsError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Cannot load the Accounts</AlertTitle>
        <AlertDescription>{accountsError}</AlertDescription>
      </Alert>
    )
  }
  if (accounts === null) return <Skeleton className="h-8 w-64" />
  if (account === null) return <AccountNotFound />

  // Only an IKZE Account has the IKZE page.
  if (pathname === paths.ikze(account.id) && account.type !== "IKZE") {
    return <Navigate to={paths.account(account.id)} replace />
  }

  // Only a Tier Account has the Tiers Breakdown page.
  if (
    pathname === paths.tiersBreakdown(account.id) &&
    !isTierAccount(account)
  ) {
    return <Navigate to={paths.account(account.id)} replace />
  }

  return <Outlet context={account} />
}

// useAccount gives the Account of the AccountLayout to a page of the Account.
export function useAccount(): Account {
  return useOutletContext<Account>()
}
