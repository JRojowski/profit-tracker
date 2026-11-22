import type { ReactNode } from "react"
import { useSearchParams } from "react-router"

import { BenchmarkChart } from "@/components/benchmark-chart"
import { LoadedView } from "@/components/loaded-view"
import { PeriodPills } from "@/components/period-pills"
import { ResultBadge } from "@/components/result-badge"
import { SectionHeading } from "@/components/section-heading"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useLoad } from "@/hooks/use-load"
import {
  fetchAccountBenchmarks,
  type BenchmarkReturn,
  type Benchmarks,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { parsePeriod, writePeriod } from "@/lib/benchmark-period"
import { formatDate } from "@/lib/format"
import { useAccount } from "@/pages/account-layout"

// AccountBenchmarksPage compares the Time-Weighted Return of one Account with the Benchmark Return of each
// Benchmark for one period. The period is in the URL. All values use the Account currency.
export function AccountBenchmarksPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const [searchParams, setSearchParams] = useSearchParams()
  const period = parsePeriod(searchParams)
  const benchmarks = useLoad(
    `${account.id}:${period}`,
    valuesVersion,
    (signal) => fetchAccountBenchmarks(account.id, period, signal)
  )

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <Card className="min-w-0">
        <CardHeader>
          <SectionHeading
            title="Benchmarks"
            description="The return of the Account and of each Benchmark for the same days."
            action={
              <PeriodPills
                value={period}
                onChange={(next) =>
                  setSearchParams((params) => writePeriod(params, next), {
                    replace: true,
                  })
                }
              />
            }
          />
        </CardHeader>
        <CardContent className="flex min-w-0 flex-col gap-4">
          <LoadedView
            loaded={benchmarks}
            errorTitle="Cannot load the Benchmarks"
          >
            {(data) =>
              data.available ? (
                <>
                  <BenchmarkChart data={data} />
                  <BenchmarkTable data={data} />
                </>
              ) : (
                <p className="text-sm text-muted-foreground">
                  The Account has no Value History. The tool cannot compare it
                  with the Benchmarks.
                </p>
              )
            }
          </LoadedView>
        </CardContent>
      </Card>
    </div>
  )
}

// BenchmarkTable shows the days and the currency of the comparison, and a row for the user and for each Benchmark.
// On a narrow screen the figures show under the name, because three columns do not fit.
function BenchmarkTable({ data }: { data: Benchmarks }) {
  return (
    <>
      {data.baseDay !== null && data.lastDay !== null && (
        <p className="text-sm text-muted-foreground">
          From <span className="num">{formatDate(data.baseDay)}</span> to{" "}
          <span className="num">{formatDate(data.lastDay)}</span>, in{" "}
          <span className="num">{data.currency}</span>.
        </p>
      )}
      <Table>
        <TableHeader>
          <TableRow>
            <TableHead>Name</TableHead>
            <TableHead className="hidden text-right md:table-cell">
              Return
            </TableHead>
            <TableHead className="hidden text-right md:table-cell">
              Difference
            </TableHead>
          </TableRow>
        </TableHeader>
        <TableBody>
          <FigureRow
            name="Your Account"
            detail="Time-Weighted Return"
            returnCell={<ResultBadge value={data.user.return} />}
            differenceCell={null}
          />
          {data.benchmarks.map((benchmark) => (
            <BenchmarkRow key={benchmark.id} benchmark={benchmark} />
          ))}
        </TableBody>
      </Table>
    </>
  )
}

// BenchmarkRow shows the name, the fund, the Benchmark Return and the difference of one Benchmark.
// A Benchmark with no price for the whole period shows the reason in place of the numbers.
function BenchmarkRow({ benchmark }: { benchmark: BenchmarkReturn }) {
  const available = benchmark.status === "available"
  return (
    <FigureRow
      name={benchmark.name}
      detail={benchmark.fund}
      returnCell={
        available ? <ResultBadge value={benchmark.return} /> : undefined
      }
      differenceCell={
        available ? (
          <ResultBadge value={benchmark.difference} unit="pp" />
        ) : undefined
      }
    />
  )
}

type FigureRowProps = {
  name: string
  detail: string
  // returnCell and differenceCell are the figures. Undefined means that the tool has no price for the full period.
  // Null means a figure that the row does not have.
  returnCell: ReactNode | undefined
  differenceCell: ReactNode | undefined
}

// FigureRow is one row of the table: the name with its detail and the two figures, in columns on a wide screen.
function FigureRow({
  name,
  detail,
  returnCell,
  differenceCell,
}: FigureRowProps) {
  const missing = returnCell === undefined
  return (
    <TableRow>
      <TableCell className="font-medium whitespace-normal">
        {name}
        <span className="block text-xs font-normal break-words text-muted-foreground">
          {detail}
        </span>
        <span className="mt-2 flex flex-wrap items-center gap-x-4 gap-y-1.5 text-xs font-normal text-muted-foreground md:hidden">
          {missing ? (
            NO_PRICE
          ) : (
            <>
              <span className="flex items-center gap-1.5">
                Return {returnCell}
              </span>
              {differenceCell !== null && (
                <span className="flex items-center gap-1.5">
                  Difference {differenceCell}
                </span>
              )}
            </>
          )}
        </span>
      </TableCell>
      {missing ? (
        <TableCell
          colSpan={2}
          className="hidden text-right whitespace-normal text-muted-foreground md:table-cell"
        >
          {NO_PRICE}
        </TableCell>
      ) : (
        <>
          <TableCell className="hidden text-right md:table-cell">
            {returnCell}
          </TableCell>
          <TableCell className="hidden text-right text-muted-foreground md:table-cell">
            {differenceCell === null ? "—" : differenceCell}
          </TableCell>
        </>
      )}
    </TableRow>
  )
}

// NO_PRICE is the reason that a Benchmark has no figures.
const NO_PRICE = "No price for the full period"
