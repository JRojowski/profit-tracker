import { CircleAlertIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import type { UnsupportedEntry } from "@/lib/api"

type UnsupportedEntriesAlertProps = {
  entries: UnsupportedEntry[]
}

// UnsupportedEntriesAlert tells that a Kraken export has entries that the tool does not support, and lists them.
// Each row shows the reason, the time, the type with the subtype, and the asset of one entry.
export function UnsupportedEntriesAlert({
  entries,
}: UnsupportedEntriesAlertProps) {
  const count = entries.length
  return (
    <Alert variant="destructive">
      <CircleAlertIcon />
      <AlertTitle>
        The Import stopped: {count} unsupported{" "}
        {count === 1 ? "entry" : "entries"}
      </AlertTitle>
      <AlertDescription className="min-w-0">
        <p>
          The tool does not read these entries of the export yet. It saved
          nothing.
        </p>
        <ul className="flex min-w-0 flex-col divide-y divide-destructive/20 rounded-md border border-destructive/30">
          {entries.map((entry, index) => (
            <li
              key={`${entry.time}-${entry.type}-${entry.asset}-${index}`}
              className="flex min-w-0 flex-col gap-1 px-3 py-2 md:flex-row md:items-baseline md:gap-4"
            >
              <span className="font-medium text-foreground md:w-64 md:shrink-0">
                {capitalize(entry.reason)}
              </span>
              <span className="flex min-w-0 flex-wrap gap-x-3 gap-y-1 text-muted-foreground">
                <span className="num">
                  {new Date(entry.time).toLocaleString()}
                </span>
                <span className="font-mono break-all">
                  {entry.subtype
                    ? `${entry.type} / ${entry.subtype}`
                    : entry.type}
                </span>
                <span className="font-mono">{entry.asset}</span>
              </span>
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  )
}

// capitalize writes the first letter of a text as a capital letter.
function capitalize(text: string): string {
  return text.charAt(0).toUpperCase() + text.slice(1)
}
