// dayMs is the length of one day in milliseconds.
export const dayMs = 24 * 60 * 60 * 1000

// dateTime reads a YYYY-MM-DD date as milliseconds since 1970 at midnight UTC.
export function dateTime(date: string): number {
  const [year, month, day] = date.split("-").map(Number)
  return Date.UTC(year, month - 1, day)
}

// monthTicks gives the first day of each month in the range, as milliseconds since 1970 in UTC.
// It keeps every nth month, so that the axis has at most max ticks.
export function monthTicks(first: number, last: number, max: number): number[] {
  const start = new Date(first)
  const months: number[] = []
  for (let month = start.getUTCMonth() + 1; ; month++) {
    const time = Date.UTC(start.getUTCFullYear(), month, 1)
    if (time > last) break
    months.push(time)
  }
  const step = Math.max(1, Math.ceil(months.length / max))
  return months.filter((_, index) => index % step === 0)
}

// axisDate shows a day of the time axis: the month and the year for a long period, else the day and the month.
export function axisDate(time: number, long: boolean): string {
  return new Date(time).toLocaleDateString(undefined, {
    timeZone: "UTC",
    ...(long
      ? { month: "short", year: "2-digit" }
      : { day: "numeric", month: "short" }),
  })
}
