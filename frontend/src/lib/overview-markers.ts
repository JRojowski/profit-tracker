import { sumDecimals, weightedAverage } from "@/lib/decimal"
import { polishDate } from "@/lib/format"
import type { ClosedPosition, OverviewFill, OverviewPrice } from "@/lib/api"

// MarkerKind is the kind of a marker on the price chart: a buy marker joins Fills, a sell marker joins Closed Positions.
export type MarkerKind = "buy" | "sell"

// MarkerOperation is one operation that a marker joins: a Fill or a Closed Position. The tooltip lists each one.
export type MarkerOperation = {
  // id is the XTB ID of the Fill or of the Closed Position.
  id: string
  // positionId is the ID of the Lot of the operation.
  positionId: string
  // time is an RFC 3339 time in UTC.
  time: string
  volume: string
  // price is null when the operation has no price.
  price: string | null
}

// ChartMarker is one marker: all operations of one kind on one calendar day in Polish time.
export type ChartMarker = {
  kind: MarkerKind
  // date is YYYY-MM-DD, the day of the operations.
  date: string
  // price is the broker price: the volume-weighted average of the priced operations. It is decimal text.
  price: string
  volume: string
  operations: MarkerOperation[]
  // highlighted is true when the marker belongs to the highlighted Closed Position: its sell, or a Fill of its Lot.
  highlighted: boolean
}

// ChartRow is one x position of the chart: a day with a close, a marker day, or both.
// markers holds the marker of each kind on that day. The chart draws a series per kind from it.
export type ChartRow = {
  date: string
  // t is the day at 00:00 UTC in milliseconds. It is the x value, so that gaps in the series keep their width.
  t: number
  // close is the geometry of the price line. closeText is the exact text. Both are undefined for a day with no close.
  close?: number
  closeText?: string
  markers: Partial<Record<MarkerKind, ChartMarker>>
  // markerValues holds the y value of the marker of each kind, for the series of that kind.
  markerValues: Partial<Record<MarkerKind, number>>
}

// ChartModel is the data of the price chart.
export type ChartModel = {
  rows: ChartRow[]
  // unplaced is the number of operations on a day where no operation has a price. They have no vertical position.
  unplaced: number
  // highlighted tells that a Closed Position is highlighted. Then the markers that are not highlighted show dimmed.
  highlighted: boolean
  // currency is the currency of the y axis: the currency of the series, or the Quote Currency when the line is not drawn.
  // It is null when the series is empty and the Quote Currency is not known.
  currency: string | null
  // mismatch is set when the series has another currency than the Quote Currency. The model then has no price line.
  mismatch: { seriesCurrency: string; quoteCurrency: string } | null
}

// dayValue gives the x value of a YYYY-MM-DD date: the day at 00:00 UTC in milliseconds.
export function dayValue(date: string): number {
  const [year, month, day] = date.split("-").map(Number)
  return Date.UTC(year, month - 1, day)
}

// joinMarkers joins the operations of one day into one marker of the kind. The operations are in time order.
// A day where no operation has a price gives no marker, and its operations are counted in unplaced.
// An operation is highlighted when isHighlighted says so, and then its marker is highlighted.
function joinMarkers(
  kind: MarkerKind,
  operations: (MarkerOperation & { time: string })[],
  isHighlighted: (operation: MarkerOperation) => boolean
): { markers: ChartMarker[]; unplaced: number } {
  const days = new Map<string, MarkerOperation[]>()
  for (const operation of operations) {
    const date = polishDate(operation.time)
    days.set(date, [...(days.get(date) ?? []), operation])
  }
  const markers: ChartMarker[] = []
  let unplaced = 0
  for (const [date, dayOperations] of days) {
    const priced = dayOperations.filter((operation) => operation.price !== null)
    const price = weightedAverage(
      priced.map((operation) => ({
        value: operation.price as string,
        weight: operation.volume,
      }))
    )
    if (price === null) {
      unplaced += dayOperations.length
      continue
    }
    markers.push({
      kind,
      date,
      price,
      volume: sumDecimals(dayOperations.map((operation) => operation.volume)),
      operations: dayOperations,
      highlighted: dayOperations.some(isHighlighted),
    })
  }
  return { markers, unplaced }
}

// buildChartModel merges the price series and the markers into one list of rows in date order.
// When the currency of the series and the Quote Currency are both known and differ, the series is left out.
// The markers are in the broker price, so a line in another currency would be on another scale.
// A marker day that is not in the series, or is outside it, gets its own row, so its marker still shows.
// A buy marker joins the Fills of one day. A sell marker joins the Closed Positions of one day, at their close price.
// The highlighted Closed Position highlights its sell marker and the buy markers with a Fill of its Lot.
// The floats are for the geometry only. The text comes from the exact decimal text.
export function buildChartModel(
  prices: { currency: string | null; series: OverviewPrice[] },
  quoteCurrency: string | null,
  fills: OverviewFill[],
  closedPositions: ClosedPosition[],
  highlight: ClosedPosition | null
): ChartModel {
  const mismatch =
    prices.currency !== null &&
    quoteCurrency !== null &&
    prices.currency !== quoteCurrency
      ? { seriesCurrency: prices.currency, quoteCurrency }
      : null
  const series = mismatch ? [] : prices.series
  const rows = new Map<string, ChartRow>()
  for (const point of series) {
    rows.set(point.date, {
      date: point.date,
      t: dayValue(point.date),
      close: Number(point.close),
      closeText: point.close,
      markers: {},
      markerValues: {},
    })
  }
  const buys = joinMarkers(
    "buy",
    fills.map((fill) => ({
      id: fill.xtbId,
      positionId: fill.positionId,
      time: fill.time,
      volume: fill.volume,
      price: fill.price,
    })),
    (operation) => operation.positionId === highlight?.positionId
  )
  const sells = joinMarkers(
    "sell",
    closedPositions.map((closedPosition) => ({
      id: closedPosition.xtbId,
      positionId: closedPosition.positionId,
      time: closedPosition.time,
      volume: closedPosition.volume,
      price: closedPosition.closePrice,
    })),
    (operation) => operation.id === highlight?.xtbId
  )
  const unplaced = buys.unplaced + sells.unplaced
  for (const marker of [...buys.markers, ...sells.markers]) {
    const row = rows.get(marker.date) ?? {
      date: marker.date,
      t: dayValue(marker.date),
      markers: {},
      markerValues: {},
    }
    row.markers[marker.kind] = marker
    row.markerValues[marker.kind] = Number(marker.price)
    rows.set(marker.date, row)
  }
  return {
    rows: [...rows.values()].sort((a, b) => a.t - b.t),
    unplaced,
    highlighted: highlight !== null,
    currency: mismatch ? mismatch.quoteCurrency : prices.currency,
    mismatch,
  }
}
