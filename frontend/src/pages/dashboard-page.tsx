import { useEffect, useState } from "react"

import { AccountSummaryTiles } from "@/components/account-summary-tiles"
import { HoldingsTable } from "@/components/holdings-table"
import { RenameAccountForm } from "@/components/rename-account-form"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import { Skeleton } from "@/components/ui/skeleton"
import {
  errorMessage,
  fetchAccounts,
  fetchAccountSummary,
  fetchHoldings,
  type Account,
  type AccountSummary,
  type Holding,
} from "@/lib/api"

// AccountState holds the values of the selected Account.
type AccountState = {
  accountId: number
  summary: AccountSummary
  holdings: Holding[]
}

// ValuesError holds the error of the last failed load of Account values.
type ValuesError = {
  accountId: number
  message: string
}

export function DashboardPage() {
  const [accounts, setAccounts] = useState<Account[] | null>(null)
  const [selectedId, setSelectedId] = useState<number | null>(null)
  const [state, setState] = useState<AccountState | null>(null)
  const [renaming, setRenaming] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const [valuesError, setValuesError] = useState<ValuesError | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    fetchAccounts(controller.signal)
      .then((list) => {
        setAccounts(list)
        setSelectedId(list[0]?.id ?? null)
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
    Promise.all([
      fetchAccountSummary(selectedId, controller.signal),
      fetchHoldings(selectedId, controller.signal),
    ])
      .then(([summary, holdings]) => {
        setState({ accountId: selectedId, summary, holdings })
        setValuesError(null)
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setValuesError({ accountId: selectedId, message: errorMessage(err) })
      })
    return () => controller.abort()
  }, [selectedId])

  function handleRenamed(renamed: Account) {
    setAccounts(
      (list) =>
        list?.map((account) =>
          account.id === renamed.id ? renamed : account
        ) ?? null
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
            onValueChange={(value) => setSelectedId(Number(value))}
          >
            <SelectTrigger className="w-64" aria-label="Account">
              <SelectValue placeholder="Select an Account" />
            </SelectTrigger>
            <SelectContent>
              {accounts.map((account) => (
                <SelectItem key={account.id} value={String(account.id)}>
                  {account.name} ({account.currency})
                </SelectItem>
              ))}
            </SelectContent>
          </Select>
          <Button variant="outline" onClick={() => setRenaming(true)}>
            Rename
          </Button>
        </div>
      )}

      {currentError ? (
        <Alert variant="destructive">
          <AlertTitle>Cannot load the Account values</AlertTitle>
          <AlertDescription>{currentError}</AlertDescription>
        </Alert>
      ) : current ? (
        <>
          <AccountSummaryTiles summary={current.summary} />
          <Card>
            <CardHeader>
              <CardTitle>Holdings</CardTitle>
            </CardHeader>
            <CardContent>
              <HoldingsTable
                holdings={current.holdings}
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
