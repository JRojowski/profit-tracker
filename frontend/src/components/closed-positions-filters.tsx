import { useState } from "react"
import {
  CalendarIcon,
  CheckIcon,
  ChevronsUpDownIcon,
  RotateCcw,
} from "lucide-react"

import { Button } from "@/components/ui/button"
import { Calendar } from "@/components/ui/calendar"
import {
  Command,
  CommandEmpty,
  CommandGroup,
  CommandInput,
  CommandItem,
  CommandList,
} from "@/components/ui/command"
import {
  Popover,
  PopoverContent,
  PopoverTrigger,
} from "@/components/ui/popover"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import {
  activeYear,
  allYears,
  hasActiveFilters,
  instrumentLabel,
  noFilters,
  yearRange,
  type ClosedPositionFilters,
} from "@/lib/closed-position-filters"
import { formatDate } from "@/lib/format"
import { cn } from "@/lib/utils"

// selectedItem marks the selected year with the accent, as in the mockup. The default muted background is too faint.
const selectedItem =
  "data-[state=on]:bg-primary/15 data-[state=on]:text-primary"

// InstrumentOption is one Instrument that has a Closed Position on the Account.
export type InstrumentOption = { ticker: string; name: string }

type ClosedPositionsFiltersProps = {
  filters: ClosedPositionFilters
  onChange: (filters: ClosedPositionFilters) => void
  // years are the calendar years of all Closed Positions in Polish time, as text.
  years: string[]
  // instruments are the options of the Instrument filter, sorted by ticker.
  instruments: InstrumentOption[]
}

// ClosedPositionsFilters is the toolbar row of the Closed Positions filters.
// The order is: year buttons, From, To, Instrument, Reset filters.
export function ClosedPositionsFilters({
  filters,
  onChange,
  years,
  instruments,
}: ClosedPositionsFiltersProps) {
  return (
    <div className="flex flex-wrap items-center gap-2">
      <YearButtons filters={filters} onChange={onChange} years={years} />
      <DatePicker
        label="From"
        value={filters.from}
        latest={filters.to}
        onChange={(from) => onChange({ ...filters, from })}
      />
      <DatePicker
        label="To"
        value={filters.to}
        earliest={filters.from}
        onChange={(to) => onChange({ ...filters, to })}
      />
      <InstrumentPicker
        selected={filters.instruments}
        options={instruments}
        onChange={(selected) => onChange({ ...filters, instruments: selected })}
      />
      <ResetFiltersButton filters={filters} onReset={onChange} />
    </div>
  )
}

type ResetFiltersButtonProps = {
  filters: ClosedPositionFilters
  onReset: (filters: ClosedPositionFilters) => void
}

// ResetFiltersButton removes each filter. It is disabled while no filter is active.
export function ResetFiltersButton({
  filters,
  onReset,
}: ResetFiltersButtonProps) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={!hasActiveFilters(filters)}
      onClick={() => onReset(noFilters)}
    >
      <RotateCcw data-icon="inline-start" />
      Reset filters
    </Button>
  )
}

// YearButtons sets the date range to one calendar year. A button is active only for exactly that year.
function YearButtons({
  filters,
  onChange,
  years,
}: Pick<ClosedPositionsFiltersProps, "filters" | "onChange" | "years">) {
  const active = activeYear(filters, years)
  return (
    // On a narrow screen, the buttons scroll by themselves when there are many years.
    <div className="max-w-full overflow-x-auto">
      <ToggleGroup
        type="single"
        variant="outline"
        size="sm"
        spacing={0}
        aria-label="Year"
        value={active}
        onValueChange={(value) => {
          // An empty value means that the user clicked the selected item again.
          if (!value) return
          onChange({
            ...filters,
            ...(value === allYears
              ? { from: undefined, to: undefined }
              : yearRange(value)),
          })
        }}
      >
        <ToggleGroupItem value={allYears} className={selectedItem}>
          All years
        </ToggleGroupItem>
        {years.map((year) => (
          <ToggleGroupItem
            key={year}
            value={year}
            className={cn("num", selectedItem)}
          >
            {year}
          </ToggleGroupItem>
        ))}
      </ToggleGroup>
    </div>
  )
}

// toDate reads a YYYY-MM-DD date as a calendar date at local midnight, with no time zone shift.
function toDate(date: string): Date {
  const [year, month, day] = date.split("-").map(Number)
  return new Date(year, month - 1, day)
}

