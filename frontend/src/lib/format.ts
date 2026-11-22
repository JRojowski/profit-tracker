// FormatOptions changes how an amount or a percentage shows.
export type FormatOptions = {
  // signed adds "+" to a value above zero. Use it for a profit, a loss or a result.
  signed?: boolean
}

// formatAmount shows a decimal amount from the backend in the currency.
// The backend sends exact values. The tool rounds only here, for display.
// The conversion to a float is for display only. Never calculate with the result.
// An Import of a new Account can have no currency yet. Then the amount shows with 2 decimal places and no currency.
// A value below zero always shows "−" (U+2212).
export function formatAmount(
  amount: string,
  currency: string,
  { signed = false }: FormatOptions = {}
): string {
  const value = Number(amount)
  const signDisplay = signed ? "exceptZero" : "auto"
  if (!/^[A-Z]{3}$/.test(currency)) {
    return formatNumber(amount, { signed })
  }
  return withMinusSign(
    new Intl.NumberFormat(undefined, {
      style: "currency",
      currency,
      signDisplay,
    }).format(value)
  )
}

// formatNumber shows a decimal amount with 2 decimal places and no currency, for example a label on a chart.
// The option fractionDigits sets another number of decimal places, for example 0 for a large axis label.
// The conversion to a float is for display only. A value below zero always shows "−" (U+2212).
export function formatNumber(
  amount: string,
  {
    signed = false,
    fractionDigits = 2,
  }: FormatOptions & { fractionDigits?: number } = {}
): string {
  return withMinusSign(
    new Intl.NumberFormat(undefined, {
      minimumFractionDigits: fractionDigits,
      maximumFractionDigits: fractionDigits,
      signDisplay: signed ? "exceptZero" : "auto",
    }).format(Number(amount))
  )
}

// Sign tells if a decimal amount is below zero, zero or above zero.
export type Sign = "negative" | "zero" | "positive"

// amountSign reads the sign of a decimal amount from its text, with no conversion to a float.
export function amountSign(amount: string): Sign {
  const text = amount.trim()
  if (/^[-+]?0*\.?0*$/.test(text)) return "zero"
  return text.startsWith("-") ? "negative" : "positive"
}

// signColour gives the text colour of a sign. Zero keeps the text colour.
export function signColour(sign: Sign): string {
  if (sign === "positive") return "text-positive"
  if (sign === "negative") return "text-negative"
  return ""
}

// formatVolume shows a decimal volume with up to 8 decimal places, enough for a crypto quantity such as 0.00000110 BTC.
// A share volume of an XTB export has fewer decimal places, so it shows with all its digits.
// The conversion to a float is for display only. Never calculate with the result.
export function formatVolume(volume: string): string {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 8,
  }).format(Number(volume))
}

// formatPrice shows a decimal price from the broker with 2 to 4 decimal places and no currency symbol.
// It rounds a longer price to 4 decimal places. The response does not state the currency of the price.
// The conversion to a float is for display only. Never calculate with the result.
export function formatPrice(price: string): string {
  return withMinusSign(
    new Intl.NumberFormat(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 4,
    }).format(Number(price))
  )
}

// formatRate shows an Exchange Rate with all its digits. The NBP gives at most 4 decimal places.
export function formatRate(rate: string): string {
  return new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 4,
    maximumFractionDigits: 6,
  }).format(Number(rate))
}

// formatPercent shows a decimal percentage from the backend with 2 decimal places, for example "8.85 %".
// The backend sends 8.85 for 8.85 %, not 0.0885.
export function formatPercent(
  percent: string,
  { signed = false }: FormatOptions = {}
): string {
  return withMinusSign(
    new Intl.NumberFormat(undefined, {
      style: "percent",
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
      signDisplay: signed ? "exceptZero" : "auto",
    }).format(Number(percent) / 100)
  )
}

// formatPoints shows a decimal difference of two percentages with 2 decimal places and the unit "pp", for example "+7.24 pp".
// The backend sends 7.24 for 7.24 percentage points.
export function formatPoints(
  points: string,
  { signed = false }: FormatOptions = {}
): string {
  const text = new Intl.NumberFormat(undefined, {
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
    signDisplay: signed ? "exceptZero" : "auto",
  }).format(Number(points))
  return `${withMinusSign(text)} pp`
}

// withMinusSign replaces the hyphen of a formatted negative number with the minus sign U+2212.
function withMinusSign(formatted: string): string {
  return formatted.replace("-", "\u2212")
}

// formatTime shows the time of day of a local time, for example "10:32:05".
export function formatTime(time: Date): string {
  return time.toLocaleTimeString()
}

// polishTimeZone is the time zone of the calendar year of a Closed Position and of an IKZE Contribution.
const polishTimeZone = "Europe/Warsaw"

// polishYear gives the calendar year of an RFC 3339 time in Polish time, not in the time zone of the browser.
export function polishYear(time: string): number {
  return Number(
    new Intl.DateTimeFormat("en-US", {
      timeZone: polishTimeZone,
      year: "numeric",
    }).format(new Date(time))
  )
}

// formatPolishDate shows the date of an RFC 3339 time in Polish time, in the local format.
// Then the date always agrees with polishYear.
export function formatPolishDate(time: string): string {
  return new Date(time).toLocaleDateString(undefined, {
    timeZone: polishTimeZone,
  })
}

// formatDate shows a YYYY-MM-DD date in the local format. It reads the date as a calendar date, with no time zone shift.
export function formatDate(date: string): string {
  const [year, month, day] = date.split("-").map(Number)
  return new Date(year, month - 1, day).toLocaleDateString()
}

// polishDate gives the calendar date of an RFC 3339 time in Polish time, as YYYY-MM-DD.
// Date text in this form compares in the right order with a plain string comparison.
export function polishDate(time: string): string {
  const parts = new Intl.DateTimeFormat("en-US", {
    timeZone: polishTimeZone,
    year: "numeric",
    month: "2-digit",
    day: "2-digit",
  }).formatToParts(new Date(time))
  const part = (type: string) =>
    parts.find((item) => item.type === type)?.value ?? ""
  return `${part("year")}-${part("month")}-${part("day")}`
}

// formatPolishTime shows the time of day of an RFC 3339 time in Polish time, for example "10:32:05".
// It agrees with polishDate, so a time always shows under its own calendar day.
export function formatPolishTime(time: string): string {
  return new Date(time).toLocaleTimeString(undefined, {
    timeZone: polishTimeZone,
  })
}

// formatCompact shows a number in a short form, for example "12K" or "−1.2K". It is for axes and narrow tags.
// The conversion to a float is for display only. Never calculate with the result.
export function formatCompact(value: string | number): string {
  return withMinusSign(
    new Intl.NumberFormat(undefined, {
      notation: "compact",
      maximumFractionDigits: 1,
    }).format(Number(value))
  )
}
