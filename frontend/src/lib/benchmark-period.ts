import { historyPeriods, type HistoryPeriod } from "@/lib/api"

const periodParam = "period"

// defaultPeriod is the period of the Benchmarks page when the URL has no valid period.
export const defaultPeriod: HistoryPeriod = "All"

// parsePeriod reads the period from the search parameters. It ignores a missing or invalid value.
export function parsePeriod(params: URLSearchParams): HistoryPeriod {
  const value = params.get(periodParam)
  return historyPeriods.find((period) => period === value) ?? defaultPeriod
}

// writePeriod copies the search parameters and sets the period. The default period has no parameter.
export function writePeriod(
  params: URLSearchParams,
  period: HistoryPeriod
): URLSearchParams {
  const next = new URLSearchParams(params)
  if (period === defaultPeriod) next.delete(periodParam)
  else next.set(periodParam, period)
  return next
}
