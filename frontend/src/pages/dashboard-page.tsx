import { useEffect, useState } from "react"

import { AccountSummaryTiles } from "@/components/account-summary-tiles"
import { AllAccountsSummaryView } from "@/components/all-accounts-summary"
import { HoldingsTable } from "@/components/holdings-table"
import { IkzeLimitCard } from "@/components/ikze-limit-card"
import { PriceStatus } from "@/components/price-status"
import { RenameAccountForm } from "@/components/rename-account-form"
import { SalesTable } from "@/components/sales-table"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectSeparator,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  errorMessage,
  fetchAccounts,
  fetchAccountSummary,
  fetchAllAccountsSummary,
  fetchHoldings,
  fetchSales,
  refreshPrices,
  type Account,
  type AccountSummary,
  type AllAccountsSummary,
  type Holding,
  type PriceRefresh,
  type Sale,
} from "@/lib/api"

// AccountState holds the values of the selected Account.
type AccountState = {
  accountId: number
  summary: AccountSummary
  holdings: Holding[]
  sales: Sale[]
}

// allAccounts is the value of the "All Accounts" item in the Account selector.
const allAccounts = "all"

// Selection is the id of the selected Account, or allAccounts for the view of all Accounts.
type Selection = number | typeof allAccounts

// AllAccountsState holds the values of the view of all Accounts.
type AllAccountsState = {
  accountId: typeof allAccounts
  summary: AllAccountsSummary
}

// ValuesError holds the error of the last failed load of the values of a selection.
type ValuesError = {
  accountId: Selection
  message: string
}

