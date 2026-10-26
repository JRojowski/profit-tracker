import { LoadedView } from "@/components/loaded-view"
import { OverviewHero } from "@/components/overview-hero"
import { TotalResultWaterfall } from "@/components/total-result-waterfall"
import { useLoad } from "@/hooks/use-load"
import { fetchAccountAllocation, fetchAccountSummary } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { useAccount } from "@/pages/account-layout"

// AccountOverviewPage shows the values and the results of one Account, in the Account currency.
export function AccountOverviewPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const summary = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchAccountSummary(account.id, signal)
  )
  const allocation = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchAccountAllocation(account.id, signal)
  )

  return (
    <LoadedView loaded={summary} errorTitle="Cannot load the Account values">
      {(data) => (
        <div className="flex min-w-0 flex-col gap-6">
          <OverviewHero figures={data} allocation={allocation} />
          {/* This full-width row under the hero is for the Total Result breakdown in v3. */}
          <TotalResultWaterfall
            currency={data.currency}
            totalResult={data.totalResult}
            parts={[
              { label: "Realized Profit", amount: data.realizedProfit },
              { label: "Unrealized Profit", amount: data.unrealizedProfit },
              { label: "Net dividends", amount: data.netDividends },
              { label: "Net interest", amount: data.netInterest },
              { label: "Fees", amount: data.fees },
            ]}
          />
        </div>
      )}
    </LoadedView>
  )
}
