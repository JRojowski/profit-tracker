import { useEffect, useState } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import {
  errorMessage,
  fetchIkzeLimit,
  type IkzeLimit,
  type IkzeLimitYear,
} from "@/lib/api"
import { formatAmount } from "@/lib/format"

type IkzeLimitCardProps = {
  accountId: number
}

// LimitState holds the IKZE Limit rows of one Account, or the error of the last load.
type LimitState =
  | { accountId: number; limit: IkzeLimit; error: null }
  | { accountId: number; limit: null; error: string }

// IkzeLimitCard shows the IKZE Contributions of each year against the IKZE Limit.
// The current year is at the top. The past years are in the table below.
export function IkzeLimitCard({ accountId }: IkzeLimitCardProps) {
  const [state, setState] = useState<LimitState | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    fetchIkzeLimit(accountId, controller.signal)
      .then((limit) => setState({ accountId, limit, error: null }))
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setState({ accountId, limit: null, error: errorMessage(err) })
      })
    return () => controller.abort()
  }, [accountId])

  // Show the rows only when they belong to this Account.
  const current = state?.accountId === accountId ? state : null

  return (
    <Card>
      <CardHeader>
        <CardTitle>IKZE Limit</CardTitle>
        <CardDescription>
          IKZE Contributions of each year against the limit for people who run a
          business.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {current === null ? (
          <Skeleton className="h-24 w-full" />
        ) : current.error !== null ? (
          <Alert variant="destructive">
            <AlertTitle>Cannot load the IKZE Limit</AlertTitle>
            <AlertDescription>{current.error}</AlertDescription>
          </Alert>
        ) : (
          <LimitYears limit={current.limit} />
        )}
      </CardContent>
    </Card>
  )
}

function LimitYears({ limit }: { limit: IkzeLimit }) {
  if (limit.rows.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">
        The Account has no IKZE Contributions.
      </p>
    )
  }
  const thisYear = limit.rows.find((row) => row.year === limit.currentYear)
  const pastYears = limit.rows.filter((row) => row.year !== limit.currentYear)

  return (
    <>
      {thisYear && <CurrentYear row={thisYear} />}
      {pastYears.length > 0 && <PastYears rows={pastYears} />}
    </>
  )
}

function CurrentYear({ row }: { row: IkzeLimitYear }) {
  const over = row.overLimit !== null && Number(row.overLimit) > 0
  return (
    <div className="flex flex-col gap-3">
      <div className="flex items-center gap-2">
        <h3 className="text-sm font-medium">{row.year}</h3>
        {row.limit === null && <Badge variant="outline">Limit unknown</Badge>}
        {over && <Badge variant="destructive">Above the limit</Badge>}
      </div>
      <div className="grid gap-4 sm:grid-cols-3">
        <Figure label="IKZE Contributions" amount={row.contributions} />
        <Figure label="IKZE Limit" amount={row.limit} />
        {over ? (
          <Figure label="Above the limit" amount={row.overLimit} />
        ) : (
          <Figure label="Not used" amount={row.unused} />
        )}
      </div>
    </div>
  )
}

type FigureProps = {
  label: string
  // amount is null when the tool has no IKZE Limit for the year.
  amount: string | null
}

function Figure({ label, amount }: FigureProps) {
  return (
    <div className="flex flex-col gap-1">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="num text-2xl font-semibold">{plnOrDash(amount)}</p>
    </div>
  )
}

function PastYears({ rows }: { rows: IkzeLimitYear[] }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Year</TableHead>
          <TableHead className="text-right">IKZE Contributions</TableHead>
          <TableHead className="text-right">IKZE Limit</TableHead>
          <TableHead className="text-right">Not used</TableHead>
          <TableHead className="text-right">Above the limit</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row) => (
          <TableRow key={row.year}>
            <TableCell>{row.year}</TableCell>
            <TableCell className="text-right num">
              {formatAmount(row.contributions, "PLN")}
            </TableCell>
            <TableCell className="text-right num">
              {row.limit === null
                ? "Limit unknown"
                : formatAmount(row.limit, "PLN")}
            </TableCell>
            <TableCell className="text-right num">
              {plnOrDash(row.unused)}
            </TableCell>
            <TableCell className="text-right num">
              {plnOrDash(row.overLimit)}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}

// plnOrDash shows an amount in PLN, or "—" when the tool has no IKZE Limit for the year.
function plnOrDash(amount: string | null): string {
  return amount === null ? "—" : formatAmount(amount, "PLN")
}
