import { useMemo } from "react"

import { DataTable, type DataTableColumn } from "@/components/data-table"
import { SortControl } from "@/components/sort-control"
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
import { useIsMobile } from "@/hooks/use-mobile"
import type { ClosedPosition } from "@/lib/api"
import {
  amountSign,
  formatAmount,
  formatPolishDate,
  formatVolume,
  signColour,
} from "@/lib/format"
import {
  decimalSortValue,
  sortRows,
  textSortValue,
  timeSortValue,
} from "@/lib/table-sort"
import {
  tableViewRules,
  type TableViewProps,
  type TableViewRules,
} from "@/lib/table-view"
import { cn } from "@/lib/utils"

// CLOSED_POSITIONS_VIEW_STORAGE_KEY is the localStorage key of the column order and the sorting of this table.
// All Accounts share it.
export const CLOSED_POSITIONS_VIEW_STORAGE_KEY =
  "profit-tracker:table:closed-positions"

// FIXED_COLUMN_IDS are the leading columns that the user cannot move.
const FIXED_COLUMN_IDS = ["time", "instrument"]

// closedPositionColumns gives the columns of the table in the default order. Each column has a sortValue.
// The view rules read the column ids from here, so a new column needs one edit.
export function closedPositionColumns(
  currency: string
): DataTableColumn<ClosedPosition>[] {
  return [
    {
      id: "time",
      header: "Date",
      // The date of a Closed Position is in Polish time, so that it agrees with the year filter.
      sortValue: (closedPosition) => timeSortValue(closedPosition.time),
      cell: ({ row }) => (
        <span className="num text-muted-foreground">
          {formatPolishDate(row.original.time)}
        </span>
      ),
    },
    {
      id: "instrument",
      header: "Instrument",
      sortValue: (closedPosition) => textSortValue(closedPosition.ticker),
      cell: ({ row }) => (
        <div className="flex flex-col gap-0.5">
          <span className="font-medium">{row.original.ticker}</span>
          <span className="max-w-32 truncate text-xs text-muted-foreground md:max-w-56">
            {row.original.name}
          </span>
        </div>
      ),
    },
    {
      id: "volume",
      header: "Volume",
      meta: { align: "right" },
      sortValue: (closedPosition) => decimalSortValue(closedPosition.volume),
      cell: ({ row }) => formatVolume(row.original.volume),
    },
    {
      id: "amount",
      header: "Sale amount",
      meta: { align: "right" },
      sortValue: (closedPosition) => decimalSortValue(closedPosition.amount),
      cell: ({ row }) => formatAmount(row.original.amount, currency),
    },
    {
      id: "cost",
      header: "Cost",
      meta: { align: "right" },
      sortValue: (closedPosition) => decimalSortValue(closedPosition.cost),
      cell: ({ row }) => formatAmount(row.original.cost, currency),
    },
    {
      id: "realizedProfit",
      header: "Realized Profit",
      meta: { align: "right" },
      sortValue: (closedPosition) =>
        decimalSortValue(closedPosition.realizedProfit),
      cell: ({ row }) => (
        <span
          className={cn(
            "font-medium",
            signColour(amountSign(row.original.realizedProfit))
          )}
        >
          {formatAmount(row.original.realizedProfit, currency, {
            signed: true,
          })}
        </span>
      ),
    },
  ]
}

// closedPositionsViewRules gives the rules of a stored view of this table. They come from the column definitions.
export function closedPositionsViewRules(
  columns: DataTableColumn<ClosedPosition>[]
): TableViewRules {
  return tableViewRules(columns, FIXED_COLUMN_IDS)
}

type ClosedPositionsTableProps = {
  // columns come from closedPositionColumns. The owner builds them once, for the table and for the view rules.
  columns: DataTableColumn<ClosedPosition>[]
  // closedPositions are in the default order, newest first. An empty sorting keeps this order.
  closedPositions: ClosedPosition[]
  currency: string
  // totalRealizedProfit is the exact sum of the Realized Profit of the Closed Positions.
  totalRealizedProfit: string
  // view is the column order and the sorting, which the owner keeps with useTableView.
  view: TableViewProps
}

// ClosedPositionsTable shows each Closed Position with its Realized Profit, and a total row that no sort moves.
// A wide screen shows the shared table. A narrow screen shows stacked cells and a select control for one sort.
export function ClosedPositionsTable({
  columns,
  closedPositions,
  currency,
  totalRealizedProfit,
  view,
}: ClosedPositionsTableProps) {
  const isMobile = useIsMobile()
  // The narrow layout uses only the first sort column.
  const sorted = useMemo(
    () =>
      sortRows(
        closedPositions,
        isMobile ? view.sorting.slice(0, 1) : view.sorting,
        columns
      ),
    [closedPositions, view.sorting, columns, isMobile]
  )
  if (isMobile) {
    return (
      <div className="flex flex-col gap-3">
        <SortControl
          options={columns.filter((column) => column.sortValue)}
          sorting={view.sorting}
          onSortingChange={view.onSortingChange}
        />
        <StackedTable
          closedPositions={sorted}
          currency={currency}
          totalRealizedProfit={totalRealizedProfit}
        />
      </div>
    )
  }
  return (
    <DataTable
      {...view}
      columns={columns}
      rows={sorted}
      getRowId={(closedPosition) => closedPosition.xtbId}
      footer={{
        label: "Total",
        cells: {
          realizedProfit: (
            <span className={signColour(amountSign(totalRealizedProfit))}>
              {formatAmount(totalRealizedProfit, currency, { signed: true })}
            </span>
          ),
        },
      }}
    />
  )
}

// StackedTable is the narrow layout: one column holds the Instrument, the date and the volume,
// and one column holds the Realized Profit, the sale amount and the cost. The rows are already sorted.
function StackedTable({
  closedPositions,
  currency,
  totalRealizedProfit,
}: {
  closedPositions: ClosedPosition[]
  currency: string
  totalRealizedProfit: string
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>
            <StackedHead labels={["Instrument", "Date", "Volume"]} />
          </TableHead>
          <TableHead className="text-right">
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
            <TableCell className="align-top">
              <div className="flex flex-col gap-0.5">
                <span className="font-medium">{closedPosition.ticker}</span>
                <span className="max-w-32 truncate text-xs text-muted-foreground">
                  {closedPosition.name}
                </span>
                <span className="flex flex-col gap-0.5 text-xs text-muted-foreground">
                  <span className="num">
                    {formatPolishDate(closedPosition.time)}
                  </span>
                  <span className="num">
                    {formatVolume(closedPosition.volume)}
                  </span>
                </span>
              </div>
            </TableCell>
            <TableCell className="text-right align-top">
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
                <span className="flex flex-col items-end gap-0.5 text-xs text-muted-foreground">
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
