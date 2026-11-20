import type { ReactNode } from "react"

import { AllocationDonut } from "@/components/allocation-donut"
import { FigureLabel } from "@/components/figure-label"
import { InsetSurface } from "@/components/inset-surface"
import { ResultBadge } from "@/components/result-badge"
import { Card, CardContent } from "@/components/ui/card"
import type { Loaded } from "@/hooks/use-load"
import type { Allocation, Returns } from "@/lib/api"
import {
  amountSign,
  formatAmount,
  formatPercent,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"

// OverviewFigures are the figures of the hero of an Overview, in one currency.
// An amount is null when the tool cannot calculate it. The hero then shows "—".
export type OverviewFigures = Returns & {
  currency: string
  value: string | null
  totalResult: string | null
  cash: string | null
  netDeposits: string | null
  unpricedHoldings: number
}

type OverviewHeroProps = {
  figures: OverviewFigures
  // allocation is the load of the Allocation that the right column shows.
  allocation: Loaded<Allocation>
}

// OverviewHero shows the value, the Total Result and the Annual Return at the top of an Overview.
// A second row shows the Simple Return, the cash and the Net Deposits.
// The right column shows the Allocation by Instrument.
export function OverviewHero({ figures, allocation }: OverviewHeroProps) {
  const unpriced = unpricedNote(figures.unpricedHoldings)

  return (
    <div className="flex flex-col gap-4">
      <Card className="py-0">
        <CardContent className="grid gap-6 p-5 md:p-7 xl:grid-cols-[minmax(0,1fr)_20rem] xl:gap-8">
          <div className="flex min-w-0 flex-col gap-6">
            <div className="flex flex-col gap-2">
              <FigureLabel>Value</FigureLabel>
              <p className="num text-3xl font-semibold tracking-tight sm:text-4xl md:text-5xl xl:text-6xl">
                {amountText(figures.value, figures.currency)}
              </p>
              {unpriced && (
                <p className="text-sm text-muted-foreground">{unpriced}</p>
              )}
            </div>
            <div className="grid gap-4 border-t pt-6 sm:grid-cols-2 md:grid-cols-1 lg:grid-cols-2">
              <FigureBox label="Total Result" note={unpriced}>
                <SignedFigure
                  value={figures.totalResult}
                  currency={figures.currency}
                  className="text-2xl md:text-3xl xl:text-2xl 2xl:text-3xl"
                />
              </FigureBox>
              <FigureBox
                label="Annual Return"
                note={
                  figures.annualReturn === null
                    ? (figures.annualReturnNote ?? undefined)
                    : undefined
                }
              >
                <ResultBadge
                  value={figures.annualReturn}
                  className="text-lg md:text-xl"
                />
              </FigureBox>
            </div>
          </div>
          <AllocationDonut allocation={allocation} />
        </CardContent>
      </Card>
      <div className="grid gap-4 md:grid-cols-3">
        <DetailCard
          label="Simple Return"
          note={
            figures.simpleReturn === null
              ? (figures.simpleReturnNote ?? undefined)
              : undefined
          }
        >
          <SignedFigure value={figures.simpleReturn} className="text-2xl" />
        </DetailCard>
        <DetailCard label="Cash">
          <p className="num text-2xl font-semibold">
            {amountText(figures.cash, figures.currency)}
          </p>
        </DetailCard>
        <DetailCard label="Net Deposits">
          <p className="num text-2xl font-semibold">
            {amountText(figures.netDeposits, figures.currency)}
          </p>
        </DetailCard>
      </div>
    </div>
  )
}

type FigureBoxProps = {
  label: string
  // note is a short text under the figure, or undefined.
  note?: string
  children: ReactNode
}

// FigureBox shows one medium figure of the hero.
function FigureBox({ label, note, children }: FigureBoxProps) {
  return (
    <InsetSurface className="flex min-w-0 flex-col gap-1.5 p-4">
      <p className="text-sm text-muted-foreground">{label}</p>
      {children}
      {note && <p className="text-sm text-muted-foreground">{note}</p>}
    </InsetSurface>
  )
}

// DetailCard shows one figure of the second row.
function DetailCard({ label, note, children }: FigureBoxProps) {
  return (
    <Card>
      <CardContent className="flex min-w-0 flex-col gap-1.5">
        <FigureLabel>{label}</FigureLabel>
        {children}
        {note && <p className="text-sm text-muted-foreground">{note}</p>}
      </CardContent>
    </Card>
  )
}

type SignedFigureProps = {
  // value is the decimal text, or null when the tool cannot calculate it.
  value: string | null
  // currency is set for an amount and omitted for a percent.
  currency?: string
  className?: string
}

// SignedFigure shows a result with its sign, in the colour of the sign. It shows a percent when it has no currency.
function SignedFigure({ value, currency, className }: SignedFigureProps) {
  let text = "—"
  if (value !== null) {
    text =
      currency === undefined
        ? formatPercent(value, { signed: true })
        : formatAmount(value, currency, { signed: true })
  }
  return (
    <p
      className={cn(
        "num font-semibold",
        value !== null && signColour(amountSign(value)),
        className
      )}
    >
      {text}
    </p>
  )
}

// amountText shows an amount, or "—" when the tool cannot calculate it.
function amountText(amount: string | null, currency: string): string {
  return amount === null ? "—" : formatAmount(amount, currency)
}

// unpricedNote tells that the amount leaves out the Holdings with no value.
export function unpricedNote(count: number): string | undefined {
  if (count === 0) return undefined
  return count === 1
    ? "Without 1 Holding that has no value."
    : `Without ${count} Holdings that have no value.`
}
