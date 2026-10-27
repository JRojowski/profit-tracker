import { TriangleAlertIcon } from "lucide-react"
import { useState, type ReactNode } from "react"
import { CartesianGrid, Line, LineChart, XAxis, YAxis } from "recharts"

import { FigureLabel } from "@/components/figure-label"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  ChartContainer,
  ChartTooltip,
  type ChartConfig,
} from "@/components/ui/chart"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useIsMobile } from "@/hooks/use-mobile"
import {
  historyPeriods,
  type HistoryPeriod,
  type ValueHistory,
  type ValueHistoryPoint,
  type ValuedInstrument,
} from "@/lib/api"
import { subtractDecimals } from "@/lib/decimal"
import {
  amountSign,
  formatAmount,
  formatDate,
  formatPercent,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"

const chartConfig = {
  value: { label: "Value", color: "var(--chart-1)" },
  netDeposits: { label: "Net Deposits", color: "var(--chart-other)" },
} satisfies ChartConfig

// selectedItem marks the selected period with the accent, as the year filter of the Sales page does.
const selectedItem =
  "data-[state=on]:bg-primary/15 data-[state=on]:text-primary"

// ChartPoint is one day of the chart. The sizes are for the geometry only. The texts come from the exact amounts.
type ChartPoint = {
  date: string
  // time is the day as milliseconds since 1970 in UTC, for the time axis.
  time: number
  value: number
  netDeposits: number
  point: ValueHistoryPoint
}

type ValueHistoryCardProps = {
  history: ValueHistory
}

// ValueHistoryCard shows the value and the Net Deposits of an Account for each calendar day of the selected period.
// The header shows the Time-Weighted Return of the period. The amounts under the chart are its text alternative.
export function ValueHistoryCard({ history }: ValueHistoryCardProps) {
  const [period, setPeriod] = useState<HistoryPeriod>("All")
  const points = periodPoints(history.points, history.periodStarts[period])
  const twr = history.timeWeightedReturn[period]
  const last = points.at(-1)

  return (
    <Card className="min-w-0">
      <CardHeader className="flex flex-col gap-3 md:flex-row md:items-start md:justify-between">
        <div className="flex min-w-0 flex-col gap-1.5">
          <CardTitle>Value History</CardTitle>
          <p className="flex flex-wrap items-baseline gap-x-2 text-sm text-muted-foreground">
            Time-Weighted Return
            <span
              className={cn(
                "num text-base font-semibold text-foreground",
                twr !== null && signColour(amountSign(twr))
              )}
            >
              {twr === null ? "—" : formatPercent(twr, { signed: true })}
            </span>
          </p>
          {twr === null && (
            <p className="text-sm text-muted-foreground">
              The tool cannot calculate the Time-Weighted Return of this period.
            </p>
          )}
        </div>
        <ToggleGroup
          type="single"
          variant="outline"
          size="sm"
          spacing={0}
          aria-label="Period"
          value={period}
          onValueChange={(value) => {
            // An empty value means that the user clicked the selected item again.
            if (value) setPeriod(value as HistoryPeriod)
          }}
          className="shrink-0"
        >
          {historyPeriods.map((item) => (
            <ToggleGroupItem
              key={item}
              value={item}
              className={cn("num", selectedItem)}
            >
              {item}
            </ToggleGroupItem>
          ))}
        </ToggleGroup>
      </CardHeader>
      <CardContent className="flex min-w-0 flex-col gap-4">
        {history.valuedAtCost.length > 0 && (
          <ValuedAtCostAlert instruments={history.valuedAtCost} />
        )}
        {points.length === 0 || last === undefined ? (
          <p className="text-sm text-muted-foreground">
            The Account has no Value History.
          </p>
        ) : (
          <>
            <HistoryChart points={points} currency={history.currency} />
            <LastDayAmounts point={last} currency={history.currency} />
          </>
        )}
      </CardContent>
    </Card>
  )
}

// ValuedAtCostAlert names each Instrument that the Value History values at the cost of its Lots.
function ValuedAtCostAlert({
  instruments,
}: {
  instruments: ValuedInstrument[]
}) {
  return (
    <Alert className="border-warning/40 bg-warning/10 *:[svg]:text-warning">
      <TriangleAlertIcon />
      <AlertTitle>Some Instruments are valued at cost</AlertTitle>
      <AlertDescription>
        <p>
          The tool has no price for these Instruments on some days. On these
          days, the value uses the cost of their Lots.
        </p>
        <ul className="list-inside list-disc">
          {instruments.map((instrument) => (
            <li key={instrument.ticker} className="break-words">
              <span className="num">{instrument.ticker}</span> {instrument.name}
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  )
}

type HistoryChartProps = {
  points: ValueHistoryPoint[]
  currency: string
}

// HistoryChart draws the value and the Net Deposits as two lines on a time axis.
function HistoryChart({ points, currency }: HistoryChartProps) {
  const isMobile = useIsMobile()
  const data = points.map(chartPoint)
  const first = data[0].time
  const last = data[data.length - 1].time
  // A period of more than about 4 months shows the month and the year on the axis.
  // It has one tick on the first day of some months. Then no two ticks show the same month.
  const long = last - first > 120 * dayMs
  const ticks = long ? monthTicks(first, last, isMobile ? 4 : 10) : undefined

  return (
    <ChartContainer
      config={chartConfig}
      className="aspect-auto h-64 w-full md:h-80"
      initialDimension={{ width: 320, height: 256 }}
    >
      <LineChart
        data={data}
        margin={{ top: 8, right: 8, bottom: 0, left: 0 }}
        accessibilityLayer
      >
        <CartesianGrid vertical={false} />
        <XAxis
          dataKey="time"
          type="number"
          scale="time"
          domain={[first, last]}
          ticks={ticks}
          tickLine={false}
          axisLine={false}
          tickMargin={8}
          minTickGap={isMobile ? 24 : 40}
          tickFormatter={(time: number) => axisDate(time, long)}
        />
        <YAxis
          tickLine={false}
          axisLine={false}
          width={isMobile ? 44 : 64}
          domain={["auto", "auto"]}
          tickFormatter={(value: number) => compactNumber(value)}
        />
        <ChartTooltip
          content={({ active, payload }) => (
            <HistoryTooltip
              active={active}
              point={(payload?.[0]?.payload as ChartPoint | undefined)?.point}
              currency={currency}
            />
          )}
        />
        <Line
          dataKey="netDeposits"
          type="stepAfter"
          stroke="var(--color-netDeposits)"
          strokeWidth={2}
          strokeDasharray="4 4"
          dot={false}
          isAnimationActive={false}
        />
        <Line
          dataKey="value"
          type="linear"
          stroke="var(--color-value)"
          strokeWidth={2}
          dot={false}
          isAnimationActive={false}
        />
      </LineChart>
    </ChartContainer>
  )
}

type HistoryTooltipProps = {
  active?: boolean
  point?: ValueHistoryPoint
  currency: string
}

// HistoryTooltip shows the date, the value, the Net Deposits and the Total Result of the day under the pointer.
function HistoryTooltip({ active, point, currency }: HistoryTooltipProps) {
  if (!active || !point) return null
  const totalResult = subtractDecimals(point.value, point.netDeposits)
  return (
    <div className="grid min-w-48 gap-1.5 rounded-lg border bg-popover px-2.5 py-1.5 text-xs text-popover-foreground shadow-xl">
      <span className="num font-medium">{formatDate(point.date)}</span>
      <TooltipRow label="Value" colour="value">
        {formatAmount(point.value, currency)}
      </TooltipRow>
      <TooltipRow label="Net Deposits" colour="netDeposits">
        {formatAmount(point.netDeposits, currency)}
      </TooltipRow>
      <TooltipRow label="Total Result">
        <span className={signColour(amountSign(totalResult))}>
          {formatAmount(totalResult, currency, { signed: true })}
        </span>
      </TooltipRow>
    </div>
  )
}

type TooltipRowProps = {
  label: string
  // colour is the key of the line in chartConfig, or undefined for a row with no line.
  colour?: keyof typeof chartConfig
  children: ReactNode
}

// TooltipRow shows one labelled amount of the tooltip, with the colour of its line.
function TooltipRow({ label, colour, children }: TooltipRowProps) {
  return (
    <div className="flex items-center justify-between gap-4">
      <span className="flex items-center gap-1.5 text-muted-foreground">
        {colour && (
          <span
            className="size-2 shrink-0 rounded-[2px]"
            style={{ backgroundColor: chartConfig[colour].color }}
          />
        )}
        {label}
      </span>
      <span className="num font-medium">{children}</span>
    </div>
  )
}

type LastDayAmountsProps = {
  point: ValueHistoryPoint
  currency: string
}

// LastDayAmounts shows the exact amounts of the last day of the period. It is also the legend of the chart.
function LastDayAmounts({ point, currency }: LastDayAmountsProps) {
  const totalResult = subtractDecimals(point.value, point.netDeposits)
  return (
    <div className="flex flex-col gap-2">
      <FigureLabel>
        On <span className="num">{formatDate(point.date)}</span>
      </FigureLabel>
      <ul className="grid gap-2 sm:grid-cols-3">
        <AmountBox label="Value" colour="value">
          {formatAmount(point.value, currency)}
        </AmountBox>
        <AmountBox label="Net Deposits" colour="netDeposits" dashed>
          {formatAmount(point.netDeposits, currency)}
        </AmountBox>
        <AmountBox label="Total Result">
          <span className={signColour(amountSign(totalResult))}>
            {formatAmount(totalResult, currency, { signed: true })}
          </span>
        </AmountBox>
      </ul>
    </div>
  )
}

type AmountBoxProps = {
  label: string
  colour?: keyof typeof chartConfig
  // dashed shows the key of the line as a dashed line, as the line in the chart.
  dashed?: boolean
  children: ReactNode
}

// AmountBox shows one exact amount under the chart, with the key of its line.
function AmountBox({ label, colour, dashed, children }: AmountBoxProps) {
  return (
    <li className="flex min-w-0 flex-col gap-1 rounded-lg border bg-muted/40 p-3">
      <span className="flex items-center gap-2 text-xs text-muted-foreground">
        {colour && (
          <span
            aria-hidden
            className={cn("w-4 shrink-0 border-t-2", dashed && "border-dashed")}
            style={{ borderColor: chartConfig[colour].color }}
          />
        )}
        {label}
      </span>
      <span className="num text-sm font-medium">{children}</span>
    </li>
  )
}

// dayMs is the length of one day in milliseconds.
const dayMs = 24 * 60 * 60 * 1000

// chartPoint gives the geometry of one day. The floats are for the chart only. The texts use the exact amounts.
function chartPoint(point: ValueHistoryPoint): ChartPoint {
  return {
    date: point.date,
    time: dateTime(point.date),
    value: Number(point.value),
    netDeposits: Number(point.netDeposits),
    point,
  }
}

// periodPoints selects the points from the first day of the period to the last point.
// The backend gives the first day of each period. With no first day, it gives no points.
function periodPoints(
  points: ValueHistoryPoint[],
  start: string | null
): ValueHistoryPoint[] {
  if (start === null) return []
  return points.filter((point) => point.date >= start)
}

// dateTime reads a YYYY-MM-DD date as milliseconds since 1970 at midnight UTC.
function dateTime(date: string): number {
  const [year, month, day] = date.split("-").map(Number)
  return Date.UTC(year, month - 1, day)
}

// monthTicks gives the first day of each month in the range, as milliseconds since 1970 in UTC.
// It keeps every nth month, so that the axis has at most max ticks.
function monthTicks(first: number, last: number, max: number): number[] {
  const start = new Date(first)
  const months: number[] = []
  for (let month = start.getUTCMonth() + 1; ; month++) {
    const time = Date.UTC(start.getUTCFullYear(), month, 1)
    if (time > last) break
    months.push(time)
  }
  const step = Math.max(1, Math.ceil(months.length / max))
  return months.filter((_, index) => index % step === 0)
}

// axisDate shows a day of the time axis: the month and the year for a long period, else the day and the month.
function axisDate(time: number, long: boolean): string {
  return new Date(time).toLocaleDateString(undefined, {
    timeZone: "UTC",
    ...(long
      ? { month: "short", year: "2-digit" }
      : { day: "numeric", month: "short" }),
  })
}

// compactNumber shows an axis value in a short form, for example "12K". It is for the axis only.
function compactNumber(value: number): string {
  return new Intl.NumberFormat(undefined, {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(value)
}
