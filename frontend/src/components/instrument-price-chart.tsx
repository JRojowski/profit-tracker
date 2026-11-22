import { cn } from "@/lib/utils"
import {
  chartGridProps,
  chartLineWidth,
  chartTooltipClass,
} from "@/lib/chart-style"
import { useState, type ReactElement } from "react"
import { CartesianGrid, ComposedChart, Line, XAxis, YAxis } from "recharts"

import {
  ChartContainer,
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  formatDate,
  formatNumber,
  formatPolishTime,
  formatPrice,
  formatVolume,
} from "@/lib/format"
import {
  type ChartModel,
  type ChartRow,
  type MarkerKind,
} from "@/lib/overview-markers"

const chartConfig = {
  close: { label: "Close", color: "var(--chart-2)" },
  buy: { label: "Buy", color: "var(--chart-7)" },
  sell: { label: "Sell", color: "var(--chart-8)" },
} satisfies ChartConfig

// MarkerShapeProps are the props that Recharts gives to the dot of a point.
type MarkerShapeProps = {
  cx?: number
  cy?: number
  payload?: ChartRow
}

// markerStyle gives the opacity and the outline of a marker. With a highlight, the markers that are not
// highlighted are dimmed, and a highlighted marker gets a ring.
function markerStyle(
  kind: MarkerKind,
  row: ChartRow | undefined,
  highlightActive: boolean
) {
  const highlighted = row?.markers[kind]?.highlighted === true
  return {
    opacity: highlightActive && !highlighted ? 0.3 : 1,
    highlighted,
  }
}

// markerSeries holds, for each kind of marker, its label and its shape. A shape draws around the point.
// The kinds differ in colour and in shape, so the user can tell them apart without colour vision:
// a buy is a triangle that points up and a sell is a diamond.
// A new kind needs one entry here and one entry in chartConfig.
const markerSeries: Record<
  MarkerKind,
  {
    label: string
    shape: (props: MarkerShapeProps, highlightActive: boolean) => ReactElement
  }
> = {
  buy: {
    label: "Buy",
    shape: ({ cx, cy, payload }, highlightActive) => {
      if (typeof cx !== "number" || typeof cy !== "number") return <g />
      const { opacity, highlighted } = markerStyle(
        "buy",
        payload,
        highlightActive
      )
      const k = highlighted ? 1.5 : 1
      return (
        <polygon
          points={`${cx},${cy - 7 * k} ${cx + 6.5 * k},${cy + 5 * k} ${cx - 6.5 * k},${cy + 5 * k}`}
          fill="var(--color-buy)"
          fillOpacity={opacity}
          stroke={highlighted ? "var(--foreground)" : "var(--background)"}
          strokeOpacity={opacity}
          strokeWidth={highlighted ? 2 : 1.5}
          strokeLinejoin="round"
        />
      )
    },
  },
  sell: {
    label: "Sell",
    shape: ({ cx, cy, payload }, highlightActive) => {
      if (typeof cx !== "number" || typeof cy !== "number") return <g />
      const { opacity, highlighted } = markerStyle(
        "sell",
        payload,
        highlightActive
      )
      const r = highlighted ? 10 : 6.5
      return (
        <polygon
          points={`${cx},${cy - r} ${cx + r},${cy} ${cx},${cy + r} ${cx - r},${cy}`}
          fill="var(--color-sell)"
          fillOpacity={opacity}
          stroke={highlighted ? "var(--foreground)" : "var(--background)"}
          strokeOpacity={opacity}
          strokeWidth={highlighted ? 2 : 1.5}
          strokeLinejoin="round"
        />
      )
    },
  },
}

// monthSteps are the numbers of months between two ticks that the x axis can use.
const monthSteps = [1, 2, 3, 6, 12, 24, 60, 120]

// monthTicks gives x axis ticks at the start of a month, spaced so that the axis has at most maxTicks ticks.
function monthTicks(min: number, max: number, maxTicks: number) {
  const first = new Date(min)
  const months = (max - min) / (30.44 * 86_400_000)
  const step =
    monthSteps.find((candidate) => months / candidate <= maxTicks - 1) ??
    monthSteps[monthSteps.length - 1]
  const ticks: number[] = []
  let year = first.getUTCFullYear()
  let month = first.getUTCMonth() + 1
  if (step >= 12) month = 0
  for (;;) {
    const tick = Date.UTC(year, month, 1)
    if (tick > max) break
    if (tick >= min) ticks.push(tick)
    month += step
    year += Math.floor(month / 12)
    month %= 12
  }
  return { ticks, step }
}

type InstrumentPriceChartProps = {
  model: ChartModel
  // currency is the currency of the price series, or null when the series is empty.
  currency: string | null
  // kinds are the kinds of markers that the chart draws.
  kinds: MarkerKind[]
}

