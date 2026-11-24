import { useState } from "react"

// The order of the rows that the user set by hand. A slot is one Tier and one group, for example "1:holding".
// It maps to the tickers in the order of the user.
export type BreakdownOrder = Readonly<Record<string, readonly string[]>>

const KEY_PREFIX = "profit-tracker.tiers-breakdown-order:"

// parseOrder reads the stored text. Text that has another shape gives an empty order.
function parseOrder(text: string | null): BreakdownOrder {
  if (text === null) return {}
  try {
    const value: unknown = JSON.parse(text)
    if (typeof value !== "object" || value === null || Array.isArray(value)) {
      return {}
    }
    const order: Record<string, string[]> = {}
    for (const [slot, tickers] of Object.entries(value)) {
      if (
        Array.isArray(tickers) &&
        tickers.every((ticker) => typeof ticker === "string")
      ) {
        order[slot] = tickers
      }
    }
    return order
  } catch {
    return {}
  }
}

// readOrder gives the stored order of a key, or an empty order when the storage is missing or fails.
function readOrder(key: string): BreakdownOrder {
  try {
    return parseOrder(window.localStorage.getItem(key))
  } catch {
    return {}
  }
}

// writeOrder stores the order, or removes the key when the order is empty. A failure is ignored.
function writeOrder(key: string, order: BreakdownOrder) {
  try {
    if (Object.keys(order).length === 0) window.localStorage.removeItem(key)
    else window.localStorage.setItem(key, JSON.stringify(order))
  } catch {
    // The table works with no storage. The order then lasts until the next reload.
  }
}

// useBreakdownOrder owns the manual order of the rows of the Tiers Breakdown of one view, "all" or an Account id.
// It reads the storage on mount and again when the view changes. It writes only after a change of the user.
// order maps each slot to its tickers. setSlot stores the order of one slot. clear removes the manual order.
export function useBreakdownOrder(view: string) {
  const key = KEY_PREFIX + view
  const [state, setState] = useState(() => ({ key, order: readOrder(key) }))
  let order = state.order
  // The page can stay mounted when the Account changes, so the hook reads the new key during the render.
  if (state.key !== key) {
    order = readOrder(key)
    setState({ key, order })
  }

  function change(next: BreakdownOrder) {
    setState({ key, order: next })
    writeOrder(key, next)
  }

  return {
    order,
    hasManualOrder: Object.keys(order).length > 0,
    setSlot: (slot: string, tickers: readonly string[]) =>
      change({ ...order, [slot]: tickers }),
    clear: () => change({}),
  }
}
