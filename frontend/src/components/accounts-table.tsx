import { ArrowDownIcon, ArrowUpIcon } from "lucide-react"
import { useMemo, useState } from "react"
import { Link } from "react-router"
import type { SortingState } from "@tanstack/react-table"

import { SectionHeading } from "@/components/section-heading"

import { AccountBadges } from "@/components/account-badges"
import { unpricedNote } from "@/components/overview-hero"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { AllAccountsSummary, AllAccountsRow } from "@/lib/api"
import {
  amountSign,
  formatAmount,
  formatPercent,
  signColour,
} from "@/lib/format"
import { paths } from "@/lib/routes"
import {
  decimalSortValue,
  nextSorting,
  sortRows,
  textSortValue,
  type SortableColumn,
} from "@/lib/table-sort"
import { cn } from "@/lib/utils"

type AccountsTableProps = {
  summary: AllAccountsSummary
}

// accountColumns are the sortable columns of the table. The ids are the ids of the sorting.
const accountColumns: SortableColumn<AllAccountsRow>[] = [
  { id: "account", sortValue: (row) => textSortValue(row.name) },
  { id: "value", sortValue: (row) => decimalSortValue(row.value) },
  { id: "totalResult", sortValue: (row) => decimalSortValue(row.totalResult) },
  {
    id: "annualReturn",
    sortValue: (row) => decimalSortValue(row.annualReturn),
  },
]

// SortHeader is the button of a sortable header. It shows the direction of the sort of its column.
function SortHeader({
  id,
  sorting,
  onSort,
  align,
  children,
}: {
  id: string
  sorting: SortingState
  onSort: (id: string, additive: boolean) => void
  align?: "right"
  children: React.ReactNode
}) {
  const sort = sorting.find((s) => s.id === id)
  const Arrow = sort?.desc ? ArrowDownIcon : ArrowUpIcon
  return (
    <button
      type="button"
      onClick={(event) => onSort(id, event.shiftKey)}
      className={cn(
        "-mx-1.5 inline-flex items-center gap-1 rounded-md px-1.5 py-1 outline-none hover:bg-muted focus-visible:ring-2 focus-visible:ring-ring/50",
        align === "right" && "flex-row-reverse"
      )}
    >
      <span>{children}</span>
      {sort && <Arrow aria-hidden className="size-3.5 text-muted-foreground" />}
    </button>
  )
}

// ariaSort gives the aria-sort value of the header of a column.
function ariaSort(sorting: SortingState, id: string) {
  const sort = sorting.find((s) => s.id === id)
  if (!sort) return "none"
  return sort.desc ? "descending" : "ascending"
}

