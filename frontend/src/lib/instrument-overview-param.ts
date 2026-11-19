// overviewParam is the search parameter that holds the ticker of the open Instrument Overview.
const overviewParam = "overview"

// closedParam is the search parameter that holds the XTB ID of the highlighted Closed Position.
const closedParam = "closed"

// OverviewState is the Instrument Overview that the URL describes.
export type OverviewState = {
  // ticker is the Instrument of the open panel.
  ticker: string
  // closed is the XTB ID of the highlighted Closed Position, or null for no highlight.
  closed: string | null
}

// readOverviewState gives the open Instrument Overview, or null when the panel is closed.
// A closed parameter without an overview parameter does nothing.
export function readOverviewState(
  params: URLSearchParams
): OverviewState | null {
  const ticker = params.get(overviewParam)
  if (ticker === null || ticker === "") return null
  const closed = params.get(closedParam)
  return { ticker, closed: closed === "" ? null : closed }
}

// writeOverviewState gives a copy of the search parameters with the panel state written in.
// A null state removes the overview and closed parameters together. It keeps each other parameter.
export function writeOverviewState(
  params: URLSearchParams,
  state: OverviewState | null
): URLSearchParams {
  const next = new URLSearchParams(params)
  next.delete(overviewParam)
  next.delete(closedParam)
  if (state !== null) {
    next.set(overviewParam, state.ticker)
    if (state.closed !== null) next.set(closedParam, state.closed)
  }
  return next
}
