import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import type { AccountSummary } from "@/lib/api"
import { formatAmount } from "@/lib/format"
import { cn } from "@/lib/utils"

type AccountSummaryTilesProps = {
  summary: AccountSummary
}

type Tile = {
  label: string
  amount: string
}

export function AccountSummaryTiles({ summary }: AccountSummaryTilesProps) {
  const accountTiles: Tile[] = [
    { label: "Cash", amount: summary.cash },
    { label: "Net Deposits", amount: summary.netDeposits },
    { label: "Total result", amount: summary.totalResult },
  ]
  const resultParts: Tile[] = [
    { label: "Realized Profit", amount: summary.realizedProfit },
    { label: "Net dividends", amount: summary.netDividends },
    { label: "Net interest", amount: summary.netInterest },
    { label: "Fees", amount: summary.fees },
  ]

  return (
    <div className="flex flex-col gap-4">
      <div className="grid gap-4 sm:grid-cols-3">
        {accountTiles.map((tile) => (
          <SummaryTile
            key={tile.label}
            tile={tile}
            currency={summary.currency}
            titleClassName="text-2xl"
          />
        ))}
      </div>
      <div className="flex flex-col gap-2">
        <h3 className="text-sm font-medium text-muted-foreground">
          Parts of the total result
        </h3>
        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
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

function SummaryTile({ tile, currency, titleClassName }: SummaryTileProps) {
  return (
    <Card>
      <CardHeader>
        <CardDescription>{tile.label}</CardDescription>
        <CardTitle className={cn("tabular-nums", titleClassName)}>
          {formatAmount(tile.amount, currency)}
        </CardTitle>
      </CardHeader>
    </Card>
  )
}
