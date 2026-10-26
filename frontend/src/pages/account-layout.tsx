import {
  Navigate,
  NavLink,
  Outlet,
  useLocation,
  useOutletContext,
} from "react-router"

import { AccountNotFound } from "@/components/account-not-found"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import { Tabs, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { useRouteAccount } from "@/hooks/use-route-account"
import type { Account } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { paths } from "@/lib/routes"

// AccountLayout shows the tabs of one Account and the page of the selected tab.
export function AccountLayout() {
  const { accounts, accountsError } = useAppState()
  const account = useRouteAccount(accounts)
  // A trailing slash opens the same tab.
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

  const tabs = [
    { label: "Overview", to: paths.account(account.id) },
    { label: "Holdings", to: paths.holdings(account.id) },
    { label: "Sales", to: paths.sales(account.id) },
    ...(account.type === "IKZE"
      ? [{ label: "IKZE", to: paths.ikze(account.id) }]
      : []),
  ]
  // Only an IKZE Account has the IKZE page.
  if (pathname === paths.ikze(account.id) && account.type !== "IKZE") {
    return <Navigate to={paths.account(account.id)} replace />
  }

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <Tabs value={pathname}>
        <div className="-mx-4 [scrollbar-width:none] overflow-x-auto overflow-y-hidden border-b px-4 pb-1.5 md:mx-0 md:px-0">
          <TabsList variant="line" className="h-10">
            {tabs.map((tab) => (
              <TabsTrigger
                key={tab.to}
                value={tab.to}
                asChild
                className="flex-none px-3 after:bg-primary data-active:text-primary"
              >
                <NavLink to={tab.to} end>
                  {tab.label}
                </NavLink>
              </TabsTrigger>
            ))}
          </TabsList>
        </div>
      </Tabs>
      <Outlet context={account} />
    </div>
  )
}

// useAccount gives the Account of the AccountLayout to a tab page.
export function useAccount(): Account {
  return useOutletContext<Account>()
}
