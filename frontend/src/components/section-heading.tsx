import type { ReactNode } from "react"

import { cn } from "@/lib/utils"

type SectionHeadingProps = {
  // label is the small uppercase category above the title. It must add information, so omit it when it only repeats the title.
  label?: string
  title: ReactNode
  // level is the heading level. A page section is h2. A section inside a panel is h3.
  level?: "h2" | "h3"
  // description is one line of text below the title.
  description?: ReactNode
  // action is the area at the right of the title, for example a button or a selector.
  action?: ReactNode
  className?: string
}

// SectionHeading is the shared heading of a section: an optional label in the primary colour, the title and an optional action area.
// The sections of the pages and of the Instrument Overview panel all use it.
export function SectionHeading({
  label,
  title,
  level: Heading = "h2",
  description,
  action,
  className,
}: SectionHeadingProps) {
  const small = Heading === "h3"
  return (
    <div
      data-slot="section-heading"
      className={cn(
        "flex min-w-0 flex-wrap items-start justify-between gap-x-4 gap-y-3",
        className
      )}
    >
      <div className="flex min-w-0 flex-col gap-1">
        {label && (
          <p className="text-xs font-semibold tracking-wider text-primary uppercase">
            {label}
          </p>
        )}
        <Heading
          className={cn(
            "flex flex-wrap items-center gap-2 font-heading leading-snug font-semibold tracking-tight",
            small ? "text-sm" : "text-lg"
          )}
        >
          {title}
        </Heading>
        {description && (
          <div className="text-sm text-muted-foreground">{description}</div>
        )}
      </div>
      {action && <div className="shrink-0">{action}</div>}
    </div>
  )
}
