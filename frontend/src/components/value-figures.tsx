import {
  amountSign,
  formatAmount,
  formatPercent,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"

// ProfitFigure shows a profit with its percentage under it, in the colour of its sign.
export function ProfitFigure({
  profit,
  percent,
  currency,
}: {
  profit: string
  percent: string | null
  currency: string
}) {
  return (
    <div className={cn("flex flex-col", signColour(amountSign(profit)))}>
      <span>{formatAmount(profit, currency, { signed: true })}</span>
      {percent !== null && (
        <span className="text-xs">
          {formatPercent(percent, { signed: true })}
        </span>
      )}
    </div>
  )
}

// Stacked shows a value with a small muted line under it, for example a price and its date.
export function Stacked({ main, sub }: { main: string; sub: string }) {
  return (
    <div className="flex flex-col">
      <span>{main}</span>
      <span className="text-xs text-muted-foreground">{sub}</span>
    </div>
  )
}

// NoValue marks an empty cell. The note under the table tells why a Holding has no value.
// A Holding in the Account currency has no Exchange Rate.
export function NoValue() {
  return <span className="text-muted-foreground">—</span>
}
