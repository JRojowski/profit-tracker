import type { Account, AssistantStep } from "@/lib/api"

// assistantStepText gives the compact text of a step, for example "Closed Positions · All Accounts · NVDA.US ·
// 2025-01-01 to 2025-12-31 · worst 5". Each function of the Assistant has one case. A function that this module
// does not know shows its name and its shared arguments, so that the user still sees what the model did.
export function assistantStepText(
  step: AssistantStep,
  accounts: Account[] | null
): string {
  const parts = [
    stepLabel(step.function),
    ...argumentParts(step, accounts),
    ...extraParts(step),
  ]
  return parts.join(" · ")
}

// stepLabel gives the name that the screen uses for a function.
function stepLabel(name: string): string {
  switch (name) {
    case "get_holdings":
      return "Holdings"
    case "get_results":
      return "Results"
    case "get_closed_positions":
      return "Closed Positions"
    case "get_income_and_fees":
      return "Income and fees"
    case "get_allocation_and_net_deposits":
      return "Allocation and Net Deposits"
    default:
      return name
  }
}

// argumentParts gives the parts of the shared arguments: the scope, the Instrument and the period.
// An absent accountId means all Accounts. An Account that is not known shows its ID.
function argumentParts(
  step: AssistantStep,
  accounts: Account[] | null
): string[] {
  const args = step.arguments
  const parts: string[] = []
  const accountId = args.accountId
  if (typeof accountId === "number") {
    const account = accounts?.find((candidate) => candidate.id === accountId)
    parts.push(account ? account.name : `Account ${accountId}`)
  } else {
    parts.push("All Accounts")
  }
  if (typeof args.instrument === "string" && args.instrument !== "") {
    parts.push(args.instrument)
  }
  const from = typeof args.from === "string" ? args.from : ""
  const to = typeof args.to === "string" ? args.to : ""
  if (from !== "" && to !== "") parts.push(`${from} to ${to}`)
  else if (from !== "") parts.push(`from ${from}`)
  else if (to !== "") parts.push(`to ${to}`)
  return parts
}

// extraParts gives the parts of the arguments that only one function has: the order and the row limit of the
// Closed Positions, and the year of the Allocation and Net Deposits.
function extraParts(step: AssistantStep): string[] {
  const args = step.arguments
  switch (step.function) {
    case "get_closed_positions": {
      // Without an order, the backend gives the newest Closed Positions first.
      const limit =
        typeof args.limit === "number" && args.limit > 0 ? args.limit : null
      const order =
        args.order === "profit_asc"
          ? "worst"
          : args.order === "profit_desc"
            ? "best"
            : limit !== null
              ? "newest"
              : null
      if (order === null) return []
      return [limit === null ? order : `${order} ${limit}`]
    }
    case "get_allocation_and_net_deposits":
      return typeof args.year === "number" ? [String(args.year)] : []
    default:
      return []
  }
}
