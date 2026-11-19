import type { SortingState } from "@tanstack/react-table"

import { compareDecimals } from "@/lib/decimal"

// SortingProps is the sorting of a table and the way to change it. The owner of the table keeps the state.
export type SortingProps = {
  sorting: SortingState
  onSortingChange: (sorting: SortingState) => void
}

// SortValue is the value that a sort compares: text, or a decimal amount as exact text.
// A row with no value uses null.
export type SortValue =
  { kind: "text"; text: string } | { kind: "decimal"; amount: string } | null

// textSortValue makes the sort value of a text, for example a ticker.
export function textSortValue(text: string): SortValue {
  return { kind: "text", text }
}

// decimalSortValue makes the sort value of a decimal amount from the backend. A null amount gives no value.
export function decimalSortValue(amount: string | null): SortValue {
  return amount === null ? null : { kind: "decimal", amount }
}

// SortableColumn is the part of a column that the sort needs.
export type SortableColumn<TRow> = {
  id: string
  // sortValue gives the sort value of a row. A column with no sortValue has no sort.
  sortValue?: (row: TRow) => SortValue
}

// MAX_SORT_COLUMNS is the largest number of columns that one sort can use.
export const MAX_SORT_COLUMNS = 2

// nextSorting gives the sorting after a click on the header of one column.
// A click on one column goes through ascending, descending and no sort.
// A click without Shift keeps only that column. A click with Shift (additive) keeps the other column.
// A Shift-click on a new column adds it as sort 2, or replaces sort 2 when there are two sorts already.
export function nextSorting(
  sorting: SortingState,
  columnId: string,
  additive: boolean
): SortingState {
  const current = sorting.find((sort) => sort.id === columnId)
  const next = cycle(current?.desc)

  if (!additive) {
    return next === undefined ? [] : [{ id: columnId, desc: next }]
  }
  if (current !== undefined) {
    return next === undefined
      ? sorting.filter((sort) => sort.id !== columnId)
      : sorting.map((sort) =>
          sort.id === columnId ? { id: columnId, desc: next } : sort
        )
  }
  return [
    ...sorting.slice(0, MAX_SORT_COLUMNS - 1),
    { id: columnId, desc: false },
  ]
}

// cycle gives the direction after a click. The input undefined means no sort, false means ascending, true means descending.
// The output undefined means no sort.
function cycle(desc: boolean | undefined): boolean | undefined {
  if (desc === undefined) return false
  return desc ? undefined : true
}

// compareSortValues compares two values that are not null, in ascending order.
// Decimal amounts use the exact comparison. Values of different kinds are equal.
function compareSortValues(
  a: NonNullable<SortValue>,
  b: NonNullable<SortValue>
): number {
  if (a.kind === "decimal" && b.kind === "decimal") {
    return compareDecimals(a.amount, b.amount)
  }
  if (a.kind === "text" && b.kind === "text")
    return a.text.localeCompare(b.text)
  return 0
}

// sortRows gives a sorted copy of the rows. It is the only sort of the tables.
// A row with no value goes last in both directions. The sort is stable.
// An empty sorting keeps the order of the rows.
export function sortRows<TRow>(
  rows: TRow[],
  sorting: SortingState,
  columns: SortableColumn<TRow>[]
): TRow[] {
  const sorts = sorting.flatMap((sort) => {
    const sortValue = columns.find((column) => column.id === sort.id)?.sortValue
    return sortValue === undefined ? [] : [{ sortValue, desc: sort.desc }]
  })
  if (sorts.length === 0) return rows
  return rows
    .map((row, index) => ({ row, index }))
    .sort((a, b) => {
      for (const { sortValue, desc } of sorts) {
        const valueA = sortValue(a.row)
        const valueB = sortValue(b.row)
        if (valueA === null && valueB === null) continue
        if (valueA === null) return 1
        if (valueB === null) return -1
        const result = compareSortValues(valueA, valueB)
        if (result !== 0) return desc ? -result : result
      }
      return a.index - b.index
    })
    .map(({ row }) => row)
}
