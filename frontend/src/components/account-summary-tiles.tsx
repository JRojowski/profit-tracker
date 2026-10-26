import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { ReturnTile } from "@/components/return-tile"
import type { AccountSummary } from "@/lib/api"
import { formatAmount } from "@/lib/format"
import { cn } from "@/lib/utils"

type AccountSummaryTilesProps = {
  summary: AccountSummary
}

export type Tile = {
  label: string
  // amount is null when the tool cannot calculate it. The tile then shows "—".
  amount: string | null
  // note is a short text under the amount, or undefined.
  note?: string
}

export function AccountSummaryTiles({ summary }: AccountSummaryTilesProps) {
  const unpriced = unpricedNote(summary.unpricedHoldings)
  const accountTiles: Tile[] = [
    { label: "Value", amount: summary.value, note: unpriced },
    { label: "Cash", amount: summary.cash },
    { label: "Net Deposits", amount: summary.netDeposits },
    { label: "Total Result", amount: summary.totalResult, note: unpriced },
  ]
  const resultParts: Tile[] = [
    { label: "Realized Profit", amount: summary.realizedProfit },
    {
      label: "Unrealized Profit",
      amount: summary.unrealizedProfit,
      note: unpriced,
    },
    { label: "Net dividends", amount: summary.netDividends },
    { label: "Net interest", amount: summary.netInterest },
    { label: "Fees", amount: summary.fees },
  ]

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {accountTiles.map((tile) => (
          <SummaryTile
            key={tile.label}
            tile={tile}
            currency={summary.currency}
            titleClassName="text-2xl"
          />
        ))}
        <ReturnTile returns={summary} />
      </div>
      <div className="flex flex-col gap-2">
        <h3 className="text-sm font-medium text-muted-foreground">
          Parts of the total result
        </h3>
        <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-5">
          {resultParts.map((tile) => (
            <SummaryTile
              key={tile.label}
              tile={tile}
              currency={summary.currency}
              titleClassName="text-xl"
            />
          ))}
        </div>
      </div>
    </div>
  )
}

type SummaryTileProps = {
  tile: Tile
  currency: string
  titleClassName: string
}

export function SummaryTile({
  tile,
  currency,
  titleClassName,
}: SummaryTileProps) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{tile.label}</CardDescription>
        <CardTitle className={cn("tabular-nums", titleClassName)}>
          {tile.amount === null ? "—" : formatAmount(tile.amount, currency)}
        </CardTitle>
        {tile.note && <CardDescription>{tile.note}</CardDescription>}
      </CardHeader>
    </Card>
  )
}

// unpricedNote tells that the amount leaves out the Holdings with no value.
export function unpricedNote(count: number): string | undefined {
  if (count === 0) return undefined
  return count === 1
    ? "Without 1 Holding that has no value."
    : `Without ${count} Holdings that have no value.`
}
