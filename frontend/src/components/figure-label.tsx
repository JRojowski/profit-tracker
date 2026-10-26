import type { ReactNode } from "react"

// FigureLabel is the small section label above a figure or a chart, in Geist Mono capitals.
export function FigureLabel({ children }: { children: ReactNode }) {
  return (
    <p className="font-mono text-xs tracking-wider text-muted-foreground uppercase">
      {children}
    </p>
  )
}
