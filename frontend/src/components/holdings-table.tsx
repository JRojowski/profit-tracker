import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { Holding } from "@/lib/api"
import {
  formatAmount,
  formatDate,
  formatPercent,
  formatVolume,
} from "@/lib/format"

type HoldingsTableProps = {
  holdings: Holding[]
  currency: string
}

export function HoldingsTable({ holdings, currency }: HoldingsTableProps) {
  if (holdings.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        The Account has no Holdings.
      </p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Instrument</TableHead>
          <TableHead className="text-right">Volume</TableHead>
          <TableHead className="text-right">Average cost</TableHead>
          <TableHead className="text-right">Cost</TableHead>
          <TableHead className="text-right">Price</TableHead>
          <TableHead className="text-right">Value</TableHead>
          <TableHead className="text-right">Unrealized Profit</TableHead>
          <TableHead className="text-right">%</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {holdings.map((holding) => (
          <TableRow key={holding.ticker}>
            <TableCell>
              <div className="flex flex-col">
                <span>
                  {holding.ticker}{" "}
                  <span className="text-muted-foreground">{holding.name}</span>
                </span>
                {holding.note && (
                  <span className="text-xs whitespace-normal text-muted-foreground">
                    {holding.note}
                  </span>
                )}
              </div>
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatVolume(holding.volume)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatAmount(holding.averageCost, currency)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatAmount(holding.cost, currency)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {holding.price !== null && holding.priceDate !== null ? (
                <div className="flex flex-col">
                  <span>{formatAmount(holding.price, currency)}</span>
                  <span className="text-xs text-muted-foreground">
                    {formatDate(holding.priceDate)}
                  </span>
                </div>
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {holding.value !== null ? (
                formatAmount(holding.value, currency)
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {holding.unrealizedProfit !== null ? (
                formatAmount(holding.unrealizedProfit, currency)
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {holding.unrealizedProfitPercent !== null ? (
                formatPercent(holding.unrealizedProfitPercent)
              ) : (
                <NoValue />
              )}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

// NoValue marks a cell of a Holding that has no price. The note of the Holding tells why.
function NoValue() {
  return <span className="text-muted-foreground">—</span>
}
