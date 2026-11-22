import type { SortingState } from "@tanstack/react-table"
import { useState } from "react"

import {
  defaultTableView,
  isDefaultTableView,
  parseStoredView,
  serializeView,
  type TableView,
  type TableViewProps,
  type TableViewRules,
} from "@/lib/table-view"

// readStorage gives the stored text of a key, or null when the storage is missing or fails.
function readStorage(key: string): string | null {
  try {
    return window.localStorage.getItem(key)
  } catch {
    return null
  }
}

// writeStorage stores a text, or removes the key when the text is null. A failure is ignored.
function writeStorage(key: string, text: string | null) {
  try {
    if (text === null) window.localStorage.removeItem(key)
    else window.localStorage.setItem(key, text)
  } catch {
    // The table works with no storage. The view then lasts until the next reload.
  }
}

// useTableView owns the column order and the sorting of one table and keeps them in localStorage under the key.
// It reads the storage on mount and again when the key changes. It writes the storage only after a change of the user.
// A stored value that does not agree with the rules gives the default view and stays in the storage until the first change.
// The default view is not stored: a change that gives the default view removes the key.
// props goes to the table. isDefault tells if the view is the default view. reset gives the default view.
export function useTableView(storageKey: string, rules: TableViewRules) {
  const [state, setState] = useState<{ key: string; view: TableView }>(() => ({
    key: storageKey,
    view: parseStoredView(readStorage(storageKey), rules),
  }))
  let view = state.view
  // A page can stay mounted when the Account changes, and the new Account can use another key.
  // Then the hook reads the view of the new key during the render.
  if (state.key !== storageKey) {
    view = parseStoredView(readStorage(storageKey), rules)
    setState({ key: storageKey, view })
  }

  function change(next: TableView) {
    setState({ key: storageKey, view: next })
    writeStorage(
      storageKey,
      isDefaultTableView(next, rules) ? null : serializeView(next)
    )
  }

  const props: TableViewProps = {
    sorting: view.sorting,
    onSortingChange: (sorting: SortingState) => change({ ...view, sorting }),
    columnOrder: view.columnOrder,
    onColumnOrderChange: (columnOrder) => change({ ...view, columnOrder }),
    fixedColumnIds: rules.fixedColumnIds,
  }
  return {
    props,
    isDefault: isDefaultTableView(view, rules),
    reset: () => change(defaultTableView(rules)),
  }
}
