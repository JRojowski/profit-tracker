import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import { historyPeriods, type HistoryPeriod } from "@/lib/api"

type PeriodPillsProps = {
  value: HistoryPeriod
  onChange: (period: HistoryPeriod) => void
}

// PeriodPills is the selector of the period of the Value History and of the Benchmarks.
export function PeriodPills({ value, onChange }: PeriodPillsProps) {
  return (
    <ToggleGroup
      type="single"
      size="sm"
      data-pills
      aria-label="Period"
      value={value}
      onValueChange={(next) => {
        // An empty value means that the user clicked the selected item again.
        if (next) onChange(next as HistoryPeriod)
      }}
      className="shrink-0"
    >
      {historyPeriods.map((item) => (
        <ToggleGroupItem key={item} value={item} className="num">
          {item}
        </ToggleGroupItem>
      ))}
    </ToggleGroup>
  )
}
