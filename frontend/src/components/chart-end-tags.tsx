import { usePlotArea, useXAxisScale, useYAxisScale } from "recharts"

import { spreadTags, tagHeight, tagWidth } from "@/lib/end-tags"

// tagOffset is the free space between the end of a line and its tag, in pixels.
export const tagOffset = 6

// EndTag is one tag at the end of a line.
export type EndTag = {
  // value is the last value of the line, as a float for the geometry only.
  value: number
  // text is the formatted amount that the tag shows.
  text: string
  // fill is the color of the series. ink is the text color, with a contrast of 4.5:1 or more on the fill.
  fill: string
  ink: string
}

type ChartEndTagsProps = {
  // time is the x value of the last point.
  time: number
  tags: EndTag[]
}

// ChartEndTags draws a small pill with the last value at the end of each line.
// It must be a child of a Recharts chart. It moves tags that overlap apart vertically.
// The tags are decoration. The amount row under the chart is the text alternative.
export function ChartEndTags({ time, tags }: ChartEndTagsProps) {
  const xScale = useXAxisScale()
  const yScale = useYAxisScale()
  const plot = usePlotArea()
  if (!xScale || !yScale || !plot) return null
  const x = xScale(time)
  if (typeof x !== "number") return null
  const wanted = tags.map((tag) => yScale(tag.value))
  if (wanted.some((y) => typeof y !== "number")) return null
  const centers = spreadTags(wanted as number[], plot.y, plot.y + plot.height)
  return (
    <g aria-hidden data-slot="chart-end-tags">
      {tags.map((tag, index) => {
        const width = tagWidth(tag.text)
        const left = x + tagOffset
        const centre = centers[index]
        return (
          <g key={index}>
            <rect
              x={left}
              y={centre - tagHeight / 2}
              width={width}
              height={tagHeight}
              rx={tagHeight / 2}
              fill={tag.fill}
            />
            <text
              className="num"
              x={left + width / 2}
              y={centre}
              textAnchor="middle"
              dominantBaseline="central"
              fontSize={11}
              fontWeight={500}
              fill={tag.ink}
            >
              {tag.text}
            </text>
          </g>
        )
      })}
    </g>
  )
}
