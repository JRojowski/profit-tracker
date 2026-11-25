import { useState } from "react"

// readKeys gives the stored Tier keys of a storage key, or an empty list when the storage is missing or fails.
function readKeys(key: string): string[] {
  try {
    const value: unknown = JSON.parse(window.localStorage.getItem(key) ?? "[]")
    return Array.isArray(value)
      ? value.filter((item): item is string => typeof item === "string")
      : []
  } catch {
    return []
  }
}

// writeKeys stores the Tier keys, or removes the key when the list is empty. A failure is ignored.
function writeKeys(key: string, tiers: ReadonlySet<string>) {
  try {
    if (tiers.size === 0) window.localStorage.removeItem(key)
    else window.localStorage.setItem(key, JSON.stringify([...tiers]))
  } catch {
    // The table works with no storage. The Tiers then stay collapsed until the next reload.
  }
}

// useCollapsedTiers owns the set of collapsed Tiers of one table and keeps it in localStorage under the key.
// It reads the storage on mount and again when the key changes. It writes only after a change of the user.
// collapsed holds the Tier keys. toggle collapses or expands one Tier.
export function useCollapsedTiers(storageKey: string) {
  const [state, setState] = useState(() => ({
    key: storageKey,
    tiers: new Set(readKeys(storageKey)) as ReadonlySet<string>,
  }))
  let tiers = state.tiers
  // The page can stay mounted when the Account changes, so the hook reads the new key during the render.
  if (state.key !== storageKey) {
    tiers = new Set(readKeys(storageKey))
    setState({ key: storageKey, tiers })
  }

  function toggle(tier: string) {
    const next = new Set(tiers)
    if (!next.delete(tier)) next.add(tier)
    setState({ key: storageKey, tiers: next })
    writeKeys(storageKey, next)
  }

  return { collapsed: tiers, toggle }
}
