import {
  CartesianGrid,
  ComposedChart,
  Line,
  ReferenceLine,
  XAxis,
  YAxis,
} from "recharts"

import {
  ChartEndTags,
  tagOffset,
  type EndTag,
} from "@/components/chart-end-tags"
import {
  ChartContainer,
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart"
import { Toggle } from "@/components/ui/toggle"
import { useBenchmarkSelection } from "@/hooks/use-benchmark-selection"
import { useIsMobile } from "@/hooks/use-mobile"
import type { BenchmarkPoint, Benchmarks } from "@/lib/api"
import { chartGridProps, chartTooltipClass } from "@/lib/chart-style"
import { axisDate, dateTime, dayMs, monthTicks } from "@/lib/chart-time"
import { tagWidth } from "@/lib/end-tags"
import { formatCompact, formatDate, formatPercent } from "@/lib/format"
import { cn } from "@/lib/utils"

// userKey is the key of the line of the user in the chart data and in the chart config.
const userKey = "user"

// benchmarkLineWidth is the stroke width of a Benchmark line.
const benchmarkLineWidth = 1.25

// userLineWidth is the stroke width of the user line: 1.3 times the width of a Benchmark line.
const userLineWidth = benchmarkLineWidth * 1.3

// benchmarkOpacity is the stroke opacity of a Benchmark line: 75% transparent.
const benchmarkOpacity = 0.25

// drawOrder puts the user line last, so that the chart draws it on top of the Benchmark lines.
function drawOrder(lines: ChartLine[]): ChartLine[] {
  return [...lines].sort(
    (a, b) => Number(a.key === userKey) - Number(b.key === userKey)
  )
}

// LineStyle is the colour of a line and the colour of the text on its end tag.
type LineStyle = { color: string; ink: string }

// userStyle is the style of the user line: the foreground colour, which no Benchmark uses. The tag text is inverted.
export const userStyle: LineStyle = {
  color: "var(--foreground)",
  ink: "var(--background)",
}

// benchmarkStyles gives the style of the line of each Benchmark. A Benchmark with an unknown id uses otherStyle.
const benchmarkStyles: Record<string, LineStyle> = {
  sp500: { color: "var(--chart-2)", ink: "var(--chart-2-foreground)" },
  nasdaq100: { color: "var(--chart-6)", ink: "var(--chart-6-foreground)" },
  world: { color: "var(--chart-4)", ink: "var(--chart-4-foreground)" },
  bitcoin: { color: "var(--chart-7)", ink: "var(--chart-7-foreground)" },
  wig20tr: { color: "var(--chart-5)", ink: "var(--chart-5-foreground)" },
  mwig40tr: { color: "var(--chart-8)", ink: "var(--chart-8-foreground)" },
  gold: { color: "var(--chart-other)", ink: "var(--chart-other-foreground)" },
}
const otherStyle = benchmarkStyles.world

// styleOf gives the style of a line by its key.
function styleOf(key: string): LineStyle {
  return key === userKey ? userStyle : (benchmarkStyles[key] ?? otherStyle)
}

// ChartLine is one line of the chart: the user or one Benchmark.
type ChartLine = { key: string; label: string; series: BenchmarkPoint[] }

// ChartRow is one day of the chart. The numbers are for the geometry only. The tooltip shows the exact texts.
type ChartRow = {
  date: string
  time: number
  texts: Record<string, string>
  [key: string]: number | string | Record<string, string>
}

type BenchmarkChartProps = {
  data: Benchmarks
  // subjectLabel names the line of the user in the legend and in the tooltip, as the table does.
  subjectLabel: string
}

// BenchmarkChart shows the cumulative return of the user and of the Benchmarks for each day of the period.
// A toggle for each Benchmark shows or hides its line. The browser stores the selection.
// A Benchmark that is not available has a disabled toggle and no line. The table is the text alternative.
export function BenchmarkChart({ data, subjectLabel }: BenchmarkChartProps) {
  const isMobile = useIsMobile()
  const { isSelected, toggle } = useBenchmarkSelection()
  const allIds = data.benchmarks.map((benchmark) => benchmark.id)
  const lines: ChartLine[] = [
    { key: userKey, label: subjectLabel, series: data.user.series },
    ...data.benchmarks
      .filter(
        (benchmark) =>
          benchmark.status === "available" && isSelected(benchmark.id)
      )
      .map((benchmark) => ({
        key: benchmark.id,
        label: benchmark.name,
        series: benchmark.series,
      })),
  ]
  const rows = chartRows(lines)
  const config: ChartConfig = Object.fromEntries(
    lines.map((line) => [
      line.key,
      { label: line.label, color: styleOf(line.key).color },
    ])
  )
  const tags: EndTag[] = lines.flatMap((line) => {
    const last = line.series.at(-1)
    return last === undefined
      ? []
      : [
          {
            value: Number(last.return),
            text: formatPercent(last.return, { signed: true }),
            fill: styleOf(line.key).color,
            ink: styleOf(line.key).ink,
          },
        ]
  })

  return (
    <div className="flex min-w-0 flex-col gap-3">
      <div
        role="group"
        aria-label="Lines of the chart"
        className="flex flex-wrap gap-2"
      >
        {data.benchmarks.map((benchmark) => {
          const available = benchmark.status === "available"
          return (
            <Toggle
              key={benchmark.id}
              size="sm"
              variant="outline"
              pressed={available && isSelected(benchmark.id)}
              disabled={!available}
              onPressedChange={() => toggle(benchmark.id, allIds)}
              className="gap-2 data-[state=off]:text-muted-foreground"
            >
              <span
                aria-hidden
                className="size-2 shrink-0 rounded-[2px]"
                style={{ backgroundColor: styleOf(benchmark.id).color }}
              />
              {benchmark.name}
            </Toggle>
          )
        })}
      </div>
      {rows.length > 0 && (
        <LinesView
          rows={rows}
          lines={lines}
          config={config}
          tags={tags}
          compact={isMobile}
        />
      )}
    </div>
  )
}

type LinesViewProps = {
  rows: ChartRow[]
  lines: ChartLine[]
  config: ChartConfig
  tags: EndTag[]
  compact: boolean
}

// LinesView draws the lines on a time axis, with an end tag for each line.
function LinesView({ rows, lines, config, tags, compact }: LinesViewProps) {
  const first = rows[0].time
  const last = rows[rows.length - 1].time
  // The right margin holds the widest tag, so that no tag clips.
  const tagMargin =
    Math.max(...tags.map((tag) => tagWidth(tag.text))) + tagOffset + 2
  // A period of more than about 4 months shows the month and the year on the axis, as the Value History chart does.
  const long = last - first > 120 * dayMs
  const ticks = long ? monthTicks(first, last, compact ? 4 : 10) : undefined

  return (
    <ChartContainer
      config={config}
      className="aspect-auto h-64 w-full md:h-80"
      initialDimension={{ width: 320, height: 256 }}
    >
      <ComposedChart
        data={rows}
        margin={{ top: 8, right: tagMargin, bottom: 0, left: 0 }}
        accessibilityLayer
      >
        <CartesianGrid {...chartGridProps} />
        <ReferenceLine y={0} stroke="var(--border)" />
        <XAxis
          dataKey="time"
          type="number"
          scale="time"
          domain={[first, last]}
          ticks={ticks}
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          minTickGap={compact ? 24 : 40}
          tickFormatter={(time: number) => axisDate(time, long)}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          width={compact ? 48 : 56}
          domain={["auto", "auto"]}
          tickFormatter={(value: number) => `${formatCompact(value)} %`}
        />
        <ChartTooltip
          content={({ active, payload }) => (
            <BenchmarkTooltip
              active={active}
              row={payload?.[0]?.payload as ChartRow | undefined}
              lines={lines}
              config={config}
            />
          )}
        />
        {drawOrder(lines).map((line) => (
          <Line
            key={line.key}
            dataKey={line.key}
            type="linear"
            stroke={`var(--color-${line.key})`}
            strokeWidth={
              line.key === userKey ? userLineWidth : benchmarkLineWidth
            }
            strokeOpacity={line.key === userKey ? 1 : benchmarkOpacity}
            dot={false}
            isAnimationActive={false}
          />
        ))}
        <ChartEndTags time={last} tags={tags} />
      </ComposedChart>
    </ChartContainer>
  )
}

type BenchmarkTooltipProps = {
  active?: boolean
  row?: ChartRow
  lines: ChartLine[]
  config: ChartConfig
}

// BenchmarkTooltip shows the date and the return of each visible line on that day.
function BenchmarkTooltip({
  active,
  row,
  lines,
  config,
}: BenchmarkTooltipProps) {
  if (!active || !row) return null
  return (
    <div
      className={cn(
        "grid min-w-48 gap-1.5 px-2.5 py-1.5 text-xs",
        chartTooltipClass
      )}
    >
      <span className="num font-medium">{formatDate(row.date)}</span>
      {lines.map((line) => {
        const text = row.texts[line.key]
        if (text === undefined) return null
        return (
          <div
            key={line.key}
            className="flex items-center justify-between gap-4"
          >
            <span className="flex items-center gap-1.5 text-muted-foreground">
              <span
                aria-hidden
                className="size-2 shrink-0 rounded-[2px]"
                style={{ backgroundColor: config[line.key]?.color }}
              />
              {line.label}
            </span>
            <span className="num font-medium">
              {formatPercent(text, { signed: true })}
            </span>
          </div>
        )
      })}
    </div>
  )
}

// chartRows joins the series of the lines into one row for each day. The floats are for the geometry only.
function chartRows(lines: ChartLine[]): ChartRow[] {
  const byDate = new Map<string, ChartRow>()
  for (const line of lines) {
    for (const point of line.series) {
      let row = byDate.get(point.date)
      if (row === undefined) {
        row = { date: point.date, time: dateTime(point.date), texts: {} }
        byDate.set(point.date, row)
      }
      row[line.key] = Number(point.return)
      row.texts[line.key] = point.return
    }
  }
  return [...byDate.values()].sort((a, b) => a.time - b.time)
}
