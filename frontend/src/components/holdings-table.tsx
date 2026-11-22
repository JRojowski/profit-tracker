import { useMemo } from "react"

import { CurrencyBadge } from "@/components/account-badges"
import { NoValue, ProfitFigure, Stacked } from "@/components/value-figures"
import { DataTable, type DataTableColumn } from "@/components/data-table"
import { SortControl } from "@/components/sort-control"
import { PriceSourceDialog } from "@/components/price-source-dialog"
import {
  PriceWarningBadge,
  PriceWarningBadgeButton,
} from "@/components/price-warnings"
import { Badge } from "@/components/ui/badge"
import { useIsMobile } from "@/hooks/use-mobile"
import type { Holding, Provider } from "@/lib/api"
import { percentOf, sumDecimals } from "@/lib/decimal"
import {
  amountSign,
  formatAmount,
  formatDate,
  formatPercent,
  formatVolume,
  signColour,
} from "@/lib/format"
import { isKraken } from "@/lib/provider"
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
  // view is the column order and the sorting, which the owner keeps with useTableView.
  view: TableViewProps
  // onOpenOverview opens the Instrument Overview of a Holding. A row of the wide table runs it.
  onOpenOverview: (ticker: string) => void
}

// HOLDINGS_VIEW_STORAGE_KEY is the localStorage key of the column order and the sorting of this table.
const HOLDINGS_VIEW_STORAGE_KEY = "profit-tracker:table:holdings"

// holdingsViewStorageKey gives the storage key of the table view for a Provider.
// A Kraken Account has the extra Average Cost column, so its view has its own key and keeps the XTB view intact.
export function holdingsViewStorageKey(provider: Provider): string {
  return isKraken(provider)
    ? `${HOLDINGS_VIEW_STORAGE_KEY}:kraken`
    : HOLDINGS_VIEW_STORAGE_KEY
}

// AVERAGE_COST_COLUMN_ID is the id of the Average Cost column. Only the table of a Kraken Account has it.
const AVERAGE_COST_COLUMN_ID = "averageCost"

// FIXED_COLUMN_IDS are the leading columns that the user cannot move.
const FIXED_COLUMN_IDS = ["instrument"]

// holdingsViewRules gives the rules of a stored view of this table. They come from the column definitions.
export function holdingsViewRules(
  columns: DataTableColumn<Holding>[]
): TableViewRules {
  return tableViewRules(columns, FIXED_COLUMN_IDS)
}

// HoldingsTable shows the Holdings of an Account: a table on a wide screen and a list of compact rows on a narrow screen.
// A row of the wide table opens the Instrument Overview. A row of the list opens it too.
// A Holding with a Price Warning has a badge.
// A note under the table lists the Holdings with no value or with no known cost.
export function HoldingsTable({
  holdings,
  columns,
  currency,
  view,
  onOpenOverview,
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
          // The narrow rows show the Average Cost when the columns have it, so that the columns stay the one source.
          showAverageCost={columns.some(
            (column) => column.id === AVERAGE_COST_COLUMN_ID
          )}
          onOpenOverview={onOpenOverview}
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
          onOpenOverview={onOpenOverview}
        />
      )}
      <NoValueNote holdings={holdings} />
    </div>
  )
}

