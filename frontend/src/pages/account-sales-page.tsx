import { useState } from "react"

import { FigureLabel } from "@/components/figure-label"
import { LoadedView } from "@/components/loaded-view"
import { SalesTable } from "@/components/sales-table"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { useLoad } from "@/hooks/use-load"
import { fetchSales, type Sale } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { sumDecimals } from "@/lib/decimal"
import { amountSign, formatAmount, polishYear, signColour } from "@/lib/format"
import { cn } from "@/lib/utils"
import { useAccount } from "@/pages/account-layout"

// allYears is the value of the year filter that shows the sales of all years.
const allYears = "all"

// selectedItem marks the selected year with the accent, as in the mockup. The default muted background is too faint.
const selectedItem =
  "data-[state=on]:bg-primary/15 data-[state=on]:text-primary"

// AccountSalesPage shows the total Realized Profit and the number of sales of one Account,
// and the table of the sales, newest first. A filter selects one calendar year in Polish time.
export function AccountSalesPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const [year, setYear] = useState(allYears)
  const sales = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchSales(account.id, signal)
  )

  return (
    <LoadedView loaded={sales} errorTitle="Cannot load the sales">
      {(data) => (
        <SalesView
          sales={data}
          currency={account.currency}
          year={year}
          onYearChange={setYear}
        />
      )}
    </LoadedView>
  )
}

type SalesViewProps = {
  // sales are in the order of the backend, oldest first.
  sales: Sale[]
  currency: string
  // year is the selected year as text, or allYears.
  year: string
  onYearChange: (year: string) => void
}

// SalesView shows the figures and the table of the sales of the selected year.
function SalesView({ sales, currency, year, onYearChange }: SalesViewProps) {
  const newestFirst = sales.toReversed()
  const years = [
    ...new Set(newestFirst.map((sale) => String(polishYear(sale.time)))),
  ]
  // The selected year can be missing, for example after an Import. Then the page shows all years.
  const selected = years.includes(year) ? year : allYears
  const shown =
    selected === allYears
      ? newestFirst
      : newestFirst.filter((sale) => String(polishYear(sale.time)) === selected)
  const total = sumDecimals(shown.map((sale) => sale.realizedProfit))
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
            <FigureLabel>Sales</FigureLabel>
            <p className="num text-2xl font-semibold sm:text-3xl md:text-4xl">
              {shown.length}
            </p>
            <p className="text-sm text-muted-foreground">{period}</p>
          </CardContent>
        </Card>
      </div>
      <Card className="min-w-0">
        <CardHeader>
          <CardTitle>Sales</CardTitle>
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
          {sales.length === 0 ? (
            <p className="text-sm text-muted-foreground">
              The Account has no sales.
            </p>
          ) : (
            <SalesTable
              sales={shown}
              currency={currency}
              totalRealizedProfit={total}
            />
          )}
        </CardContent>
      </Card>
    </div>
  )
}
