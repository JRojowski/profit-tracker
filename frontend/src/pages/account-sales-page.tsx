import { LoadedView } from "@/components/loaded-view"
import { SalesTable } from "@/components/sales-table"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { useLoad } from "@/hooks/use-load"
import { fetchSales } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { useAccount } from "@/pages/account-layout"

// AccountSalesPage shows the sales of one Account with their Realized Profit.
export function AccountSalesPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const sales = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchSales(account.id, signal)
  )

  return (
    <LoadedView loaded={sales} errorTitle="Cannot load the sales">
      {(data) => (
        <Card>
          <CardHeader>
            <CardTitle>Sales</CardTitle>
          </CardHeader>
          <CardContent>
            <SalesTable sales={data} currency={account.currency} />
          </CardContent>
        </Card>
      )}
    </LoadedView>
  )
}
