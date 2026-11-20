// tagHeight is the height of an end tag in pixels.
export const tagHeight = 20

// tagGap is the smallest free space between two end tags in pixels.
export const tagGap = 2

// tagCharWidth is the width of one character of the tag text in pixels. The text uses a monospace font of 11 px.
const tagCharWidth = 6.8

// tagPaddingX is the space at each side of the tag text in pixels.
const tagPaddingX = 7

// tagWidth gives the width of a tag for a text, in pixels.
export function tagWidth(text: string): number {
  return Math.ceil(text.length * tagCharWidth + tagPaddingX * 2)
}

// spreadTags moves the vertical centers of the end tags apart, so that no two tags overlap.
// Tags that touch form a group. The group stays centered on the mean of its wanted centers.
// Then the groups move inside the range from min to max, when the range has room for all tags.
// It returns the new centers in the order of the input.
export function spreadTags(
  centers: number[],
  min: number,
  max: number
): number[] {
  const step = tagHeight + tagGap
  const order = centers
    .map((center, index) => ({ center, index }))
    .sort((a, b) => a.center - b.center || a.index - b.index)
  // A group holds the first and the last position in the order, and the center of its first tag.
  type Group = { from: number; to: number; sum: number; top: number }
  const place = (group: Group): number => {
    const count = group.to - group.from + 1
    const mean = group.sum / count
    const top = mean - ((count - 1) * step) / 2
    const low = min + tagHeight / 2
    const high = max - tagHeight / 2 - (count - 1) * step
    return Math.max(low, Math.min(top, high))
  }
  const groups: Group[] = []
  order.forEach(({ center }, position) => {
    let group: Group = { from: position, to: position, sum: center, top: 0 }
    group.top = place(group)
    // A new group that reaches the group above it joins that group.
    for (
      let previous = groups[groups.length - 1];
      previous !== undefined &&
      previous.top + (previous.to - previous.from) * step + step > group.top;
      previous = groups[groups.length - 1]
    ) {
      groups.pop()
      group = {
        from: previous.from,
        to: group.to,
        sum: previous.sum + group.sum,
        top: 0,
      }
      group.top = place(group)
    }
    groups.push(group)
  })
  const result = new Array<number>(centers.length)
  for (const group of groups) {
    for (let position = group.from; position <= group.to; position++) {
      result[order[position].index] = group.top + (position - group.from) * step
    }
  }
  return result
}
