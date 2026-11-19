import { useCallback, useMemo } from "react"
import { useSearchParams } from "react-router"

import {
  readOverviewState,
  writeOverviewState,
  type OverviewState,
} from "@/lib/instrument-overview-param"

// OverviewUrl is the URL state of the Instrument Overview panel.
export type OverviewUrl = {
  // state is the open panel, or null when the panel is closed.
  state: OverviewState | null
  // open shows the Instrument, with a highlighted Closed Position when closed is set. It pushes a history entry
  // when the panel is closed, so that Back closes the panel. It replaces the entry when the panel is already open.
  open: (ticker: string, closed?: string) => void
  // close removes the overview and closed parameters together. It replaces the entry, so that Back does not open the panel again.
  close: () => void
}

// useOverviewUrl reads and writes the search parameters of the Instrument Overview. It keeps each other parameter,
// for example the filters of the Closed Positions page.
export function useOverviewUrl(): OverviewUrl {
  const [searchParams, setSearchParams] = useSearchParams()
  const state = useMemo(() => readOverviewState(searchParams), [searchParams])
  const isOpen = state !== null
  const open = useCallback(
    (ticker: string, closed?: string) =>
      setSearchParams(
        (params) =>
          writeOverviewState(params, { ticker, closed: closed ?? null }),
        { replace: isOpen }
      ),
    [setSearchParams, isOpen]
  )
  const close = useCallback(
    () =>
      setSearchParams((params) => writeOverviewState(params, null), {
        replace: true,
      }),
    [setSearchParams]
  )
  return { state, open, close }
}
