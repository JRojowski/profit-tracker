import { TriangleAlertIcon } from "lucide-react"
import { Link } from "react-router"

import { AccountsTable } from "@/components/accounts-table"
import { LoadedView } from "@/components/loaded-view"
import { OverviewHero } from "@/components/overview-hero"
import { TotalResultWaterfall } from "@/components/total-result-waterfall"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useLoad } from "@/hooks/use-load"
import {
  fetchAllAccountsAllocation,
  fetchAllAccountsSummary,
  type Allocation,
  type AllAccountsSummary,
} from "@/lib/api"
import type { Loaded } from "@/hooks/use-load"
import { useAppState } from "@/lib/app-state"
import { paths } from "@/lib/routes"

// AllAccountsPage is the Overview of all Accounts, in PLN by ADR 0004.
export function AllAccountsPage() {
  const { accounts, accountsError, valuesVersion } = useAppState()
  const summary = useLoad("all", valuesVersion, fetchAllAccountsSummary)
  const allocation = useLoad("all", valuesVersion, fetchAllAccountsAllocation)

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
      {(data) => <AllAccountsOverview summary={data} allocation={allocation} />}
    </LoadedView>
  )
}

// AllAccountsOverview shows the totals of all Accounts in PLN, the parts of the Total Result and one row for each Account.
function AllAccountsOverview({
  summary,
  allocation,
}: {
  summary: AllAccountsSummary
  allocation: Loaded<Allocation>
}) {
  return (
    <div className="flex min-w-0 flex-col gap-6">
      {summary.warnings.length > 0 && (
        <Alert variant={summary.available ? "default" : "destructive"}>
          <TriangleAlertIcon />
          <AlertTitle>
            {summary.available
              ? "Check the totals"
              : "The tool cannot calculate the totals"}
          </AlertTitle>
          <AlertDescription>
            <ul className="list-inside list-disc">
              {summary.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}
      <OverviewHero
        figures={{ ...summary, currency: "PLN" }}
        allocation={allocation}
      />
      {/* This full-width row under the hero is for the Total Result breakdown in v3. */}
      <TotalResultWaterfall
        currency="PLN"
        totalResult={summary.totalResult}
        parts={[
          { label: "Realized Profit", amount: summary.realizedProfit },
          { label: "Unrealized Profit", amount: summary.unrealizedProfit },
          { label: "Net dividends", amount: summary.netDividends },
          { label: "Net interest", amount: summary.netInterest },
          { label: "Fees", amount: summary.fees },
          { label: "Currency Result", amount: summary.currencyResult },
        ]}
      />
      <AccountsTable summary={summary} />
    </div>
  )
}
