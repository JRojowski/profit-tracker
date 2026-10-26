// ScaledDecimal is a decimal amount as an integer of units and the number of decimal places of one unit.
type ScaledDecimal = { units: bigint; scale: number }

// parseDecimal reads a decimal amount from the backend, for example "-12.50", with no conversion to a float.
function parseDecimal(amount: string): ScaledDecimal {
  const match = /^([-+]?)(\d*)(?:\.(\d*))?$/.exec(amount.trim())
  if (match === null || (match[2] === "" && (match[3] ?? "") === "")) {
    throw new Error(`"${amount}" is not a decimal amount`)
  }
  const [, sign, whole, fraction = ""] = match
  const units = BigInt((whole || "0") + fraction)
  return { units: sign === "-" ? -units : units, scale: fraction.length }
}

// sumDecimals adds decimal amounts from the backend exactly. It gives the sum as decimal text.
// The sum has the largest number of decimal places of the amounts. An empty list gives "0".
export function sumDecimals(amounts: string[]): string {
  const parsed = amounts.map(parseDecimal)
  const scale = Math.max(0, ...parsed.map((amount) => amount.scale))
  let total = 0n
  for (const amount of parsed) {
    total += amount.units * 10n ** BigInt(scale - amount.scale)
  }
  const negative = total < 0n
  const digits = (negative ? -total : total).toString().padStart(scale + 1, "0")
  const whole = digits.slice(0, digits.length - scale)
  const fraction = digits.slice(digits.length - scale)
  return (negative ? "-" : "") + whole + (scale > 0 ? "." + fraction : "")
}
