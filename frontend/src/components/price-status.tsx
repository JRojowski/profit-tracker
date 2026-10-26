import { RefreshCwIcon, TriangleAlertIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import type { AccountSummary, PriceRefresh } from "@/lib/api"
import { formatDate, formatTime } from "@/lib/format"

type PriceStatusProps = {
  summary: AccountSummary
  refreshing: boolean
  // refresh is the result of the last price refresh, or null before the first refresh.
  refresh: PriceRefresh | null
  // refreshedAt is the time of the last successful price refresh, or null.
  refreshedAt: Date | null
  // refreshError is the error of the last price refresh, or null.
  refreshError: string | null
  onRefresh: () => void
}

// PriceStatus shows the date of the prices, the "Refresh prices" button and the price warnings.
export function PriceStatus({
  summary,
  refreshing,
  refresh,
  refreshedAt,
  refreshError,
  onRefresh,
}: PriceStatusProps) {
  return (
    <div className="flex flex-col gap-2">
      <div className="flex items-center gap-4">
        <p className="text-sm text-muted-foreground">
          {summary.priceDate
            ? `Prices from ${formatDate(summary.priceDate)}`
            : "No prices"}
        </p>
        <Button variant="outline" onClick={onRefresh} disabled={refreshing}>
          <RefreshCwIcon data-icon="inline-start" />
          {refreshing ? "Refreshing…" : "Refresh prices"}
        </Button>
        {refresh && refreshedAt && (
          <p className="text-sm text-muted-foreground" role="status">
            Got {refresh.refreshed}{" "}
            {refresh.refreshed === 1 ? "price" : "prices"} at{" "}
            {formatTime(refreshedAt)}.
          </p>
        )}
      </div>
      {refreshError && (
        <Alert variant="destructive">
          <TriangleAlertIcon />
          <AlertTitle>Cannot refresh the prices</AlertTitle>
          <AlertDescription>{refreshError}</AlertDescription>
        </Alert>
      )}
      {refresh && refresh.failures.length > 0 && (
        <Alert variant="destructive">
          <TriangleAlertIcon />
          <AlertTitle>Some Instruments got no new price</AlertTitle>
          <AlertDescription>
            <ul className="list-inside list-disc">
              {refresh.failures.map((failure) => (
                <li key={failure.ticker}>
                  {failure.ticker}: {failure.error}
                </li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}
      {summary.stalePrices && (
        <Alert>
          <TriangleAlertIcon />
          <AlertTitle>The prices are not current</AlertTitle>
          <AlertDescription>
            {summary.priceDate
              ? "A Holding has no price from today. The values use the last stored prices."
              : "No Holding has a price. Refresh the prices."}
          </AlertDescription>
        </Alert>
      )}
    </div>
  )
}
