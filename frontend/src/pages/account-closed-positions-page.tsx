import { useMemo } from "react"
import { useSearchParams } from "react-router"

import { FigureLabel } from "@/components/figure-label"
import { LoadedView } from "@/components/loaded-view"
import {
  CLOSED_POSITIONS_VIEW_STORAGE_KEY,
  closedPositionColumns,
  ClosedPositionsTable,
  closedPositionsViewRules,
} from "@/components/closed-positions-table"
import {
  ClosedPositionsFilters,
  ResetFiltersButton,
} from "@/components/closed-positions-filters"
import { ResetColumnsButton } from "@/components/reset-columns-button"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { useLoad } from "@/hooks/use-load"
import { useIsMobile } from "@/hooks/use-mobile"
import { useTableView } from "@/hooks/use-table-view"
import { fetchClosedPositions, type ClosedPosition } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import {
  applyClosedPositionFilters,
  describeFilters,
  parseClosedPositionFilters,
  writeClosedPositionFilters,
  type ClosedPositionFilters,
} from "@/lib/closed-position-filters"
import { sumDecimals } from "@/lib/decimal"
import { amountSign, formatAmount, polishYear, signColour } from "@/lib/format"
import { cn } from "@/lib/utils"
import { useAccount } from "@/pages/account-layout"

// AccountClosedPositionsPage shows the total Realized Profit and the number of Closed Positions of one Account,
// and the table of the Closed Positions, newest first. The filters are in the URL.
export function AccountClosedPositionsPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const closedPositions = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchClosedPositions(account.id, signal)
  )

  return (
    <LoadedView
      loaded={closedPositions}
      errorTitle="Cannot load the Closed Positions"
    >
      {(data) => (
        <ClosedPositionsView
          closedPositions={data}
          currency={account.currency}
        />
      )}
    </LoadedView>
  )
}

type ClosedPositionsViewProps = {
  // closedPositions are in the order of the backend, oldest first.
  closedPositions: ClosedPosition[]
  currency: string
}

// ClosedPositionsView shows the figures and the table of the Closed Positions that the filters select.
function ClosedPositionsView({
  closedPositions,
  currency,
}: ClosedPositionsViewProps) {
  // The hook is here, and not in the table, so that the card header can hold the "Reset columns" button.
  const isMobile = useIsMobile()
  const columns = useMemo(() => closedPositionColumns(currency), [currency])
  const view = useTableView(
    CLOSED_POSITIONS_VIEW_STORAGE_KEY,
    useMemo(() => closedPositionsViewRules(columns), [columns])
  )
  const newestFirst = closedPositions.toReversed()
  const years = [
    ...new Set(
      newestFirst.map((closedPosition) =>
        String(polishYear(closedPosition.time))
      )
    ),
  ]
  // The options are the Instruments of all rows, sorted by ticker, not of the filtered rows.
  const instruments = useMemo(() => {
    const names = new Map<string, string>()
    for (const item of closedPositions) names.set(item.ticker, item.name)
    return [...names]
      .map(([ticker, name]) => ({ ticker, name }))
      .sort((a, b) => (a.ticker < b.ticker ? -1 : 1))
  }, [closedPositions])
  // The filters live only in the URL. An invalid value is ignored and the URL stays as it is.
  const [searchParams, setSearchParams] = useSearchParams()
  const filters = useMemo(
    () =>
      parseClosedPositionFilters(
        searchParams,
        instruments.map((item) => item.ticker)
      ),
    [searchParams, instruments]
  )
  const changeFilters = (next: ClosedPositionFilters) =>
    setSearchParams((params) => writeClosedPositionFilters(params, next), {
      replace: true,
    })
  const shown = applyClosedPositionFilters(newestFirst, filters)
  const total = sumDecimals(
    shown.map((closedPosition) => closedPosition.realizedProfit)
  )
  const period = describeFilters(filters)

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <div className="grid gap-4 md:grid-cols-2">
        <Card>
          <CardContent className="flex min-w-0 flex-col gap-1.5">
            <FigureLabel>Realized Profit</FigureLabel>
            <p
              className={cn(
                "num text-2xl font-semibold sm:text-3xl md:text-4xl",
                signColour(amountSign(total))
              )}
            >
              {formatAmount(total, currency, { signed: true })}
            </p>
            <p className="text-sm text-muted-foreground">{period}</p>
          </CardContent>
        </Card>
        <Card>
          <CardContent className="flex min-w-0 flex-col gap-1.5">
            <FigureLabel>Closed Positions</FigureLabel>
            <p className="num text-2xl font-semibold sm:text-3xl md:text-4xl">
              {shown.length}
            </p>
            <p className="text-sm text-muted-foreground">{period}</p>
          </CardContent>
        </Card>
      </div>
      <Card className="min-w-0">
        <CardHeader>
          <CardTitle>Closed Positions</CardTitle>
          {!isMobile && (
            <CardAction>
              <ResetColumnsButton
                isDefault={view.isDefault}
                onReset={view.reset}
              />
            </CardAction>
          )}
        </CardHeader>
        <CardContent className="flex min-w-0 flex-col gap-4">
          {closedPositions.length > 0 && (
            <ClosedPositionsFilters
              filters={filters}
              onChange={changeFilters}
              years={years}
              instruments={instruments}
            />
          )}
          {closedPositions.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              The Account has no Closed Positions.
            </p>
          ) : shown.length === 0 ? (
            <div className="flex flex-col items-start gap-3">
              <p className="text-sm text-muted-foreground">
                No Closed Positions match the filters.
              </p>
              <ResetFiltersButton filters={filters} onReset={changeFilters} />
            </div>
          ) : (
            <ClosedPositionsTable
              columns={columns}
              closedPositions={shown}
              currency={currency}
              totalRealizedProfit={total}
              view={view.props}
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
