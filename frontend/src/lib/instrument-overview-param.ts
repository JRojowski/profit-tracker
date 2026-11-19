// overviewParam is the search parameter that holds the ticker of the open Instrument Overview.
const overviewParam = "overview"

// readOverviewTicker gives the ticker of the open Instrument Overview, or null when the panel is closed.
export function readOverviewTicker(params: URLSearchParams): string | null {
  const ticker = params.get(overviewParam)
  return ticker === null || ticker === "" ? null : ticker
}

// writeOverviewTicker gives a copy of the search parameters with the ticker written in, or removed for null.
// It keeps each other parameter.
export function writeOverviewTicker(
  params: URLSearchParams,
  ticker: string | null
): URLSearchParams {
  const next = new URLSearchParams(params)
  if (ticker === null) next.delete(overviewParam)
  else next.set(overviewParam, ticker)
  return next
}
