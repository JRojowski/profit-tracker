import type { Account, AssistantStep } from "@/lib/api"

// assistantStepText gives the compact text of a step, for example "Holdings · XTB USD · NVDA.US".
// Each function of the Assistant has one case. A function that this module does not know shows its name and
// its shared arguments, so that the user still sees what the model did.
export function assistantStepText(
  step: AssistantStep,
  accounts: Account[] | null
): string {
  const parts = [stepLabel(step.function), ...argumentParts(step, accounts)]
  return parts.join(" · ")
}

// stepLabel gives the name that the screen uses for a function.
function stepLabel(name: string): string {
  switch (name) {
    case "get_holdings":
      return "Holdings"
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
