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

// subtractDecimals gives a minus b exactly, as decimal text, for example a Total Result from a value and Net Deposits.
export function subtractDecimals(a: string, b: string): string {
  const text = b.trim().replace(/^\+/, "")
  const negated = text.startsWith("-") ? text.slice(1) : "-" + text
  return sumDecimals([a, negated])
}

// percentOf gives part as a percentage of whole, as decimal text with 4 decimal places, with exact integer maths.
// The last place is rounded half away from zero. A whole of zero gives null.
export function percentOf(part: string, whole: string): string | null {
  const a = parseDecimal(part)
  const b = parseDecimal(whole)
  if (b.units === 0n) return null
  const scale = Math.max(a.scale, b.scale)
  const numerator = a.units * 10n ** BigInt(scale - a.scale) * 1_000_000n
  const denominator = b.units * 10n ** BigInt(scale - b.scale)
  const negative = numerator < 0n !== denominator < 0n
  const absNumerator = numerator < 0n ? -numerator : numerator
  const absDenominator = denominator < 0n ? -denominator : denominator
  const rounded = (2n * absNumerator + absDenominator) / (2n * absDenominator)
  const digits = rounded.toString().padStart(5, "0")
  const text = `${digits.slice(0, -4)}.${digits.slice(-4)}`
  return negative && rounded !== 0n ? `-${text}` : text
}

// compareDecimals compares two decimal amounts from the backend exactly. It gives -1, 0 or 1.
export function compareDecimals(a: string, b: string): -1 | 0 | 1 {
  const x = parseDecimal(a)
  const y = parseDecimal(b)
  const scale = Math.max(x.scale, y.scale)
  const unitsX = x.units * 10n ** BigInt(scale - x.scale)
  const unitsY = y.units * 10n ** BigInt(scale - y.scale)
  if (unitsX === unitsY) return 0
  return unitsX < unitsY ? -1 : 1
}
