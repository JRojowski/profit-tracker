import { LoadedView } from "@/components/loaded-view"
import { OverviewHero } from "@/components/overview-hero"
import {
  TotalResultWaterfall,
  type WaterfallPart,
} from "@/components/total-result-waterfall"
import { ValueHistoryCard } from "@/components/value-history-card"
import { useLoad } from "@/hooks/use-load"
import {
  fetchAccountAllocation,
  fetchAccountSummary,
  fetchAccountValueHistory,
  type AccountSummary,
  type Provider,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { formatAmount } from "@/lib/format"
import { isKraken } from "@/lib/provider"
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
  const history = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchAccountValueHistory(account.id, signal)
  )

  return (
    <LoadedView loaded={summary} errorTitle="Cannot load the Account values">
      {(data) => (
        <div className="flex min-w-0 flex-col gap-6">
          <OverviewHero figures={data} allocation={allocation} />
          <LoadedView
            loaded={history}
            errorTitle="Cannot load the Value History"
          >
            {(data) => (
              <ValueHistoryCard
                history={data}
                emptyText="The Account has no Value History."
              />
            )}
          </LoadedView>
          <TotalResultWaterfall
            currency={data.currency}
            totalResult={data.totalResult}
            parts={totalResultParts(data, account.provider)}
            totalNote={
              isKraken(account.provider)
                ? `including fees: ${formatAmount(data.feeSum, data.currency)}`
                : undefined
            }
          />
        </div>
      )}
    </LoadedView>
  )
}

// totalResultParts gives the parts of the Total Result of an Account in the order of the bars.
// A Kraken Account has the Rewards part and no Fees part, because its fees are in the cost (ADR 0009).
function totalResultParts(
  summary: AccountSummary,
  provider: Provider
): WaterfallPart[] {
  const parts: WaterfallPart[] = [
    { label: "Realized Profit", amount: summary.realizedProfit },
    { label: "Unrealized Profit", amount: summary.unrealizedProfit },
    { label: "Net dividends", amount: summary.netDividends },
    { label: "Net interest", amount: summary.netInterest },
  ]
  if (isKraken(provider)) {
    return [...parts, { label: "Rewards", amount: summary.rewards }]
  }
  return [...parts, { label: "Fees", amount: summary.fees }]
}
