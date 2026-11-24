import { Fragment } from "react"

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
import { amountSign, formatAmount, formatShare, signColour } from "@/lib/format"
import { cn } from "@/lib/utils"

// RebalancingTable shows each Tier with its Instrument Targets and its Shared Part, and the "No Tier" group.
// The amounts of a part are in PLN. Each Planned Purchase is in the currency of its Account.
export function RebalancingTable({ data }: { data: Rebalancing }) {
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
              <PartRow
                strong
                label={tier.name}
                part={tier}
                purchases={tier.plannedPurchases}
              />
              {tier.instrumentTargets.map((target) => (
                <PartRow
                  key={target.ticker}
                  indent
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
                  <>
                    {tier.sharedPart.name || "Shared Part"}
                    {tier.sharedPart.name && (
                      <span className="text-muted-foreground">
                        {" "}
                        (Shared Part)
                      </span>
                    )}
                  </>
                }
                part={tier.sharedPart}
                purchases={[tier.sharedPart.plannedPurchase]}
              />
              {tier.sharedPart.instruments.map((instrument) => (
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
  purchases: RebalancingPurchase[]
  // strong marks a Tier or the "No Tier" group.
  strong?: boolean
  // indent marks a part inside a Tier.
  indent?: boolean
}

// PartRow shows the value, the shares, the Gap and the Planned Purchases of one part.
function PartRow({ label, part, purchases, strong, indent }: PartRowProps) {
  return (
    <TableRow className={cn(strong && "bg-muted/40 font-medium")}>
      <TableCell className={cn("whitespace-normal", indent && "pl-8")}>
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
        <PurchaseCell purchases={purchases} />
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
        <span className={cn("num", signColour(sign))}>
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
