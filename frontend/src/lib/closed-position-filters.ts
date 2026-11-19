import type { ClosedPosition } from "@/lib/api"
import { formatDate, polishDate } from "@/lib/format"

// ClosedPositionFilters is the valid filter of the Closed Positions page.
// An empty instruments list and an absent date mean "no limit".
export type ClosedPositionFilters = {
  // instruments are tickers, sorted.
  instruments: string[]
  // from and to are YYYY-MM-DD dates in Polish time, both inclusive.
  from?: string
  to?: string
}

const instrumentParam = "instrument"
const fromParam = "from"
const toParam = "to"

// noFilters is the filter that selects each Closed Position.
export const noFilters: ClosedPositionFilters = { instruments: [] }

// isCalendarDate tells if the text is a real calendar date in the form YYYY-MM-DD.
function isCalendarDate(text: string | null): text is string {
  if (text === null || !/^\d{4}-\d{2}-\d{2}$/.test(text)) return false
  const [year, month, day] = text.split("-").map(Number)
  const date = new Date(Date.UTC(year, month - 1, day))
  return (
    date.getUTCFullYear() === year &&
    date.getUTCMonth() === month - 1 &&
    date.getUTCDate() === day
  )
}

// parseClosedPositionFilters reads the filter from the search parameters. It ignores each invalid value.
// An instrument that is not in knownTickers is ignored. When from is after to, both dates are ignored.
export function parseClosedPositionFilters(
  params: URLSearchParams,
  knownTickers: readonly string[]
): ClosedPositionFilters {
  const instruments = [
    ...new Set(
      params
        .getAll(instrumentParam)
        .filter((ticker) => knownTickers.includes(ticker))
    ),
  ].sort()
  let from = params.get(fromParam)
  let to = params.get(toParam)
  if (!isCalendarDate(from)) from = null
  if (!isCalendarDate(to)) to = null
  if (from !== null && to !== null && from > to) {
    from = null
    to = null
  }
  return { instruments, from: from ?? undefined, to: to ?? undefined }
}

// writeClosedPositionFilters gives a copy of the search parameters with the filter written in.
// It keeps each other parameter.
export function writeClosedPositionFilters(
  params: URLSearchParams,
  filters: ClosedPositionFilters
): URLSearchParams {
  const next = new URLSearchParams(params)
  next.delete(instrumentParam)
  next.delete(fromParam)
  next.delete(toParam)
  for (const ticker of filters.instruments) next.append(instrumentParam, ticker)
  if (filters.from) next.set(fromParam, filters.from)
  if (filters.to) next.set(toParam, filters.to)
  return next
}

// hasActiveFilters tells if the filter limits the rows.
export function hasActiveFilters(filters: ClosedPositionFilters): boolean {
  return (
    filters.instruments.length > 0 ||
    filters.from !== undefined ||
    filters.to !== undefined
  )
}

// applyClosedPositionFilters keeps the Closed Positions that the filter selects, in the same order.
// The day limits compare the Polish calendar date of the close time.
export function applyClosedPositionFilters(
  closedPositions: ClosedPosition[],
  filters: ClosedPositionFilters
): ClosedPosition[] {
  return closedPositions.filter((closedPosition) => {
    if (
      filters.instruments.length > 0 &&
      !filters.instruments.includes(closedPosition.ticker)
    ) {
      return false
    }
    const day = polishDate(closedPosition.time)
    if (filters.from !== undefined && day < filters.from) return false
    if (filters.to !== undefined && day > filters.to) return false
    return true
  })
}

// yearRange gives the filter dates of one calendar year.
export function yearRange(year: string): { from: string; to: string } {
  return { from: `${year}-01-01`, to: `${year}-12-31` }
}

// allYears is the year value of "no date range".
export const allYears = "all"

// activeYear gives the year button that matches the date range exactly: allYears, a year of years, or "" for another range.
export function activeYear(
  filters: ClosedPositionFilters,
  years: string[]
): string {
  if (filters.from === undefined && filters.to === undefined) return allYears
  return (
    years.find((year) => {
      const range = yearRange(year)
      return filters.from === range.from && filters.to === range.to
    }) ?? ""
  )
}

// instrumentLabel names a selection of Instruments: "Instrument" for none, the ticker for one, "N Instruments" for more.
export function instrumentLabel(tickers: string[]): string {
  if (tickers.length === 0) return "Instrument"
  if (tickers.length === 1) return tickers[0]
  return `${tickers.length} Instruments`
}

// describePeriod gives the date part of the caption, for example "2025" or "All years".
function describePeriod({ from, to }: ClosedPositionFilters): string {
  if (from === undefined && to === undefined) return "All years"
  const year = from?.slice(0, 4)
  if (from === `${year}-01-01` && to === `${year}-12-31`) return String(year)
  if (from !== undefined && to !== undefined) {
    return `${formatDate(from)} \u2013 ${formatDate(to)}`
  }
  return from !== undefined
    ? `From ${formatDate(from)}`
    : `To ${formatDate(to as string)}`
}

// describeFilters gives a short caption of the filter, for example "2025 · PEP.US".
export function describeFilters(filters: ClosedPositionFilters): string {
  const period = describePeriod(filters)
  if (filters.instruments.length === 0) return period
  return `${period} \u00b7 ${instrumentLabel(filters.instruments)}`
}