export function DashboardPage() {
  const [accounts, setAccounts] = useState<Account[] | null>(null)
  const [selectedId, setSelectedId] = useState<Selection | null>(null)
  const [state, setState] = useState<AccountState | AllAccountsState | null>(
    null
  )
  const [renaming, setRenaming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [valuesError, setValuesError] = useState<ValuesError | null>(null)
  // reloadCount changes after a price refresh, so the values of the Account load again.
  const [reloadCount, setReloadCount] = useState(0)
  const [refreshing, setRefreshing] = useState(false)
  const [refresh, setRefresh] = useState<PriceRefresh | null>(null)
  // refreshedAt is the time of the last successful price refresh. A repeated refresh with the same prices
  // changes only this time, so the user sees that the refresh ran.
  const [refreshedAt, setRefreshedAt] = useState<Date | null>(null)
  const [refreshError, setRefreshError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    fetchAccounts(controller.signal)
      .then((list) => {
        setAccounts(list)
        // The tool opens the view of all Accounts at start.
        setSelectedId(list.length > 0 ? allAccounts : null)
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setError(errorMessage(err))
      })
    return () => controller.abort()
  }, [])

  useEffect(() => {
    if (selectedId === null) return
    const controller = new AbortController()
    if (selectedId === allAccounts) {
      fetchAllAccountsSummary(controller.signal)
        .then((summary) => {
          setState({ accountId: allAccounts, summary })
          setValuesError(null)
        })
        .catch((err: unknown) => {
          if (controller.signal.aborted) return
          setValuesError({ accountId: allAccounts, message: errorMessage(err) })
        })
      return () => controller.abort()
    }
    Promise.all([
      fetchAccountSummary(selectedId, controller.signal),
      fetchHoldings(selectedId, controller.signal),
      fetchSales(selectedId, controller.signal),
    ])
      .then(([summary, holdings, sales]) => {
        setState({ accountId: selectedId, summary, holdings, sales })
        setValuesError(null)
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setValuesError({ accountId: selectedId, message: errorMessage(err) })
      })
    return () => controller.abort()
  }, [selectedId, reloadCount])

  function handleRefreshPrices() {
    setRefreshing(true)
    refreshPrices()
      .then((result) => {
        setRefresh(result)
        setRefreshedAt(new Date())
        setRefreshError(null)
        setReloadCount((count) => count + 1)
      })
      .catch((err: unknown) => {
        setRefresh(null)
        setRefreshedAt(null)
        setRefreshError(errorMessage(err))
      })
      .finally(() => setRefreshing(false))
  }

  function handleRenamed(renamed: Account) {
    setAccounts(
      (list) =>
        list
          ?.map((account) => (account.id === renamed.id ? renamed : account))
          .sort(byNameThenId) ?? null
    )
    setRenaming(false)
  }

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Cannot load the Accounts</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  if (accounts === null) {
    return <Skeleton className="h-8 w-64" />
  }

  if (accounts.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        There are no Accounts yet. Import an export to create one.
      </p>
    )
  }

  const selected = accounts.find((account) => account.id === selectedId)
  const priceStatus = (summary: AccountSummary | AllAccountsSummary) => (
    <PriceStatus
      summary={summary}
      refreshing={refreshing}
      refresh={refresh}
      refreshedAt={refreshedAt}
      refreshError={refreshError}
      onRefresh={handleRefreshPrices}
    />
  )
  // Show the values only when they belong to the selected Account.
  const current = state?.accountId === selectedId ? state : null
  const currentError =
    valuesError?.accountId === selectedId ? valuesError.message : null

  return (
    <div className="flex flex-col gap-4">
      {renaming && selected ? (
        <RenameAccountForm
          key={selected.id}
          account={selected}
          onRenamed={handleRenamed}
          onCancel={() => setRenaming(false)}
        />
      ) : (
        <div className="flex items-center gap-2">
          <Select
            value={selectedId === null ? undefined : String(selectedId)}
            onValueChange={(value) =>
              setSelectedId(value === allAccounts ? allAccounts : Number(value))
            }
          >
            <SelectTrigger className="w-64" aria-label="Account">
              <SelectValue placeholder="Select an Account" />
            </SelectTrigger>
            <SelectContent>
              <SelectItem value={allAccounts}>All Accounts</SelectItem>
              <SelectSeparator />
              {accounts.map((account) => (
                <SelectItem key={account.id} value={String(account.id)}>
                  {account.name} ({account.currency})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          {selected && (
            <Button variant="outline" onClick={() => setRenaming(true)}>
              Rename
            </Button>
          )}
        </div>
      )}

      {currentError ? (
        <Alert variant="destructive">
          <AlertTitle>Cannot load the Account values</AlertTitle>
          <AlertDescription>{currentError}</AlertDescription>
        </Alert>
      ) : current?.accountId === allAccounts ? (
        <>
          {priceStatus(current.summary)}
          <AllAccountsSummaryView summary={current.summary} />
        </>
      ) : current ? (
        <>
          {priceStatus(current.summary)}
          <AccountSummaryTiles summary={current.summary} />
          {selected?.type === "IKZE" && (
            <IkzeLimitCard accountId={current.accountId} />
          )}
          <Card>
            <CardHeader>
              <CardTitle>Holdings</CardTitle>
            </CardHeader>
            <CardContent>
              <HoldingsTable
                holdings={current.holdings}
                currency={current.summary.currency}
                onPriceSourceChanged={handleRefreshPrices}
              />
            </CardContent>
          </Card>
          <Card>
            <CardHeader>
              <CardTitle>Sales</CardTitle>
            </CardHeader>
            <CardContent>
              <SalesTable
                sales={current.sales}
                currency={current.summary.currency}
              />
            </CardContent>
          </Card>
        </>
      ) : (
        <div className="flex flex-col gap-4">
          <Skeleton className="h-24 w-full" />
          <Skeleton className="h-48 w-full" />
        </div>
      )}
    </div>
  )
}

// byNameThenId sorts the Accounts in the order of GET /api/accounts.
function byNameThenId(a: Account, b: Account): number {
  if (a.name !== b.name) return a.name < b.name ? -1 : 1
  return a.id - b.id
}
