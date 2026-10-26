import { useEffect, useRef, useState } from "react"

import { errorMessage } from "@/lib/api"

// Loaded is the state of one load from the backend.
export type Loaded<T> =
  | { status: "loading" }
  | { status: "ok"; data: T }
  | { status: "error"; error: string }

type ScopedResult<T> = { scope: string; result: Loaded<T> }

// useLoad loads data when the scope or the version changes. The scope must name all the inputs of load.
// A null scope loads nothing and gives "loading", for example while the inputs are not known yet.
// A new scope gives "loading", so a view never shows the data of another Account.
// A new version of the same scope keeps the old data on the screen until the new data arrives.
export function useLoad<T>(
  scope: string | null,
  version: number,
  load: (signal: AbortSignal) => Promise<T>
): Loaded<T> {
  const [state, setState] = useState<ScopedResult<T> | null>(null)
  // loadRef holds the latest load function. The scope and the version, not the function identity, start a new load.
  const loadRef = useRef(load)
  useEffect(() => {
    loadRef.current = load
  })

  useEffect(() => {
    if (scope === null) return
    const controller = new AbortController()
    loadRef
      .current(controller.signal)
      .then((data) => setState({ scope, result: { status: "ok", data } }))
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setState({
          scope,
          result: { status: "error", error: errorMessage(err) },
        })
      })
    return () => controller.abort()
  }, [scope, version])

  return scope !== null && state?.scope === scope
    ? state.result
    : { status: "loading" }
}
