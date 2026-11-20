import type { HTMLAttributes } from "react"

import { cn } from "@/lib/utils"

type InsetSurfaceProps = HTMLAttributes<HTMLElement> & {
  // as picks the element, so a list item stays a list item.
  as?: "div" | "li" | "dl" | "ul" | "section"
}

// InsetSurface is the shared box inside a card, for example one figure or one amount. It defines the border, the tint and the radius.
export function InsetSurface({
  as: Element = "div",
  className,
  ...props
}: InsetSurfaceProps) {
  return (
    <Element
      data-slot="inset-surface"
      className={cn("rounded-lg border bg-muted/40", className)}
      {...props}
    />
  )
}
