import { Link, useSearchParams } from "react-router"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useLoad } from "@/hooks/use-load"
import { fetchAllAccountsBenchmarks } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { parsePeriod } from "@/lib/benchmark-period"
import { paths } from "@/lib/routes"
import { BenchmarksBody } from "@/pages/account-benchmarks-page"

// AllAccountsBenchmarksPage compares the Time-Weighted Return of all Accounts with the Benchmark Return of each
// Benchmark for one period. The period is in the URL. All values are in PLN by ADR 0004.
export function AllAccountsBenchmarksPage() {
  const { accounts, accountsError, valuesVersion } = useAppState()
  const [searchParams] = useSearchParams()
  const period = parsePeriod(searchParams)
  const benchmarks = useLoad(period, valuesVersion, (signal) =>
    fetchAllAccountsBenchmarks(period, signal)
  )

  if (accountsError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Cannot load the Accounts</AlertTitle>
        <AlertDescription>{accountsError}</AlertDescription>
      </Alert>
    )
  }
  if (accounts === null) return <Skeleton className="h-24 w-full" />
  if (accounts.length === 0) {
    return (
      <div className="flex flex-col items-start gap-3">
        <p className="text-sm text-muted-foreground">
          There are no Accounts yet. Import an export to create one.
        </p>
        <Button asChild>
          <Link to={paths.import}>Import an export</Link>
        </Button>
      </div>
    )
  }

  return (
    <BenchmarksBody
      loaded={benchmarks}
      subject="Accounts"
      emptyText="The Accounts have no Value History. The tool cannot compare them with the Benchmarks."
    />
  )
}
