import { ArrowDown, ArrowUp } from "lucide-react"

import { Button } from "@/components/ui/button"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import type { SortingProps } from "@/lib/table-sort"

// DEFAULT_ORDER is the value of the select option for no sort.
const DEFAULT_ORDER = "default"

// SortControl is the select control of the narrow layout: the column and the direction of one sort.
// An empty sorting is the default order. The options are the sortable columns.
export function SortControl({
  options,
  sorting,
  onSortingChange,
}: SortingProps & { options: { id: string; header: string }[] }) {
  const sort = sorting[0]
  const Arrow = sort?.desc ? ArrowDown : ArrowUp
  return (
    <div className="flex items-center gap-2">
      <Select
        value={sort?.id ?? DEFAULT_ORDER}
        onValueChange={(id) =>
          onSortingChange(
            id === DEFAULT_ORDER ? [] : [{ id, desc: sort?.desc ?? false }]
          )
        }
      >
        <SelectTrigger className="min-w-0 flex-1" aria-label="Sort by">
          <SelectValue />
        </SelectTrigger>
        <SelectContent>
          <SelectItem value={DEFAULT_ORDER}>Default order</SelectItem>
          {options.map((option) => (
            <SelectItem key={option.id} value={option.id}>
              {option.header}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
      <Button
        type="button"
        variant="outline"
        size="icon"
        disabled={sort === undefined}
        aria-label={sort?.desc ? "Descending" : "Ascending"}
        title={sort?.desc ? "Descending" : "Ascending"}
        onClick={() =>
          sort && onSortingChange([{ id: sort.id, desc: !sort.desc }])
        }
      >
        <Arrow />
      </Button>
    </div>
  )
}
