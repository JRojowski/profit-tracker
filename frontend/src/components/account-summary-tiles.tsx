import {
  Card,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import type { AccountSummary } from "@/lib/api"
import { formatAmount } from "@/lib/format"

type AccountSummaryTilesProps = {
  summary: AccountSummary
}

export function AccountSummaryTiles({ summary }: AccountSummaryTilesProps) {
  const tiles = [
    { label: "Cash", amount: summary.cash },
    { label: "Net Deposits", amount: summary.netDeposits },
  ]

  return (
    <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-4">
      {tiles.map((tile) => (
        <Card key={tile.label}>
          <CardHeader>
            <CardDescription>{tile.label}</CardDescription>
            <CardTitle className="text-2xl tabular-nums">
              {formatAmount(tile.amount, summary.currency)}
            </CardTitle>
          </CardHeader>
        </Card>
      ))}
    </div>
  )
}
