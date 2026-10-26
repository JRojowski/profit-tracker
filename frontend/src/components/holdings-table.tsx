import { CurrencyBadge } from "@/components/account-badges"
import { signColour } from "@/components/overview-hero"
import { PriceSourceDialog } from "@/components/price-source-dialog"
import { Badge } from "@/components/ui/badge"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useIsMobile } from "@/hooks/use-mobile"
import type { Holding } from "@/lib/api"
import {
  amountSign,
  formatAmount,
  formatDate,
  formatPercent,
  formatRate,
  formatVolume,
} from "@/lib/format"
import { cn } from "@/lib/utils"

type HoldingsTableProps = {
  holdings: Holding[]
  currency: string
  // onPriceSourceChanged runs after the user changed the price source of an Instrument.
  onPriceSourceChanged: () => void
}

// HoldingsTable shows the Holdings of an Account: a table on a wide screen and a list of compact rows on a narrow screen.
// Each row opens the price source dialog. A note under the table lists the Holdings with no value.
export function HoldingsTable({
  holdings,
  currency,
  onPriceSourceChanged,
}: HoldingsTableProps) {
  const isMobile = useIsMobile()

  if (holdings.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        The Account has no Holdings.
      </p>
    )
  }

  return (
    <div className="flex min-w-0 flex-col gap-4">
      {isMobile ? (
        <HoldingRows
          holdings={holdings}
          currency={currency}
          onPriceSourceChanged={onPriceSourceChanged}
        />
      ) : (
        <WideTable
          holdings={holdings}
          currency={currency}
          onPriceSourceChanged={onPriceSourceChanged}
        />
      )}
      <NoValueNote holdings={holdings} />
    </div>
  )
}

