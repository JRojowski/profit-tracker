import { AccountSummaryTiles } from "@/components/account-summary-tiles"
import { LoadedView } from "@/components/loaded-view"
import { useLoad } from "@/hooks/use-load"
import { fetchAccountSummary } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { useAccount } from "@/pages/account-layout"

// AccountOverviewPage shows the values and the results of one Account, in the Account currency.
export function AccountOverviewPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const summary = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchAccountSummary(account.id, signal)
  )

  return (
    <LoadedView loaded={summary} errorTitle="Cannot load the Account values">
      {(data) => <AccountSummaryTiles summary={data} />}
    </LoadedView>
  )
}
