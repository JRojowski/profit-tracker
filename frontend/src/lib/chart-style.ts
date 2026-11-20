// chartLineWidth is the stroke width of a line of a line chart, in pixels.
export const chartLineWidth = 1.5

// chartGridProps is the light horizontal grid of each chart that has a grid.
// The lines use the border token at a low opacity, so they stay behind the data.
export const chartGridProps = {
  vertical: false,
  stroke: "var(--border)",
  strokeOpacity: 0.6,
} as const

// chartTooltipClass is the surface of a chart tooltip: the larger radius and the raised shadow.
export const chartTooltipClass =
  "rounded-xl border bg-popover text-popover-foreground shadow-(--shadow-raised)"
