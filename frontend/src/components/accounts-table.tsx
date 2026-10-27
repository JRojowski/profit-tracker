import { Link } from "react-router"

import { AccountBadges } from "@/components/account-badges"
import { unpricedNote } from "@/components/overview-hero"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableFooter,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type { AllAccountsSummary } from "@/lib/api"
import {
  amountSign,
  formatAmount,
  formatPercent,
  signColour,
} from "@/lib/format"
import { paths } from "@/lib/routes"
import { cn } from "@/lib/utils"

type AccountsTableProps = {
  summary: AllAccountsSummary
}

// AccountsTable shows the value, the Total Result and the Annual Return of each Account in PLN, and a total row.
// A click on a row opens the Overview of that Account.
export function AccountsTable({ summary }: AccountsTableProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Accounts</CardTitle>
      </CardHeader>
      <CardContent>
        <Table>
          <TableHeader>
            <TableRow>
              <TableHead>Account</TableHead>
              <TableHead className="text-right">
                <span className="hidden md:inline">Value</span>
                {/* On a narrow screen, one column holds the three figures, one under the other. */}
                <span className="flex flex-col items-end py-1 leading-tight md:hidden">
                  <span>Value</span>
                  <span>Total Result</span>
                  <span>Annual Return</span>
                </span>
              </TableHead>
              <TableHead className="hidden text-right md:table-cell">
                Total Result
              </TableHead>
              <TableHead className="hidden text-right md:table-cell">
                Annual Return
              </TableHead>
            </TableRow>
          </TableHeader>
          <TableBody>
            {summary.accounts.map((row) => (
              <TableRow key={row.accountId} className="relative cursor-pointer">
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
                    {row.note && (
                      <span className="text-sm whitespace-normal text-muted-foreground">
                        {row.note}
                      </span>
                    )}
                    {row.unpricedHoldings > 0 && (
                      <span className="text-sm whitespace-normal text-muted-foreground">
                        {unpricedNote(row.unpricedHoldings)}
                      </span>
                    )}
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
          <span className="max-w-48 font-sans text-xs font-normal whitespace-normal text-muted-foreground">
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
