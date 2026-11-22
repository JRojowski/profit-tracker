import { ArrowDownRightIcon, ArrowUpRightIcon } from "lucide-react"

import {
  amountSign,
  formatAmount,
  formatPercent,
  formatPoints,
  type Sign,
} from "@/lib/format"
import { cn } from "@/lib/utils"

type ResultBadgeProps = {
  // value is the decimal text from the backend, or null when the tool cannot calculate it.
  value: string | null
  // currency is set for an amount and omitted for a percent.
  currency?: string
  // unit "pp" shows the value as a difference in percentage points.
  unit?: "pp"
  className?: string
}

// tints gives the tint and the text colour of each sign. Zero is neutral. Each pair keeps the text at 4.5:1 or more in both themes.
// The neutral pair has 9.21:1 in the light theme and 10.92:1 in the dark theme.
const tints: Record<Sign, string> = {
  positive: "bg-positive/10 text-positive",
  negative: "bg-negative/10 text-negative",
  zero: "bg-muted text-secondary-foreground",
}

// ResultBadge shows a signed percent or a signed amount in a pill.
// The tint comes from the sign. The sign and the arrow keep colour from being the only signal.
// A zero or a missing value shows a neutral badge with no arrow.
export function ResultBadge({
  value,
  currency,
  unit,
  className,
}: ResultBadgeProps) {
  const sign = value === null ? "zero" : amountSign(value)
  const Arrow =
    sign === "positive"
      ? ArrowUpRightIcon
      : sign === "negative"
        ? ArrowDownRightIcon
        : null
  const text =
    value === null
      ? "—"
      : unit === "pp"
        ? formatPoints(value, { signed: true })
        : currency === undefined
          ? formatPercent(value, { signed: true })
          : formatAmount(value, currency, { signed: true })

  return (
    <span
      data-slot="result-badge"
      className={cn(
        "inline-flex w-fit items-center gap-1 rounded-full px-2.5 py-0.5 num text-sm font-semibold",
        tints[sign],
        className
      )}
    >
      {Arrow && <Arrow aria-hidden className="size-3.5" />}
      {text}
    </span>
  )
}
