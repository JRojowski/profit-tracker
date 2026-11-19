import { useMemo } from "react"

import { CurrencyBadge } from "@/components/account-badges"
import { DataTable, type DataTableColumn } from "@/components/data-table"
import { SortControl } from "@/components/sort-control"
import { PriceSourceDialog } from "@/components/price-source-dialog"
import {
  PriceWarningBadge,
  PriceWarningBadgeButton,
} from "@/components/price-warnings"
import { Badge } from "@/components/ui/badge"
import { useIsMobile } from "@/hooks/use-mobile"
import type { Holding } from "@/lib/api"
import { percentOf, sumDecimals } from "@/lib/decimal"
import {
  amountSign,
  formatAmount,
  formatDate,
  formatPercent,
  formatRate,
  formatVolume,
  signColour,
} from "@/lib/format"
import {
  decimalSortValue,
  sortRows,
  textSortValue,
  type SortingProps,
} from "@/lib/table-sort"
import {
  tableViewRules,
  type TableViewProps,
  type TableViewRules,
} from "@/lib/table-view"
import { cn } from "@/lib/utils"

type HoldingsTableProps = {
  holdings: Holding[]
  // columns come from holdingColumns. The owner builds them once, for the table and for the view rules.
  columns: DataTableColumn<Holding>[]
  currency: string
  // onPriceSourceChanged runs after the user changed the price source of an Instrument.
  onPriceSourceChanged: () => void
  // view is the column order and the sorting, which the owner keeps with useTableView.
  view: TableViewProps
}

// HOLDINGS_VIEW_STORAGE_KEY is the localStorage key of the column order and the sorting of this table.
export const HOLDINGS_VIEW_STORAGE_KEY = "profit-tracker:table:holdings"

// FIXED_COLUMN_IDS are the leading columns that the user cannot move.
const FIXED_COLUMN_IDS = ["instrument"]

// holdingsViewRules gives the rules of a stored view of this table. They come from the column definitions.
export function holdingsViewRules(
  columns: DataTableColumn<Holding>[]
): TableViewRules {
  return tableViewRules(columns, FIXED_COLUMN_IDS)
}

// HoldingsTable shows the Holdings of an Account: a table on a wide screen and a list of compact rows on a narrow screen.
// Each row opens the price source dialog. A Holding with a Price Warning has a badge.
// A note under the table lists the Holdings with no value.
export function HoldingsTable({
  holdings,
  columns,
  currency,
  onPriceSourceChanged,
  view,
}: HoldingsTableProps) {
  const isMobile = useIsMobile()
  // The wide table and the cards share one view. The default sorting is the order of the response.
  const { sorting, onSortingChange } = view
  // The rows are sorted once, here. The cards use only the first sort column.
  const sorted = useMemo(
    () => sortRows(holdings, isMobile ? sorting.slice(0, 1) : sorting, columns),
    [holdings, sorting, columns, isMobile]
  )
  const totals = useMemo(() => holdingTotals(holdings), [holdings])

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
          holdings={sorted}
          totals={totals}
          sortOptions={columns.filter((column) => column.sortValue)}
          currency={currency}
          onPriceSourceChanged={onPriceSourceChanged}
          sorting={sorting}
          onSortingChange={onSortingChange}
        />
      ) : (
        <WideTable
          holdings={sorted}
          totals={totals}
          columns={columns}
          currency={currency}
          view={view}
        />
      )}
      <NoValueNote holdings={holdings} />
    </div>
  )
}

