import {
  Bar,
  BarChart,
  Cell,
  LabelList,
  ReferenceLine,
  XAxis,
  YAxis,
} from "recharts"

import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ChartContainer, type ChartConfig } from "@/components/ui/chart"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  amountSign,
  formatAmount,
  formatNumber,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"

// WaterfallPart is one part of the Total Result. The amount is null when the tool cannot calculate it.
export type WaterfallPart = {
  label: string
  amount: string | null
}

type TotalResultWaterfallProps = {
  // parts are the parts of the Total Result, in the order of the bars.
  parts: WaterfallPart[]
  totalResult: string | null
  currency: string
}

// BarKind selects the colour of a bar.
type BarKind = "positive" | "negative" | "zero" | "total"

// WaterfallBar is one bar of the chart. The range is for the geometry only. The text comes from the exact amount.
type WaterfallBar = {
  label: string
  range: [number, number]
  kind: BarKind
  text: string
}

const chartConfig = {
  positive: { label: "Profit", color: "var(--positive)" },
  negative: { label: "Loss", color: "var(--negative)" },
  zero: { label: "Zero", color: "var(--muted-foreground)" },
  total: { label: "Total Result", color: "var(--primary)" },
} satisfies ChartConfig

// gridColumns holds the literal grid classes of the boxes for each count, so that Tailwind finds them.
const gridColumns: Record<number, string> = {
  6: "lg:grid-cols-6",
  7: "lg:grid-cols-7",
}

// TotalResultWaterfall shows how each part adds to the Total Result. Each bar starts at the sum of the bars before it.
// The last bar is the Total Result from zero. A row of boxes under the chart shows the exact amounts.
export function TotalResultWaterfall({
  parts,
  totalResult,
  currency,
}: TotalResultWaterfallProps) {
  const isMobile = useIsMobile()
  const bars = waterfallBars(parts, totalResult)
  const boxes: WaterfallPart[] = [
    ...parts,
    { label: "Total Result", amount: totalResult },
  ]

  return (
    <Card>
      <CardHeader>
        <CardTitle>Total Result breakdown</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {bars === null ? (
          <p className="text-sm text-muted-foreground">
            The chart is not available, because the tool cannot calculate all
            parts.
          </p>
        ) : isMobile ? (
          <NarrowChart bars={bars} />
        ) : (
          <WideChart bars={bars} />
        )}
        <ul
          className={cn(
            "grid grid-cols-2 gap-2 sm:grid-cols-3",
            gridColumns[boxes.length]
          )}
        >
          {boxes.map((box, index) => (
            <AmountBox
              key={box.label}
              part={box}
              currency={currency}
              total={index === boxes.length - 1}
            />
          ))}
        </ul>
      </CardContent>
    </Card>
  )
}

// WideChart shows the bars as columns, with the amount above each bar.
function WideChart({ bars }: { bars: WaterfallBar[] }) {
  return (
    <ChartContainer config={chartConfig} className="aspect-auto h-72 w-full">
      <BarChart
        data={bars}
        margin={{ top: 24, right: 8, bottom: 0, left: 8 }}
        accessibilityLayer
      >
        <XAxis
          dataKey="label"
          tickLine={false}
          axisLine={false}
          interval={0}
          tickMargin={8}
        />
        <YAxis hide domain={["dataMin", "dataMax"]} />
        <ReferenceLine y={0} stroke="var(--border)" />
        <Bar
          dataKey="range"
          radius={4}
          maxBarSize={56}
          minPointSize={3}
          isAnimationActive={false}
        >
          {bars.map((bar) => (
            <Cell key={bar.label} fill={`var(--color-${bar.kind})`} />
          ))}
          <LabelList
            dataKey="text"
            position="top"
            offset={8}
            content={(props) => <BarLabel {...props} bars={bars} />}
          />
        </Bar>
      </BarChart>
    </ChartContainer>
  )
}