// AccountsTable shows the value, the Total Result and the Annual Return of each Account in PLN, and a total row.
// A click on a header sorts the rows by that column. A click on a row opens the Overview of that Account.
// Each row has the same height. A note under the name of an Account is one line.
export function AccountsTable({ summary }: AccountsTableProps) {
  const [sorting, setSorting] = useState<SortingState>([])
  const rows = useMemo(
    () => sortRows(summary.accounts, sorting, accountColumns),
    [summary.accounts, sorting]
  )
  const sort = (id: string, additive: boolean) =>
    setSorting(nextSorting(sorting, id, additive))
  return (
    <Card>
      <CardHeader>
        <SectionHeading title="Accounts" />
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead aria-sort={ariaSort(sorting, "account")}>
                <SortHeader id="account" sorting={sorting} onSort={sort}>
                  Account
                </SortHeader>
              </TableHead>
              <TableHead
                className="text-right"
                aria-sort={ariaSort(sorting, "value")}
              >
                <span className="hidden md:inline">
                  <SortHeader
                    id="value"
                    sorting={sorting}
                    onSort={sort}
                    align="right"
                  >
                    Value
                  </SortHeader>
                </span>
                {/* On a narrow screen, one column holds the three figures, one under the other. */}
                <span className="flex flex-col items-end leading-tight md:hidden">
                  <SortHeader
                    id="value"
                    sorting={sorting}
                    onSort={sort}
                    align="right"
                  >
                    Value
                  </SortHeader>
                  <SortHeader
                    id="totalResult"
                    sorting={sorting}
                    onSort={sort}
                    align="right"
                  >
                    Total Result
                  </SortHeader>
                  <SortHeader
                    id="annualReturn"
                    sorting={sorting}
                    onSort={sort}
                    align="right"
                  >
                    Annual Return
                  </SortHeader>
                </span>
              </TableHead>
              <TableHead
                className="hidden text-right md:table-cell"
                aria-sort={ariaSort(sorting, "totalResult")}
              >
                <SortHeader
                  id="totalResult"
                  sorting={sorting}
                  onSort={sort}
                  align="right"
                >
                  Total Result
                </SortHeader>
              </TableHead>
              <TableHead
                className="hidden text-right md:table-cell"
                aria-sort={ariaSort(sorting, "annualReturn")}
              >
                <SortHeader
                  id="annualReturn"
                  sorting={sorting}
                  onSort={sort}
                  align="right"
                >
                  Annual Return
                </SortHeader>
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {rows.map((row) => (
              <TableRow
                key={row.accountId}
                className="relative h-28 cursor-pointer md:h-16"
              >
                <TableCell>
                  <div className="flex flex-col gap-1">
                    <span className="flex items-center gap-2">
                      {/* The link covers the whole row, so that a click on the row opens the Account. */}
                      <Link
                        to={paths.account(row.accountId)}
                        className="font-medium after:absolute after:inset-0 hover:underline"
                      >
                        {row.name}
                      </Link>
                      <AccountBadges account={row} />
                    </span>
                    <span
                      className="max-w-40 truncate text-sm text-muted-foreground md:max-w-md"
                      title={rowNote(row)}
                    >
                      {rowNote(row)}
                    </span>
                  </div>
                </TableCell>
                <TableCell className="text-right align-top num">
                  <FiguresCell figures={row} />
                </TableCell>
                <TableCell className="hidden text-right align-top num md:table-cell">
                  <AmountText amount={row.totalResult} signed />
                </TableCell>
                <TableCell className="hidden text-right align-top md:table-cell">
                  <ReturnText
                    percent={row.annualReturn}
                    note={row.annualReturnNote}
                  />
                </TableCell>
              </TableRow>
            ))}
          </TableBody>
          <TableFooter>
            <TableRow>
              <TableCell className="font-medium">Total</TableCell>
              <TableCell className="text-right num">
                <FiguresCell figures={summary} />
              </TableCell>
              <TableCell className="hidden text-right num md:table-cell">
                <AmountText amount={summary.totalResult} signed />
              </TableCell>
              <TableCell className="hidden text-right md:table-cell">
                <ReturnText
                  percent={summary.annualReturn}
                  note={summary.annualReturnNote}
                />
              </TableCell>
            </TableRow>
          </TableFooter>
        </Table>
      </CardContent>
    </Card>
  )
}

// rowNote gives the one-line note under the name of an Account, or "-" when there is none.
function rowNote(row: AllAccountsRow) {
  const notes = [row.note, unpricedNote(row.unpricedHoldings)].filter(
    (note): note is string => Boolean(note)
  )
  return notes.length > 0 ? notes.join(" ") : "-"
}

type AmountTextProps = {
  amount: string | null
  // signed shows the sign and the colour of a result.
  signed?: boolean
}

// AmountText shows an amount in PLN, or "—" when the tool cannot convert it.
function AmountText({ amount, signed = false }: AmountTextProps) {
  if (amount === null) return <>—</>
  return (
    <span className={cn(signed && signColour(amountSign(amount)))}>
      {formatAmount(amount, "PLN", { signed })}
    </span>
  )
}

type ReturnTextProps = {
  percent: string | null
  note: string | null
}

// ReturnText shows a return with its sign and colour. A null return shows "—" and the note that tells why.
function ReturnText({ percent, note }: ReturnTextProps) {
  if (percent === null) {
    return (
      <span className="flex flex-col items-end gap-1">
        <span className="num">—</span>
        {note && (
          <span
            className="max-w-32 truncate font-sans text-xs font-normal text-muted-foreground md:max-w-48"
            title={note ?? undefined}
          >
            {note}
          </span>
        )}
      </span>
    )
  }
  return (
    <span className={cn("num", signColour(amountSign(percent)))}>
      {formatPercent(percent, { signed: true })}
    </span>
  )
}

type FiguresCellProps = {
  figures: {
    value: string | null
    totalResult: string | null
    annualReturn: string | null
    annualReturnNote: string | null
  }
}

// FiguresCell shows the value. On a narrow screen it also shows the Total Result and the Annual Return under it,
// in place of their own columns.
function FiguresCell({ figures }: FiguresCellProps) {
  return (
    <span className="flex flex-col items-end gap-1">
      <AmountText amount={figures.value} />
      <span className="flex flex-col items-end gap-1 md:hidden">
        <AmountText amount={figures.totalResult} signed />
        <ReturnText
          percent={figures.annualReturn}
          note={figures.annualReturnNote}
        />
      </span>
    </span>
  )
}
