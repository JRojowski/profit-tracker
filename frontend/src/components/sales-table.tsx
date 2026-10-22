import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { Sale } from "@/lib/api"
import { formatAmount, formatVolume } from "@/lib/format"

type SalesTableProps = {
  sales: Sale[]
  currency: string
}

export function SalesTable({ sales, currency }: SalesTableProps) {
  if (sales.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">The Account has no sales.</p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Date</TableHead>
          <TableHead>Instrument</TableHead>
          <TableHead className="text-right">Volume</TableHead>
          <TableHead className="text-right">Sale amount</TableHead>
          <TableHead className="text-right">Cost</TableHead>
          <TableHead className="text-right">Realized Profit</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {sales.map((sale) => (
          <TableRow key={sale.xtbId}>
            <TableCell>{new Date(sale.time).toLocaleDateString()}</TableCell>
            <TableCell>
              {sale.ticker}{" "}
              <span className="text-muted-foreground">{sale.name}</span>
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatVolume(sale.volume)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatAmount(sale.amount, currency)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatAmount(sale.cost, currency)}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {formatAmount(sale.realizedProfit, currency)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
