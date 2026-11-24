import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import type { Tier, Tiers } from "@/lib/api"
import { formatPercent } from "@/lib/format"

// TiersList shows the saved Tiers in the order of their order numbers, each with its Instrument Targets and its
// Shared Part. It is read-only. The "Edit targets" mode changes the Tiers.
export function TiersList({ saved }: { saved: Tiers }) {
  const open = new Map(
    saved.instruments.map((i) => [i.ticker, i.hasOpenHolding])
  )
  return (
    <div className="grid gap-4 md:grid-cols-2">
      {saved.tiers.map((tier) => (
        <TierCard key={tier.order} tier={tier} open={open} />
      ))}
    </div>
  )
}

// NoOpenHoldingBadge marks an Instrument that has no open Holding.
export function NoOpenHoldingBadge() {
  return <Badge variant="outline">No open Holding</Badge>
}

type TierCardProps = { tier: Tier; open: Map<string, boolean> }

// TierCard shows one Tier.
function TierCard({ tier, open }: TierCardProps) {
  const sharedName = tier.sharedPart.name || "Shared Part"
  return (
    <Card>
      <CardHeader>
        <CardTitle className="flex min-w-0 flex-wrap items-center gap-2">
          <span className="num font-mono text-muted-foreground">
            #{tier.order}
          </span>
          <span className="min-w-0 break-words">{tier.name}</span>
          <Badge variant="secondary" className="ml-auto num">
            {formatPercent(tier.targetShare)}
          </Badge>
        </CardTitle>
        <CardDescription>Target Share of all Holdings</CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        {tier.instrumentTargets.length > 0 && (
          <ul className="flex flex-col gap-1.5">
            {tier.instrumentTargets.map((target) => (
              <li
                key={target.ticker}
                className="flex min-w-0 items-center justify-between gap-3 text-sm"
              >
                <span className="flex min-w-0 flex-wrap items-center gap-2">
                  <span className="font-mono">{target.ticker}</span>
                  <span className="min-w-0 truncate text-muted-foreground">
                    {target.name}
                  </span>
                  {open.get(target.ticker) === false && <NoOpenHoldingBadge />}
                </span>
                <span className="num">{formatPercent(target.targetShare)}</span>
              </li>
            ))}
          </ul>
        )}
        <div className="flex flex-col gap-1.5 rounded-lg border bg-muted/40 p-3">
          <div className="flex items-center justify-between gap-3 text-sm font-medium">
            <span className="min-w-0 break-words">{sharedName}</span>
            <span className="num">
              {formatPercent(tier.sharedPart.targetShare)}
            </span>
          </div>
          {tier.sharedInstruments.length === 0 ? (
            <p className="text-xs text-muted-foreground">No Instruments.</p>
          ) : (
            <ul className="flex flex-wrap gap-x-3 gap-y-1 text-xs">
              {tier.sharedInstruments.map((instrument) => (
                <li
                  key={instrument.ticker}
                  className="flex items-center gap-1.5"
                >
                  <span className="font-mono">{instrument.ticker}</span>
                  {open.get(instrument.ticker) === false && (
                    <NoOpenHoldingBadge />
                  )}
                </li>
              ))}
            </ul>
          )}
        </div>
      </CardContent>
    </Card>
  )
}
