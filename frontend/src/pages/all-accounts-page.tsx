import { Link } from "react-router"

import { AllAccountsSummaryView } from "@/components/all-accounts-summary"
import { LoadedView } from "@/components/loaded-view"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useLoad } from "@/hooks/use-load"
import { fetchAllAccountsSummary } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { paths } from "@/lib/routes"

// AllAccountsPage is the Overview of all Accounts, in PLN by ADR 0004.
export function AllAccountsPage() {
  const { accounts, accountsError, valuesVersion } = useAppState()
  const summary = useLoad("all", valuesVersion, fetchAllAccountsSummary)

  if (accountsError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Cannot load the Accounts</AlertTitle>
        <AlertDescription>{accountsError}</AlertDescription>
      </Alert>
    )
  }
  if (accounts === null) return <Skeleton className="h-24 w-full" />
  if (accounts.length === 0) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p className="text-sm text-muted-foreground">
          There are no Accounts yet. Import an export to create one.
        </p>
        <Button asChild>
          <Link to={paths.import}>Import an export</Link>
        </Button>
      </div>
    )
  }

  return (
    <LoadedView loaded={summary} errorTitle="Cannot load the Account values">
      {(data) => <AllAccountsSummaryView summary={data} />}
    </LoadedView>
  )
}
