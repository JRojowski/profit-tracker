import {
  FlexRender,
  rowSortingFeature,
  tableFeatures,
  useTable,
  type ColumnDef,
  type RowData,
  type SortingState,
} from "@tanstack/react-table"
import { ArrowDown, ArrowUp } from "lucide-react"
import type { ReactNode } from "react"

import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  nextSorting,
  type SortableColumn,
  type SortingProps,
} from "@/lib/table-sort"
import { cn } from "@/lib/utils"

// ColumnMeta holds what the table needs to know about a column, besides the TanStack options.
type ColumnMeta = {
  // align "right" aligns the header, the cells and the footer cell to the right. Numbers use it.
  align?: "right"
}

const features = tableFeatures({
  rowSortingFeature,
  columnMeta: {} as ColumnMeta,
})

// DataTableColumn is a column definition of the DataTable. The id must be stable.
// A column with a sortValue has a sort. The label is the header text, which the sort controls also use.
export type DataTableColumn<TRow extends RowData> = ColumnDef<
  typeof features,
  TRow
> &
  SortableColumn<TRow> & { header: string }

// DataTableFooter is the footer row of a table: a label for the first column and the content of other cells by column id.
export type DataTableFooter = {
  label: ReactNode
  cells: Record<string, ReactNode>
}

type DataTableProps<TRow extends RowData> = SortingProps & {
  columns: DataTableColumn<TRow>[]
  // rows are already sorted by the caller with sortRows. The table does not sort them.
  rows: TRow[]
  getRowId: (row: TRow) => string
  footer?: DataTableFooter
}

// alignClass gives the classes of a header, a cell or a footer cell for the alignment of its column.
function alignClass(align: "right" | undefined) {
  return align === "right" ? "text-right" : undefined
}

// cellAlignClass is alignClass for a cell. A right-aligned column holds numbers, so its cells get the num class.
function cellAlignClass(align: "right" | undefined) {
  return align === "right" ? "text-right num" : undefined
}

// ariaSort gives the aria-sort value of a header for the direction of its column.
function ariaSort(direction: "asc" | "desc" | undefined) {
  if (direction === undefined) return "none"
  return direction === "asc" ? "ascending" : "descending"
}

// sortState gives the direction and the priority (1 or 2) of a column in the sorting.
// A column that is not in the sorting has no direction and priority 0.
function sortState(sorting: SortingState, columnId: string) {
  const index = sorting.findIndex((sort) => sort.id === columnId)
  if (index < 0) return { direction: undefined, priority: 0 }
  return {
    direction: sorting[index].desc ? ("desc" as const) : ("asc" as const),
    priority: index + 1,
  }
}

// DataTable shows rows in the shadcn table. The caller sorts the rows and owns the sorting.
// A click on a header changes the sorting by up to two columns. The footer row stays at the bottom.
export function DataTable<TRow extends RowData>({
  columns,
  rows,
  getRowId,
  sorting,
  onSortingChange,
  footer,
}: DataTableProps<TRow>) {
  const table = useTable({
    features,
    columns,
    data: rows,
    getRowId,
    manualSorting: true,
    state: { sorting },
  })
  const sortable = new Set(
    columns.filter((column) => column.sortValue).map((column) => column.id)
  )

  return (
    <Table>
      <TableHeader>
        {table.getHeaderGroups().map((headerGroup) => (
          <TableRow key={headerGroup.id}>
            {headerGroup.headers.map((header) => (
              <SortableHeaderCell
                key={header.id}
                align={header.column.columnDef.meta?.align}
                sortable={sortable.has(header.column.id)}
                {...sortState(sorting, header.column.id)}
                onSort={(additive) =>
                  onSortingChange(
                    nextSorting(sorting, header.column.id, additive)
                  )
                }
              >
                <FlexRender header={header} />
              </SortableHeaderCell>
            ))}
          </TableRow>
        ))}
      </TableHeader>
      <TableBody>
        {table.getRowModel().rows.map((row) => (
          <TableRow key={row.id}>
            {row.getAllCells().map((cell) => (
              <TableCell
                key={cell.id}
                className={cellAlignClass(cell.column.columnDef.meta?.align)}
              >
                <FlexRender cell={cell} />
              </TableCell>
            ))}
          </TableRow>
        ))}
      </TableBody>
      {footer && (
        <TableFooter>
          <TableRow>
            {table.getAllLeafColumns().map((column, index) => (
              <TableCell
                key={column.id}
                className={cellAlignClass(column.columnDef.meta?.align)}
              >
                {index === 0 ? footer.label : footer.cells[column.id]}
              </TableCell>
            ))}
          </TableRow>
        </TableFooter>
      )}
    </Table>
  )
}

type SortableHeaderCellProps = {
  align?: "right"
  sortable: boolean
  // direction is undefined when the column is not in the sort.
  direction: "asc" | "desc" | undefined
  // priority is 1 or 2 for a column in the sort. It is 0 otherwise.
  priority: number
  // onSort runs on a click. additive is true when Shift was held.
  onSort: (additive: boolean) => void
  children: ReactNode
}

// SortableHeaderCell is one header cell. A sortable column has a button with the label, the arrow and the priority.
function SortableHeaderCell({
  align,
  sortable,
  direction,
  priority,
  onSort,
  children,
}: SortableHeaderCellProps) {
  if (!sortable) {
    return <TableHead className={alignClass(align)}>{children}</TableHead>
  }
  const Arrow = direction === "desc" ? ArrowDown : ArrowUp
  return (
    <TableHead className={alignClass(align)} aria-sort={ariaSort(direction)}>
      <button
        type="button"
        onClick={(event) => onSort(event.shiftKey)}
        className={cn(
          "-mx-1.5 inline-flex items-center gap-1 rounded-md px-1.5 py-1 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring/50",
          align === "right" && "flex-row-reverse"
        )}
      >
        <span>{children}</span>
        {direction !== undefined && (
          <span className="inline-flex items-center text-xs text-muted-foreground">
            <Arrow aria-hidden className="size-3.5" />
            <span className="num">{priority}</span>
          </span>
        )}
      </button>
    </TableHead>
  )
}
