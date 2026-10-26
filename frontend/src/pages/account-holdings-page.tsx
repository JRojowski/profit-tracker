import { HoldingsTable } from "@/components/holdings-table"
import { LoadedView } from "@/components/loaded-view"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useLoad } from "@/hooks/use-load"
import { fetchHoldings } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { useAccount } from "@/pages/account-layout"

// AccountHoldingsPage shows the Holdings of one Account.
export function AccountHoldingsPage() {
  const account = useAccount()
  const { valuesVersion, refreshAllPrices } = useAppState()
  const holdings = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchHoldings(account.id, signal)
  )

  return (
    <LoadedView loaded={holdings} errorTitle="Cannot load the Holdings">
      {(data) => (
        <Card>
          <CardHeader>
            <CardTitle>Holdings</CardTitle>
          </CardHeader>
          <CardContent>
            <HoldingsTable
              holdings={data}
              currency={account.currency}
              onPriceSourceChanged={refreshAllPrices}
            />
          </CardContent>
        </Card>
      )}
    </LoadedView>
  )
}
