import { TriangleAlertIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
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
          {refresh.result.refreshed === 1 ? "price" : "prices"} at{" "}
          <span className="num">{formatTime(refresh.refreshedAt)}</span>
        </span>
      )}
    </p>
  )
}

type PriceAlertsProps = {
  dates: PriceDates | null
  refresh: PriceRefreshState
}

// PriceAlerts shows the failures of the last refresh and the warnings about prices and Exchange Rates that are not current.
export function PriceAlerts({ dates, refresh }: PriceAlertsProps) {
  const failures = refresh.result?.failures ?? []
  const rateFailures = refresh.result?.rateFailures ?? []
  const alerts = [
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
              <li key={failure.ticker}>
                {failure.ticker}: {failure.error}
              </li>
            ))}
          </ul>
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
