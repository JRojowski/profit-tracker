import { TriangleAlertIcon } from "lucide-react"
import type { ReactNode } from "react"
import { Cell, Pie, PieChart } from "recharts"

import { FigureLabel } from "@/components/figure-label"
import { LoadedView } from "@/components/loaded-view"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  ChartContainer,
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"
import type { Loaded } from "@/hooks/use-load"
import type { Allocation, AllocationPart } from "@/lib/api"
import { formatAmount, formatPercent } from "@/lib/format"
import { cn } from "@/lib/utils"

// chartConfig gives each part its colour: the parts in order of value, then the cash and "Other".
const chartConfig = {
  part1: { label: "Part 1", color: "var(--chart-1)" },
  part2: { label: "Part 2", color: "var(--chart-2)" },
  part3: { label: "Part 3", color: "var(--chart-3)" },
  part4: { label: "Part 4", color: "var(--chart-4)" },
  part5: { label: "Part 5", color: "var(--chart-5)" },
  part6: { label: "Part 6", color: "var(--chart-6)" },
  part7: { label: "Part 7", color: "var(--chart-7)" },
  part8: { label: "Part 8", color: "var(--chart-8)" },
  cash: { label: "Cash", color: "var(--chart-cash)" },
  other: { label: "Other", color: "var(--chart-other)" },
} satisfies ChartConfig

// ColourKey names the colour of one part of the chart, a key of chartConfig.
type ColourKey = keyof typeof chartConfig

// DonutPart is one part of the chart. The size is for the geometry only. The text comes from the exact share and value.
type DonutPart = {
  key: string
  colour: ColourKey
  name: string
  size: number
  share: string
  value: string
  // detail is a short text after the name, for example "with cash". It is undefined for most parts.
  detail?: string
  // others are the Holdings in "Other". They are empty for the other parts.
  others: AllocationPart[]
}

// AllocationView selects the parts of a chart: the Instruments with the cash and "Other", or the Quote Currencies.
export type AllocationView = "instrument" | "quoteCurrency"

type AllocationDonutProps = {
  allocation: Loaded<Allocation>
}

// AllocationDonut shows the Allocation by Instrument in the hero of an Overview: a donut chart and a legend.
// The legend is the text alternative of the chart. It shows the name, the share and the value of each part.
export function AllocationDonut({ allocation }: AllocationDonutProps) {
  return (
    <section
      aria-label="Allocation"
      className="@container flex min-w-0 flex-col gap-4 border-t pt-6 xl:border-t-0 xl:border-l xl:pt-0 xl:pl-8"
    >
      <FigureLabel>Allocation</FigureLabel>
      <LoadedView loaded={allocation} errorTitle="Cannot load the Allocation">
        {(data) => <AllocationChart allocation={data} view="instrument" />}
      </LoadedView>
    </section>
  )
}

type AllocationChartProps = {
  allocation: Allocation
  view: AllocationView
}

