import { chartTooltipClass } from "@/lib/chart-style"
import { Bar, BarChart, Cell, ReferenceLine, XAxis, YAxis } from "recharts"

import {
  ChartContainer,
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart"
import { useIsMobile } from "@/hooks/use-mobile"
import type { Holding } from "@/lib/api"
import {
  amountSign,
  formatNumber,
  formatPercent,
  type Sign,
} from "@/lib/format"
import { cn } from "@/lib/utils"

const chartConfig = {
  positive: { label: "Profit", color: "var(--positive)" },
  negative: { label: "Loss", color: "var(--negative)" },
  zero: { label: "Zero", color: "var(--muted-foreground)" },
} satisfies ChartConfig

// ProfitBar is one bar of the chart. The size is for the geometry only. The text comes from the exact amount.
type ProfitBar = {
  ticker: string
  name: string
  size: number
  sign: Sign
  text: string
  percent: string | null
}

// tickOffset is the gap in pixels between a category axis and the x position that Recharts gives to its ticks.
const tickOffset = 8

// charWidth is the width of one character of Geist Mono in the text-xs size, in pixels.
const charWidth = 7.4

type UnrealizedProfitChartProps = {
  holdings: Holding[]
  // onOpenOverview opens the Instrument Overview of a Holding. A click on a bar, or Enter on a ticker, runs it.
  onOpenOverview: (ticker: string) => void
}

// UnrealizedProfitChart shows the Unrealized Profit of each Holding as a horizontal bar around a zero line.
// The bars go from the largest profit to the largest loss. A Holding with no Unrealized Profit is not in the chart.
// The exact amount at the right of each bar is the text alternative of the chart.
// A click on a bar opens the Instrument Overview. The ticker at the left of a bar is a button for the keyboard.
export function UnrealizedProfitChart({
  holdings,
  onOpenOverview,
}: UnrealizedProfitChartProps) {
  const isMobile = useIsMobile()
  const bars = profitBars(holdings)
  const missing = holdings.length - bars.length
  const missingNote = missingProfitNote(missing)

  if (bars.length === 0) {
    return (
      <div className="flex flex-col gap-2 text-sm text-muted-foreground">
        <p>No Holding has an Unrealized Profit.</p>
        {missingNote && <p>{missingNote}</p>}
      </div>
    )
  }

  const rowHeight = isMobile ? 28 : 32
  const tickerWidth = textWidth(bars.map((bar) => bar.ticker))
  const amountWidth = textWidth(bars.map((bar) => bar.text))
  const sizes = bars.map((bar) => bar.size)
  const domain = [Math.min(0, ...sizes), Math.max(0, ...sizes)]

  return (
    <div className="flex flex-col gap-3">
      <ChartContainer
        config={chartConfig}
        className="aspect-auto w-full"
        style={{ height: bars.length * rowHeight + 8 }}
        initialDimension={{ width: 320, height: bars.length * rowHeight + 8 }}
      >
        <BarChart
          data={bars}
          layout="vertical"
          margin={{ top: 4, right: 0, bottom: 4, left: 0 }}
          accessibilityLayer
        >
          <XAxis type="number" hide domain={domain} />
          <YAxis
            yAxisId="ticker"
            type="category"
            interval={0}
            dataKey="ticker"
            tickLine={false}
            axisLine={false}
            width={tickerWidth}
            tick={(props: TickProps) => (
              <TickerTick {...props} bars={bars} onOpen={onOpenOverview} />
            )}
          />
          <YAxis
            yAxisId="amount"
            orientation="right"
            type="category"
            interval={0}
            dataKey="text"
            tickLine={false}
            axisLine={false}
            width={amountWidth}
            tick={(props: TickProps) => (
              <AmountTick {...props} bars={bars} width={amountWidth} />
            )}
          />
          <ChartTooltip
            cursor={false}
            content={({ active, payload }) => (
              <BarTooltip
                active={active}
                bar={payload?.[0]?.payload as ProfitBar | undefined}
              />
            )}
          />
          <ReferenceLine
            x={0}
            yAxisId="ticker"
            stroke="var(--muted-foreground)"
            strokeOpacity={0.6}
          />
          <Bar
            dataKey="size"
            yAxisId="ticker"
            radius={3}
            maxBarSize={isMobile ? 14 : 18}
            minPointSize={2}
            isAnimationActive={false}
            className="cursor-pointer"
            onClick={(_, index) => onOpenOverview(bars[index].ticker)}
          >
            {bars.map((bar) => (
              <Cell key={bar.ticker} fill={`var(--color-${bar.sign})`} />
            ))}
          </Bar>
        </BarChart>
      </ChartContainer>
      {missingNote && (
        <p className="text-sm text-muted-foreground">{missingNote}</p>
      )}
    </div>
  )
}

type TickProps = {
  x?: number | string
  y?: number | string
  index?: number
}

// TickerTick draws the ticker of a Holding at the left of its bar.
function TickerTick({
  x,
  y,
  index,
  bars,
  onOpen,
}: TickProps & { bars: ProfitBar[]; onOpen: (ticker: string) => void }) {
  const bar = bars[index ?? 0]
  return (
    <text
      role="button"
      tabIndex={0}
      aria-label={`Open the Instrument Overview of ${bar.ticker}`}
      onClick={() => onOpen(bar.ticker)}
      onKeyDown={(event) => {
        if (event.key !== "Enter" && event.key !== " ") return
        event.preventDefault()
        onOpen(bar.ticker)
      }}
      x={Number(x ?? 0)}
      y={Number(y ?? 0)}
      dy={4}
      textAnchor="end"
      className="cursor-pointer fill-foreground num text-xs font-medium outline-none focus-visible:underline"
    >
      {bar.ticker}
    </text>
  )
}

// AmountTick draws the signed amount at the right of a bar, in the colour of its sign.
function AmountTick({
  x,
  y,
  index,
  bars,
  width,
}: TickProps & { bars: ProfitBar[]; width: number }) {
  const bar = bars[index ?? 0]
  return (
    <text
      x={Number(x ?? 0) + width - tickOffset}
      y={Number(y ?? 0)}
      dy={4}
      textAnchor="end"
      className={cn("num text-xs", labelFill[bar.sign])}
    >
      {bar.text}
    </text>
  )
}

// labelFill holds the SVG fill class of the amount of each sign.
const labelFill: Record<Sign, string> = {
  positive: "fill-positive",
  negative: "fill-negative",
  zero: "fill-muted-foreground",
}

type BarTooltipProps = {
  active?: boolean
  bar?: ProfitBar
}

// BarTooltip shows the name of the Instrument under the pointer, with its Unrealized Profit and percentage.
function BarTooltip({ active, bar }: BarTooltipProps) {
  if (!active || !bar) return null
  return (
    <div
      className={cn(
        "grid min-w-40 gap-1 px-2.5 py-1.5 text-xs",
        chartTooltipClass
      )}
    >
      <span className="font-medium">{bar.name}</span>
      <span className={cn("num", labelText[bar.sign])}>
        {bar.text}
        {bar.percent !== null &&
          ` (${formatPercent(bar.percent, { signed: true })})`}
      </span>
    </div>
  )
}

// labelText holds the text colour class of each sign.
const labelText: Record<Sign, string> = {
  positive: "text-positive",
  negative: "text-negative",
  zero: "text-muted-foreground",
}

// profitBars builds the bars of the Holdings that have an Unrealized Profit, from the largest profit to the largest loss.
// The floats are for the order and the geometry only. The labels show the exact text from the backend.
function profitBars(holdings: Holding[]): ProfitBar[] {
  const bars: ProfitBar[] = []
  for (const holding of holdings) {
    if (holding.unrealizedProfit === null) continue
    bars.push({
      ticker: holding.ticker,
      name: holding.name,
      size: Number(holding.unrealizedProfit),
      sign: amountSign(holding.unrealizedProfit),
      text: formatNumber(holding.unrealizedProfit, { signed: true }),
      percent: holding.unrealizedProfitPercent,
    })
  }
  return bars.sort(
    (a, b) => b.size - a.size || a.ticker.localeCompare(b.ticker)
  )
}

// textWidth gives the width in pixels of the longest text, with a small gap.
function textWidth(texts: string[]): number {
  const longest = Math.max(...texts.map((text) => text.length))
  return Math.ceil(longest * charWidth) + 2 * tickOffset
}

// missingProfitNote tells how many Holdings have no Unrealized Profit and are not in the chart.
function missingProfitNote(count: number): string | undefined {
  if (count === 0) return undefined
  return count === 1
    ? "1 Holding has no Unrealized Profit and is not in the chart."
    : `${count} Holdings have no Unrealized Profit and are not in the chart.`
}
