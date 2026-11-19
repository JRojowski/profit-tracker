import type { ReactElement } from "react"
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
} satisfies ChartConfig

// MarkerShapeProps are the props that Recharts gives to the dot of a point.
type MarkerShapeProps = { cx?: number; cy?: number }

// markerSeries holds, for each kind of marker, its label and its shape. A shape draws around the point (0, 0).
// The kinds differ in colour and in shape, so the user can tell them apart without colour vision.
// A new kind needs one entry here and one entry in chartConfig.
const markerSeries: Record<
  MarkerKind,
  { label: string; shape: (props: MarkerShapeProps) => ReactElement }
> = {
  buy: {
    label: "Buy",
    shape: ({ cx, cy }) =>
      typeof cx === "number" && typeof cy === "number" ? (
        <polygon
          points={`${cx},${cy - 7} ${cx + 6.5},${cy + 5} ${cx - 6.5},${cy + 5}`}
          fill="var(--color-buy)"
          stroke="var(--background)"
          strokeWidth={1.5}
          strokeLinejoin="round"
        />
      ) : (
        <g />
      ),
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

// InstrumentPriceChart draws the daily closes as a line and the markers of the Fills on it.
// It shows the full series. A day with a marker but no close still has its marker.
// The tooltip of a day lists each operation of its marker. The list of Fills under the chart is the text alternative.
export function InstrumentPriceChart({
  model,
  currency,
  kinds,
}: InstrumentPriceChartProps) {
  const isMobile = useIsMobile()
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
        <CartesianGrid vertical={false} />
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
          width={isMobile ? 50 : 58}
          tickFormatter={(value: number) => formatNumber(String(value))}
        />
        <ChartTooltip
          cursor={{ stroke: "var(--muted-foreground)", strokeOpacity: 0.5 }}
          content={({ active, payload }) => (
            <DayTooltip
              active={active}
              row={payload?.[0]?.payload as ChartRow | undefined}
              currency={currency}
            />
          )}
        />
        <Line
          dataKey="close"
          name="Close"
          type="monotone"
          stroke="var(--color-close)"
          strokeWidth={1.75}
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
            dot={markerSeries[kind].shape}
            activeDot={markerSeries[kind].shape}
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
    <div className="grid min-w-44 gap-1.5 rounded-lg border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-xl">
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
