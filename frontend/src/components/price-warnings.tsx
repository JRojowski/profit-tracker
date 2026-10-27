import { PencilIcon, TriangleAlertIcon } from "lucide-react"
import type { ComponentProps } from "react"

import { PriceSourceDialog } from "@/components/price-source-dialog"
import { PriceWarningText } from "@/components/price-warning-text"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import type { NamedPriceSource, PriceWarning } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { cn } from "@/lib/utils"

// warningBadge gives a badge the colours of a warning.
const warningBadge = "border-warning/40 bg-warning/10 text-warning"

// PriceWarningBadge marks an Instrument with a Price Warning. It is a plain badge, for example inside a row that is a button.
export function PriceWarningBadge({
  className,
  ...props
}: ComponentProps<"span">) {
  return (
    <Badge variant="outline" className={cn(warningBadge, className)} {...props}>
      <TriangleAlertIcon data-icon="inline-start" />
      Price Warning
    </Badge>
  )
}

type PriceWarningBadgeButtonProps = {
  instrument: NamedPriceSource
  warnings: PriceWarning[]
  onChanged: () => void
}

// PriceWarningBadgeButton is a badge that opens the price source dialog of the Instrument with its Price Warnings.
export function PriceWarningBadgeButton({
  instrument,
  warnings,
  onChanged,
}: PriceWarningBadgeButtonProps) {
  return (
    <PriceSourceDialog
      instrument={instrument}
      warnings={warnings}
      onChanged={onChanged}
      trigger={
        <Badge
          asChild
          variant="outline"
          className={cn(warningBadge, "cursor-pointer hover:bg-warning/20")}
        >
          <button
            type="button"
            aria-label={`Show the Price Warning of ${instrument.ticker}`}
          >
            <TriangleAlertIcon data-icon="inline-start" />
            Price Warning
          </button>
        </Badge>
      }
    />
  )
}

// InstrumentWarnings are the Price Warnings of one Instrument.
type InstrumentWarnings = {
  instrument: PriceWarning
  warnings: PriceWarning[]
}

// byInstrument groups the Price Warnings by broker ticker and keeps the order of the first warning of each Instrument.
function byInstrument(warnings: PriceWarning[]): InstrumentWarnings[] {
  const groups = new Map<string, InstrumentWarnings>()
  for (const warning of warnings) {
    const group = groups.get(warning.ticker)
    if (group) group.warnings.push(warning)
    else
      groups.set(warning.ticker, { instrument: warning, warnings: [warning] })
  }
  return [...groups.values()]
}

// PriceWarningList lists the Price Warnings by Instrument. Each Instrument has a button that opens the price source dialog.
// After a change in the dialog, the tool refreshes the prices, and the views load again.
export function PriceWarningList({ warnings }: { warnings: PriceWarning[] }) {
  const { refreshAllPrices } = useAppState()
  return (
    <ul className="flex flex-col gap-3">
      {byInstrument(warnings).map(({ instrument, warnings }) => (
        <li
          key={instrument.ticker}
          className="flex flex-col items-start gap-2 sm:flex-row sm:justify-between sm:gap-4"
        >
          <div className="flex min-w-0 flex-col gap-0.5">
            <span className="break-words text-foreground">
              <span className="num font-medium">{instrument.ticker}</span>{" "}
              {instrument.name}
            </span>
            {warnings.map((warning) => (
              <span key={warning.kind} className="break-words">
                <PriceWarningText warning={warning} />
              </span>
            ))}
          </div>
          <PriceSourceDialog
            instrument={instrument}
            warnings={warnings}
            onChanged={refreshAllPrices}
            trigger={
              <Button
                variant="outline"
                size="sm"
                className="shrink-0"
                aria-label={`Edit the price source of ${instrument.ticker}`}
              >
                <PencilIcon data-icon="inline-start" />
                Edit the price source
              </Button>
            }
          />
        </li>
      ))}
    </ul>
  )
}

// countInstruments gives the number of Instruments with a Price Warning.
export function countInstruments(warnings: PriceWarning[]): number {
  return new Set(warnings.map((warning) => warning.ticker)).size
}
