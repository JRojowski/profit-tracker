import { useCallback, useMemo } from "react"
import { useSearchParams } from "react-router"

import {
  readOverviewTicker,
  writeOverviewTicker,
} from "@/lib/instrument-overview-param"

// OverviewTicker is the URL state of the Instrument Overview panel.
export type OverviewTicker = {
  // ticker is the Instrument of the open panel, or null when the panel is closed.
  ticker: string | null
  // open shows the Instrument. It pushes a history entry when the panel is closed, so that Back closes the panel.
  // It replaces the entry when the panel is already open.
  open: (ticker: string) => void
  // close removes the parameter. It replaces the entry, so that Back does not open the panel again.
  close: () => void
}

// useOverviewTicker reads and writes the search parameter of the Instrument Overview. It keeps each other parameter.
export function useOverviewTicker(): OverviewTicker {
  const [searchParams, setSearchParams] = useSearchParams()
  const ticker = useMemo(() => readOverviewTicker(searchParams), [searchParams])
  const open = useCallback(
    (next: string) =>
      setSearchParams((params) => writeOverviewTicker(params, next), {
        replace: ticker !== null,
      }),
    [setSearchParams, ticker]
  )
  const close = useCallback(
    () =>
      setSearchParams((params) => writeOverviewTicker(params, null), {
        replace: true,
      }),
    [setSearchParams]
  )
  return { ticker, open, close }
}
