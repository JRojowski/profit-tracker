import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { Holding } from "@/lib/api"
import { formatAmount, formatVolume } from "@/lib/format"

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
        </TableRow>
      </TableHeader>
      <TableBody>
        {holdings.map((holding) => (
          <TableRow key={holding.ticker}>
            <TableCell>
              {holding.ticker}{" "}
              <span className="text-muted-foreground">{holding.name}</span>
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
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
