// formatAmount shows a decimal amount from the backend in the currency.
// The backend sends exact values. The tool rounds only here, for display.
// The conversion to a float is for display only. Never calculate with the result.
export function formatAmount(amount: string, currency: string): string {
  return new Intl.NumberFormat(undefined, {
    style: "currency",
    currency,
  }).format(Number(amount))
}

// formatVolume shows a decimal volume with all its digits.
export function formatVolume(volume: string): string {
  return new Intl.NumberFormat(undefined, {
    maximumFractionDigits: 10,
  }).format(Number(volume))
}