// holdingColumns gives the columns of the wide table. Each sortable column has its own sortValue.
// The select control of the cards reads the sortable columns from here, so a new column needs one edit.
export function holdingColumns(
  currency: string,
  onPriceSourceChanged: () => void
): DataTableColumn<Holding>[] {
  return [
    {
      id: "instrument",
      header: "Instrument",
      sortValue: (holding) => textSortValue(holding.ticker),
      cell: ({ row }) => {
        const holding = row.original
        return (
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
            {holding.priceWarnings.length > 0 && (
              <span className="pt-1">
                <PriceWarningBadgeButton
                  instrument={holding}
                  warnings={holding.priceWarnings}
                  onChanged={onPriceSourceChanged}
                />
              </span>
            )}
          </div>
        )
      },
    },
    {
      id: "volume",
      header: "Volume",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.volume),
      cell: ({ row }) => formatVolume(row.original.volume),
    },
    {
      id: "averageCost",
      header: "Average cost",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.averageCost),
      cell: ({ row }) => formatAmount(row.original.averageCost, currency),
    },
    {
      id: "cost",
      header: "Cost",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.cost),
      cell: ({ row }) => formatAmount(row.original.cost, currency),
    },
    {
      id: "price",
      header: "Price",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.price),
      cell: ({ row }) => {
        const holding = row.original
        return holding.price !== null && holding.priceDate !== null ? (
          <Stacked
            main={formatAmount(
              holding.price,
              holding.quoteCurrency ?? currency
            )}
            sub={formatDate(holding.priceDate)}
          />
        ) : (
          <NoValue />
        )
      },
    },
    {
      id: "exchangeRate",
      header: "Exchange Rate",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.exchangeRate),
      cell: ({ row }) => {
        const holding = row.original
        return holding.exchangeRate !== null &&
          holding.exchangeRateCurrency !== null &&
          holding.exchangeRateDate !== null ? (
          <Stacked
            main={`1 ${holding.exchangeRateCurrency} = ${formatRate(holding.exchangeRate)} PLN`}
            sub={formatDate(holding.exchangeRateDate)}
          />
        ) : (
          <NoValue />
        )
      },
    },
    {
      id: "value",
      header: "Value",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.value),
      cell: ({ row }) =>
        row.original.value !== null ? (
          <span className="font-medium">
            {formatAmount(row.original.value, currency)}
          </span>
        ) : (
          <NoValue />
        ),
    },
    {
      id: "unrealizedProfit",
      header: "Unrealized Profit",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.unrealizedProfit),
      cell: ({ row }) => {
        const holding = row.original
        return holding.unrealizedProfit !== null ? (
          <ProfitFigure
            profit={holding.unrealizedProfit}
            percent={holding.unrealizedProfitPercent}
            currency={currency}
          />
        ) : (
          <NoValue />
        )
      },
    },
    {
      id: "priceSource",
      header: "Price source",
      cell: ({ row }) => {
        const holding = row.original
        return (
          <div className="flex items-center gap-2">
            <span className="num text-xs text-muted-foreground">
              {holding.priceTicker}
            </span>
            {(holding.priceTickerManual || holding.quoteCurrencyManual) && (
              <Badge variant="secondary">manual</Badge>
            )}
            <PriceSourceDialog
              instrument={holding}
              warnings={holding.priceWarnings}
              onChanged={onPriceSourceChanged}
            />
          </div>
        )
      },
    },
  ]
}

// ProfitFigure shows an Unrealized Profit with its percentage under it, in the colour of its sign.
function ProfitFigure({
  profit,
  percent,
  currency,
}: {
  profit: string
  percent: string | null
  currency: string
}) {
  return (
    <div className={cn("flex flex-col", signColour(amountSign(profit)))}>
      <span>{formatAmount(profit, currency, { signed: true })}</span>
      {percent !== null && (
        <span className="text-xs">
          {formatPercent(percent, { signed: true })}
        </span>
      )}
    </div>
  )
}

// hasKnownValue tells that a Holding has a value and an Unrealized Profit. The totals and the note use only this test.
function hasKnownValue(holding: Holding): boolean {
  return holding.value !== null && holding.unrealizedProfit !== null
}

// HoldingTotals are the sums over the Holdings with a known value.
type HoldingTotals = {
  cost: string
  value: string
  unrealizedProfit: string
  // percent is the total Unrealized Profit as a percentage of the total cost. It is null when the cost is zero.
  percent: string | null
  // included is the number of Holdings in the sums. excluded is the number of Holdings with no known value.
  included: number
  excluded: number
}

