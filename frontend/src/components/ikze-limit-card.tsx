import { FigureLabel } from "@/components/figure-label"
import { StackedHead } from "@/components/stacked-head"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { Progress } from "@/components/ui/progress"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { IkzeLimit, IkzeLimitYear } from "@/lib/api"
import { amountSign, formatAmount, formatPercent } from "@/lib/format"
import { cn } from "@/lib/utils"

// overBar gives the bar of a year above the limit the colour of a loss.
const overBar = "[&>[data-slot=progress-indicator]]:bg-negative"

type IkzeLimitCardsProps = {
  limit: IkzeLimit
}

// IkzeLimitCards shows the IKZE Contributions of the current year against the IKZE Limit,
// and the table of the past years. The years are calendar years in Polish time.
export function IkzeLimitCards({ limit }: IkzeLimitCardsProps) {
  if (limit.rows.length === 0) {
    return (
      <Card>
        <CardContent>
          <p className="text-sm text-muted-foreground">
            The Account has no IKZE Contributions.
          </p>
        </CardContent>
      </Card>
    )
  }
  const thisYear = limit.rows.find((row) => row.year === limit.currentYear)
  const pastYears = limit.rows.filter((row) => row.year !== limit.currentYear)

  return (
    <div className="flex min-w-0 flex-col gap-6">
      {thisYear && <CurrentYearCard row={thisYear} />}
      {pastYears.length > 0 && <PastYearsCard rows={pastYears} />}
    </div>
  )
}

// CurrentYearCard shows the IKZE Contributions of the current year as a large figure,
// the IKZE Limit, a bar of the used part and the part that is not used.
function CurrentYearCard({ row }: { row: IkzeLimitYear }) {
  const over = isOverLimit(row)
  const used = usedPercent(row)

  return (
    <Card>
      <CardContent className="flex flex-col gap-6">
        <div className="grid gap-6 md:grid-cols-[minmax(0,1fr)_14rem] md:items-center">
          <div className="flex min-w-0 flex-col gap-3">
            <div className="flex flex-wrap items-center gap-2">
              <FigureLabel>IKZE Contributions {row.year}</FigureLabel>
              {row.limit === null && (
                <Badge variant="outline">Limit unknown</Badge>
              )}
              {over && <Badge variant="destructive">Above the limit</Badge>}
            </div>
            <div className="flex flex-wrap items-baseline gap-x-3 gap-y-1">
              <p className="num text-3xl font-semibold tracking-tight sm:text-4xl md:text-5xl">
                {formatAmount(row.contributions, "PLN")}
              </p>
              {row.limit !== null && (
                <p className="text-sm text-muted-foreground">
                  of the IKZE Limit{" "}
                  <span className="num font-medium text-foreground">
                    {formatAmount(row.limit, "PLN")}
                  </span>
                </p>
              )}
            </div>
            {used !== null && (
              <div className="flex flex-col gap-2">
                <div className="flex items-center justify-between gap-2 text-sm">
                  <span className="text-muted-foreground">Used</span>
                  <span
                    className={cn(
                      "num font-medium",
                      over ? "text-negative" : "text-primary"
                    )}
                  >
                    {formatPercent(String(used))}
                  </span>
                </div>
                <Progress
                  value={Math.min(used, 100)}
                  aria-label="Used part of the IKZE Limit"
                  className={cn("h-2", over && overBar)}
                />
              </div>
            )}
          </div>
          <div className="flex min-w-0 flex-col gap-1.5 rounded-lg border bg-muted/40 p-4">
            {over ? (
              <>
                <FigureLabel>Above the limit</FigureLabel>
                <p className="num text-2xl font-semibold text-negative">
                  {plnOrDash(row.overLimit)}
                </p>
              </>
            ) : (
              <>
                <FigureLabel>Not used</FigureLabel>
                <p className="num text-2xl font-semibold">
                  {plnOrDash(row.unused)}
                </p>
              </>
            )}
          </div>
        </div>
        <p className="text-sm text-muted-foreground">
          The year counts in Polish time.
        </p>
      </CardContent>
    </Card>
  )
}