// WideTable shows all details of each Holding in columns.
function WideTable({
  holdings,
  currency,
  onPriceSourceChanged,
}: HoldingsTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Instrument</TableHead>
          <TableHead className="text-right">Volume</TableHead>
          <TableHead className="text-right">Average cost</TableHead>
          <TableHead className="text-right">Cost</TableHead>
          <TableHead className="text-right">Price</TableHead>
          <TableHead className="text-right">Exchange Rate</TableHead>
          <TableHead className="text-right">Value</TableHead>
          <TableHead className="text-right">Unrealized Profit</TableHead>
          <TableHead>Price source</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {holdings.map((holding) => (
          <TableRow key={holding.ticker}>
            <TableCell>
              <div className="flex flex-col gap-0.5">
                <span className="flex items-center gap-2">
                  <span className="font-medium">{holding.ticker}</span>
                  {holding.quoteCurrency && (
                    <CurrencyBadge currency={holding.quoteCurrency} />
                  )}
                </span>
                <span className="max-w-56 truncate text-xs text-muted-foreground">
                  {holding.name}
                </span>
              </div>
            </TableCell>
            <TableCell className="text-right num">
              {formatVolume(holding.volume)}
            </TableCell>
            <TableCell className="text-right num">
              {formatAmount(holding.averageCost, currency)}
            </TableCell>
            <TableCell className="text-right num">
              {formatAmount(holding.cost, currency)}
            </TableCell>
            <TableCell className="text-right num">
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
            </TableCell>
            <TableCell className="text-right num">
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
            </TableCell>
            <TableCell className="text-right num font-medium">
              {holding.value !== null ? (
                formatAmount(holding.value, currency)
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right num">
              {holding.unrealizedProfit !== null ? (
                <div
                  className={cn(
                    "flex flex-col",
                    signColour(amountSign(holding.unrealizedProfit))
                  )}
                >
                  <span>
                    {formatAmount(holding.unrealizedProfit, currency, {
                      signed: true,
                    })}
                  </span>
                  {holding.unrealizedProfitPercent !== null && (
                    <span className="text-xs">
                      {formatPercent(holding.unrealizedProfitPercent, {
                        signed: true,
                      })}
                    </span>
                  )}
                </div>
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell>
              <div className="flex items-center gap-2">
                <span className="num text-xs text-muted-foreground">
                  {holding.priceTicker}
                </span>
                {(holding.priceTickerManual || holding.quoteCurrencyManual) && (
                  <Badge variant="secondary">manual</Badge>
                )}
                <PriceSourceDialog
                  holding={holding}
                  onChanged={onPriceSourceChanged}
                />
              </div>
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

// HoldingRows shows each Holding as one compact row. A tap on the row opens the price source dialog.
function HoldingRows({
  holdings,
  currency,
  onPriceSourceChanged,
}: HoldingsTableProps) {
  return (
    <ul className="-mx-4 flex flex-col divide-y border-y">
      {holdings.map((holding) => (
        <li key={holding.ticker}>
          <PriceSourceDialog
            holding={holding}
            onChanged={onPriceSourceChanged}
            trigger={
              <button
                type="button"
                className="flex w-full items-start justify-between gap-3 px-4 py-3 text-left outline-none hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-inset"
              >
                <span className="flex min-w-0 flex-col gap-1">
                  <span className="flex min-w-0 items-center gap-2">
                    <span className="truncate font-medium">
                      {holding.ticker}
                    </span>
                    {holding.quoteCurrency && (
                      <CurrencyBadge currency={holding.quoteCurrency} />
                    )}
                  </span>
                  <span className="num text-xs text-muted-foreground">
                    {formatVolume(holding.volume)} ×{" "}
                    {holding.price !== null
                      ? formatAmount(
                          holding.price,
                          holding.quoteCurrency ?? currency
                        )
                      : "—"}
                  </span>
                  <span className="sr-only">Edit the price source.</span>
                </span>
                <span className="flex shrink-0 flex-col items-end gap-1">
                  {holding.value !== null ? (
                    <span className="num text-sm font-medium">
                      {formatAmount(holding.value, currency)}
                    </span>
                  ) : (
                    <span className="text-sm text-muted-foreground">
                      No value
                    </span>
                  )}
                  {holding.unrealizedProfit !== null && (
                    <span
                      className={cn(
                        "num text-xs",
                        signColour(amountSign(holding.unrealizedProfit))
                      )}
                    >
                      {formatAmount(holding.unrealizedProfit, currency, {
                        signed: true,
                      })}
                      {holding.unrealizedProfitPercent !== null &&
                        ` (${formatPercent(holding.unrealizedProfitPercent, { signed: true })})`}
                    </span>
                  )}
                </span>
              </button>
            }
          />
        </li>
      ))}
    </ul>
  )
}

// NoValueNote lists the Holdings with no value and tells why each one has no value.
function NoValueNote({ holdings }: { holdings: Holding[] }) {
  const unpriced = holdings.filter((holding) => holding.value === null)
  if (unpriced.length === 0) return null
  return (
    <div className="flex flex-col gap-1.5 text-sm text-muted-foreground">
      <p>
        {unpriced.length === 1
          ? "1 Holding has no value. It is not in the value of the Account."
          : `${unpriced.length} Holdings have no value. They are not in the value of the Account.`}
      </p>
      <ul className="flex flex-col gap-1">
        {unpriced.map((holding) => (
          <li key={holding.ticker}>
            <span className="font-medium text-foreground">
              {holding.ticker}
            </span>
            {holding.note && `: ${holding.note}`}
          </li>
        ))}
      </ul>
    </div>
  )
}

// Stacked shows a value with a small muted line under it, for example a price and its date.
function Stacked({ main, sub }: { main: string; sub: string }) {
  return (
    <div className="flex flex-col">
      <span>{main}</span>
      <span className="text-xs text-muted-foreground">{sub}</span>
    </div>
  )
}

// NoValue marks an empty cell. The note under the table tells why a Holding has no value.
// A Holding in the Account currency has no Exchange Rate.
function NoValue() {
  return <span className="text-muted-foreground">—</span>
}
