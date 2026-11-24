import { ChevronDownIcon, ChevronRightIcon } from "lucide-react"
import { useState } from "react"

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
  "Unrealized Profit",
  "Realized Profit",
  "Total profit",
  "Return on Cost",
  "Last sale",
]

type TiersBreakdownTableProps = { breakdown: TiersBreakdown }

// TiersBreakdownTable shows the Tiers in order and the "No Tier" group last. Each Instrument has a Holding row and
// a Closed Positions row. The Closed Positions row expands in place. Each Tier has a sum row, and the table has a total row.
// The view of all Accounts uses the same table.
export function TiersBreakdownTable({ breakdown }: TiersBreakdownTableProps) {
  const [expanded, setExpanded] = useState<ReadonlySet<string>>(new Set())
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

  return (
    <Table className="text-xs">
      <TableHeader>
        <TableRow>
          {headers.map((header, index) => (
            <TableHead key={header} className={index > 0 ? "text-right" : ""}>
              {header}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      {groups.map((group) => (
        <GroupRows
          key={group.name}
          group={group}
          currency={currency}
          expanded={expanded}
          onToggle={toggle}
        />
      ))}
      <TableFooter>
        <SumRow label="Total" sum={breakdown.total} currency={currency} />
      </TableFooter>
    </Table>
  )
}

type GroupRowsProps = {
  group: BreakdownGroup
  currency: string
  expanded: ReadonlySet<string>
  onToggle: (key: string) => void
}

// GroupRows shows the heading row, the Instruments and the sum row of one Tier.
function GroupRows({ group, currency, expanded, onToggle }: GroupRowsProps) {
  return (
    <TableBody>
      <TableRow className="bg-muted/50 hover:bg-muted/50">
        <TableCell
          colSpan={headers.length}
          className="font-mono text-xs tracking-wider uppercase"
        >
          {group.name}
        </TableCell>
      </TableRow>
      {group.instruments.map((instrument) => {
        const key = `${group.name}:${instrument.ticker}`
        return (
          <InstrumentRows
            key={key}
            instrument={instrument}
            currency={currency}
            open={expanded.has(key)}
            onToggle={() => onToggle(key)}
          />
        )
      })}
      <SumRow
        label={`Sum of ${group.name}`}
        sum={group.sum}
        currency={currency}
      />
    </TableBody>
  )
}

type InstrumentRowsProps = {
  instrument: BreakdownInstrument
  currency: string
  open: boolean
  onToggle: () => void
}

// InstrumentRows shows the Holding row and the Closed Positions row of one Instrument, and the single Closed Positions
// when the row is open.
function InstrumentRows({
  instrument,
  currency,
  open,
  onToggle,
}: InstrumentRowsProps) {
  const { holding, closedPositions } = instrument
  return (
    <>
      {holding && (
        <HoldingRow
          instrument={instrument}
          holding={holding}
          currency={currency}
        />
      )}
      {closedPositions && (
        <ClosedRows
          instrument={instrument}
          closed={closedPositions}
          currency={currency}
          open={open}
          onToggle={onToggle}
          showName={holding === null}
        />
      )}
    </>
  )
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
  instrument,
  holding,
  currency,
}: {
  instrument: BreakdownInstrument
  holding: BreakdownHolding
  currency: string
}) {
  return (
    <TableRow>
      <TableCell className="pl-8">
        <InstrumentCell instrument={instrument} label="Holding" showName />
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
        <Dash />
      </NumberCell>
      <NumberCell>
        <Profit amount={holding.unrealizedProfit} currency={currency} />
      </NumberCell>
      <NumberCell>
        <Percent percent={holding.returnOnCost} />
      </NumberCell>
      <NumberCell>
        <Dash />
      </NumberCell>
    </TableRow>
  )
}

type ClosedRowsProps = {
  instrument: BreakdownInstrument
  closed: BreakdownClosedPositions
  currency: string
  open: boolean
  onToggle: () => void
  showName: boolean
}

// ClosedRows shows the Closed Positions row of an Instrument. The button opens the single Closed Positions below it.
function ClosedRows({
  instrument,
  closed,
  currency,
  open,
  onToggle,
  showName,
}: ClosedRowsProps) {
  const Chevron = open ? ChevronDownIcon : ChevronRightIcon
  const label = `${closed.count} Closed ${closed.count === 1 ? "Position" : "Positions"}`
  return (
    <>
      <TableRow>
        <TableCell className="pl-2">
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
          <Dash />
        </NumberCell>
        <NumberCell>
          <Profit amount={closed.realizedProfit} currency={currency} />
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
                <span className="ml-2">{position.account.name}</span>
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
              <Dash />
            </NumberCell>
            <NumberCell>
              <Profit amount={position.realizedProfit} currency={currency} />
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

// SumRow shows a sum of a Tier or of the whole table. Its Return on Cost uses the cost of the open Lots with a price
// and of the Closed Positions.
function SumRow({
  label,
  sum,
  currency,
}: {
  label: string
  sum: BreakdownSum
  currency: string
}) {
  return (
    <TableRow className="font-medium">
      <TableCell>{label}</TableCell>
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
        <Profit amount={sum.unrealizedProfit} currency={currency} />
      </NumberCell>
      <NumberCell>
        <Profit amount={sum.realizedProfit} currency={currency} />
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
