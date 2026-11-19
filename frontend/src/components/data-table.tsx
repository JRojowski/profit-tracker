import {
  closestCenter,
  type CollisionDetection,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type Announcements,
  type DragEndEvent,
  type ScreenReaderInstructions,
} from "@dnd-kit/core"
import { restrictToHorizontalAxis } from "@dnd-kit/modifiers"
import {
  arrayMove,
  horizontalListSortingStrategy,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import {
  columnOrderingFeature,
  FlexRender,
  rowSortingFeature,
  tableFeatures,
  useTable,
  type ColumnDef,
  type RowData,
  type SortingState,
} from "@tanstack/react-table"
import { ArrowDown, ArrowUp, GripVertical } from "lucide-react"
import type { KeyboardEvent, MouseEvent, ReactNode } from "react"

import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { nextSorting, type SortableColumn } from "@/lib/table-sort"
import type { TableViewProps } from "@/lib/table-view"
import { cn } from "@/lib/utils"

// ColumnMeta holds what the table needs to know about a column, besides the TanStack options.
type ColumnMeta = {
  // align "right" aligns the header, the cells and the footer cell to the right. Numbers use it.
  align?: "right"
}

const features = tableFeatures({
  rowSortingFeature,
  columnOrderingFeature,
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

type DataTableProps<TRow extends RowData> = TableViewProps & {
  columns: DataTableColumn<TRow>[]
  // rows are already sorted by the caller with sortRows. The table does not sort them.
  rows: TRow[]
  getRowId: (row: TRow) => string
  footer?: DataTableFooter
  // onRowAction makes each row reachable with Tab. A click on the row, or Enter on the focused row, runs it.
  // A click on a control inside the row, and a key press inside a control, do not run it.
  onRowAction?: (row: TRow) => void
}

// DRAG_DISTANCE is the pointer movement in pixels before a drag starts. A click moves less.
const DRAG_DISTANCE = 8

// SCREEN_READER_INSTRUCTIONS tell the user of a screen reader how to move a column.
const SCREEN_READER_INSTRUCTIONS: ScreenReaderInstructions = {
  draggable:
    "To move a column, press Space or Enter on its handle. Use the left and right arrow keys to move it. Press Space or Enter to drop it, or Escape to cancel.",
}

// alignClass gives the classes of a header, a cell or a footer cell for the alignment of its column.
function alignClass(align: "right" | undefined) {
  return align === "right" ? "text-right" : undefined
}

// cellAlignClass is alignClass for a cell. A right-aligned column holds numbers, so its cells get the num class.
function cellAlignClass(align: "right" | undefined) {
  return align === "right" ? "text-right num" : undefined
}

// interactiveSelector matches a control inside a row. A click on it belongs to the control, not to the row action.
const interactiveSelector =
  "button, a, input, select, textarea, label, [role='button'], [role='checkbox']"

// rowActionProps gives the props that make a table row run an action on a click and on Enter.
// Events of a dialog that a control in the row opens also reach the row through React, so the handlers check the DOM.
function rowActionProps(action: () => void) {
  return {
    tabIndex: 0,
    className:
      "cursor-pointer outline-none focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-inset",
    onClick: (event: MouseEvent<HTMLTableRowElement>) => {
      const target = event.target as Element
      if (!event.currentTarget.contains(target)) return
      if (target.closest(interactiveSelector)) return
      action()
    },
    onKeyDown: (event: KeyboardEvent<HTMLTableRowElement>) => {
      if (event.key !== "Enter" || event.target !== event.currentTarget) return
      event.preventDefault()
      action()
    },
  }
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
  onRowAction,
  columnOrder,
  onColumnOrderChange,
  fixedColumnIds,
}: DataTableProps<TRow>) {
  const table = useTable({
    features,
    columns,
    data: rows,
    getRowId,
    manualSorting: true,
    state: { sorting, columnOrder },
  })
  const sortable = new Set(
    columns.filter((column) => column.sortValue).map((column) => column.id)
  )
  const labels = new Map(columns.map((column) => [column.id, column.header]))
  const movableIds = columnOrder.filter((id) => !fixedColumnIds.includes(id))
  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: DRAG_DISTANCE },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )

  // Only the movable columns can be a drop target. A drag that ends in front of the first movable column drops nowhere.
  const collisionDetection: CollisionDetection = (args) => {
    const targets = args.droppableContainers.filter((container) =>
      movableIds.includes(String(container.id))
    )
    const lefts = targets.flatMap((container) => {
      const rect = args.droppableRects.get(container.id)
      return rect ? [rect.left] : []
    })
    const { collisionRect } = args
    if (
      lefts.length === 0 ||
      collisionRect.left + collisionRect.width / 2 < Math.min(...lefts)
    ) {
      return []
    }
    return closestCenter({ ...args, droppableContainers: targets })
  }

  function handleDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    const from = columnOrder.indexOf(String(active.id))
    const to = columnOrder.indexOf(String(over.id))
    if (from < 0 || to < 0 || !movableIds.includes(String(over.id))) return
    onColumnOrderChange(arrayMove(columnOrder, from, to))
  }

  const label = (id: string | number) => labels.get(String(id)) ?? String(id)
  const announcements: Announcements = {
    onDragStart: ({ active }) => `Picked up the column ${label(active.id)}.`,
    onDragOver: ({ active, over }) =>
      over
        ? `The column ${label(active.id)} is over the column ${label(over.id)}.`
        : undefined,
    onDragEnd: ({ active, over }) =>
      over && over.id !== active.id
        ? `The column ${label(active.id)} was dropped at the place of the column ${label(over.id)}.`
        : `The column ${label(active.id)} was dropped. Its place did not change.`,
    onDragCancel: ({ active }) =>
      `Moving the column ${label(active.id)} was cancelled.`,
  }

  return (
    <DndContext
      sensors={sensors}
      collisionDetection={collisionDetection}
      modifiers={[restrictToHorizontalAxis]}
      onDragEnd={handleDragEnd}
      accessibility={{
        announcements,
        screenReaderInstructions: SCREEN_READER_INSTRUCTIONS,
      }}
    >
      <Table>
        <TableHeader>
          {table.getHeaderGroups().map((headerGroup) => (
            <TableRow key={headerGroup.id}>
              <SortableContext
                items={movableIds}
                strategy={horizontalListSortingStrategy}
              >
                {headerGroup.headers.map((header) => (
                  <SortableHeaderCell
                    key={header.id}
                    id={header.column.id}
                    label={label(header.column.id)}
                    movable={!fixedColumnIds.includes(header.column.id)}
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
              </SortableContext>
            </TableRow>
          ))}
        </TableHeader>
        <TableBody>
          {table.getRowModel().rows.map((row) => (
            <TableRow
              key={row.id}
              {...(onRowAction
                ? rowActionProps(() => onRowAction(row.original))
                : {})}
            >
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
    </DndContext>
  )
}

type SortableHeaderCellProps = {
  id: string
  // label is the text of the header. The name of the move handle uses it.
  label: string
  // movable is false for a fixed column.
  movable: boolean
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
// A movable column has a grip handle for the keyboard. A pointer drag starts from the whole cell after a small movement.
function SortableHeaderCell({
  id,
  label,
  movable,
  align,
  sortable,
  direction,
  priority,
  onSort,
  children,
}: SortableHeaderCellProps) {
  const {
    attributes,
    listeners,
    setNodeRef,
    setActivatorNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id, disabled: !movable })
  const Arrow = direction === "desc" ? ArrowDown : ArrowUp
  const content = sortable ? (
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
  ) : (
    children
  )
  return (
    <TableHead
      ref={setNodeRef}
      style={{ transform: CSS.Translate.toString(transform), transition }}
      className={cn(
        "group/head relative",
        alignClass(align),
        isDragging && "z-10 bg-muted shadow-md"
      )}
      aria-sort={sortable ? ariaSort(direction) : undefined}
      {...(movable ? listeners : {})}
    >
      {movable && (
        <button
          type="button"
          ref={setActivatorNodeRef}
          {...attributes}
          aria-label={`Move column ${label}`}
          className={cn(
            "absolute top-1/2 left-0.5 z-10 inline-flex -translate-y-1/2 cursor-grab touch-none items-center rounded-sm bg-muted text-muted-foreground opacity-0 outline-none group-hover/head:opacity-100 focus-visible:opacity-100 focus-visible:ring-2 focus-visible:ring-ring/50",
            isDragging && "cursor-grabbing opacity-100"
          )}
        >
          <GripVertical aria-hidden className="size-3" />
        </button>
      )}
      {content}
    </TableHead>
  )
}
