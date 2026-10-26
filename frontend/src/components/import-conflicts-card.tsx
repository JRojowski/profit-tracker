import { Badge } from "@/components/ui/badge"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
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

// fields are the rows of the comparison, in display order. numeric marks a field that shows a number.
const fields: {
  key: keyof CashOperationData
  label: string
  numeric: boolean
  format: (data: CashOperationData, currency: string) => string
}[] = [
  { key: "type", label: "Type", numeric: false, format: (data) => data.type },
  {
    key: "time",
    label: "Time",
    numeric: true,
    format: (data) => new Date(data.time).toLocaleString(),
  },
  {
    key: "amount",
    label: "Amount",
    numeric: true,
    format: (data, currency) => formatAmount(data.amount, currency),
  },
  {
    key: "comment",
    label: "Comment",
    numeric: false,
    format: (data) => data.comment || none,
  },
  {
    key: "ticker",
    label: "Ticker",
    numeric: true,
    format: (data) => data.ticker || none,
  },
  {
    key: "positionId",
    label: "Position ID",
    numeric: true,
    format: (data) => data.positionId || none,
  },
  {
    key: "volume",
    label: "Volume",
    numeric: true,
    format: (data) => (data.volume === null ? none : formatVolume(data.volume)),
  },
  {
    key: "price",
    label: "Price",
    numeric: true,
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

// warningBadge is the look of the "No decision" badge and of the count of the open conflicts.
const warningBadge = "border-warning/40 bg-warning/10 text-warning"

// ImportConflictsCard shows each Import Conflict of an Import with its decision control.
// A changed-data conflict shows the stored data and the new data side by side, with the changed fields highlighted.
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
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle className="flex flex-wrap items-center gap-2">
          Import Conflicts
          <Badge variant="secondary" className="num font-mono">
            {conflicts.length}
          </Badge>
          {openCount > 0 ? (
            <Badge variant="outline" className={cn("font-mono", warningBadge)}>
              {openCount} with no decision
            </Badge>
          ) : (
            <Badge
              variant="outline"
              className="border-positive/30 bg-positive/10 font-mono text-positive"
            >
              All decided
            </Badge>
          )}
        </CardTitle>
        <CardDescription>
          A Cash Operation with a known XTB ID but different data needs a
          decision: keep the stored data or use the new data. A sale with a
          Realized Profit that differs from Closed Positions by more than 0.01
          needs your acceptance. You can confirm the Import when each conflict
          has a decision.
        </CardDescription>
      </CardHeader>
      <CardContent className="flex flex-col gap-4">
        {conflicts.map((conflict) => (
          <section
            key={conflict.id}
            aria-label={`Import Conflict, XTB ID ${conflict.xtbId}`}
            className={cn(
              "flex min-w-0 flex-col gap-4 rounded-lg border p-3 md:p-4",
              conflict.decision === null && "border-warning/40"
            )}
          >
            <div className="flex flex-wrap items-center justify-between gap-2">
              <span className="min-w-0 text-sm">
                <span className="font-medium">
                  {conflict.kind === "check-mismatch"
                    ? "Realized Profit check, sale"
                    : "Changed data"}
                </span>
                <span className="text-muted-foreground"> · XTB ID </span>
                <span className="num font-medium">{conflict.xtbId}</span>
              </span>
              {conflict.decision === null ? (
                <Badge
                  variant="outline"
                  className={cn("font-mono", warningBadge)}
                >
                  No decision
                </Badge>
              ) : (
                <Badge
                  variant="outline"
                  className="border-primary/30 bg-primary/10 font-mono text-primary"
                >
                  {decisionLabels[conflict.decision]}
                </Badge>
              )}
            </div>
            {conflict.kind === "check-mismatch" ? (
              <ProfitCheckList
                sale={conflict.new}
                check={conflict.check}
                currency={currency}
              />
            ) : (
              <div className="grid min-w-0 gap-3 md:grid-cols-2">
                <DataPanel
                  title="Stored data"
                  data={conflict.existing}
                  other={conflict.new}
                  currency={currency}
                />
                <DataPanel
                  title="New data"
                  data={conflict.new}
                  other={conflict.existing}
                  currency={currency}
                  isNew
                />
              </div>
            )}
            <div className="flex flex-wrap items-center justify-between gap-2 border-t pt-3">
              <span className="text-sm text-muted-foreground">Decision</span>
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
                  <ToggleGroupItem
                    key={decision}
                    value={decision}
                    className="data-[state=on]:bg-primary/15 data-[state=on]:text-primary"
                  >
                    {decisionLabels[decision]}
                  </ToggleGroupItem>
                ))}
              </ToggleGroup>
            </div>
          </section>
        ))}
      </CardContent>
    </Card>
  )
}