// InstrumentPriceChart draws the daily closes as a line and the buy markers and sell markers on it.
// It shows the full series. A day with a marker but no close still has its marker.
// The tooltip of a day lists each operation of its marker. The lists under the chart are the text alternative.
export function InstrumentPriceChart({
  model,
  currency,
  kinds,
}: InstrumentPriceChartProps) {
  const isMobile = useIsMobile()
  // hovered is the row of the marker under the pointer. The series is dense, so the tooltip of the nearest day
  // can be a day next to the marker. The tooltip of a hovered marker always shows the day of that marker.
  const [hovered, setHovered] = useState<ChartRow | null>(null)
  const { rows } = model
  const values = rows.flatMap((row) => [
    ...(row.close === undefined ? [] : [row.close]),
    ...Object.values(row.markerValues),
  ])
  const lowest = Math.min(...values)
  const highest = Math.max(...values)
  const pad = (highest - lowest || Math.abs(highest) || 1) * 0.08
  const first = rows[0].t
  const last = rows[rows.length - 1].t
  // One day wide domain keeps a single point away from the axis.
  const span = last === first ? 86_400_000 * 15 : 0
  const domain: [number, number] = [first - span, last + span]
  const { ticks, step } = monthTicks(domain[0], domain[1], isMobile ? 3 : 5)
  // A price of 1000 or more, for example of a crypto asset, needs no decimal places on the axis.
  // Without them the label fits the width of the axis.
  const axisDigits =
    Math.max(Math.abs(lowest), Math.abs(highest)) >= 1000 ? 0 : 2
  // A long label, for example 103,713, gets a wider axis. About 7 px for each character of the 12 px label.
  const longestLabel = Math.max(
    ...[lowest - pad, highest + pad].map(
      (value) =>
        formatNumber(String(value), { fractionDigits: axisDigits }).length
    )
  )
  const axisWidth = Math.max(isMobile ? 50 : 58, longestLabel * 7 + 8)

  return (
    <ChartContainer
      config={chartConfig}
      className="aspect-auto h-60 w-full"
      initialDimension={{ width: 320, height: 240 }}
    >
      <ComposedChart
        data={rows}
        margin={{ top: 12, right: 12, bottom: 0, left: 0 }}
        accessibilityLayer={false}
      >
        <CartesianGrid {...chartGridProps} />
        <XAxis
          dataKey="t"
          type="number"
          scale="time"
          domain={domain}
          ticks={ticks}
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          tickFormatter={(value: number) =>
            new Date(value).toLocaleDateString(undefined, {
              timeZone: "UTC",
              year: "numeric",
              ...(step < 12 ? { month: "short" } : {}),
            })
          }
        />
        <YAxis
          type="number"
          domain={[lowest - pad, highest + pad]}
          tickCount={5}
          tickLine={false}
          axisLine={false}
          width={axisWidth}
          tickFormatter={(value: number) =>
            formatNumber(String(value), { fractionDigits: axisDigits })
          }
        />
        <ChartTooltip
          cursor={{ stroke: "var(--muted-foreground)", strokeOpacity: 0.5 }}
          content={({ active, payload }) => (
            <DayTooltip
              active={active}
              row={hovered ?? (payload?.[0]?.payload as ChartRow | undefined)}
              currency={currency}
            />
          )}
        />
        <Line
          dataKey="close"
          name="Close"
          type="monotone"
          stroke="var(--color-close)"
          strokeWidth={chartLineWidth}
          dot={false}
          activeDot={false}
          connectNulls
          isAnimationActive={false}
        />
        {kinds.map((kind) => (
          <Line
            key={kind}
            dataKey={(row: ChartRow) => row.markerValues[kind]}
            name={markerSeries[kind].label}
            stroke="none"
            dot={(props: MarkerShapeProps) => (
              <g
                onMouseEnter={() => setHovered(props.payload ?? null)}
                onMouseLeave={() => setHovered(null)}
              >
                {markerSeries[kind].shape(props, model.highlighted)}
              </g>
            )}
            activeDot={false}
            isAnimationActive={false}
          />
        ))}
      </ComposedChart>
    </ChartContainer>
  )
}

type DayTooltipProps = {
  active?: boolean
  row?: ChartRow
  currency: string | null
}

// DayTooltip shows the close of a day and, for a day with a marker, each operation with its time, volume and price.
function DayTooltip({ active, row, currency }: DayTooltipProps) {
  if (!active || !row) return null
  const markers = (Object.keys(row.markers) as MarkerKind[]).flatMap((kind) => {
    const marker = row.markers[kind]
    return marker ? [marker] : []
  })
  return (
    <div
      className={cn(
        "grid min-w-44 gap-1.5 px-2.5 py-1.5 text-xs",
        chartTooltipClass
      )}
    >
      <span className="font-medium">{formatDate(row.date)}</span>
      {row.closeText !== undefined && (
        <span className="flex justify-between gap-4">
          <span className="text-muted-foreground">Close</span>
          <span className="num">
            {formatPrice(row.closeText)} {currency}
          </span>
        </span>
      )}
      {markers.map((marker) => (
        <div key={marker.kind} className="grid gap-1 border-t pt-1.5">
          <span className="font-medium">
            {markerSeries[marker.kind].label}
            {marker.operations.length > 1 && (
              <span className="num font-normal text-muted-foreground">
                {" "}
                · average {formatPrice(marker.price)}
              </span>
            )}
          </span>
          {marker.operations.map((operation) => (
            <span
              key={operation.id}
              className="flex justify-between gap-4 num text-muted-foreground"
            >
              <span>{formatPolishTime(operation.time)}</span>
              <span className="text-foreground">
                {formatVolume(operation.volume)} ×{" "}
                {operation.price === null ? "—" : formatPrice(operation.price)}
              </span>
            </span>
          ))}
        </div>
      ))}
    </div>
  )
}