// holdingColumns gives the columns of the wide table. Each sortable column has its own sortValue.
// The select control of the cards reads the sortable columns from here, so a new column needs one edit.
// A Kraken Account has no Lots. Its table has the Average Cost column after the volume.
export function holdingColumns(
  currency: string,
  provider: Provider,
  onPriceSourceChanged: () => void
): DataTableColumn<Holding>[] {
  const averageCost: DataTableColumn<Holding>[] = isKraken(provider)
    ? [
        {
          id: AVERAGE_COST_COLUMN_ID,
          header: "Average Cost",
          meta: { align: "right" },
          sortValue: (holding) => decimalSortValue(holding.averageCost),
          cell: ({ row }) =>
            row.original.averageCost !== null ? (
              formatAmount(row.original.averageCost, currency)
            ) : (
              <NoValue />
            ),
        },
      ]
    : []
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
    ...averageCost,
    {
      id: "cost",
      header: "Cost",
      meta: { align: "right" },
      sortValue: (holding) => decimalSortValue(holding.cost),
      cell: ({ row }) =>
        row.original.cost !== null ? (
          formatAmount(row.original.cost, currency)
        ) : (
          <NoValue />
        ),
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

// hasNoKnownCost tells that a Holding has a value, but no known cost and no Unrealized Profit.
// On a Kraken Account, a Reward with no stored close gives this state (ADR 0009).
function hasNoKnownCost(holding: Holding): boolean {
  return holding.value !== null && holding.unrealizedProfit === null
}

// HoldingTotals are the sums of the Holdings. A sum is null when no Holding has a known amount for it.
type HoldingTotals = {
  // value is the sum of the Holdings with a value.
  value: string | null
  // cost and unrealizedProfit are the sums of the Holdings with a value and a known cost.
  cost: string | null
  unrealizedProfit: string | null
  // percent is the total Unrealized Profit as a percentage of the total cost. It is null when the cost is zero.
  percent: string | null
  // noValue is the number of Holdings with no value. They are in no sum.
  noValue: number
  // noCost is the number of Holdings with a value but no known cost. They are only in the value sum.
  noCost: number
}

// holdingTotals sums the Holdings. A Holding with no value is in no sum. A Holding with no known cost is only in the value.
function holdingTotals(holdings: Holding[]): HoldingTotals {
  const values: string[] = []
  const costs: string[] = []
  const profits: string[] = []
  for (const holding of holdings) {
    if (holding.value === null) continue
    values.push(holding.value)
    if (holding.cost === null || holding.unrealizedProfit === null) continue
    costs.push(holding.cost)
    profits.push(holding.unrealizedProfit)
  }
  const cost = costs.length > 0 ? sumDecimals(costs) : null
  const unrealizedProfit = profits.length > 0 ? sumDecimals(profits) : null
  return {
    value: values.length > 0 ? sumDecimals(values) : null,
    cost,
    unrealizedProfit,
    percent:
      cost !== null && unrealizedProfit !== null
        ? percentOf(unrealizedProfit, cost)
        : null,
    noValue: holdings.length - values.length,
    noCost: values.length - costs.length,
  }
}

// ExcludedNote tells how many Holdings are not in the totals.
function ExcludedNote({ totals }: { totals: HoldingTotals }) {
  if (totals.noValue === 0 && totals.noCost === 0) return null
  return (
    <span className="flex max-w-56 flex-col text-xs font-normal whitespace-normal text-muted-foreground">
      {totals.noValue > 0 && (
        <span>Without {holdingCount(totals.noValue)} with no known value</span>
      )}
      {totals.noCost > 0 && (
        <span>
          Cost and Unrealized Profit without {holdingCount(totals.noCost)} with
          no known cost
        </span>
      )}
    </span>
  )
}

// holdingCount gives a count of Holdings as text, for example "1 Holding" or "2 Holdings".
function holdingCount(count: number): string {
  return count === 1 ? "1 Holding" : `${count} Holdings`
}

// WideTable shows all details of each Holding in columns, with a total row.
// The holdings are already sorted.
function WideTable({
  holdings,
  totals,
  columns,
  currency,
  view,
  onOpenOverview,
}: {
  onOpenOverview: (ticker: string) => void
  view: TableViewProps
  holdings: Holding[]
  totals: HoldingTotals
  columns: DataTableColumn<Holding>[]
  currency: string
}) {
  return (
    <DataTable
      {...view}
      columns={columns}
      rows={holdings}
      getRowId={(holding) => holding.ticker}
      onRowAction={(holding) => onOpenOverview(holding.ticker)}
      footer={{
        label: (
          <div className="flex flex-col gap-0.5">
            <span>Total</span>
            <ExcludedNote totals={totals} />
          </div>
        ),
        cells: {
          cost:
            totals.cost !== null ? (
              formatAmount(totals.cost, currency)
            ) : (
              <NoValue />
            ),
          value:
            totals.value !== null ? (
              formatAmount(totals.value, currency)
            ) : (
              <NoValue />
            ),
          unrealizedProfit:
            totals.unrealizedProfit !== null ? (
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
// A tap on a row opens the Instrument Overview, which has the price source control and the Price Warnings.
function HoldingRows({
  holdings,
  totals,
  sortOptions,
  currency,
  showAverageCost,
  onOpenOverview,
  sorting,
  onSortingChange,
}: SortingProps & {
  holdings: Holding[]
  totals: HoldingTotals
  sortOptions: DataTableColumn<Holding>[]
  currency: string
  showAverageCost: boolean
  onOpenOverview: (ticker: string) => void
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
            <button
              type="button"
              onClick={() => onOpenOverview(holding.ticker)}
              className="flex w-full items-start justify-between gap-3 px-4 py-3 text-left outline-none hover:bg-muted/50 focus-visible:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-inset"
            >
              <span className="flex min-w-0 flex-col gap-1">
                <span className="flex min-w-0 items-center gap-2">
                  <span className="truncate font-medium">{holding.ticker}</span>
                  {holding.quoteCurrency && (
                    <CurrencyBadge currency={holding.quoteCurrency} />
                  )}
                </span>
                {holding.priceWarnings.length > 0 && <PriceWarningBadge />}
                {/* A long crypto volume can push the price to a second line. The line never runs under the value. */}
                <span className="font-mono text-xs text-muted-foreground">
                  <span className="num">{formatVolume(holding.volume)} ×</span>{" "}
                  <span className="num">
                    {holding.price !== null ? (
                      formatAmount(
                        holding.price,
                        holding.quoteCurrency ?? currency
                      )
                    ) : (
                      <NoValue />
                    )}
                  </span>
                </span>
                {showAverageCost && (
                  <span className="num text-xs text-muted-foreground">
                    Average Cost{" "}
                    {holding.averageCost !== null ? (
                      formatAmount(holding.averageCost, currency)
                    ) : (
                      <NoValue />
                    )}
                  </span>
                )}
                <span className="sr-only">Open the Instrument Overview.</span>
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
                {holding.unrealizedProfit !== null ? (
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
                ) : (
                  // The wide table shows the same mark in the Unrealized Profit column.
                  holding.value !== null && (
                    <span className="text-xs">
                      <NoValue />
                    </span>
                  )
                )}
              </span>
            </button>
          </li>
        ))}
        <li className="flex items-start justify-between gap-3 bg-muted/50 px-4 py-3 font-medium">
          <span className="flex min-w-0 flex-col gap-1">
            <span>Total</span>
            <ExcludedNote totals={totals} />
            {totals.cost !== null && (
              <span className="num text-xs font-normal text-muted-foreground">
                Cost {formatAmount(totals.cost, currency)}
              </span>
            )}
          </span>
          <span className="flex shrink-0 flex-col items-end gap-1 text-right">
            {totals.value !== null ? (
              <span className="num text-sm">
                {formatAmount(totals.value, currency)}
              </span>
            ) : (
              <NoValue />
            )}
            {totals.value !== null && (
              <span className="num text-xs font-normal">
                {totals.unrealizedProfit !== null ? (
                  <ProfitFigure
                    profit={totals.unrealizedProfit}
                    percent={totals.percent}
                    currency={currency}
                  />
                ) : (
                  <NoValue />
                )}
              </span>
            )}
          </span>
        </li>
      </ul>
    </div>
  )
}

// NoValueNote lists the Holdings with no value and the Holdings with no known cost. It tells why for each one.
function NoValueNote({ holdings }: { holdings: Holding[] }) {
  const unpriced = holdings.filter((holding) => holding.value === null)
  const noCost = holdings.filter(hasNoKnownCost)
  if (unpriced.length === 0 && noCost.length === 0) return null
  return (
    <div className="flex flex-col gap-3 text-sm text-muted-foreground">
      {unpriced.length > 0 && (
        <NoteGroup
          text={
            unpriced.length === 1
              ? "1 Holding has no value. It is not in the value of the Account."
              : `${unpriced.length} Holdings have no value. They are not in the value of the Account.`
          }
          holdings={unpriced}
        />
      )}
      {noCost.length > 0 && (
        <NoteGroup
          text={
            noCost.length === 1
              ? "1 Holding has no known cost. Its value is in the value of the Account. It has no Unrealized Profit."
              : `${noCost.length} Holdings have no known cost. Their values are in the value of the Account. They have no Unrealized Profit.`
          }
          holdings={noCost}
        />
      )}
    </div>
  )
}

// NoteGroup shows one sentence and the ticker and the note of each Holding that it is about.
function NoteGroup({ text, holdings }: { text: string; holdings: Holding[] }) {
  return (
    <div className="flex flex-col gap-1.5">
      <p>{text}</p>
      <ul className="flex flex-col gap-1">
        {holdings.map((holding) => (
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
