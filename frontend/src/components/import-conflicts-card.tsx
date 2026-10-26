import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import type {
  CashOperationData,
  ConflictDecision,
  ImportConflict,
  ProfitCheck,
} from "@/lib/api"
import { formatAmount, formatVolume } from "@/lib/format"
import { cn } from "@/lib/utils"

type ImportConflictsCardProps = {
  conflicts: ImportConflict[]
  // currency is the Account currency. A conflict exists only for a known Account.
  currency: string
  busy: boolean
  onDecide: (conflictId: number, decision: ConflictDecision) => void
}

// none marks a field that the Cash Operation does not have.
const none = "—"

// fields are the rows of the comparison table, in display order.
const fields: {
  key: keyof CashOperationData
  label: string
  format: (data: CashOperationData, currency: string) => string
}[] = [
  { key: "type", label: "Type", format: (data) => data.type },
  {
    key: "time",
    label: "Time",
    format: (data) => new Date(data.time).toLocaleString(),
  },
  {
    key: "amount",
    label: "Amount",
    format: (data, currency) => formatAmount(data.amount, currency),
  },
  { key: "comment", label: "Comment", format: (data) => data.comment },
  { key: "ticker", label: "Ticker", format: (data) => data.ticker || none },
  {
    key: "positionId",
    label: "Position ID",
    format: (data) => data.positionId || none,
  },
  {
    key: "volume",
    label: "Volume",
    format: (data) => (data.volume === null ? none : formatVolume(data.volume)),
  },
  {
    key: "price",
    label: "Price",
    // The price is in the Instrument currency, which can differ from the Account currency.
    format: (data) => (data.price === null ? none : formatVolume(data.price)),
  },
]

const decisionLabels: Record<ConflictDecision, string> = {
  "keep-existing": "Keep existing",
  "use-new": "Use new",
  accept: "Accept",
}

// decisionsByKind are the decisions that the backend accepts for each conflict kind.
const decisionsByKind: Record<ImportConflict["kind"], ConflictDecision[]> = {
  "changed-data": ["keep-existing", "use-new"],
  "check-mismatch": ["accept"],
}

export function ImportConflictsCard({
  conflicts,
  currency,
  busy,
  onDecide,
}: ImportConflictsCardProps) {
  const openCount = conflicts.filter(
    (conflict) => conflict.decision === null
  ).length

  return (
    <Card>
      <CardHeader>
        <CardTitle>Import Conflicts</CardTitle>
        <CardDescription>
          A Cash Operation with a known XTB ID but different data needs a
          decision: keep the stored data or use the new data. A sale with a
          Realized Profit that differs from Closed Positions by more than 0.01
          needs your acceptance. You can confirm the Import when each conflict
          has a decision.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-6">
        <p className="text-sm text-muted-foreground">
          {openCount === 0
            ? "All conflicts have a decision."
            : `${openCount} of ${conflicts.length} conflicts have no decision.`}
        </p>
        {conflicts.map((conflict) => (
          <div key={conflict.id} className="flex flex-col gap-2">
            <div className="flex items-center justify-between gap-2">
              <span className="text-sm font-medium">
                {conflict.kind === "check-mismatch"
                  ? "Realized Profit check, sale"
                  : "Changed data,"}{" "}
                XTB ID {conflict.xtbId}
              </span>
              {conflict.decision === null ? (
                <Badge variant="destructive">No decision</Badge>
              ) : (
                <Badge>{decisionLabels[conflict.decision]}</Badge>
              )}
            </div>
            {conflict.kind === "check-mismatch" ? (
              <ProfitCheckTable
                sale={conflict.new}
                check={conflict.check}
                currency={currency}
              />
            ) : (
              <ChangedDataTable
                existing={conflict.existing}
                incoming={conflict.new}
                currency={currency}
              />
            )}
            <ToggleGroup
              type="single"
              variant="outline"
              aria-label={`Decision for XTB ID ${conflict.xtbId}`}
              value={conflict.decision ?? ""}
              disabled={busy}
              onValueChange={(value) => {
                // An empty value means that the user clicked the selected item again.
                if (value) onDecide(conflict.id, value as ConflictDecision)
              }}
            >
              {decisionsByKind[conflict.kind].map((decision) => (
                <ToggleGroupItem key={decision} value={decision}>
                  {decisionLabels[decision]}
                </ToggleGroupItem>
              ))}
            </ToggleGroup>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}

type ChangedDataTableProps = {
  existing: CashOperationData
  incoming: CashOperationData
  currency: string
}

// ChangedDataTable compares the stored data and the new data of a Cash Operation.
function ChangedDataTable({
  existing,
  incoming,
  currency,
}: ChangedDataTableProps) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead />
          <TableHead>Stored data</TableHead>
          <TableHead>New data</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {fields.map((field) => {
          const changed = existing[field.key] !== incoming[field.key]
          return (
            <TableRow key={field.key}>
              <TableCell className="text-muted-foreground">
                {field.label}
              </TableCell>
              <TableCell
                className={cn("whitespace-normal", changed && "font-medium")}
              >
                {field.format(existing, currency)}
              </TableCell>
              <TableCell
                className={cn("whitespace-normal", changed && "font-medium")}
              >
                {field.format(incoming, currency)}
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}

type ProfitCheckTableProps = {
  sale: CashOperationData
  check: ProfitCheck
  currency: string
}

// ProfitCheckTable shows the Realized Profit of a sale next to the Profit/Loss in Closed Positions.
function ProfitCheckTable({ sale, check, currency }: ProfitCheckTableProps) {
  return (
    <Table>
      <TableBody>
        <TableRow>
          <TableCell className="text-muted-foreground">Sale</TableCell>
          <TableCell className="text-right whitespace-normal">
            {sale.ticker} · {new Date(sale.time).toLocaleString()} ·{" "}
            {sale.comment}
          </TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="text-muted-foreground">
            Realized Profit from the Ledger
          </TableCell>
          <TableCell className="text-right num">
            {formatAmount(check.calculated, currency)}
          </TableCell>
        </TableRow>
        <TableRow>
          <TableCell className="text-muted-foreground">
            Profit/Loss in Closed Positions
          </TableCell>
          <TableCell className="text-right num">
            {formatAmount(check.closedPositions, currency)}
          </TableCell>
        </TableRow>
      </TableBody>
    </Table>
  )
}
