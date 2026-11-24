import {
  closestCenter,
  DndContext,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
  type Announcements,
  type CollisionDetection,
  type DragEndEvent,
} from "@dnd-kit/core"
import { restrictToVerticalAxis } from "@dnd-kit/modifiers"
import {
  arrayMove,
  SortableContext,
  sortableKeyboardCoordinates,
  useSortable,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable"
import { CSS } from "@dnd-kit/utilities"
import {
  ChevronDownIcon,
  ChevronRightIcon,
  GripVerticalIcon,
} from "lucide-react"
import { useState } from "react"

import { Button } from "@/components/ui/button"
import { useBreakdownOrder } from "@/hooks/use-breakdown-order"
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type {
  BreakdownClosedPositions,
  BreakdownGroup,
  BreakdownHolding,
  BreakdownInstrument,
  BreakdownSum,
  TiersBreakdown,
} from "@/lib/api"
import {
  amountSign,
  formatAmount,
  formatDate,
  formatPercent,
  formatPolishDate,
  formatVolume,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"

// Dash marks a cell that has no figure.
function Dash() {
  return <span className="text-muted-foreground">–</span>
}

// Amount shows a plain amount, or a dash when there is none.
function Amount({
  amount,
  currency,
}: {
  amount: string | null
  currency: string
}) {
  if (amount === null) return <Dash />
  return <span className="num">{formatAmount(amount, currency)}</span>
}

// Profit shows a profit or a loss with its sign and colour, or a dash when there is none.
function Profit({
  amount,
  currency,
}: {
  amount: string | null
  currency: string
}) {
  if (amount === null) return <Dash />
  return (
    <span className={cn("num", signColour(amountSign(amount)))}>
      {formatAmount(amount, currency, { signed: true })}
    </span>
  )
}

// Percent shows a Return on Cost with its sign and colour, or a dash when there is none.
function Percent({ percent }: { percent: string | null }) {
  if (percent === null) return <Dash />
  return (
    <span className={cn("num", signColour(amountSign(percent)))}>
      {formatPercent(percent, { signed: true })}
    </span>
  )
}

// NumberCell is a right-aligned cell of the table.
function NumberCell({
  children,
  className,
}: {
  children: React.ReactNode
  className?: string
}) {
  return (
    <TableCell className={cn("text-right", className)}>{children}</TableCell>
  )
}

// The columns of the table. A row fills only the cells that it has.
const headers = [
  "Instrument",
  "Value",
  "Cost",
  "Sale amount",
  "Profit",
  "Return on Cost",
  "Last purchase or sale",
]

type TiersBreakdownTableProps = {
  breakdown: TiersBreakdown
  // view is "all" for all Accounts, or the id of one Account. It keys the manual order of the rows.
  view: string
}

type RowGroup = "holding" | "closed"

// DRAG_DISTANCE is the distance in pixels that the pointer moves before a drag starts.
const DRAG_DISTANCE = 4

// slotOf names one Tier and one group of rows. It keys the manual order.
function slotOf(group: BreakdownGroup, rows: RowGroup) {
  return `${group.order ?? "none"}:${rows}`
}

// rowId is the drag id of a row. The slot comes first, so a drag can stay in its slot.
function rowId(slot: string, ticker: string) {
  return `${slot}|${ticker}`
}

// orderRows puts the rows of the manual order first. The other rows follow by date, newest first, then by ticker.
function orderRows(
  instruments: BreakdownInstrument[],
  rows: RowGroup,
  manual: readonly string[] | undefined
) {
  const date = (instrument: BreakdownInstrument) =>
    (rows === "holding"
      ? instrument.holding?.lastPurchaseDate
      : instrument.closedPositions?.lastSaleDate) ?? ""
  const own = instruments.filter((instrument) =>
    rows === "holding" ? instrument.holding : instrument.closedPositions
  )
  const byDate = [...own].sort(
    (a, b) => date(b).localeCompare(date(a)) || a.ticker.localeCompare(b.ticker)
  )
  const position = new Map((manual ?? []).map((ticker, i) => [ticker, i]))
  const stored = byDate
    .filter((instrument) => position.has(instrument.ticker))
    .sort((a, b) => position.get(a.ticker)! - position.get(b.ticker)!)
  const rest = byDate.filter((instrument) => !position.has(instrument.ticker))
  return [...stored, ...rest]
}

// TiersBreakdownTable shows the Tiers in order and the "No Tier" group last. Each Tier row shows the Tier sums. Each Tier has a group of Holding rows,
// then a group of Closed Positions rows. The rows follow the date order until the user moves a row.
// A row moves only inside its group and its Tier. The Closed Positions row expands in place. The table has a total row.
// The view of all Accounts uses the same table.
export function TiersBreakdownTable({
  breakdown,
  view,
}: TiersBreakdownTableProps) {
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set())
  const { order, hasManualOrder, setSlot, clear } = useBreakdownOrder(view)
  const toggle = (key: string) =>
    setExpanded((current) => {
      const next = new Set(current)
      if (!next.delete(key)) next.add(key)
      return next
    })
  const groups = breakdown.noTier
    ? [...breakdown.tiers, breakdown.noTier]
    : breakdown.tiers
  const { currency } = breakdown

  // visible holds the tickers of each slot in the shown order.
  const visible = new Map<string, string[]>()
  const ordered = groups.map((group) => {
    const holding = orderRows(
      group.instruments,
      "holding",
      order[slotOf(group, "holding")]
    )
    const closed = orderRows(
      group.instruments,
      "closed",
      order[slotOf(group, "closed")]
    )
    visible.set(
      slotOf(group, "holding"),
      holding.map((i) => i.ticker)
    )
    visible.set(
      slotOf(group, "closed"),
      closed.map((i) => i.ticker)
    )
    return { group, holding, closed }
  })

  const sensors = useSensors(
    useSensor(PointerSensor, {
      activationConstraint: { distance: DRAG_DISTANCE },
    }),
    useSensor(KeyboardSensor, {
      coordinateGetter: sortableKeyboardCoordinates,
    })
  )
  const slotOfId = (id: string | number) => String(id).split("|")[0]
  const tickerOfId = (id: string | number) =>
    String(id).slice(String(id).indexOf("|") + 1)

  // Only the rows of the slot of the dragged row can be a drop target.
  const collisionDetection: CollisionDetection = (args) =>
    closestCenter({
      ...args,
      droppableContainers: args.droppableContainers.filter(
        (container) => slotOfId(container.id) === slotOfId(args.active.id)
      ),
    })

  function handleDragEnd({ active, over }: DragEndEvent) {
    if (!over || active.id === over.id) return
    const slot = slotOfId(active.id)
    if (slotOfId(over.id) !== slot) return
    const tickers = visible.get(slot) ?? []
    const from = tickers.indexOf(tickerOfId(active.id))
    const to = tickers.indexOf(tickerOfId(over.id))
    if (from < 0 || to < 0) return
    setSlot(slot, arrayMove(tickers, from, to))
  }

  const announcements: Announcements = {
    onDragStart: ({ active }) => `Picked up the row ${tickerOfId(active.id)}.`,
    onDragOver: ({ active, over }) =>
      over
        ? `The row ${tickerOfId(active.id)} is over the row ${tickerOfId(over.id)}.`
        : undefined,
    onDragEnd: ({ active, over }) =>
      over && over.id !== active.id
        ? `The row ${tickerOfId(active.id)} was dropped at the place of the row ${tickerOfId(over.id)}.`
        : `The row ${tickerOfId(active.id)} was dropped. Its place did not change.`,
    onDragCancel: ({ active }) =>
      `Moving the row ${tickerOfId(active.id)} was cancelled.`,
  }

  return (
    <div className="flex min-w-0 flex-col gap-3">
      {hasManualOrder && (
        <div className="flex justify-end">
          <Button variant="outline" size="sm" onClick={clear}>
            Restore date order
          </Button>
        </div>
      )}
      <DndContext
        sensors={sensors}
        collisionDetection={collisionDetection}
        modifiers={[restrictToVerticalAxis]}
        onDragEnd={handleDragEnd}
        accessibility={{ announcements }}
      >
        <Table className="text-xs">
          <TableHeader>
            <TableRow>
              {headers.map((header, index) => (
                <TableHead
                  key={header}
                  className={index > 0 ? "text-right" : ""}
                >
                  {header}
                </TableHead>
              ))}
            </TableRow>
          </TableHeader>
          {ordered.map(({ group, holding, closed }, index) => (
            <GroupRows
              first={index === 0}
              key={group.name}
              group={group}
              holding={holding}
              closed={closed}
              currency={currency}
              expanded={expanded}
              onToggle={toggle}
            />
          ))}
          <TableFooter>
            <SumRow
              label="Total"
              sum={breakdown.total}
              currency={currency}
              variant="total"
            />
          </TableFooter>
        </Table>
      </DndContext>
    </div>
  )
}

type GroupRowsProps = {
  group: BreakdownGroup
  holding: BreakdownInstrument[]
  closed: BreakdownInstrument[]
  currency: string
  expanded: ReadonlySet<string>
  onToggle: (key: string) => void
  first: boolean
}

// GroupRows shows the Tier row with the Tier sums, the Holding rows and the Closed Positions rows of one Tier.
function GroupRows({
  group,
  holding,
  closed,
  currency,
  expanded,
  onToggle,
  first,
}: GroupRowsProps) {
  const holdingSlot = slotOf(group, "holding")
  const closedSlot = slotOf(group, "closed")
  return (
    <TableBody>
      {!first && (
        <TableRow aria-hidden className="h-8 border-0 hover:bg-transparent">
          <TableCell colSpan={headers.length} className="p-0" />
        </TableRow>
      )}
      <SumRow
        label={group.name}
        sum={group.sum}
        currency={currency}
        variant="tier"
      />
      <SortableContext
        items={holding.map((i) => rowId(holdingSlot, i.ticker))}
        strategy={verticalListSortingStrategy}
      >
        {holding.map((instrument) => (
          <HoldingRow
            key={instrument.ticker}
            id={rowId(holdingSlot, instrument.ticker)}
            instrument={instrument}
            holding={instrument.holding!}
            currency={currency}
          />
        ))}
      </SortableContext>
      <SortableContext
        items={closed.map((i) => rowId(closedSlot, i.ticker))}
        strategy={verticalListSortingStrategy}
      >
        {closed.map((instrument) => {
          const key = `${group.name}:${instrument.ticker}`
          return (
            <ClosedRows
              key={instrument.ticker}
              id={rowId(closedSlot, instrument.ticker)}
              instrument={instrument}
              closed={instrument.closedPositions!}
              currency={currency}
              open={expanded.has(key)}
              onToggle={() => onToggle(key)}
              showName={instrument.holding === null}
            />
          )
        })}
      </SortableContext>
    </TableBody>
  )
}

// useSortableRow gives the props that make a table row movable by its handle.
function useSortableRow(id: string, ticker: string) {
  const {
    attributes,
    listeners,
    setNodeRef,
    setActivatorNodeRef,
    transform,
    transition,
    isDragging,
  } = useSortable({ id })
  return {
    rowProps: {
      ref: setNodeRef,
      style: { transform: CSS.Translate.toString(transform), transition },
      className: isDragging ? "relative z-10 bg-muted shadow-md" : undefined,
    },
    handle: (
      <button
        type="button"
        ref={setActivatorNodeRef}
        {...attributes}
        {...listeners}
        aria-label={`Move row ${ticker}`}
        className={cn(
          "inline-flex shrink-0 cursor-grab touch-none items-center rounded-sm text-muted-foreground outline-none hover:text-foreground focus-visible:ring-2 focus-visible:ring-ring",
          isDragging && "cursor-grabbing"
        )}
      >
        <GripVerticalIcon aria-hidden className="size-4" />
      </button>
    ),
  }
}

// InstrumentCell shows the ticker of an Instrument with the label of the row. It shows the name when asked.
function InstrumentCell({
  instrument,
  label,
  showName,
}: {
  instrument: BreakdownInstrument
  label: string
  showName: boolean
}) {
  return (
    <div className="flex flex-col gap-0.5">
      <span className="font-medium">
        {instrument.ticker}
        <span className="ml-2 text-xs font-normal text-muted-foreground">
          {label}
        </span>
      </span>
      {showName && (
        <span className="max-w-32 truncate text-xs text-muted-foreground md:max-w-56">
          {instrument.name}
        </span>
      )}
    </div>
  )
}

// HoldingRow shows the value, the cost, the Unrealized Profit and the Return on Cost of a Holding.
// A Holding with no price shows "no value".
function HoldingRow({
  id,
  instrument,
  holding,
  currency,
}: {
  id: string
  instrument: BreakdownInstrument
  holding: BreakdownHolding
  currency: string
}) {
  const { rowProps, handle } = useSortableRow(id, instrument.ticker)
  return (
    <TableRow {...rowProps}>
      <TableCell className="pl-2">
        <div className="flex items-start gap-1">
          <span className="mt-0.5 flex">{handle}</span>
          <span className="size-4 shrink-0" aria-hidden />
          <InstrumentCell instrument={instrument} label="Holding" showName />
        </div>
      </TableCell>
      <NumberCell>
        {holding.noValue ? (
          <span className="text-muted-foreground">no value</span>
        ) : (
          <Amount amount={holding.value} currency={currency} />
        )}
      </NumberCell>
      <NumberCell>
        <Amount amount={holding.cost} currency={currency} />
      </NumberCell>
      <NumberCell>
        <Dash />
      </NumberCell>
      <NumberCell>
        <Profit amount={holding.unrealizedProfit} currency={currency} />
      </NumberCell>
      <NumberCell>
        <Percent percent={holding.returnOnCost} />
      </NumberCell>
      <NumberCell className="num text-muted-foreground">
        {formatDate(holding.lastPurchaseDate)}
      </NumberCell>
    </TableRow>
  )
}

type ClosedRowsProps = {
  id: string
  instrument: BreakdownInstrument
  closed: BreakdownClosedPositions
  currency: string
  open: boolean
  onToggle: () => void
  showName: boolean
}

// ClosedRows shows the Closed Positions row of an Instrument. The button opens the single Closed Positions below it.
function ClosedRows({
  id,
  instrument,
  closed,
  currency,
  open,
  onToggle,
  showName,
}: ClosedRowsProps) {
  const { rowProps, handle } = useSortableRow(id, instrument.ticker)
  const Chevron = open ? ChevronDownIcon : ChevronRightIcon
  const label = `${closed.count} Closed ${closed.count === 1 ? "Position" : "Positions"}`
  return (
    <>
      <TableRow {...rowProps}>
        <TableCell className="pl-2">
          <div className="flex items-start gap-1">
            <span className="mt-0.5 flex">{handle}</span>
            <button
              type="button"
              aria-expanded={open}
              aria-label={`${label} of ${instrument.ticker}`}
              onClick={onToggle}
              className="flex items-start gap-1 rounded-sm text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
            >
              <Chevron className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
              <InstrumentCell
                instrument={instrument}
                label={label}
                showName={showName}
              />
            </button>
          </div>
        </TableCell>
        <NumberCell>
          <Dash />
        </NumberCell>
        <NumberCell>
          <Amount amount={closed.cost} currency={currency} />
        </NumberCell>
        <NumberCell>
          <Amount amount={closed.saleAmount} currency={currency} />
        </NumberCell>
        <NumberCell>
          <Profit amount={closed.realizedProfit} currency={currency} />
        </NumberCell>
        <NumberCell>
          <Percent percent={closed.returnOnCost} />
        </NumberCell>
        <NumberCell className="num text-muted-foreground">
          {formatDate(closed.lastSaleDate)}
        </NumberCell>
      </TableRow>
      {open &&
        closed.positions.map((position) => (
          <TableRow key={position.xtbId} className="bg-muted/30">
            <TableCell className="pl-12 text-muted-foreground">
              <span className="num">
                {formatPolishDate(position.openTime)} →{" "}
                {formatPolishDate(position.time)}
              </span>
              <span className="ml-2 num">{formatVolume(position.volume)}</span>
              {position.account !== undefined && (
                <span className="block max-w-64 truncate">
                  {position.account.name}
                </span>
              )}
            </TableCell>
            <NumberCell>
              <Dash />
            </NumberCell>
            <NumberCell>
              <Amount amount={position.cost} currency={currency} />
            </NumberCell>
            <NumberCell>
              <Amount amount={position.amount} currency={currency} />
            </NumberCell>
            <NumberCell>
              <Profit amount={position.realizedProfit} currency={currency} />
            </NumberCell>
            <NumberCell>
              <Dash />
            </NumberCell>
            <NumberCell className="num text-muted-foreground">
              {formatPolishDate(position.time)}
            </NumberCell>
          </TableRow>
        ))}
    </>
  )
}

// SumRow shows the sums in the Tier row, or in the Total row of the whole table. Its Return on Cost uses the cost of the open Lots with a price
// and of the Closed Positions.
function SumRow({
  label,
  sum,
  currency,
  variant,
}: {
  label: string
  sum: BreakdownSum
  currency: string
  variant: "tier" | "total"
}) {
  return (
    <TableRow
      className={cn(
        variant === "tier" && "bg-muted/50 font-semibold hover:bg-muted/50",
        variant === "total" &&
          "border-t-2 border-foreground/40 bg-muted/50 text-sm font-bold hover:bg-muted/50"
      )}
    >
      <TableCell
        className={
          variant === "tier" ? "font-mono tracking-wider uppercase" : ""
        }
      >
        {label}
      </TableCell>
      <NumberCell>
        <Amount amount={sum.value} currency={currency} />
      </NumberCell>
      <NumberCell>
        <Amount amount={sum.cost} currency={currency} />
      </NumberCell>
      <NumberCell>
        <Dash />
      </NumberCell>
      <NumberCell>
        <Profit amount={sum.totalProfit} currency={currency} />
      </NumberCell>
      <NumberCell>
        <Percent percent={sum.returnOnCost} />
      </NumberCell>
      <NumberCell>
        <Dash />
      </NumberCell>
    </TableRow>
  )
}