type DataPanelProps = {
  title: string
  data: CashOperationData
  // other is the data of the other side. A field that differs from it is highlighted.
  other: CashOperationData
  currency: string
  // isNew marks the panel of the new data. It shows the "Changed" tag next to each changed field.
  isNew?: boolean
}

// DataPanel shows the data of one side of a changed-data conflict.
function DataPanel({
  title,
  data,
  other,
  currency,
  isNew = false,
}: DataPanelProps) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-2 rounded-md border bg-muted/30 p-3",
        isNew && "border-primary/30"
      )}
    >
      <p
        className={cn(
          "font-mono text-xs tracking-wider uppercase",
          isNew ? "text-primary" : "text-muted-foreground"
        )}
      >
        {title}
      </p>
      <dl className="flex flex-col text-sm">
        {fields.map((field) => {
          const changed = data[field.key] !== other[field.key]
          const value = field.format(data, currency)
          return (
            <div
              key={field.key}
              className="flex min-w-0 items-baseline justify-between gap-3 border-b border-border/60 py-1.5 last:border-b-0"
            >
              <dt className="flex shrink-0 items-center gap-1.5 text-muted-foreground">
                {field.label}
                {changed && isNew && (
                  <span className="font-mono text-[10px] text-warning uppercase">
                    Changed
                  </span>
                )}
              </dt>
              <dd
                className={cn(
                  "min-w-0 text-right break-words",
                  field.numeric && "num",
                  value === none && "text-muted-foreground",
                  changed &&
                    "rounded-sm bg-warning/15 px-1.5 font-medium text-warning"
                )}
              >
                {value}
              </dd>
            </div>
          )
        })}
      </dl>
    </div>
  )
}

type ProfitCheckListProps = {
  sale: CashOperationData
  check: ProfitCheck
  currency: string
}

// ProfitCheckList shows the Realized Profit of a sale next to the Profit/Loss in Closed Positions.
function ProfitCheckList({ sale, check, currency }: ProfitCheckListProps) {
  return (
    <dl className="flex flex-col rounded-md border bg-muted/30 px-3 py-1 text-sm">
      <div className="flex min-w-0 items-baseline justify-between gap-3 border-b border-border/60 py-1.5">
        <dt className="shrink-0 text-muted-foreground">Sale</dt>
        <dd className="min-w-0 text-right break-words">
          {sale.ticker} · {new Date(sale.time).toLocaleString()} ·{" "}
          {sale.comment}
        </dd>
      </div>
      <div className="flex min-w-0 items-baseline justify-between gap-3 border-b border-border/60 py-1.5">
        <dt className="text-muted-foreground">
          Realized Profit from the Ledger
        </dt>
        <dd className="num font-medium text-warning">
          {formatAmount(check.calculated, currency, { signed: true })}
        </dd>
      </div>
      <div className="flex min-w-0 items-baseline justify-between gap-3 py-1.5">
        <dt className="text-muted-foreground">
          Profit/Loss in Closed Positions
        </dt>
        <dd className="num font-medium text-warning">
          {formatAmount(check.closedPositions, currency, {
            signed: true,
          })}
        </dd>
      </div>
    </dl>
  )
}
