import { ChevronDownIcon, ChevronRightIcon } from "lucide-react"
import { Fragment, useState } from "react"

import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type {
  Rebalancing,
  RebalancingHolding,
  RebalancingPart,
  RebalancingPurchase,
} from "@/lib/api"
import { amountSign, formatAmount, formatShare } from "@/lib/format"
import { cn } from "@/lib/utils"

// RebalancingTable shows each Tier with its Instrument Targets and its Shared Part, and the "No Tier" group.
// The amounts of a part are in PLN. Each Planned Purchase is in the currency of its Account.
export function RebalancingTable({ data }: { data: Rebalancing }) {
  // collapsed holds the order of each Tier whose Shared Part hides its Instruments.
  const [collapsed, setCollapsed] = useState<ReadonlySet<number>>(new Set())
  const toggle = (order: number) =>
    setCollapsed((current) => {
      const next = new Set(current)
      if (!next.delete(order)) next.add(order)
      return next
    })
  return (
    <div className="min-w-0 overflow-x-auto rounded-lg border">
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Part</TableHead>
            <TableHead className="text-right">Value</TableHead>
            <TableHead className="text-right">Actual Share</TableHead>
            <TableHead className="text-right">Target Share</TableHead>
            <TableHead className="text-right">Gap</TableHead>
            <TableHead className="text-right">Planned Purchase</TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          {data.tiers.map((tier) => (
            <Fragment key={tier.order}>
              <PartRow strong label={tier.name} part={tier} />
              {tier.instrumentTargets.map((target) => (
                <PartRow
                  key={target.ticker}
                  indent
                  chevronGap
                  label={
                    <>
                      <span className="font-mono">{target.ticker}</span>{" "}
                      <span className="text-muted-foreground">
                        {target.name}
                      </span>
                      {target.noValue && <NoValueBadge />}
                    </>
                  }
                  part={target}
                  purchases={[target.plannedPurchase]}
                />
              ))}
              <PartRow
                indent
                label={
                  <SharedPartToggle
                    open={!collapsed.has(tier.order)}
                    onToggle={() => toggle(tier.order)}
                  >
                    {tier.sharedPart.name || "Shared Part"}
                    {tier.sharedPart.name && (
                      <span className="text-muted-foreground">
                        {" "}
                        (Shared Part)
                      </span>
                    )}
                  </SharedPartToggle>
                }
                part={tier.sharedPart}
                purchases={[tier.sharedPart.plannedPurchase]}
              />
              {!collapsed.has(tier.order) &&
                tier.sharedPart.instruments.map((instrument) => (
                  <HoldingRow key={instrument.ticker} holding={instrument} />
                ))}
            </Fragment>
          ))}
          {data.noTier && data.noTier.holdings.length > 0 && (
            <>
              <PartRow
                strong
                label="No Tier"
                part={data.noTier}
                purchases={[]}
              />
              {data.noTier.holdings.map((holding) => (
                <HoldingRow key={holding.ticker} holding={holding} />
              ))}
            </>
          )}
        </TableBody>
      </Table>
    </div>
  )
}

type PartRowProps = {
  label: React.ReactNode
  part: RebalancingPart
  // purchases is missing for a Tier, which shows no Planned Purchase.
  purchases?: RebalancingPurchase[]
  // strong marks a Tier or the "No Tier" group.
  strong?: boolean
  // indent marks a part inside a Tier.
  indent?: boolean
  // chevronGap shifts the label right by the empty edge of the chevron icon.
  chevronGap?: boolean
}

// PartRow shows the value, the shares, the Gap and the Planned Purchases of one part.
function PartRow({
  label,
  part,
  purchases,
  strong,
  indent,
  chevronGap,
}: PartRowProps) {
  return (
    <TableRow className={cn(strong && "bg-muted/40 font-medium")}>
      <TableCell
        className={cn(
          "whitespace-normal",
          indent && (chevronGap ? "pl-9" : "pl-8")
        )}
      >
        {label}
      </TableCell>
      <TableCell className="text-right num">
        {formatAmount(part.value, "PLN")}
      </TableCell>
      <TableCell className="text-right num">
        {formatShare(part.actualShare)}
      </TableCell>
      <TableCell className="text-right num">
        {formatShare(part.targetShare)}
      </TableCell>
      <TableCell className="text-right">
        <GapCell gap={part.gap} />
      </TableCell>
      <TableCell className="text-right">
        {purchases && <PurchaseCell purchases={purchases} />}
      </TableCell>
    </TableRow>
  )
}

// GapCell shows a Gap with its sign. A negative Gap has the negative colour and the label "Over target".
function GapCell({ gap }: { gap: string }) {
  const sign = amountSign(gap)
  if (sign === "negative") {
    return (
      <span className="flex flex-col items-end gap-0.5">
        <span className="num">
          {formatAmount(gap, "PLN", { signed: true })}
        </span>
        <span className="text-xs text-muted-foreground">Over target</span>
      </span>
    )
  }
  return (
    <span className="num">{formatAmount(gap, "PLN", { signed: true })}</span>
  )
}

// PurchaseCell shows each Planned Purchase in the currency of its Account. A zero purchase and a missing Account show a dash.
function PurchaseCell({ purchases }: { purchases: RebalancingPurchase[] }) {
  const shown = purchases.filter(
    (p) => p.amount !== null && amountSign(p.amount) === "positive"
  )
  if (shown.length === 0) {
    return <span className="text-muted-foreground">—</span>
  }
  return (
    <span className="flex flex-col items-end">
      {shown.map((p) => (
        <span key={p.currency} className="num font-medium">
          {formatAmount(p.amount ?? "0", p.currency)}
        </span>
      ))}
    </span>
  )
}

// HoldingRow shows an Instrument of a Shared Part or of the "No Tier" group with its value and its share. It has no Planned Purchase.
function HoldingRow({ holding }: { holding: RebalancingHolding }) {
  return (
    <TableRow className="text-muted-foreground">
      <TableCell className="pl-14 whitespace-normal">
        <span className="font-mono">{holding.ticker}</span> {holding.name}
        {holding.noValue && <NoValueBadge />}
      </TableCell>
      <TableCell className="text-right num">
        {holding.value === null ? "—" : formatAmount(holding.value, "PLN")}
      </TableCell>
      <TableCell className="text-right num">
        {holding.share === null ? "—" : formatShare(holding.share)}
      </TableCell>
      <TableCell />
      <TableCell />
      <TableCell />
    </TableRow>
  )
}

// NoValueBadge marks an Instrument that has no price.
function NoValueBadge() {
  return (
    <Badge variant="outline" className="ml-2">
      No value
    </Badge>
  )
}

// SharedPartToggle shows a chevron button that expands or collapses the Instruments of a Shared Part.
function SharedPartToggle({
  open,
  onToggle,
  children,
}: {
  open: boolean
  onToggle: () => void
  children: React.ReactNode
}) {
  const Chevron = open ? ChevronDownIcon : ChevronRightIcon
  return (
    <button
      type="button"
      aria-expanded={open}
      onClick={onToggle}
      className="flex items-start gap-1 rounded-sm text-left outline-none focus-visible:ring-2 focus-visible:ring-ring"
    >
      <Chevron className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <span>{children}</span>
    </button>
  )
}
