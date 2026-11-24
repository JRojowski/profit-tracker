import type { TierError, TierInstrument, Tiers, TiersRequest } from "./api"

// DraftTier is one Tier in the local copy of the "Edit targets" mode. The key identifies the Tier while the user
// changes its order number. The fields are the text of the inputs.
export type DraftTier = {
  key: number
  order: string
  name: string
  targetShare: string
  sharedPartName: string
}

// DraftAssignment puts an Instrument in a Tier of the draft. An empty targetShare puts it in the Shared Part.
export type DraftAssignment = { tierKey: number; targetShare: string }

// TiersDraft is the local copy of the configuration. The mode sends it with one PUT on save.
export type TiersDraft = {
  tiers: DraftTier[]
  assignments: Record<string, DraftAssignment>
  nextKey: number
}

// draftFromTiers copies the saved configuration into a draft.
export function draftFromTiers(saved: Tiers): TiersDraft {
  const keyByOrder = new Map<number, number>()
  const tiers = saved.tiers.map((tier, index) => {
    keyByOrder.set(tier.order, index)
    return {
      key: index,
      order: String(tier.order),
      name: tier.name,
      targetShare: tier.targetShare,
      sharedPartName: tier.sharedPart.name,
    }
  })
  const assignments: Record<string, DraftAssignment> = {}
  // The Tiers list each assigned ticker, so an assignment never drops out of a save.
  for (const tier of saved.tiers) {
    const tierKey = keyByOrder.get(tier.order)
    if (tierKey === undefined) continue
    for (const target of tier.instrumentTargets)
      assignments[target.ticker] = { tierKey, targetShare: target.targetShare }
    for (const shared of tier.sharedInstruments)
      assignments[shared.ticker] = { tierKey, targetShare: "" }
  }
  for (const instrument of saved.instruments) {
    const tierKey =
      instrument.tierOrder === null
        ? undefined
        : keyByOrder.get(instrument.tierOrder)
    if (tierKey === undefined) continue
    assignments[instrument.ticker] = {
      tierKey,
      targetShare: instrument.targetShare ?? "",
    }
  }
  return { tiers, assignments, nextKey: tiers.length }
}

// addTier adds a Tier with the next order number and a Target Share of 0.
export function addTier(draft: TiersDraft): TiersDraft {
  const highest = Math.max(0, ...draft.tiers.map((t) => Number(t.order) || 0))
  const tier: DraftTier = {
    key: draft.nextKey,
    order: String(highest + 1),
    name: "",
    targetShare: "0",
    sharedPartName: "",
  }
  return { ...draft, tiers: [...draft.tiers, tier], nextKey: draft.nextKey + 1 }
}

// removeTier deletes a Tier. The Instruments of the Tier get no Tier.
export function removeTier(draft: TiersDraft, key: number): TiersDraft {
  const assignments = Object.fromEntries(
    Object.entries(draft.assignments).filter(([, a]) => a.tierKey !== key)
  )
  return {
    ...draft,
    tiers: draft.tiers.filter((t) => t.key !== key),
    assignments,
  }
}

// instrumentsOfTier gives the tickers that the draft assigns to a Tier, in ticker order.
export function instrumentsOfTier(draft: TiersDraft, key: number): string[] {
  return Object.entries(draft.assignments)
    .filter(([, a]) => a.tierKey === key)
    .map(([ticker]) => ticker)
    .sort()
}

// decimalText changes a decimal comma to a dot and trims the text.
function decimalText(text: string): string {
  return text.trim().replace(",", ".")
}

// validateOrders gives an error for each Tier whose order number is not a whole number of 1 or more.
export function validateOrders(draft: TiersDraft): TierError[] {
  const errors: TierError[] = []
  draft.tiers.forEach((tier, index) => {
    if (!/^[1-9]\d*$/.test(tier.order.trim()))
      errors.push({
        tierIndex: index,
        ticker: "",
        message: "Enter an order number that is a whole number of 1 or more.",
      })
  })
  return errors
}

// toRequest turns the draft into the body of PUT /api/tiers. The Tiers keep the order of the draft, so the
// tierIndex of an error is the index in the draft. Call validateOrders first. A Target Share goes with a dot.
export function toRequest(draft: TiersDraft): TiersRequest {
  return {
    tiers: draft.tiers.map((tier) => ({
      order: Number(tier.order.trim()),
      name: tier.name.trim(),
      targetShare:
        tier.targetShare.trim() === "" ? "0" : decimalText(tier.targetShare),
      sharedPartName: tier.sharedPartName.trim(),
      instruments: instrumentsOfTier(draft, tier.key).map((ticker) => ({
        ticker,
        targetShare:
          draft.assignments[ticker].targetShare.trim() === ""
            ? null
            : decimalText(draft.assignments[ticker].targetShare),
      })),
    })),
  }
}

// moveInstrument gives an Instrument a Tier, or no Tier when tierKey is null. A move clears the Target Share.
export function moveInstrument(
  draft: TiersDraft,
  ticker: string,
  tierKey: number | null
): TiersDraft {
  const assignments = { ...draft.assignments }
  if (tierKey === null) delete assignments[ticker]
  else if (assignments[ticker]?.tierKey !== tierKey)
    assignments[ticker] = { tierKey, targetShare: "" }
  return { ...draft, assignments }
}

// sortedInstruments gives the Instruments for assignment, the open Holdings first.
export function sortedInstruments(list: TierInstrument[]): TierInstrument[] {
  return [...list].sort(
    (a, b) =>
      Number(b.hasOpenHolding) - Number(a.hasOpenHolding) ||
      a.ticker.localeCompare(b.ticker)
  )
}
