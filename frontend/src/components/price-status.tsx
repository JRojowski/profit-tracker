import { RefreshCwIcon, TriangleAlertIcon } from "lucide-react"

import { countInstruments, PriceWarningList } from "@/components/price-warnings"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Progress } from "@/components/ui/progress"
import type { PriceFailure, RefreshProgress } from "@/lib/api"
import type { PriceRefreshState } from "@/lib/app-state"
import { formatDate, formatTime } from "@/lib/format"
import { cn } from "@/lib/utils"

// PriceDates tells how current the prices and the Exchange Rates of a view are.
export type PriceDates = {
  priceDate: string | null
  exchangeRateDate: string | null
  stalePrices: boolean
  staleRates: boolean
}

type PriceDatesLineProps = {
  // dates is null while the dates of the view load.
  dates: PriceDates | null
  refresh: PriceRefreshState
  className?: string
}

// PriceDatesLine shows the date of the prices and the Exchange Rates, and the result of the last refresh.
export function PriceDatesLine({
  dates,
  refresh,
  className,
}: PriceDatesLineProps) {
  return (
    <p
      className={cn(
        "flex flex-wrap items-center gap-x-1.5 font-mono text-xs text-muted-foreground",
        className
      )}
    >
      {dates && (
        <>
          <span className="whitespace-nowrap">
            {dates.priceDate ? (
              <>
                Prices:{" "}
                <span className="num text-foreground">
                  {formatDate(dates.priceDate)}
                </span>
              </>
            ) : (
              "No prices"
            )}
          </span>
          {dates.exchangeRateDate && (
            <span className="whitespace-nowrap">
              · Exchange Rates:{" "}
              <span className="num text-foreground">
                {formatDate(dates.exchangeRateDate)}
              </span>
            </span>
          )}
        </>
      )}
      {refresh.result && refresh.refreshedAt && (
        <span className="whitespace-nowrap" role="status">
          · Got {refresh.result.refreshed}{" "}
          {refresh.result.refreshed === 1 ? "price" : "prices"}
          {refresh.result.historyStored > 0 && (
            <>
              {" "}
              and{" "}
              <span className="num">
                {refresh.result.historyStored.toLocaleString()}
              </span>{" "}
              {refresh.result.historyStored === 1
                ? "daily price"
                : "daily prices"}
            </>
          )}{" "}
          at <span className="num">{formatTime(refresh.refreshedAt)}</span>
        </span>
      )}
    </p>
  )
}

// PriceKind tells which prices of an Instrument failed: the current price or the daily prices.
type PriceKind = "current" | "daily"

// KindedFailure is one failure of a price refresh, with the kind of the prices that failed.
type KindedFailure = PriceFailure & { kind: PriceKind }

// withKind gives each failure the kind of the prices that failed.
function withKind(failures: PriceFailure[], kind: PriceKind): KindedFailure[] {
  return failures.map((failure) => ({ ...failure, kind }))
}

type PriceAlertsProps = {
  dates: PriceDates | null
  refresh: PriceRefreshState
}

