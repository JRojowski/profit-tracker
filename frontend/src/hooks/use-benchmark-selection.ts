import { useState } from "react"

// selectionKey is the localStorage key of the Benchmarks that the chart shows. Both views of the Benchmarks use it.
const selectionKey = "profit-tracker:benchmarks:selected"

// readSelection gives the stored Benchmark ids, or null when nothing valid is stored or the storage fails.
function readSelection(): string[] | null {
  try {
    const text = window.localStorage.getItem(selectionKey)
    if (text === null) return null
    const value: unknown = JSON.parse(text)
    if (!Array.isArray(value)) return null
    return value.filter((id): id is string => typeof id === "string")
  } catch {
    return null
  }
}

// writeSelection stores the Benchmark ids, or removes the key when ids is null. A failure is ignored.
function writeSelection(ids: string[] | null) {
  try {
    if (ids === null) window.localStorage.removeItem(selectionKey)
    else window.localStorage.setItem(selectionKey, JSON.stringify(ids))
  } catch {
    // The chart works with no storage. The selection then lasts until the next reload.
  }
}

// useBenchmarkSelection keeps the ids of the Benchmarks that the chart shows, in localStorage.
// It reads the storage once, on mount, and writes it only after a change of the user.
// With nothing stored, all Benchmarks show. A stored id that no Benchmark has is ignored.
// The selection of all Benchmarks is not stored: a change that gives it removes the key.
export function useBenchmarkSelection() {
  const [stored, setStored] = useState<string[] | null>(readSelection)

  return {
    isSelected: (id: string) => stored === null || stored.includes(id),
    // toggle shows or hides one Benchmark. allIds are the ids of all known Benchmarks.
    toggle: (id: string, allIds: string[]) => {
      const current = stored ?? allIds
      const next = current.includes(id)
        ? current.filter((item) => item !== id)
        : [...current, id]
      const value = allIds.every((item) => next.includes(item)) ? null : next
      setStored(value)
      writeSelection(value)
    },
  }
}
