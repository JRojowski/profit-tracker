import type { ComponentProps } from "react"
import Markdown, { type Components, type ExtraProps } from "react-markdown"
import remarkGfm from "remark-gfm"

import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { cn } from "@/lib/utils"

type HastNode = NonNullable<ExtraProps["node"]>

// A cell is a number when it holds a signed figure with an optional unit, such as "+1 234,56 USD" or "12.5%".
const numberPattern = /^[+\-−(]?\d[\d\s.,]*\)?\s*(%|[A-Za-z]{3})?$/

// columnClasses holds the number styling for each of the first eight table columns. A wider table keeps plain
// columns after the eighth.
const columnClasses = [
  "[&_tr>:nth-child(1)]:text-right [&_tr>:nth-child(1)]:num",
  "[&_tr>:nth-child(2)]:text-right [&_tr>:nth-child(2)]:num",
  "[&_tr>:nth-child(3)]:text-right [&_tr>:nth-child(3)]:num",
  "[&_tr>:nth-child(4)]:text-right [&_tr>:nth-child(4)]:num",
  "[&_tr>:nth-child(5)]:text-right [&_tr>:nth-child(5)]:num",
  "[&_tr>:nth-child(6)]:text-right [&_tr>:nth-child(6)]:num",
  "[&_tr>:nth-child(7)]:text-right [&_tr>:nth-child(7)]:num",
  "[&_tr>:nth-child(8)]:text-right [&_tr>:nth-child(8)]:num",
]

// nodeText gives the plain text of a node of the Markdown tree.
function nodeText(node: HastNode | HastNode["children"][number]): string {
  if (node.type === "text") return node.value
  if (node.type === "element") return node.children.map(nodeText).join("")
  return ""
}

// numberColumns gives the classes for the columns of a table in which each body cell is a number.
// The header cell of such a column gets the same alignment, so that it stands above its figures.
function numberColumns(table: HastNode | undefined): string[] {
  if (table === undefined) return []
  const rows: HastNode[] = []
  const collect = (node: HastNode) => {
    for (const child of node.children) {
      if (child.type !== "element") continue
      if (child.tagName === "tr" && !isHeaderRow(child)) rows.push(child)
      else collect(child)
    }
  }
  collect(table)
  if (rows.length === 0) return []
  return columnClasses.filter((_, column) =>
    rows.every((row) => {
      const cells = row.children.filter((child) => child.type === "element")
      const cell = cells[column]
      return cell !== undefined && numberPattern.test(nodeText(cell).trim())
    })
  )
}

// isHeaderRow tells whether a row of the Markdown tree holds header cells.
function isHeaderRow(row: HastNode): boolean {
  return row.children.some(
    (child) => child.type === "element" && child.tagName === "th"
  )
}

// components maps each Markdown element to an element with the tokens of the app. A heading is bold text of the
// normal size. A link opens in a new tab. An image shows its alt text, because the Assistant must not load a
// remote file. A table scrolls inside the panel.
const components: Components = {
  p: (props) => <p className="text-sm leading-relaxed" {...omitNode(props)} />,
  h1: Heading,
  h2: Heading,
  h3: Heading,
  h4: Heading,
  h5: Heading,
  h6: Heading,
  strong: (props) => <strong className="font-semibold" {...omitNode(props)} />,
  em: (props) => <em className="italic" {...omitNode(props)} />,
  ul: (props) => (
    <ul
      className="flex list-disc flex-col gap-1 pl-5 text-sm marker:text-muted-foreground"
      {...omitNode(props)}
    />
  ),
  ol: (props) => (
    <ol
      className="flex list-decimal flex-col gap-1 pl-5 text-sm marker:text-muted-foreground"
      {...omitNode(props)}
    />
  ),
  li: (props) => <li className="pl-0.5 leading-relaxed" {...omitNode(props)} />,
  blockquote: (props) => (
    <blockquote
      className="border-l-2 pl-3 text-sm text-muted-foreground"
      {...omitNode(props)}
    />
  ),
  hr: () => <hr className="border-border" />,
  a: (props) =>
    props.href ? (
      <a
        className="text-primary underline underline-offset-2 hover:no-underline"
        {...omitNode(props)}
        target="_blank"
        rel="noreferrer noopener"
      />
    ) : (
      // The component removes an unsafe URL. The text stays, and it is no link.
      <span>{props.children}</span>
    ),
  img: ({ alt }) => <span>{alt}</span>,
  pre: (props) => (
    <pre
      className="max-w-full overflow-x-auto rounded-md bg-muted p-3 font-mono text-xs [&_code]:bg-transparent [&_code]:p-0"
      {...omitNode(props)}
    />
  ),
  code: (props) => (
    <code
      className="rounded-sm bg-muted px-1 py-0.5 font-mono text-[0.85em] break-words"
      {...omitNode(props)}
    />
  ),
  table: ({ node, ...props }) => (
    <Table
      className={cn("text-xs", numberColumns(node))}
      {...(props as ComponentProps<"table">)}
    />
  ),
  thead: (props) => <TableHeader {...omitNode(props)} />,
  tbody: (props) => <TableBody {...omitNode(props)} />,
  tr: (props) => <TableRow {...omitNode(props)} />,
  th: (props) => <TableHead className="h-8 px-2" {...omitNode(props)} />,
  td: (props) => (
    <TableCell className="px-2 py-1.5 whitespace-nowrap" {...omitNode(props)} />
  ),
}

// Heading shows a heading of the answer as bold text of the normal size.
function Heading(props: ComponentProps<"p"> & ExtraProps) {
  return <p className="text-sm font-semibold" {...omitNode(props)} />
}

// omitNode removes the Markdown tree node from the props, so that it does not reach the DOM element.
function omitNode<T extends ExtraProps>(props: T): Omit<T, "node"> {
  const { node: _node, ...rest } = props
  void _node
  return rest
}

// AssistantMarkdown shows the answer of the Assistant as Markdown. Raw HTML in the answer stays visible as
// text, and the unsafe URL of a link is removed, because the component uses neither a raw HTML plugin nor its own
// URL transform.
export function AssistantMarkdown({ children }: { children: string }) {
  return (
    <div className="flex min-w-0 flex-col gap-2 break-words">
      <Markdown remarkPlugins={[remarkGfm]} components={components}>
        {children}
      </Markdown>
    </div>
  )
}
