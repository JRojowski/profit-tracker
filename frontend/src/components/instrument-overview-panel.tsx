import { useMemo } from "react"
import { PencilIcon } from "lucide-react"

import { CurrencyBadge } from "@/components/account-badges"
import { InstrumentPriceChart } from "@/components/instrument-price-chart"
import { LoadedView } from "@/components/loaded-view"
import { PriceSourceDialog } from "@/components/price-source-dialog"
import { PriceWarningBadgeButton } from "@/components/price-warnings"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { useLoad } from "@/hooks/use-load"
import {
  fetchInstrumentOverview,
  type InstrumentOverview,
  type PriceWarning,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import {
  formatPolishDate,
  formatPolishTime,
  formatPrice,
  formatVolume,
} from "@/lib/format"
import { buildChartModel, type MarkerKind } from "@/lib/overview-markers"

// markerKinds are the kinds of markers that the chart draws. A later ticket adds "sell".
const markerKinds: MarkerKind[] = ["buy"]

type InstrumentOverviewPanelProps = {
  accountId: number
  // ticker is the broker ticker of the Instrument.
  ticker: string
  // warnings are the Price Warnings of the Instrument, from the Holdings of the Account. They can be empty.
  warnings: PriceWarning[]
  onClose: () => void
}

// InstrumentOverviewPanel shows the Instrument Overview of one Instrument on one Account in a panel at the right side.
// The panel does not block the page, so the user can select another row while it is open.
// It is open for as long as it is mounted. The owner mounts it for the ticker in the URL.
export function InstrumentOverviewPanel({
  accountId,
  ticker,
  warnings,
  onClose,
}: InstrumentOverviewPanelProps) {
  const { valuesVersion, refreshAllPrices } = useAppState()
  const loaded = useLoad(`${accountId}:${ticker}`, valuesVersion, (signal) =>
    fetchInstrumentOverview(accountId, ticker, signal)
  )
  const overview =
    loaded.status === "ok" && loaded.data !== null ? loaded.data : null

  return (
    <Sheet
      open
      modal={false}
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
    >
      <SheetContent
        data-overview-panel=""
        className="data-[side=right]:w-full data-[side=right]:sm:max-w-xl"
        // A click on the page behind the panel must not close it. The panel closes with its own control and with Escape.
        onInteractOutside={(event) => event.preventDefault()}
      >
        <SheetHeader className="gap-1.5 border-b pr-12">
          <SheetTitle className="flex flex-wrap items-center gap-2">
            <span className="num font-mono">{ticker}</span>
            {overview?.quoteCurrency && (
              <CurrencyBadge currency={overview.quoteCurrency} />
            )}
            {overview && warnings.length > 0 && (
              <PriceWarningBadgeButton
                instrument={overview}
                warnings={warnings}
                onChanged={refreshAllPrices}
              />
            )}
          </SheetTitle>
          <SheetDescription>
            {overview ? overview.name : "Instrument Overview"}
          </SheetDescription>
          {overview && (
            <PriceSourceLine
              overview={overview}
              warnings={warnings}
              onChanged={refreshAllPrices}
            />
          )}
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-4 pb-4">
          {loaded.status === "ok" && loaded.data === null ? (
            <UnknownInstrument ticker={ticker} />
          ) : (
            <LoadedView
              loaded={loaded}
              errorTitle="Cannot load the Instrument Overview"
            >
              {(data) =>
                data === null ? (
                  <UnknownInstrument ticker={ticker} />
                ) : (
                  <OverviewBody overview={data} />
                )
              }
            </LoadedView>
          )}
        </div>
      </SheetContent>
    </Sheet>
  )
}

// UnknownInstrument is the empty state for a ticker that the Account never held, for example from an old link.
function UnknownInstrument({ ticker }: { ticker: string }) {
  return (
    <div className="flex flex-col gap-1 py-6 text-sm text-muted-foreground">
      <p className="font-medium text-foreground">No such Instrument</p>
      <p>
        This Account never held an Instrument with the ticker{" "}
        <span className="num">{ticker}</span>.
      </p>
    </div>
  )
}

type PriceSourceLineProps = {
  overview: InstrumentOverview
  warnings: PriceWarning[]
  onChanged: () => void
}

// PriceSourceLine shows the price ticker of the Instrument and the control that changes the price source.
function PriceSourceLine({
  overview,
  warnings,
  onChanged,
}: PriceSourceLineProps) {
  return (
    <div className="flex flex-wrap items-center gap-2 pt-1 text-xs text-muted-foreground">
      <span>Price source</span>
      <span className="num">{overview.priceTicker}</span>
      {(overview.priceTickerManual || overview.quoteCurrencyManual) && (
        <Badge variant="secondary">manual</Badge>
      )}
      <PriceSourceDialog
        instrument={overview}
        warnings={warnings}
        onChanged={onChanged}
        trigger={
          <Button variant="outline" size="xs">
            <PencilIcon data-icon="inline-start" />
            Change the price source
          </Button>
        }
      />
    </div>
  )
}

// OverviewBody shows the price chart and the list of Fills of the Instrument.
function OverviewBody({ overview }: { overview: InstrumentOverview }) {
  const { prices, quoteCurrency, fills } = overview
  const model = useMemo(
    () => buildChartModel(prices, quoteCurrency, fills),
    [prices, quoteCurrency, fills]
  )
  const { currency, mismatch } = model
  return (
    <>
      <section className="flex flex-col gap-3">
        <div className="flex flex-wrap items-baseline justify-between gap-x-4 gap-y-1">
          <h3 className="font-heading text-sm font-medium">Price</h3>
          <span className="text-xs text-muted-foreground">
            {mismatch
              ? `Broker prices in ${mismatch.quoteCurrency}`
              : currency
                ? `Daily close in ${currency}`
                : "Daily close"}
            {" · "}
            <span aria-hidden className="text-chart-7">
              ▲
            </span>{" "}
            Buy
          </span>
        </div>
        {prices.series.length === 0 && (
          <Alert className="border-warning/40 bg-warning/10 *:[svg]:text-warning">
            <AlertTitle>No price data</AlertTitle>
            <AlertDescription>
              The price source has no data for the price ticker{" "}
              <span className="num">{overview.priceTicker}</span>. The chart
              shows only the Fills. Change the price source to get a price line.
            </AlertDescription>
          </Alert>
        )}
        {mismatch && (
          <Alert className="border-warning/40 bg-warning/10 *:[svg]:text-warning">
            <AlertTitle>Price line not shown</AlertTitle>
            <AlertDescription>
              The price source gives the price in{" "}
              <span className="num">{mismatch.seriesCurrency}</span>, but the
              Quote Currency is{" "}
              <span className="num">{mismatch.quoteCurrency}</span>. The chart
              shows only the Fills. Change the price source to get a price line.
            </AlertDescription>
          </Alert>
        )}
        {model.rows.length > 0 && (
          <InstrumentPriceChart
            model={model}
            currency={currency}
            kinds={markerKinds}
          />
        )}
        {model.unplaced > 0 && (
          <p className="text-xs text-muted-foreground">
            {model.unplaced === 1
              ? "1 Fill has no price and no marker."
              : `${model.unplaced} Fills have no price and no marker.`}
          </p>
        )}
      </section>
      <FillList overview={overview} />
    </>
  )
}

// FillList lists each Fill with its date, time, volume and price. It is the text alternative of the Fill markers.
function FillList({ overview }: { overview: InstrumentOverview }) {
  const { fills } = overview
  return (
    <section className="flex flex-col gap-2">
      <h3 className="font-heading text-sm font-medium">
        Fills <span className="num text-muted-foreground">{fills.length}</span>
      </h3>
      <ul className="flex flex-col divide-y rounded-lg border text-sm">
        {fills.map((fill) => (
          <li
            key={fill.xtbId}
            className="flex items-baseline justify-between gap-3 px-3 py-2"
          >
            <span className="num">
              {formatPolishDate(fill.time)}{" "}
              <span className="text-xs text-muted-foreground">
                {formatPolishTime(fill.time)}
              </span>
            </span>
            <span className="num">
              {formatVolume(fill.volume)} ×{" "}
              {fill.price === null ? "—" : formatPrice(fill.price)}
            </span>
          </li>
        ))}
      </ul>
    </section>
  )
}