// PastYearsCard shows the IKZE Contributions of each past year against the IKZE Limit, newest first.
function PastYearsCard({ rows }: { rows: IkzeLimitYear[] }) {
  return (
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>Past years</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>
                <span className="hidden md:inline">Year</span>
                {/* On a narrow screen, one column holds the year and the used part. */}
                <StackedHead labels={["Year", "Used"]} />
              </TableHead>
              <TableHead className="hidden text-right md:table-cell">
                IKZE Contributions
              </TableHead>
              <TableHead className="hidden text-right md:table-cell">
                IKZE Limit
              </TableHead>
              <TableHead className="hidden md:table-cell">Used</TableHead>
              <TableHead className="text-right">
                <span className="hidden md:inline">Not used</span>
                {/* On a narrow screen, one column holds the three amounts. */}
                <StackedHead
                  labels={["IKZE Contributions", "IKZE Limit", "Not used"]}
                  alignEnd
                />
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow key={row.year}>
                <TableCell className="align-top md:align-middle">
                  <span className="flex flex-col gap-2">
                    <span className="num font-medium">{row.year}</span>
                    <span className="md:hidden">
                      <UsedBar row={row} />
                    </span>
                  </span>
                </TableCell>
                <TableCell className="hidden text-right num md:table-cell">
                  {formatAmount(row.contributions, "PLN")}
                </TableCell>
                <TableCell className="hidden text-right md:table-cell">
                  <LimitText limit={row.limit} />
                </TableCell>
                <TableCell className="hidden md:table-cell">
                  <UsedBar row={row} />
                </TableCell>
                <TableCell className="text-right align-top md:align-middle">
                  <span className="flex flex-col items-end gap-0.5">
                    <span className="num md:hidden">
                      {formatAmount(row.contributions, "PLN")}
                    </span>
                    <span className="md:hidden">
                      <LimitText limit={row.limit} />
                    </span>
                    <NotUsedText row={row} />
                  </span>
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
        </Table>
      </CardContent>
    </Card>
  )
}

// LimitText shows the IKZE Limit of a year, or "Limit unknown" when the tool has no limit for the year.
function LimitText({ limit }: { limit: string | null }) {
  if (limit === null) {
    return <span className="text-muted-foreground">Limit unknown</span>
  }
  return <span className="num">{formatAmount(limit, "PLN")}</span>
}

// UsedBar shows the used part of the IKZE Limit of a year as a percentage and a small bar.
// A year with no IKZE Limit shows "—".
function UsedBar({ row }: { row: IkzeLimitYear }) {
  const used = usedPercent(row)
  if (used === null) return <span className="num">—</span>
  const over = isOverLimit(row)
  return (
    <span className="flex items-center gap-3">
      <span
        className={cn(
          "w-16 num font-medium md:text-right",
          over && "text-negative"
        )}
      >
        {formatPercent(String(used))}
      </span>
      <Progress
        value={Math.min(used, 100)}
        aria-label={`Used part of the IKZE Limit ${row.year}`}
        className={cn("w-20 md:w-28", over && overBar)}
      />
    </span>
  )
}

// NotUsedText shows the part of the IKZE Limit that is not used, or the amount above the limit.
function NotUsedText({ row }: { row: IkzeLimitYear }) {
  if (isOverLimit(row)) {
    return (
      <span className="flex flex-col items-end">
        <span className="num text-negative">{plnOrDash(row.overLimit)}</span>
        <span className="text-xs text-negative">above the limit</span>
      </span>
    )
  }
  return <span className="num">{plnOrDash(row.unused)}</span>
}

// isOverLimit tells if the IKZE Contributions of a year are above the IKZE Limit.
function isOverLimit(row: IkzeLimitYear): boolean {
  return row.overLimit !== null && amountSign(row.overLimit) === "positive"
}

// usedPercent gives the IKZE Contributions as a percentage of the IKZE Limit, or null when the limit is unknown.
// The conversion to a float is for the bar and the display only. Never calculate an amount with it.
function usedPercent(row: IkzeLimitYear): number | null {
  if (row.limit === null) return null
  const limit = Number(row.limit)
  if (limit <= 0) return 100
  return (Number(row.contributions) / limit) * 100
}

// plnOrDash shows an amount in PLN, or "—" when the tool has no IKZE Limit for the year.
function plnOrDash(amount: string | null): string {
  return amount === null ? "—" : formatAmount(amount, "PLN")
}