// AllocationChart shows a donut chart of the Allocation and its legend, or a note when there is nothing to show.
// The legend is the text alternative of the chart. It shows the name, the share and the value of each part.
export function AllocationChart({ allocation, view }: AllocationChartProps) {
  if (!allocation.available) {
    return (
      <Alert>
        <TriangleAlertIcon />
        <AlertTitle>The Allocation is not available</AlertTitle>
        {allocation.warnings.length > 0 && (
          <AlertDescription>
            <ul className="list-inside list-disc">
              {allocation.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </AlertDescription>
        )}
      </Alert>
    )
  }

  const parts =
    view === "instrument"
      ? instrumentParts(allocation)
      : quoteCurrencyParts(allocation)
  const unpriced = unpricedAllocationNote(allocation.unpricedHoldings)
  if (parts.length === 0) {
    return (
      <Note>
        <p>There is no cash and no Holding with a known value.</p>
        {unpriced && <p>{unpriced}</p>}
      </Note>
    )
  }

  return (
    <div className="flex flex-col gap-4">
      <div className="flex flex-col items-center gap-5 @lg:flex-row @lg:items-start">
        <Donut parts={parts} />
        <Legend parts={parts} currency={allocation.currency} />
      </div>
      {unpriced && <p className="text-sm text-muted-foreground">{unpriced}</p>}
    </div>
  )
}

// Donut draws the parts as a ring. Each part shows its name, share and value in a tooltip.
function Donut({ parts }: { parts: DonutPart[] }) {
  return (
    <ChartContainer
      config={chartConfig}
      className="aspect-square size-40 shrink-0"
      initialDimension={{ width: 160, height: 160 }}
    >
      <PieChart accessibilityLayer>
        <ChartTooltip
          cursor={false}
          content={({ active, payload }) => (
            <PartTooltip
              active={active}
              part={payload?.[0]?.payload as DonutPart | undefined}
            />
          )}
        />
        <Pie
          data={parts}
          dataKey="size"
          nameKey="name"
          innerRadius="62%"
          outerRadius="100%"
          paddingAngle={parts.length > 1 ? 1 : 0}
          minAngle={3}
          stroke="var(--card)"
          strokeWidth={1}
          startAngle={90}
          endAngle={-270}
          isAnimationActive={false}
        >
          {parts.map((part) => (
            <Cell key={part.key} fill={`var(--color-${part.colour})`} />
          ))}
        </Pie>
      </PieChart>
    </ChartContainer>
  )
}

type PartTooltipProps = {
  active?: boolean
  part?: DonutPart
}

// PartTooltip shows the share of the part under the pointer. For "Other", it lists its Holdings with their shares.
function PartTooltip({ active, part }: PartTooltipProps) {
  if (!active || !part) return null
  return (
    <div className="grid min-w-40 gap-1.5 rounded-lg border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-xl">
      <div className="flex items-center justify-between gap-4 font-medium">
        <span>{part.name}</span>
        <span className="num">{formatPercent(part.share)}</span>
      </div>
      {part.others.length > 0 && (
        <div className="text-muted-foreground">
          <OtherList holdings={part.others} />
        </div>
      )}
    </div>
  )
}

// OtherList lists the Holdings in "Other" with their shares. It takes the text colour of its tooltip.
function OtherList({ holdings }: { holdings: AllocationPart[] }) {
  return (
    <ul className="grid gap-1">
      {holdings.map((holding) => (
        <li
          key={holding.ticker}
          className="flex items-center justify-between gap-4"
        >
          <span className="truncate">{holding.name}</span>
          <span className="num">{formatPercent(holding.share)}</span>
        </li>
      ))}
    </ul>
  )
}

type LegendProps = {
  parts: DonutPart[]
  currency: string
}

// Legend lists each part with its colour, name, share and value.
function Legend({ parts, currency }: LegendProps) {
  return (
    <ul className="grid w-full min-w-0 grid-cols-[auto_minmax(0,1fr)_auto_auto] items-center gap-x-3 gap-y-1.5 text-sm">
      {parts.map((part) => (
        <li
          key={part.key}
          className="col-span-4 grid grid-cols-subgrid items-center"
        >
          <span
            aria-hidden
            className={cn("size-2.5 rounded-xs", swatchClass[part.colour])}
          />
          <PartName part={part} />
          <span className="text-right num text-muted-foreground">
            {formatPercent(part.share)}
          </span>
          <span className="text-right num">
            {formatAmount(part.value, currency)}
          </span>
        </li>
      ))}
    </ul>
  )
}

// PartName shows the name of a part. "Other" also shows the count of its Holdings and lists them in a tooltip.
function PartName({ part }: { part: DonutPart }) {
  if (part.others.length === 0) {
    return (
      <span className="flex min-w-0 items-baseline gap-1.5">
        <span className="truncate" title={part.name}>
          {part.name}
        </span>
        {part.detail && (
          <span className="shrink-0 text-xs text-muted-foreground">
            {part.detail}
          </span>
        )}
      </span>
    )
  }
  return (
    <Tooltip>
      <TooltipTrigger asChild>
        <button
          type="button"
          className="flex min-w-0 cursor-help flex-wrap items-baseline gap-x-1 text-left"
        >
          <span className="underline decoration-dotted underline-offset-4">
            {part.name}
          </span>
          <span className="text-muted-foreground">
            ({holdingCount(part.others.length)})
          </span>
        </button>
      </TooltipTrigger>
      <TooltipContent className="grid min-w-40 gap-1.5">
        <OtherList holdings={part.others} />
      </TooltipContent>
    </Tooltip>
  )
}

// swatchClass holds the literal background class of each colour key, so that Tailwind finds them.
const swatchClass: Record<ColourKey, string> = {
  part1: "bg-chart-1",
  part2: "bg-chart-2",
  part3: "bg-chart-3",
  part4: "bg-chart-4",
  part5: "bg-chart-5",
  part6: "bg-chart-6",
  part7: "bg-chart-7",
  part8: "bg-chart-8",
  cash: "bg-chart-cash",
  other: "bg-chart-other",
}

// Note shows a muted text in place of the chart.
function Note({ children }: { children: ReactNode }) {
  return (
    <div className="flex flex-col gap-2 text-sm text-muted-foreground">
      {children}
    </div>
  )
}

// instrumentParts builds the parts of the Allocation by Instrument, in the order of the chart: the Holdings, the cash and "Other".
function instrumentParts(allocation: Allocation): DonutPart[] {
  const { holdings, cash, other } = allocation.byInstrument
  const parts = holdings.map((holding, index) =>
    donutPart(holding.ticker, partColour(index), holding)
  )
  if (cash) parts.push(donutPart("cash", "cash", { ...cash, name: "Cash" }))
  if (other) {
    parts.push(
      donutPart("other", "other", { ...other, name: "Other" }, other.holdings)
    )
  }
  return parts
}

// quoteCurrencyParts builds the parts of the Allocation by Quote Currency, largest value first.
// The cash counts in the Account currency, so that part tells "with cash".
function quoteCurrencyParts(allocation: Allocation): DonutPart[] {
  const hasCash = allocation.byInstrument.cash !== null
  return allocation.byQuoteCurrency.map((part, index) => ({
    ...donutPart(part.currency, partColour(index), {
      ...part,
      name: part.currency,
    }),
    detail:
      hasCash && part.currency === allocation.currency
        ? "with cash"
        : undefined,
  }))
}

// partColour gives the colour of the part at an index in the order of value. After 8 parts, the colours repeat.
function partColour(index: number): ColourKey {
  return `part${(index % 8) + 1}` as ColourKey
}

// donutPart builds one part of the chart. The size is the share as a float, for the geometry of the ring only.
function donutPart(
  key: string,
  colour: ColourKey,
  source: { name: string; share: string; value: string },
  others: AllocationPart[] = []
): DonutPart {
  return {
    key,
    colour,
    name: source.name,
    size: Number(source.share),
    share: source.share,
    value: source.value,
    others,
  }
}

// holdingCount shows a count of Holdings, for example "1 Holding" or "5 Holdings".
function holdingCount(count: number): string {
  return count === 1 ? "1 Holding" : `${count} Holdings`
}

// unpricedAllocationNote tells how many Holdings have no known value and are not in the chart.
function unpricedAllocationNote(count: number): string | undefined {
  if (count === 0) return undefined
  return count === 1
    ? "1 Holding has no known value and is not in the chart."
    : `${count} Holdings have no known value and are not in the chart.`
}
