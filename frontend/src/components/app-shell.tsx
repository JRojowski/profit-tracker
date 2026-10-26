import { RefreshCwIcon } from "lucide-react"
import { Outlet, useLocation, useMatch } from "react-router"

import { AccountBadges } from "@/components/account-badges"
import { AppSidebar } from "@/components/app-sidebar"
import {
  PriceAlerts,
  PriceDatesLine,
  type PriceDates,
} from "@/components/price-status"
import { RenameAccountDialog } from "@/components/rename-account-dialog"
import { ThemeSwitch } from "@/components/theme-switch"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  SidebarInset,
  SidebarProvider,
  SidebarTrigger,
} from "@/components/ui/sidebar"
import { useLoad } from "@/hooks/use-load"
import { useRouteAccount } from "@/hooks/use-route-account"
import {
  fetchAccountSummary,
  fetchAllAccountsSummary,
  type Account,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { accountPattern, paths } from "@/lib/routes"

// AppShell holds the sidebar and the top bar. Each page renders only its content in the Outlet.
export function AppShell() {
  const { accounts, backendUnavailable, priceRefresh, refreshAllPrices } =
    useAppState()
  const account = useRouteAccount(accounts)
  const accountRoute = useMatch(`${accountPattern}/*`) !== null
  // On the page of an Account, the view is not known until the Accounts load.
  const viewKnown = !accountRoute || accounts !== null
  const dates = useViewPriceDates(viewKnown, account)

  return (
    <SidebarProvider>
      <AppSidebar />
      <SidebarInset className="min-w-0">
        <header className="sticky top-0 z-20 flex flex-col border-b bg-background/90 backdrop-blur">
          <div className="flex h-14 items-center gap-2 px-4 md:px-8">
            <SidebarTrigger className="md:hidden" />
            {viewKnown && <ViewTitle account={account} />}
            <div className="ml-auto flex items-center gap-2 md:gap-4">
              <PriceDatesLine
                dates={dates}
                refresh={priceRefresh}
                className="hidden lg:flex"
              />
              {backendUnavailable && (
                <Badge variant="destructive">Backend unavailable</Badge>
              )}
              <Button
                variant="outline"
                aria-label="Refresh prices"
                onClick={refreshAllPrices}
                disabled={priceRefresh.refreshing}
                className="border-primary/30 text-primary hover:text-primary"
              >
                <RefreshCwIcon data-icon="inline-start" />
                <span className="hidden sm:inline">
                  {priceRefresh.refreshing ? "Refreshing…" : "Refresh prices"}
                </span>
              </Button>
              <ThemeSwitch />
            </div>
          </div>
          <PriceDatesLine
            dates={dates}
            refresh={priceRefresh}
            className="border-t px-4 py-1.5 md:px-8 lg:hidden"
          />
        </header>
        <div className="mx-auto flex w-full max-w-7xl min-w-0 flex-col gap-6 p-4 md:p-8">
          <PriceAlerts dates={dates} refresh={priceRefresh} />
          <Outlet />
        </div>
      </SidebarInset>
    </SidebarProvider>
  )
}

// ViewTitle shows the title of the view. An Account also shows its badges and the rename button.
function ViewTitle({ account }: { account: Account | null }) {
  const { pathname } = useLocation()
  const accountRoute = useMatch(`${accountPattern}/*`)

  if (account) {
    return (
      <div className="flex min-w-0 items-center gap-2">
        <h1 className="truncate text-base font-semibold tracking-tight">
          {account.name}
        </h1>
        <AccountBadges account={account} />
        <RenameAccountDialog account={account} />
      </div>
    )
  }

  let title = "All Accounts"
  if (pathname === paths.import) title = "Import"
  else if (pathname === paths.imports) title = "Imports"
  else if (accountRoute && pathname !== paths.allAccounts)
    title = "Unknown Account"
  return (
    <h1 className="truncate text-base font-semibold tracking-tight">{title}</h1>
  )
}

// useViewPriceDates loads the price dates of the view: the dates of the Account, or of all Accounts.
// It loads nothing while the view is not known.
function useViewPriceDates(
  viewKnown: boolean,
  account: Account | null
): PriceDates | null {
  const { valuesVersion } = useAppState()
  const accountId = account?.id ?? null
  const loaded = useLoad<PriceDates>(
    viewKnown ? String(accountId ?? "all") : null,
    valuesVersion,
    (signal) =>
      accountId === null
        ? fetchAllAccountsSummary(signal)
        : fetchAccountSummary(accountId, signal)
  )
  return loaded.status === "ok" ? loaded.data : null
}
