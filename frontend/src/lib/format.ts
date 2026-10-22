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
