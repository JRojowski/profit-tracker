import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { PriceSourceDialog } from "@/components/price-source-dialog"
import { Badge } from "@/components/ui/badge"
import type { Holding } from "@/lib/api"
import {
  formatAmount,
  formatDate,
  formatPercent,
  formatRate,
  formatVolume,
} from "@/lib/format"

type HoldingsTableProps = {
  holdings: Holding[]
  currency: string
  // onPriceSourceChanged runs after the user changed the price source of an Instrument.
  onPriceSourceChanged: () => void
}

export function HoldingsTable({
  holdings,
  currency,
  onPriceSourceChanged,
}: HoldingsTableProps) {
  if (holdings.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        The Account has no Holdings.
      </p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Instrument</TableHead>
          <TableHead>Price source</TableHead>
          <TableHead className="text-right">Volume</TableHead>
          <TableHead className="text-right">Average cost</TableHead>
          <TableHead className="text-right">Cost</TableHead>
          <TableHead className="text-right">Price</TableHead>
          <TableHead className="text-right">Exchange Rate</TableHead>
          <TableHead className="text-right">Value</TableHead>
          <TableHead className="text-right">Unrealized Profit</TableHead>
          <TableHead className="text-right">%</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {holdings.map((holding) => (
          <TableRow key={holding.ticker}>
            <TableCell>
              <div className="flex flex-col">
                <span>
                  {holding.ticker}{" "}
                  <span className="text-muted-foreground">{holding.name}</span>
                </span>
                {holding.note && (
                  <span className="text-xs whitespace-normal text-muted-foreground">
                    {holding.note}
                  </span>
                )}
              </div>
            </TableCell>
            <TableCell>
              <div className="flex items-center gap-2">
                <span>
                  {holding.priceTicker}
                  {holding.quoteCurrency && (
                    <span className="text-muted-foreground">
                      {" "}
                      {holding.quoteCurrency}
                    </span>
                  )}
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
                <div className="flex flex-col">
                  <span>
                    {formatAmount(
                      holding.price,
                      holding.quoteCurrency ?? currency
                    )}
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {formatDate(holding.priceDate)}
                  </span>
                </div>
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right num">
              {holding.exchangeRate !== null &&
              holding.exchangeRateCurrency !== null &&
              holding.exchangeRateDate !== null ? (
                <div className="flex flex-col">
                  <span>
                    1 {holding.exchangeRateCurrency} ={" "}
                    {formatRate(holding.exchangeRate)} PLN
                  </span>
                  <span className="text-xs text-muted-foreground">
                    {formatDate(holding.exchangeRateDate)}
                  </span>
                </div>
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right num">
              {holding.value !== null ? (
                formatAmount(holding.value, currency)
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right num">
              {holding.unrealizedProfit !== null ? (
                formatAmount(holding.unrealizedProfit, currency, {
                  signed: true,
                })
              ) : (
                <NoValue />
              )}
            </TableCell>
            <TableCell className="text-right num">
              {holding.unrealizedProfitPercent !== null ? (
                formatPercent(holding.unrealizedProfitPercent, { signed: true })
              ) : (
                <NoValue />
              )}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

// NoValue marks an empty cell. A Holding with no price has a note that tells why.
// A Holding in the Account currency has no Exchange Rate.
function NoValue() {
  return <span className="text-muted-foreground">—</span>
}