// PriceAlerts shows the failures and the Price Warnings of the last refresh, and the warnings about prices and
// Exchange Rates that are not current.
export function PriceAlerts({ dates, refresh }: PriceAlertsProps) {
  const failures = [
    ...withKind(refresh.result?.failures ?? [], "current"),
    ...withKind(refresh.result?.historyFailures ?? [], "daily"),
  ]
  const rateFailures = refresh.result?.rateFailures ?? []
  const stopMessage = refresh.result?.stopMessage ?? null
  const priceWarnings = refresh.result?.priceWarnings ?? []
  const warnedInstruments = countInstruments(priceWarnings)
  const alerts = [
    refresh.refreshing && (
      <RefreshProgressAlert key="progress" progress={refresh.progress} />
    ),
    stopMessage && (
      <Alert key="stop" variant="destructive">
        <TriangleAlertIcon />
        <AlertTitle>The refresh stopped early</AlertTitle>
        <AlertDescription>
          {stopMessage}. The tool keeps the prices that it got.
        </AlertDescription>
      </Alert>
    ),
    refresh.error && (
      <Alert key="error" variant="destructive">
        <TriangleAlertIcon />
        <AlertTitle>Cannot refresh the prices</AlertTitle>
        <AlertDescription>{refresh.error}</AlertDescription>
      </Alert>
    ),
    failures.length > 0 && (
      <Alert key="failures" variant="destructive">
        <TriangleAlertIcon />
        <AlertTitle>Some Instruments got no new price</AlertTitle>
        <AlertDescription>
          <ul className="list-inside list-disc">
            {failures.map((failure) => (
              <li key={`${failure.kind}-${failure.ticker}`}>
                {failure.ticker}
                {failure.kind === "daily" && " (daily prices)"}: {failure.error}
              </li>
            ))}
          </ul>
        </AlertDescription>
      </Alert>
    ),
    priceWarnings.length > 0 && (
      <Alert
        key="price-warnings"
        className="border-warning/40 bg-warning/10 *:[svg]:text-warning"
      >
        <TriangleAlertIcon />
        <AlertTitle>
          <span className="num">{warnedInstruments}</span>{" "}
          {warnedInstruments === 1
            ? "Instrument has a Price Warning"
            : "Instruments have a Price Warning"}
        </AlertTitle>
        <AlertDescription>
          <PriceWarningList warnings={priceWarnings} />
        </AlertDescription>
      </Alert>
    ),
    rateFailures.length > 0 && (
      <Alert key="rate-failures" variant="destructive">
        <TriangleAlertIcon />
        <AlertTitle>Some currencies got no new Exchange Rate</AlertTitle>
        <AlertDescription>
          <ul className="list-inside list-disc">
            {rateFailures.map((failure) => (
              <li key={failure.currency}>
                {failure.currency}: {failure.error}
              </li>
            ))}
          </ul>
        </AlertDescription>
      </Alert>
    ),
    dates?.staleRates && (
      <Alert key="stale-rates">
        <TriangleAlertIcon />
        <AlertTitle>The Exchange Rates are not current</AlertTitle>
        <AlertDescription>
          {dates.exchangeRateDate
            ? "The tool did not get the Exchange Rates today. The values use the last stored Exchange Rates."
            : "The tool has no Exchange Rate that the values need. Refresh the prices."}
        </AlertDescription>
      </Alert>
    ),
    dates?.stalePrices && (
      <Alert key="stale-prices">
        <TriangleAlertIcon />
        <AlertTitle>The prices are not current</AlertTitle>
        <AlertDescription>
          {dates.priceDate
            ? "A Holding has no price from today. The values use the last stored prices."
            : "No Holding has a price. Refresh the prices."}
        </AlertDescription>
      </Alert>
    ),
  ].filter(Boolean)

  if (alerts.length === 0) return null
  return <div className="flex flex-col gap-2">{alerts}</div>
}

// RefreshProgressAlert shows the progress of a running price refresh: the step, the count and the Instrument.
// The count shows only during the step of the daily prices. Before the first read, the step is not known.
function RefreshProgressAlert({
  progress,
}: {
  progress: RefreshProgress | null
}) {
  const daily = progress !== null && progress.running && progress.total > 0
  return (
    <Alert role="status">
      <RefreshCwIcon className="animate-spin" />
      <AlertTitle>Refreshing the prices</AlertTitle>
      <AlertDescription className="flex flex-col gap-2">
        {daily ? (
          <>
            <span>
              Getting the daily prices:{" "}
              <span className="num text-foreground">
                {progress.done} / {progress.total}
              </span>
              {progress.ticker && (
                <>
                  {" "}
                  ·{" "}
                  <span className="num text-foreground">{progress.ticker}</span>
                </>
              )}
            </span>
            <Progress
              value={(progress.done / progress.total) * 100}
              aria-label="Daily prices"
            />
          </>
        ) : (
          <span>
            Getting the current prices and the Exchange Rates. The first refresh
            can take some minutes.
          </span>
        )}
      </AlertDescription>
    </Alert>
  )
}