// toDateText writes the local calendar day of a Date as YYYY-MM-DD.
function toDateText(date: Date): string {
  const month = String(date.getMonth() + 1).padStart(2, "0")
  const day = String(date.getDate()).padStart(2, "0")
  return `${String(date.getFullYear()).padStart(4, "0")}-${month}-${day}`
}

type DatePickerProps = {
  label: string
  value: string | undefined
  // earliest and latest limit the days that the calendar lets the user pick, so that the range stays valid.
  earliest?: string
  latest?: string
  onChange: (value: string | undefined) => void
}

// DatePicker is a button that opens a calendar for one optional date. The popover closes after a pick or a clear.
function DatePicker({
  label,
  value,
  earliest,
  latest,
  onChange,
}: DatePickerProps) {
  const [open, setOpen] = useState(false)
  const selected = value === undefined ? undefined : toDate(value)
  const change = (next: string | undefined) => {
    onChange(next)
    setOpen(false)
  }
  const disabled = [
    ...(earliest === undefined ? [] : [{ before: toDate(earliest) }]),
    ...(latest === undefined ? [] : [{ after: toDate(latest) }]),
  ]
  return (
    <Popover open={open} onOpenChange={setOpen}>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`${label} date: ${value === undefined ? "not set" : formatDate(value)}`}
        >
          <CalendarIcon data-icon="inline-start" />
          {value === undefined ? (
            label
          ) : (
            <>
              {label}: <span className="num">{formatDate(value)}</span>
            </>
          )}
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-auto max-w-[calc(100vw-2rem)] p-0"
        align="start"
      >
        <Calendar
          mode="single"
          aria-label={`${label} date calendar`}
          selected={selected}
          defaultMonth={selected}
          disabled={disabled}
          onSelect={(date) => change(date ? toDateText(date) : undefined)}
        />
        <div className="border-t p-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="w-full"
            disabled={value === undefined}
            onClick={() => change(undefined)}
          >
            Clear {label} date
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}

// InstrumentPicker is a button that opens a searchable list with multiple selection.
function InstrumentPicker({
  selected,
  options,
  onChange,
}: {
  selected: string[]
  options: InstrumentOption[]
  onChange: (selected: string[]) => void
}) {
  const label = instrumentLabel(selected)
  const toggle = (ticker: string) =>
    onChange(
      (selected.includes(ticker)
        ? selected.filter((item) => item !== ticker)
        : [...selected, ticker]
      ).sort()
    )
  return (
    <Popover>
      <PopoverTrigger asChild>
        <Button
          type="button"
          variant="outline"
          size="sm"
          aria-label={`Instrument filter: ${selected.length === 0 ? "none" : selected.join(", ")}`}
        >
          {label}
          <ChevronsUpDownIcon data-icon="inline-end" />
        </Button>
      </PopoverTrigger>
      <PopoverContent
        className="w-72 max-w-[calc(100vw-2rem)] p-0"
        align="start"
      >
        <Command>
          <CommandInput placeholder="Search Instruments" />
          <CommandList>
            <CommandEmpty>No Instruments found.</CommandEmpty>
            <CommandGroup>
              {options.map((option) => (
                <CommandItem
                  key={option.ticker}
                  // The value holds the name too, so that the search finds both.
                  value={`${option.ticker} ${option.name}`}
                  onSelect={() => toggle(option.ticker)}
                >
                  <CheckIcon
                    aria-hidden="true"
                    className={cn(
                      selected.includes(option.ticker)
                        ? "opacity-100"
                        : "opacity-0"
                    )}
                  />
                  <span className="font-medium">{option.ticker}</span>
                  <span className="truncate text-xs text-muted-foreground">
                    {option.name}
                  </span>
                  {selected.includes(option.ticker) && (
                    <span className="sr-only">selected</span>
                  )}
                </CommandItem>
              ))}
            </CommandGroup>
          </CommandList>
        </Command>
        <div className="border-t p-2">
          <Button
            type="button"
            variant="ghost"
            size="sm"
            className="w-full"
            disabled={selected.length === 0}
            onClick={() => onChange([])}
          >
            Clear selection
          </Button>
        </div>
      </PopoverContent>
    </Popover>
  )
}
