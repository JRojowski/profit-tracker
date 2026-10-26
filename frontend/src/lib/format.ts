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
    return withMinusSign(
      new Intl.NumberFormat(undefined, {
        minimumFractionDigits: 2,
        maximumFractionDigits: 2,
        signDisplay,
      }).format(value)
    )
  }
  return withMinusSign(
    new Intl.NumberFormat(undefined, {
      style: "currency",
      currency,
      signDisplay,
    }).format(value)
  )
}

// formatVolume shows a decimal volume with all its digits.
export function formatVolume(volume: string): string {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 10,
  }).format(Number(volume))
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

// withMinusSign replaces the hyphen of a formatted negative number with the minus sign U+2212.
function withMinusSign(formatted: string): string {
  return formatted.replace("-", "\u2212")
}

// formatTime shows the time of day of a local time, for example "10:32:05".
export function formatTime(time: Date): string {
  return time.toLocaleTimeString()
}

// formatDate shows a YYYY-MM-DD date in the local format. It reads the date as a calendar date, with no time zone shift.
export function formatDate(date: string): string {
  const [year, month, day] = date.split("-").map(Number)
  return new Date(year, month - 1, day).toLocaleDateString()
}
