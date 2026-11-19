import type { SortingState } from "@tanstack/react-table"

import { MAX_SORT_COLUMNS, type SortingProps } from "@/lib/table-sort"

// TableView is the view of one table that the user can change: the column order and the sorting.
export type TableView = {
  columnOrder: string[]
  sorting: SortingState
}

// ColumnOrderProps is the column order of a table and the way to change it. The owner of the table keeps the state.
export type ColumnOrderProps = {
  // columnOrder holds the ids of all columns in the shown order.
  columnOrder: string[]
  onColumnOrderChange: (columnOrder: string[]) => void
}

// TableViewProps is what a table needs to show a view: the sorting, the column order and the fixed columns.
export type TableViewProps = SortingProps &
  ColumnOrderProps & {
    // fixedColumnIds are the ids of the leading columns that cannot move, in order.
    fixedColumnIds: string[]
  }

// TableViewRules are the current facts of a table that a stored view must agree with.
export type TableViewRules = {
  // defaultColumnOrder holds the ids of all columns in the default order.
  defaultColumnOrder: string[]
  // sortableColumnIds are the ids of the columns that have a sort.
  sortableColumnIds: string[]
  // fixedColumnIds are the ids of the leading columns that cannot move, in order.
  fixedColumnIds: string[]
}

// tableViewRules gives the rules of a table from its column definitions, so a new column needs one edit.
// A column with a sortValue is sortable.
export function tableViewRules(
  columns: { id: string; sortValue?: unknown }[],
  fixedColumnIds: string[]
): TableViewRules {
  return {
    defaultColumnOrder: columns.map((column) => column.id),
    sortableColumnIds: columns
      .filter((column) => column.sortValue)
      .map((column) => column.id),
    fixedColumnIds,
  }
}

// STORED_VIEW_VERSION is the version of the stored value. A stored value with another version is not used.
// Version 2 drops the views from before the Instrument Overview removed columns from the tables.
const STORED_VIEW_VERSION = 2

// defaultTableView gives the view with the default column order and no sort.
export function defaultTableView(rules: TableViewRules): TableView {
  return { columnOrder: [...rules.defaultColumnOrder], sorting: [] }
}

// isDefaultTableView tells if a view equals the default view.
export function isDefaultTableView(
  view: TableView,
  rules: TableViewRules
): boolean {
  return (
    view.sorting.length === 0 &&
    view.columnOrder.length === rules.defaultColumnOrder.length &&
    view.columnOrder.every((id, i) => id === rules.defaultColumnOrder[i])
  )
}

// isValidColumnOrder tells if an order is a permutation of the current column ids with the fixed columns in their places.
function isValidColumnOrder(value: unknown, rules: TableViewRules): boolean {
  if (!Array.isArray(value)) return false
  const ids = rules.defaultColumnOrder
  if (value.length !== ids.length) return false
  if (new Set(value).size !== value.length) return false
  if (!value.every((id) => typeof id === "string" && ids.includes(id))) {
    return false
  }
  return rules.fixedColumnIds.every((id, i) => value[i] === id)
}

// isValidSorting tells if a sorting uses only sortable columns, has at most two entries and repeats no column.
function isValidSorting(value: unknown, rules: TableViewRules): boolean {
  if (!Array.isArray(value) || value.length > MAX_SORT_COLUMNS) return false
  const seen = new Set<string>()
  for (const entry of value) {
    if (typeof entry !== "object" || entry === null) return false
    const { id, desc } = entry as { id?: unknown; desc?: unknown }
    if (typeof id !== "string" || typeof desc !== "boolean") return false
    if (!rules.sortableColumnIds.includes(id) || seen.has(id)) return false
    seen.add(id)
  }
  return true
}

// parseStoredView gives the view for a stored text. It never throws.
// A missing value, a JSON error or a wrong version gives the default view.
// The column order and the sorting are checked apart. A part that is not valid gives the default for that part.
export function parseStoredView(
  text: string | null,
  rules: TableViewRules
): TableView {
  const fallback = defaultTableView(rules)
  if (text === null) return fallback
  try {
    const value: unknown = JSON.parse(text)
    if (typeof value !== "object" || value === null) return fallback
    const stored = value as {
      version?: unknown
      columnOrder?: unknown
      sorting?: unknown
    }
    if (stored.version !== STORED_VIEW_VERSION) return fallback
    return {
      columnOrder: isValidColumnOrder(stored.columnOrder, rules)
        ? (stored.columnOrder as string[])
        : fallback.columnOrder,
      sorting: isValidSorting(stored.sorting, rules)
        ? (stored.sorting as SortingState)
        : fallback.sorting,
    }
  } catch {
    return fallback
  }
}

// serializeView gives the text to store for a view.
export function serializeView(view: TableView): string {
  return JSON.stringify({ version: STORED_VIEW_VERSION, ...view })
}