// NarrowChart shows the bars as rows, with the name at the left and the amount at the right.
function NarrowChart({ bars }: { bars: WaterfallBar[] }) {
  return (
    <ChartContainer
      config={chartConfig}
      className="aspect-auto w-full"
      style={{ height: bars.length * 36 + 8 }}
    >
      <BarChart
        data={bars}
        layout="vertical"
        margin={{ top: 4, right: 0, bottom: 4, left: 0 }}
        accessibilityLayer
      >
        <XAxis type="number" hide domain={["dataMin", "dataMax"]} />
        <YAxis
          yAxisId="label"
          type="category"
          dataKey="label"
          tickLine={false}
          axisLine={false}
          width={132}
        />
        <YAxis
          yAxisId="amount"
          orientation="right"
          type="category"
          dataKey="text"
          tickLine={false}
          axisLine={false}
          width={88}
          tick={(props: AmountTickProps) => (
            <AmountTick {...props} bars={bars} />
          )}
        />
        <ReferenceLine x={0} yAxisId="label" stroke="var(--border)" />
        <Bar
          dataKey="range"
          yAxisId="label"
          radius={3}
          maxBarSize={20}
          minPointSize={3}
          isAnimationActive={false}
        >
          {bars.map((bar) => (
            <Cell key={bar.label} fill={`var(--color-${bar.kind})`} />
          ))}
        </Bar>
      </BarChart>
    </ChartContainer>
  )
}

type BarLabelProps = {
  x?: number | string
  y?: number | string
  width?: number | string
  height?: number | string
  index?: number
  bars: WaterfallBar[]
}

// BarLabel draws the signed amount above a bar, in the colour of its sign.
function BarLabel({ x, y, width, height, index, bars }: BarLabelProps) {
  const bar = bars[index ?? 0]
  const left = Number(x ?? 0)
  const top = Math.min(Number(y ?? 0), Number(y ?? 0) + Number(height ?? 0))
  return (
    <text
      x={left + Number(width ?? 0) / 2}
      y={top - 8}
      textAnchor="middle"
      className={cn("num text-xs", labelFill[bar.kind])}
    >
      {bar.text}
    </text>
  )
}

type AmountTickProps = {
  x?: number | string
  y?: number | string
  index?: number
}

// AmountTick draws the signed amount at the right of a row, in the colour of its sign.
function AmountTick({
  x,
  y,
  index,
  bars,
}: AmountTickProps & { bars: WaterfallBar[] }) {
  const bar = bars[index ?? 0]
  return (
    <text
      x={Number(x ?? 0) + 80}
      y={Number(y ?? 0)}
      dy={4}
      textAnchor="end"
      className={cn("num text-xs", labelFill[bar.kind])}
    >
      {bar.text}
    </text>
  )
}

// labelFill holds the SVG fill class of the label of each kind of bar.
const labelFill: Record<BarKind, string> = {
  positive: "fill-positive",
  negative: "fill-negative",
  zero: "fill-muted-foreground",
  total: "fill-primary",
}

type AmountBoxProps = {
  part: WaterfallPart
  currency: string
  total: boolean
}

// AmountBox shows the exact amount of one part, or of the Total Result.
function AmountBox({ part, currency, total }: AmountBoxProps) {
  return (
    <li
      className={cn(
        "flex min-w-0 flex-col gap-1 rounded-lg border bg-muted/40 p-3",
        total && "border-primary/40 bg-primary/10"
      )}
    >
      <span
        className={cn("text-xs text-muted-foreground", total && "text-primary")}
      >
        {part.label}
      </span>
      <span
        className={cn(
          "num text-sm font-medium",
          part.amount !== null && signColour(amountSign(part.amount))
        )}
      >
        {part.amount === null
          ? "—"
          : formatAmount(part.amount, currency, { signed: true })}
      </span>
    </li>
  )
}

// waterfallBars builds the bars of the chart, or null when a part or the Total Result is not available.
// The floats are for the geometry of the bars only. The labels show the exact text from the backend.
function waterfallBars(
  parts: WaterfallPart[],
  totalResult: string | null
): WaterfallBar[] | null {
  if (totalResult === null) return null
  const bars: WaterfallBar[] = []
  let sum = 0
  for (const part of parts) {
    if (part.amount === null) return null
    const end = sum + Number(part.amount)
    bars.push({
      label: part.label,
      range: [sum, end],
      kind: amountSign(part.amount),
      text: formatNumber(part.amount, { signed: true }),
    })
    sum = end
  }
  bars.push({
    label: "Total Result",
    range: [0, Number(totalResult)],
    kind: "total",
    text: formatNumber(totalResult, { signed: true }),
  })
  return bars
}
