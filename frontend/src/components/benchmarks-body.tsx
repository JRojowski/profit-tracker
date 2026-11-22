import { TriangleAlertIcon } from "lucide-react"
import type { ReactNode } from "react"
import { useSearchParams } from "react-router"

import { BenchmarkChart } from "@/components/benchmark-chart"
import { LoadedView } from "@/components/loaded-view"
import { PeriodPills } from "@/components/period-pills"
import { ResultBadge } from "@/components/result-badge"
import { SectionHeading } from "@/components/section-heading"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { type Loaded } from "@/hooks/use-load"
import {
  type BenchmarkReturn,
  type BenchmarkStatus,
  type Benchmarks,
} from "@/lib/api"
import { parsePeriod, writePeriod } from "@/lib/benchmark-period"
import { formatDate } from "@/lib/format"

// BenchmarksBody is the body of the Benchmarks page of one Account and of all Accounts. It shows the period
// pills, which write the period to the URL, and the comparison of the loaded answer.
export function BenchmarksBody({
  loaded,
  subject,
  emptyText,
}: {
  loaded: Loaded<Benchmarks>
  // subject names the compared returns: "Account" or "Accounts".
  subject: "Account" | "Accounts"
  // emptyText tells why the tool has no comparison when the answer is not available.
  emptyText: string
}) {
  const [searchParams, setSearchParams] = useSearchParams()
  const period = parsePeriod(searchParams)

  return (
    <div className="flex min-w-0 flex-col gap-6">
      <Card className="min-w-0">
        <CardHeader>
          <SectionHeading
            title="Benchmarks"
            description={`The return of the ${subject} and of each Benchmark for the same days.`}
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
          <LoadedView loaded={loaded} errorTitle="Cannot load the Benchmarks">
            {(data) =>
              data.available ? (
                <>
                  <BenchmarkChart
                    data={data}
                    subjectLabel={subjectLabel(subject)}
                  />
                  <BenchmarkTable data={data} subject={subject} />
                </>
              ) : (
                <>
                  <p className="text-sm text-muted-foreground">{emptyText}</p>
                  {data.warnings.length > 0 && (
                    <Alert className="border-warning/40 bg-warning/10 *:[svg]:text-warning">
                      <TriangleAlertIcon />
                      <AlertTitle>
                        The tool cannot compare the returns
                      </AlertTitle>
                      <AlertDescription>
                        <ul className="list-inside list-disc">
                          {data.warnings.map((warning) => (
                            <li key={warning}>{warning}</li>
                          ))}
                        </ul>
                      </AlertDescription>
                    </Alert>
                  )}
                </>
              )
            }
          </LoadedView>
        </CardContent>
      </Card>
    </div>
  )
}

// subjectLabel is the name of the user row in the table and of the user line in the chart.
function subjectLabel(subject: "Account" | "Accounts"): string {
  return subject === "Account" ? "Your Account" : "All Accounts"
}

// BenchmarkTable shows the days and the currency of the comparison, and a row for the user and for each Benchmark.
// On a narrow screen the figures show under the name, because three columns do not fit.
function BenchmarkTable({
  data,
  subject,
}: {
  data: Benchmarks
  subject: "Account" | "Accounts"
}) {
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
            name={subjectLabel(subject)}
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
      reason={REASONS[benchmark.status]}
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
  // reason tells why the row has no figures. The row shows it in place of the figures.
  reason?: string
}

// FigureRow is one row of the table: the name with its detail and the two figures, in columns on a wide screen.
function FigureRow({
  name,
  detail,
  returnCell,
  differenceCell,
  reason,
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
            reason
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
          {reason}
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

// REASONS is the text of a row for each status. An available Benchmark has no reason.
const REASONS: Record<BenchmarkStatus, string | undefined> = {
  available: undefined,
  noFullPeriod: "No price for the full period",
  sourceFailure: "The price source failed",
}
