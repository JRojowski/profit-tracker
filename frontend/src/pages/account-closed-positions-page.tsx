import { useMemo, useState } from "react"

import { FigureLabel } from "@/components/figure-label"
import { LoadedView } from "@/components/loaded-view"
import {
  CLOSED_POSITIONS_VIEW_STORAGE_KEY,
  closedPositionColumns,
  ClosedPositionsTable,
  closedPositionsViewRules,
} from "@/components/closed-positions-table"
import { ResetColumnsButton } from "@/components/reset-columns-button"
import {
  Card,
  CardAction,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useLoad } from "@/hooks/use-load"
import { useIsMobile } from "@/hooks/use-mobile"
import { useTableView } from "@/hooks/use-table-view"
import { fetchClosedPositions, type ClosedPosition } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { sumDecimals } from "@/lib/decimal"
import { amountSign, formatAmount, polishYear, signColour } from "@/lib/format"
import { cn } from "@/lib/utils"
import { useAccount } from "@/pages/account-layout"

// allYears is the value of the year filter that shows the Closed Positions of all years.
const allYears = "all"

// selectedItem marks the selected year with the accent, as in the mockup. The default muted background is too faint.
const selectedItem =
  "data-[state=on]:bg-primary/15 data-[state=on]:text-primary"

// AccountClosedPositionsPage shows the total Realized Profit and the number of Closed Positions of one Account,
// and the table of the Closed Positions, newest first. A filter selects one calendar year in Polish time.
export function AccountClosedPositionsPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const [year, setYear] = useState(allYears)
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
          year={year}
          onYearChange={setYear}
        />
      )}
    </LoadedView>
  )
}

type ClosedPositionsViewProps = {
  // closedPositions are in the order of the backend, oldest first.
  closedPositions: ClosedPosition[]
  currency: string
  // year is the selected year as text, or allYears.
  year: string
  onYearChange: (year: string) => void
}

// ClosedPositionsView shows the figures and the table of the Closed Positions of the selected year.
function ClosedPositionsView({
  closedPositions,
  currency,
  year,
  onYearChange,
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
  // The selected year can be missing, for example after an Import. Then the page shows all years.
  const selected = years.includes(year) ? year : allYears
  const shown =
    selected === allYears
      ? newestFirst
      : newestFirst.filter(
          (closedPosition) =>
            String(polishYear(closedPosition.time)) === selected
        )
  const total = sumDecimals(
    shown.map((closedPosition) => closedPosition.realizedProfit)
  )
  const period = selected === allYears ? "All years" : selected

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
          {years.length > 0 && (
            // On a narrow screen, the filter scrolls by itself when it has many years.
            <div className="max-w-full overflow-x-auto">
              <ToggleGroup
                type="single"
                variant="outline"
                size="sm"
                spacing={0}
                aria-label="Year"
                value={selected}
                onValueChange={(value) => {
                  // An empty value means that the user clicked the selected item again.
                  if (value) onYearChange(value)
                }}
              >
                <ToggleGroupItem value={allYears} className={selectedItem}>
                  All years
                </ToggleGroupItem>
                {years.map((item) => (
                  <ToggleGroupItem
                    key={item}
                    value={item}
                    className={cn("num", selectedItem)}
                  >
                    {item}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </div>
          )}
          {closedPositions.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              The Account has no Closed Positions.
            </p>
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
