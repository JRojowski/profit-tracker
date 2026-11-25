import { ChevronDownIcon, ChevronRightIcon } from "lucide-react"

// TierToggle shows the name of a Tier as a button with a chevron. The button collapses the Tier to its Tier row or expands it.
export function TierToggle({
  open,
  onToggle,
  children,
  className,
}: {
  open: boolean
  onToggle: () => void
  children: React.ReactNode
  className?: string
}) {
  const Chevron = open ? ChevronDownIcon : ChevronRightIcon
  return (
    <button
      type="button"
      aria-expanded={open}
      onClick={onToggle}
      className={
        "flex items-start gap-1 rounded-sm text-left outline-none focus-visible:ring-2 focus-visible:ring-ring " +
        (className ?? "")
      }
    >
      <Chevron className="mt-0.5 size-4 shrink-0 text-muted-foreground" />
      <span>{children}</span>
    </button>
  )
}
