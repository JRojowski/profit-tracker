import { StackedHead } from "@/components/stacked-head"
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { ClosedPosition } from "@/lib/api"
import {
  amountSign,
  formatAmount,
  formatPolishDate,
  formatVolume,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"

type ClosedPositionsTableProps = {
  // closedPositions are in the order of the rows, newest first.
  closedPositions: ClosedPosition[]
  currency: string
  // totalRealizedProfit is the exact sum of the Realized Profit of the Closed Positions.
  totalRealizedProfit: string
}

// ClosedPositionsTable shows each Closed Position with its Realized Profit, and a total row.
// The date of a Closed Position is in Polish time, so that it agrees with the year filter.
export function ClosedPositionsTable({
  closedPositions,
  currency,
  totalRealizedProfit,
}: ClosedPositionsTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead className="hidden md:table-cell">Date</TableHead>
          <TableHead>
            <span className="hidden md:inline">Instrument</span>
            {/* On a narrow screen, one column holds the Instrument, the date and the volume. */}
            <StackedHead labels={["Instrument", "Date", "Volume"]} />
          </TableHead>
          <TableHead className="hidden text-right md:table-cell">
            Volume
          </TableHead>
          <TableHead className="hidden text-right md:table-cell">
            Sale amount
          </TableHead>
          <TableHead className="hidden text-right md:table-cell">
            Cost
          </TableHead>
          <TableHead className="text-right">
            <span className="hidden md:inline">Realized Profit</span>
            {/* On a narrow screen, one column holds the three amounts. */}
            <StackedHead
              labels={["Realized Profit", "Sale amount", "Cost"]}
              alignEnd
            />
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {closedPositions.map((closedPosition) => (
          <TableRow key={closedPosition.xtbId}>
            <TableCell className="hidden num text-muted-foreground md:table-cell">
              {formatPolishDate(closedPosition.time)}
            </TableCell>
            <TableCell className="align-top md:align-middle">
              <div className="flex flex-col gap-0.5">
                <span className="font-medium">{closedPosition.ticker}</span>
                <span className="max-w-32 truncate text-xs text-muted-foreground md:max-w-56">
                  {closedPosition.name}
                </span>
                <span className="flex flex-col gap-0.5 text-xs text-muted-foreground md:hidden">
                  <span className="num">
                    {formatPolishDate(closedPosition.time)}
                  </span>
                  <span className="num">
                    {formatVolume(closedPosition.volume)}
                  </span>
                </span>
              </div>
            </TableCell>
            <TableCell className="hidden text-right num md:table-cell">
              {formatVolume(closedPosition.volume)}
            </TableCell>
            <TableCell className="hidden text-right num md:table-cell">
              {formatAmount(closedPosition.amount, currency)}
            </TableCell>
            <TableCell className="hidden text-right num md:table-cell">
              {formatAmount(closedPosition.cost, currency)}
            </TableCell>
            <TableCell className="text-right align-top md:align-middle">
              <span className="flex flex-col items-end gap-0.5">
                <span
                  className={cn(
                    "num font-medium",
                    signColour(amountSign(closedPosition.realizedProfit))
                  )}
                >
                  {formatAmount(closedPosition.realizedProfit, currency, {
                    signed: true,
                  })}
                </span>
                <span className="flex flex-col items-end gap-0.5 text-xs text-muted-foreground md:hidden">
                  <span className="num">
                    {formatAmount(closedPosition.amount, currency)}
                  </span>
                  <span className="num">
                    {formatAmount(closedPosition.cost, currency)}
                  </span>
                </span>
              </span>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
      <TableFooter>
        <TableRow>
          <TableCell>Total</TableCell>
          <TableCell className="hidden md:table-cell" colSpan={4} />
          <TableCell
            className={cn(
              "text-right num",
              signColour(amountSign(totalRealizedProfit))
            )}
          >
            {formatAmount(totalRealizedProfit, currency, { signed: true })}
          </TableCell>
        </TableRow>
      </TableFooter>
    </Table>
  )
}
