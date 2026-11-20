import { SectionHeading } from "@/components/section-heading"
import { useEffect, useMemo, useRef, type ReactNode } from "react"
import { MessageSquareIcon, PencilIcon } from "lucide-react"

import { CurrencyBadge } from "@/components/account-badges"
import { InstrumentPriceChart } from "@/components/instrument-price-chart"
import { LoadedView } from "@/components/loaded-view"
import { NoValue, ProfitFigure, Stacked } from "@/components/value-figures"
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
  type ClosedPosition,
  type Holding,
  type InstrumentOverview,
  type OverviewLot,
  type PriceWarning,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { useAssistant } from "@/lib/assistant-state"
import {
  amountSign,
  formatAmount,
  formatDate,
  formatPolishDate,
  formatPolishTime,
  formatPrice,
  formatRate,
  formatVolume,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"
import { buildChartModel, type MarkerKind } from "@/lib/overview-markers"

// markerKinds are the kinds of markers that the chart draws.
const markerKinds: MarkerKind[] = ["buy", "sell"]

type InstrumentOverviewPanelProps = {
  accountId: number
  // ticker is the broker ticker of the Instrument.
  ticker: string
  // currency is the currency of the Account. The cost and the value are in it.
  currency: string
  // closed is the XTB ID of the Closed Position to highlight, or null. An ID that no Closed Position of the
  // Instrument has gives no highlight.
  closed: string | null
  // onSelectClosed highlights a Closed Position, or removes the highlight for null. A click on a card runs it.
  onSelectClosed: (closed: string | null) => void
  onClose: () => void
}

// InstrumentOverviewPanel shows the Instrument Overview of one Instrument on one Account in a panel at the right side.
// The panel does not block the page, so the user can select another row while it is open.
// It is open for as long as it is mounted. The owner mounts it for the ticker in the URL.
export function InstrumentOverviewPanel({
  accountId,
  ticker,
  currency,
  closed,
  onSelectClosed,
  onClose,
}: InstrumentOverviewPanelProps) {
  const { valuesVersion, refreshAllPrices } = useAppState()
  const assistant = useAssistant()
  const loaded = useLoad(`${accountId}:${ticker}`, valuesVersion, (signal) =>
    fetchInstrumentOverview(accountId, ticker, signal)
  )
  const overview =
    loaded.status === "ok" && loaded.data !== null ? loaded.data : null
  // A sold Instrument has no Holding and so no Price Warnings in the response.
  const warnings: PriceWarning[] = overview?.holding?.priceWarnings ?? []

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
        // The Assistant panel can be open above this panel and have opened before it. Then Escape closes the
        // Assistant first, as it does when the Assistant opened last.
        onEscapeKeyDown={(event) => {
          if (!assistant.open) return
          event.preventDefault()
          assistant.setOpen(false)
        }}
      >
        <SheetHeader className="gap-1.5 border-b pr-24">
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
          {assistant.available && (
            // On a phone this panel covers the top bar and the sidebar. This control opens the Assistant, which
            // takes the Instrument of this panel as its context.
            <Button
              variant="ghost"
              size="icon-sm"
              className="absolute top-3 right-12"
              aria-label="Ask the Assistant about this Instrument"
              onClick={() => assistant.setOpen(true)}
            >
              <MessageSquareIcon />
            </Button>
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
                  <OverviewBody
                    overview={data}
                    currency={currency}
                    closed={closed}
                    onSelectClosed={onSelectClosed}
                  />
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

// OverviewBody shows the price chart, the Holding and the open Lots of the Instrument.
// A fully sold Instrument has no Holding and no open Lots, so it shows the chart and the Closed Positions.
function OverviewBody({
  overview,
  currency: accountCurrency,
  closed,
  onSelectClosed,
}: {
  overview: InstrumentOverview
  currency: string
  closed: string | null
  onSelectClosed: (closed: string | null) => void
}) {
  const { prices, quoteCurrency, fills, closedPositions } = overview
  const highlight = useMemo(
    () =>
      closedPositions.find(
        (closedPosition) => closedPosition.xtbId === closed
      ) ?? null,
    [closedPositions, closed]
  )
  const model = useMemo(
    () =>
      buildChartModel(prices, quoteCurrency, fills, closedPositions, highlight),
    [prices, quoteCurrency, fills, closedPositions, highlight]
  )
  const { currency, mismatch } = model
  return (
    <>
      <section className="flex flex-col gap-3">
        <SectionHeading
          level="h3"
          title="Price"
          action={
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
              Buy{" "}
              <span aria-hidden className="text-chart-8">
                ◆
              </span>{" "}
              Sell
            </span>
          }
        />
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
              ? "1 buy or sell operation has no price and no marker."
              : `${model.unplaced} buy or sell operations have no price and no marker.`}
          </p>
        )}
      </section>
      {overview.holding && (
        <HoldingSection holding={overview.holding} currency={accountCurrency} />
      )}
      {overview.openLots.length > 0 && (
        <LotList lots={overview.openLots} currency={accountCurrency} />
      )}
      {overview.realizedProfitSum !== null && (
        <RealizedProfitSection
          sum={overview.realizedProfitSum}
          currency={accountCurrency}
        />
      )}
      {closedPositions.length > 0 && (
        <ClosedPositionList
          closedPositions={closedPositions}
          currency={accountCurrency}
          highlightedId={highlight?.xtbId ?? null}
          onSelect={onSelectClosed}
        />
      )}
    </>
  )
}

// DetailRow is one label and one value of the Holding section.
function DetailRow({
  label,
  children,
}: {
  label: string
  children: ReactNode
}) {
  return (
    <div className="flex items-baseline justify-between gap-4 py-2">
      <dt className="text-muted-foreground">{label}</dt>
      <dd className="text-right num">{children}</dd>
    </div>
  )
}

// HoldingSection shows the Holding of the Instrument in the formats of the Holdings table.
// A Holding with no value shows its note, and the empty fields show a dash.
function HoldingSection({
  holding,
  currency,
}: {
  holding: Holding
  currency: string
}) {
  return (
    <section className="flex flex-col gap-2">
      <SectionHeading level="h3" title="Holding" />
      <dl className="flex flex-col divide-y rounded-lg border px-3 text-sm">
        <DetailRow label="Volume">{formatVolume(holding.volume)}</DetailRow>
        <DetailRow label="Average cost">
          {formatAmount(holding.averageCost, currency)}
        </DetailRow>
        <DetailRow label="Cost">
          {formatAmount(holding.cost, currency)}
        </DetailRow>
        <DetailRow label="Price">
          {holding.price !== null && holding.priceDate !== null ? (
            <Stacked
              main={formatAmount(
                holding.price,
                holding.quoteCurrency ?? currency
              )}
              sub={formatDate(holding.priceDate)}
            />
          ) : (
            <NoValue />
          )}
        </DetailRow>
        <DetailRow label="Exchange Rate">
          {holding.exchangeRate !== null &&
          holding.exchangeRateCurrency !== null &&
          holding.exchangeRateDate !== null ? (
            <Stacked
              main={`1 ${holding.exchangeRateCurrency} = ${formatRate(holding.exchangeRate)} PLN`}
              sub={formatDate(holding.exchangeRateDate)}
            />
          ) : (
            <NoValue />
          )}
        </DetailRow>
        <DetailRow label="Value">
          {holding.value !== null ? (
            <span className="font-medium">
              {formatAmount(holding.value, currency)}
            </span>
          ) : (
            <NoValue />
          )}
        </DetailRow>
        <DetailRow label="Unrealized Profit">
          {holding.unrealizedProfit !== null ? (
            <ProfitFigure
              profit={holding.unrealizedProfit}
              percent={holding.unrealizedProfitPercent}
              currency={currency}
            />
          ) : (
            <NoValue />
          )}
        </DetailRow>
      </dl>
      {holding.note && (
        <p className="text-sm text-muted-foreground">{holding.note}</p>
      )}
    </section>
  )
}

// RealizedProfitSection shows the sum of the Realized Profit of all Closed Positions of the Instrument on the Account.
// The backend adds the amounts. The section shows the sum with its sign and the profit colour.
function RealizedProfitSection({
  sum,
  currency,
}: {
  sum: string
  currency: string
}) {
  return (
    <section>
      <dl className="flex flex-col rounded-lg border px-3 text-sm">
        <DetailRow label="Realized Profit">
          <span
            className={cn("font-medium", signColour(amountSign(sum)))}
            data-realized-profit-sum=""
          >
            {formatAmount(sum, currency, { signed: true })}
          </span>
        </DetailRow>
      </dl>
    </section>
  )
}

// LotList lists the open Lots with the date, the open volume, the price and the cost of the open part.
// The price is the average Fill price of the Lot. A Lot with two or more Fills lists them under its row.
// The list replaces the list of Fills, so no Fill shows twice.
function LotList({
  lots,
  currency,
}: {
  lots: OverviewLot[]
  currency: string
}) {
  return (
    <section className="flex flex-col gap-2">
      <SectionHeading
        level="h3"
        title={
          <>
            Open Lots
            <span className="num text-muted-foreground">{lots.length}</span>
          </>
        }
      />
      <ul className="flex flex-col divide-y rounded-lg border text-sm">
        {lots.map((lot) => (
          <li key={lot.positionId} className="flex flex-col gap-1 px-3 py-2">
            <div className="grid grid-cols-[1fr_auto] gap-x-3 gap-y-0.5">
              <span className="num">{formatPolishDate(lot.openTime)}</span>
              <span className="text-right num">
                {formatAmount(lot.cost, currency)}
              </span>
              <span className="num text-xs text-muted-foreground">
                {formatVolume(lot.volume)} ×{" "}
                {lot.openPrice === null ? "—" : formatPrice(lot.openPrice)}
              </span>
              <span className="text-right text-xs text-muted-foreground">
                Cost
              </span>
            </div>
            {lot.fills.length > 1 && (
              <ul className="flex flex-col gap-0.5 pl-3 text-xs text-muted-foreground">
                {lot.fills.map((fill) => (
                  <li
                    key={fill.xtbId}
                    className="flex justify-between gap-3 num"
                  >
                    <span>
                      {formatPolishDate(fill.time)}{" "}
                      {formatPolishTime(fill.time)}
                    </span>
                    <span>
                      {formatVolume(fill.volume)} ×{" "}
                      {fill.price === null ? "—" : formatPrice(fill.price)}
                    </span>
                  </li>
                ))}
              </ul>
            )}
          </li>
        ))}
      </ul>
    </section>
  )
}

// ClosedPositionList lists the Closed Positions of the Instrument, newest first, with all their figures.
// A click on a card highlights it, and a click on the highlighted card removes the highlight.
// A highlight from the URL scrolls its card into view. A highlight from a click does not scroll the list.
function ClosedPositionList({
  closedPositions,
  currency,
  highlightedId,
  onSelect,
}: {
  closedPositions: ClosedPosition[]
  currency: string
  highlightedId: string | null
  onSelect: (closed: string | null) => void
}) {
  // selectedByClick is true from a click that highlights a card until that card has rendered as highlighted.
  const selectedByClick = useRef(false)
  const consumeClick = () => {
    const clicked = selectedByClick.current
    selectedByClick.current = false
    return clicked
  }
  return (
    <section className="flex flex-col gap-2">
      <SectionHeading
        level="h3"
        title={
          <>
            Closed Positions
            <span className="num text-muted-foreground">
              {closedPositions.length}
            </span>
          </>
        }
      />
      <ul className="flex flex-col divide-y rounded-lg border text-sm">
        {closedPositions.toReversed().map((closedPosition) => {
          const highlighted = closedPosition.xtbId === highlightedId
          return (
            <ClosedPositionRow
              key={closedPosition.xtbId}
              closedPosition={closedPosition}
              currency={currency}
              highlighted={highlighted}
              consumeClick={consumeClick}
              onToggle={() => {
                selectedByClick.current = !highlighted
                onSelect(highlighted ? null : closedPosition.xtbId)
              }}
            />
          )
        })}
      </ul>
    </section>
  )
}

// ClosedPositionRow shows one Closed Position as a card in two columns: the open figures at the left and the close
// figures at the right. The card is a toggle button. Enter and Space select it.
function ClosedPositionRow({
  closedPosition,
  currency,
  highlighted,
  consumeClick,
  onToggle,
}: {
  closedPosition: ClosedPosition
  currency: string
  highlighted: boolean
  // consumeClick tells whether a click highlighted the card, and clears that fact.
  consumeClick: () => boolean
  onToggle: () => void
}) {
  const ref = useRef<HTMLDivElement>(null)
  useEffect(() => {
    if (!highlighted) return
    if (!consumeClick()) ref.current?.scrollIntoView({ block: "nearest" })
    // consumeClick changes on each render, but only a change of the highlight must scroll.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [highlighted])
  // The figures are in row order of a two-column grid: open figure, close figure, open figure, close figure, ...
  const figures: [string, ReactNode][] = [
    ["Open date", formatPolishDate(closedPosition.openTime)],
    ["Close date", formatPolishDate(closedPosition.time)],
    ["Open price", <OptionalPrice price={closedPosition.openPrice} />],
    ["Close price", <OptionalPrice price={closedPosition.closePrice} />],
    ["Volume", formatVolume(closedPosition.volume)],
    ["Sale amount", formatAmount(closedPosition.amount, currency)],
    ["Cost", formatAmount(closedPosition.cost, currency)],
    [
      "Realized Profit",
      <span
        className={cn(
          "font-medium",
          signColour(amountSign(closedPosition.realizedProfit))
        )}
      >
        {formatAmount(closedPosition.realizedProfit, currency, {
          signed: true,
        })}
      </span>,
    ],
  ]
  return (
    <li
      data-highlighted={highlighted ? "" : undefined}
      aria-current={highlighted ? "true" : undefined}
    >
      <div
        ref={ref}
        role="button"
        tabIndex={0}
        aria-pressed={highlighted}
        onClick={onToggle}
        onKeyDown={(event) => {
          if (event.key !== "Enter" && event.key !== " ") return
          event.preventDefault()
          onToggle()
        }}
        className={cn(
          "grid cursor-pointer grid-cols-2 gap-x-6 gap-y-1 px-3 py-2.5 outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-inset",
          highlighted && "bg-primary/10 ring-2 ring-primary ring-inset"
        )}
      >
        {figures.map(([label, value]) => (
          <div
            key={label}
            className="flex items-baseline justify-between gap-2"
          >
            <span className="text-xs text-muted-foreground">{label}</span>
            <span className="text-right num">{value}</span>
          </div>
        ))}
      </div>
    </li>
  )
}

// OptionalPrice shows a price with no currency symbol, or a dash when the price is absent.
function OptionalPrice({ price }: { price: string | null }) {
  return price === null ? <NoValue /> : <>{formatPrice(price)}</>
}
