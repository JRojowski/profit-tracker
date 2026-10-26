import { TriangleAlertIcon } from "lucide-react"

import {
  SummaryTile,
  unpricedNote,
  type Tile,
} from "@/components/account-summary-tiles"
import { ReturnTile } from "@/components/return-tile"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import type {
  AllAccountsAmounts,
  AllAccountsRow,
  AllAccountsSummary,
} from "@/lib/api"
import { formatAmount, formatRate } from "@/lib/format"

type AllAccountsSummaryViewProps = {
  summary: AllAccountsSummary
}

// AllAccountsSummaryView shows the totals of all Accounts in PLN and one row for each Account, by ADR 0004.
export function AllAccountsSummaryView({
  summary,
}: AllAccountsSummaryViewProps) {
  const unpriced = unpricedNote(summary.unpricedHoldings)
  const totalTiles: Tile[] = [
    { label: "Value", amount: summary.value, note: unpriced },
    { label: "Cash", amount: summary.cash },
    { label: "Net Deposits", amount: summary.netDeposits },
    { label: "Total Result", amount: summary.totalResult, note: unpriced },
  ]
  const resultParts: Tile[] = [
    { label: "Realized Profit", amount: summary.realizedProfit },
    {
      label: "Unrealized Profit",
      amount: summary.unrealizedProfit,
      note: unpriced,
    },
    { label: "Net dividends", amount: summary.netDividends },
    { label: "Net interest", amount: summary.netInterest },
    { label: "Fees", amount: summary.fees },
    {
      label: "Currency Result",
      amount: summary.currencyResult,
      note: "The effect of the Exchange Rates on the Net Deposits.",
    },
  ]

  return (
    <div className="flex flex-col gap-4">
      {summary.warnings.length > 0 && (
        <Alert variant={summary.available ? "default" : "destructive"}>
          <TriangleAlertIcon />
          <AlertTitle>
            {summary.available
              ? "Check the totals"
              : "The tool cannot calculate the totals"}
          </AlertTitle>
          <AlertDescription>
            <ul className="list-inside list-disc">
              {summary.warnings.map((warning) => (
                <li key={warning}>{warning}</li>
              ))}
            </ul>
          </AlertDescription>
        </Alert>
      )}
      <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
        {totalTiles.map((tile) => (
          <SummaryTile
            key={tile.label}
            tile={tile}
            currency="PLN"
            titleClassName="text-2xl"
          />
        ))}
        <ReturnTile returns={summary} />
      </div>
      <div className="flex flex-col gap-2">
        <h3 className="text-sm font-medium text-muted-foreground">
          Parts of the total result
        </h3>
        <div className="grid gap-4 sm:grid-cols-3 lg:grid-cols-6">
          {resultParts.map((tile) => (
            <SummaryTile
              key={tile.label}
              tile={tile}
              currency="PLN"
              titleClassName="text-xl"
            />
          ))}
        </div>
      </div>
      <Card>
        <CardHeader>
          <CardTitle>Accounts in PLN</CardTitle>
        </CardHeader>
        <CardContent>
          <AccountsTable rows={summary.accounts} />
        </CardContent>
      </Card>
    </div>
  )
}

// columns are the amount columns of the table, in the order of the tiles.
const columns: { label: string; key: keyof AllAccountsAmounts }[] = [
  { label: "Value", key: "value" },
  { label: "Cash", key: "cash" },
  { label: "Net Deposits", key: "netDeposits" },
  { label: "Total Result", key: "totalResult" },
  { label: "Realized Profit", key: "realizedProfit" },
  { label: "Unrealized Profit", key: "unrealizedProfit" },
  { label: "Net dividends", key: "netDividends" },
  { label: "Net interest", key: "netInterest" },
  { label: "Fees", key: "fees" },
  { label: "Currency Result", key: "currencyResult" },
]

function AccountsTable({ rows }: { rows: AllAccountsRow[] }) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Account</TableHead>
          <TableHead>Currency</TableHead>
          <TableHead className="text-right">Exchange Rate</TableHead>
          {columns.map((column) => (
            <TableHead key={column.key} className="text-right">
              {column.label}
            </TableHead>
          ))}
        </TableRow>
      </TableHeader>
      <TableBody>
        {rows.map((row) => (
          <TableRow key={row.accountId}>
            <TableCell>
              <div className="flex flex-col gap-1">
                <span>
                  {row.name}{" "}
                  <span className="text-muted-foreground">{row.type}</span>
                </span>
                {row.note && (
                  <span className="text-sm text-muted-foreground">
                    {row.note}
                  </span>
                )}
                {row.unpricedHoldings > 0 && (
                  <span className="text-sm text-muted-foreground">
                    {unpricedNote(row.unpricedHoldings)}
                  </span>
                )}
              </div>
            </TableCell>
            <TableCell>
              {row.currency === "PLN" ? (
                row.currency
              ) : (
                <Badge variant="outline">{row.currency}</Badge>
              )}
            </TableCell>
            <TableCell className="text-right tabular-nums">
              {row.exchangeRate === null ? "—" : formatRate(row.exchangeRate)}
            </TableCell>
            {columns.map((column) => {
              const amount = row[column.key]
              return (
                <TableCell key={column.key} className="text-right tabular-nums">
                  {amount === null ? "—" : formatAmount(amount, "PLN")}
                </TableCell>
              )
            })}
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
