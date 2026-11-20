import { ChevronRightIcon, WrenchIcon } from "lucide-react"
import { useId, useState } from "react"

import { useAppState } from "@/lib/app-state"
import type { AssistantStep } from "@/lib/api"
import { assistantStepText } from "@/lib/assistant-step-text"
import { cn } from "@/lib/utils"

// AssistantSteps shows the functions that the model used for an answer. The title holds the count, and each step
// is a row that opens to its arguments and its result. An answer with no step says so, so that the user sees that
// the answer holds no data from the tool.
export function AssistantSteps({ steps }: { steps: AssistantStep[] }) {
  if (steps.length === 0) {
    return <p className="text-xs text-muted-foreground">No function used</p>
  }
  return (
    <section
      aria-label="Steps"
      className="flex min-w-0 flex-col overflow-hidden rounded-lg border"
    >
      <h3 className="flex items-center gap-1.5 border-b bg-muted/50 px-3 py-1.5 text-xs font-medium text-muted-foreground">
        Steps
        <span className="rounded-sm bg-muted px-1.5 num text-foreground">
          {steps.length}
        </span>
      </h3>
      <ul className="flex flex-col divide-y">
        {steps.map((step, index) => (
          <StepRow key={index} step={step} />
        ))}
      </ul>
    </section>
  )
}

// StepRow shows one step as a row with its icon, its compact text and the name of its function. A click, Enter or
// Space opens the arguments and the result.
function StepRow({ step }: { step: AssistantStep }) {
  const { accounts } = useAppState()
  const [open, setOpen] = useState(false)
  const detailsId = useId()
  return (
    <li className="min-w-0">
      <button
        type="button"
        aria-expanded={open}
        aria-controls={detailsId}
        onClick={() => setOpen((value) => !value)}
        className="flex w-full items-start gap-2 px-3 py-2 text-left text-xs outline-none hover:bg-muted/50 focus-visible:ring-2 focus-visible:ring-ring/50 focus-visible:ring-inset"
      >
        <WrenchIcon
          aria-hidden="true"
          className="mt-0.5 size-3.5 shrink-0 text-muted-foreground"
        />
        <span className="flex min-w-0 flex-1 flex-wrap items-baseline justify-between gap-x-3 gap-y-0.5">
          <span className="min-w-0 break-words text-foreground">
            {assistantStepText(step, accounts)}
          </span>
          <span className="font-mono text-muted-foreground">
            {step.function}
          </span>
        </span>
        <ChevronRightIcon
          aria-hidden="true"
          className={cn(
            "mt-0.5 size-3.5 shrink-0 text-muted-foreground transition-transform",
            open && "rotate-90"
          )}
        />
      </button>
      {open && (
        <div id={detailsId} className="flex flex-col gap-3 px-3 pt-1 pb-3">
          <JsonPart label="Arguments" value={step.arguments} />
          {step.result !== undefined && (
            <JsonPart label="Result" value={step.result} scroll />
          )}
        </div>
      )}
    </li>
  )
}

// JsonPart shows a labelled JSON value as pretty-printed text. The value never becomes HTML. With scroll, the box
// has a maximum height and its own scroll, and a key can focus it.
function JsonPart({
  label,
  value,
  scroll = false,
}: {
  label: string
  value: unknown
  scroll?: boolean
}) {
  const labelId = useId()
  return (
    <div className="flex min-w-0 flex-col gap-1">
      <p id={labelId} className="text-xs font-medium text-muted-foreground">
        {label}
      </p>
      <pre
        // A box that scrolls must take focus, so that the keyboard can scroll it.
        tabIndex={scroll ? 0 : undefined}
        role={scroll ? "region" : undefined}
        aria-labelledby={scroll ? labelId : undefined}
        className={cn(
          "overflow-x-auto rounded-md bg-muted p-2 font-mono text-xs break-normal whitespace-pre text-foreground",
          scroll &&
            "max-h-60 overflow-y-auto outline-none focus-visible:ring-2 focus-visible:ring-ring/50"
        )}
      >
        {JSON.stringify(value, null, 2)}
      </pre>
    </div>
  )
}