// holdingTotals sums the Holdings with a known value. A Holding with no value is in no sum, also not in the cost.
function holdingTotals(holdings: Holding[]): HoldingTotals {
  const known = holdings.filter(hasKnownValue)
  const cost = sumDecimals(known.map((holding) => holding.cost))
  const unrealizedProfit = sumDecimals(
    known.map((holding) => holding.unrealizedProfit as string)
  )
  return {
    cost,
    value: sumDecimals(known.map((holding) => holding.value as string)),
    unrealizedProfit,
    percent: percentOf(unrealizedProfit, cost),
    included: known.length,
    excluded: holdings.length - known.length,
  }
}

// ExcludedNote tells how many Holdings are not in the totals.
function ExcludedNote({ excluded }: { excluded: number }) {
  if (excluded === 0) return null
  return (
    <span className="max-w-56 text-xs font-normal whitespace-normal text-muted-foreground">
      Without {excluded} {excluded === 1 ? "Holding" : "Holdings"} with no known
      value
    </span>
  )
}

// WideTable shows all details of each Holding in columns, with a total row.
// The holdings are already sorted.
function WideTable({
  holdings,
  totals,
  columns,
  currency,
  view,
}: {
  view: TableViewProps
  holdings: Holding[]
  totals: HoldingTotals
  columns: DataTableColumn<Holding>[]
  currency: string
}) {
  const hasKnown = totals.included > 0
  return (
    <DataTable
      {...view}
      columns={columns}
      rows={holdings}
      getRowId={(holding) => holding.ticker}
      footer={{
        label: (
          <div className="flex flex-col gap-0.5">
            <span>Total</span>
            <ExcludedNote excluded={totals.excluded} />
          </div>
        ),
        cells: {
          cost: hasKnown ? formatAmount(totals.cost, currency) : <NoValue />,
          value: hasKnown ? formatAmount(totals.value, currency) : <NoValue />,
          unrealizedProfit: hasKnown ? (
            <ProfitFigure
              profit={totals.unrealizedProfit}
              percent={totals.percent}
              currency={currency}
            />
          ) : (
            <NoValue />
          ),
        },
      }}
    />
  )
}

// HoldingRows shows a select control for the sort, each Holding as one compact row, and the totals as the last row.
// A tap on a row opens the price source dialog.
// The badge of a Price Warning is inside the row button, so the tap on the row opens the dialog with the warnings.
function HoldingRows({
  holdings,
  totals,
  sortOptions,
  currency,
  onPriceSourceChanged,
  sorting,
  onSortingChange,
}: SortingProps & {
  holdings: Holding[]
  totals: HoldingTotals
  sortOptions: DataTableColumn<Holding>[]
  currency: string
  onPriceSourceChanged: () => void
}) {
  return (
    <div className="flex flex-col gap-3">
      <SortControl
        options={sortOptions}
        sorting={sorting}
        onSortingChange={onSortingChange}
      />
      <ul className="-mx-4 flex flex-col divide-y border-y">
        {holdings.map((holding) => (
          <li key={holding.ticker}>
            <PriceSourceDialog
              instrument={holding}
              warnings={holding.priceWarnings}
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
                    {holding.priceWarnings.length > 0 && <PriceWarningBadge />}
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
        <li className="flex items-start justify-between gap-3 bg-muted/50 px-4 py-3 font-medium">
          <span className="flex min-w-0 flex-col gap-1">
            <span>Total</span>
            <ExcludedNote excluded={totals.excluded} />
            {totals.included > 0 && (
              <span className="num text-xs font-normal text-muted-foreground">
                Cost {formatAmount(totals.cost, currency)}
              </span>
            )}
          </span>
          <span className="flex shrink-0 flex-col items-end gap-1 text-right">
            {totals.included > 0 ? (
              <>
                <span className="num text-sm">
                  {formatAmount(totals.value, currency)}
                </span>
                <span className="num text-xs font-normal">
                  <ProfitFigure
                    profit={totals.unrealizedProfit}
                    percent={totals.percent}
                    currency={currency}
                  />
                </span>
              </>
            ) : (
              <NoValue />
            )}
          </span>
        </li>
      </ul>
    </div>
  )
}

// NoValueNote lists the Holdings with no value and tells why each one has no value.
function NoValueNote({ holdings }: { holdings: Holding[] }) {
  const unpriced = holdings.filter((holding) => !hasKnownValue(holding))
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
