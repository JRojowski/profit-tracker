// formatAmount shows a decimal amount from the backend in the currency.
// The backend sends exact values. The tool rounds only here, for display.
// The conversion to a float is for display only. Never calculate with the result.
// An Import of a new Account can have no currency yet. Then the amount shows with 2 decimal places and no currency.
export function formatAmount(amount: string, currency: string): string {
  const value = Number(amount)
  if (!/^[A-Z]{3}$/.test(currency)) {
    return new Intl.NumberFormat(undefined, {
      minimumFractionDigits: 2,
      maximumFractionDigits: 2,
    }).format(value)
  }
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
  }).format(value)
}

// formatVolume shows a decimal volume with all its digits.
export function formatVolume(volume: string): string {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 10,
  }).format(Number(volume))
}

// formatPercent shows a decimal percentage from the backend with 2 decimal places, for example "8.85 %".
// The backend sends 8.85 for 8.85 %, not 0.0885.
export function formatPercent(percent: string): string {
  return new Intl.NumberFormat(undefined, {
    style: "percent",
    minimumFractionDigits: 2,
    maximumFractionDigits: 2,
  }).format(Number(percent) / 100)
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
