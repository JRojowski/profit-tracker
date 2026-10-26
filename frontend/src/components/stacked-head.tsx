import { cn } from "@/lib/utils"

type StackedHeadProps = {
  labels: string[]
  // alignEnd aligns the labels to the right, above the amounts.
  alignEnd?: boolean
}

// StackedHead shows the labels of the figures that one table column holds on a narrow screen, one under the other.
// It is hidden from the breakpoint md, where each figure has its own column.
export function StackedHead({ labels, alignEnd = false }: StackedHeadProps) {
  return (
    <span
      className={cn(
        "flex flex-col py-1 leading-tight md:hidden",
        alignEnd && "items-end"
      )}
    >
      {labels.map((label) => (
        <span key={label}>{label}</span>
      ))}
    </span>
  )
}
