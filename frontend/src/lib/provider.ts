import type { Account, Provider } from "./api"

// isKraken tells that an Account or an Import has the Provider Kraken. Each Kraken rule of the screen uses this test.
// A Kraken Account has no Lots and uses the Average Cost (ADR 0009). Thus it has no Closed Positions.
// Its Total Result has a Rewards part and no fee part, because the fees are in the cost.
// A new Kraken Account must have the currency EUR.
export function isKraken(provider: Provider): boolean {
  return provider === "kraken"
}

// isTierAccount tells that an Account is a Tier Account: a regular XTB Account in PLN or USD.
// Only a Tier Account has the Tiers Breakdown page. The backend uses the same rule.
export function isTierAccount(
  account: Pick<Account, "type" | "provider" | "currency">
): boolean {
  return (
    account.type === "regular" &&
    account.provider === "xtb" &&
    (account.currency === "PLN" || account.currency === "USD")
  )
}
