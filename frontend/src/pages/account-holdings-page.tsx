import { useMemo } from "react"

import { AllocationChart } from "@/components/allocation-donut"
import {
  HOLDINGS_VIEW_STORAGE_KEY,
  holdingColumns,
  HoldingsTable,
  holdingsViewRules,
} from "@/components/holdings-table"
import { ResetColumnsButton } from "@/components/reset-columns-button"
import { LoadedView } from "@/components/loaded-view"
import { UnrealizedProfitChart } from "@/components/unrealized-profit-chart"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardAction,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { useLoad } from "@/hooks/use-load"
import { useIsMobile } from "@/hooks/use-mobile"
import { useTableView } from "@/hooks/use-table-view"
import { fetchAccountAllocation, fetchHoldings } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { useAccount } from "@/pages/account-layout"

// AccountHoldingsPage shows the Holdings of one Account: the Allocation by Quote Currency,
// the Unrealized Profit of each Holding and the table of the Holdings.
export function AccountHoldingsPage() {
  const account = useAccount()
  const isMobile = useIsMobile()
  const { valuesVersion, refreshAllPrices } = useAppState()
  // The columns are built once. The table and the rules of the stored view use them.
  const columns = useMemo(
    () => holdingColumns(account.currency, refreshAllPrices),
    [account.currency, refreshAllPrices]
  )
  const view = useTableView(
    HOLDINGS_VIEW_STORAGE_KEY,
    useMemo(() => holdingsViewRules(columns), [columns])
  )
  const holdings = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchHoldings(account.id, signal)
  )
  const allocation = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchAccountAllocation(account.id, signal)
  )

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <div className="grid min-w-0 gap-6 lg:grid-cols-2">
        <Card className="min-w-0">
          <CardHeader>
            <CardTitle>Allocation by Quote Currency</CardTitle>
            <CardDescription>
              The cash counts in the Account currency.
            </CardDescription>
          </CardHeader>
          <CardContent className="@container">
            <LoadedView
              loaded={allocation}
              errorTitle="Cannot load the Allocation"
            >
              {(data) => (
                <AllocationChart allocation={data} view="quoteCurrency" />
              )}
            </LoadedView>
          </CardContent>
        </Card>
        <Card className="min-w-0">
          <CardHeader>
            <CardTitle>Unrealized Profit by Holding</CardTitle>
            <CardDescription>In {account.currency}.</CardDescription>
          </CardHeader>
          <CardContent>
            <LoadedView loaded={holdings} errorTitle="Cannot load the Holdings">
              {(data) => <UnrealizedProfitChart holdings={data} />}
            </LoadedView>
          </CardContent>
        </Card>
      </div>
      <Card className="min-w-0">
        <CardHeader>
          <CardTitle className="flex items-center gap-2">
            Holdings
            {holdings.status === "ok" && (
              <Badge variant="secondary" className="num font-mono">
                {holdings.data.length}
              </Badge>
            )}
          </CardTitle>
          {!isMobile && (
            <CardAction>
              <ResetColumnsButton
                isDefault={view.isDefault}
                onReset={view.reset}
              />
            </CardAction>
          )}
        </CardHeader>
        <CardContent>
          <LoadedView loaded={holdings} errorTitle="Cannot load the Holdings">
            {(data) => (
              <HoldingsTable
                holdings={data}
                columns={columns}
                currency={account.currency}
                onPriceSourceChanged={refreshAllPrices}
                view={view.props}
              />
            )}
          </LoadedView>
        </CardContent>
      </Card>
    </div>
  )
}
