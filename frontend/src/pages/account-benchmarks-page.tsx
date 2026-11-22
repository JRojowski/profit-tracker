import { useSearchParams } from "react-router"

import { BenchmarksBody } from "@/components/benchmarks-body"
import { useLoad } from "@/hooks/use-load"
import { fetchAccountBenchmarks } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { parsePeriod } from "@/lib/benchmark-period"
import { useAccount } from "@/pages/account-layout"

// AccountBenchmarksPage compares the Time-Weighted Return of one Account with the Benchmark Return of each
// Benchmark for one period. The period is in the URL. All values use the Account currency.
export function AccountBenchmarksPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const [searchParams] = useSearchParams()
  const period = parsePeriod(searchParams)
  const benchmarks = useLoad(
    `${account.id}:${period}`,
    valuesVersion,
    (signal) => fetchAccountBenchmarks(account.id, period, signal)
  )

  return (
    <BenchmarksBody
      loaded={benchmarks}
      subject="Account"
      emptyText="The Account has no Value History. The tool cannot compare it with the Benchmarks."
    />
  )
}
