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
} from "@/lib/api"
import { formatAmount } from "@/lib/format"
import { cn } from "@/lib/utils"

type ImportConflictsCardProps = {
  conflicts: ImportConflict[]
  // currency is the Account currency. A conflict exists only for a known Account.
  currency: string
  busy: boolean
  onDecide: (conflictId: number, decision: ConflictDecision) => void
}

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
]

const decisionLabels: Record<ConflictDecision, string> = {
  "keep-existing": "Keep existing",
  "use-new": "Use new",
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
          These Cash Operations have a known XTB ID but different data. Select
          the data to keep for each one. You can confirm the Import when each
          conflict has a decision.
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
                XTB ID {conflict.xtbId}
              </span>
              {conflict.decision === null ? (
                <Badge variant="destructive">No decision</Badge>
              ) : (
                <Badge>{decisionLabels[conflict.decision]}</Badge>
              )}
            </div>
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
                  const changed =
                    conflict.existing[field.key] !== conflict.new[field.key]
                  return (
                    <TableRow key={field.key}>
                      <TableCell className="text-muted-foreground">
                        {field.label}
                      </TableCell>
                      <TableCell
                        className={cn(
                          "whitespace-normal",
                          changed && "font-medium"
                        )}
                      >
                        {field.format(conflict.existing, currency)}
                      </TableCell>
                      <TableCell
                        className={cn(
                          "whitespace-normal",
                          changed && "font-medium"
                        )}
                      >
                        {field.format(conflict.new, currency)}
                      </TableCell>
                    </TableRow>
                  )
                })}
              </TableBody>
            </Table>
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
              <ToggleGroupItem value="keep-existing">
                {decisionLabels["keep-existing"]}
              </ToggleGroupItem>
              <ToggleGroupItem value="use-new">
                {decisionLabels["use-new"]}
              </ToggleGroupItem>
            </ToggleGroup>
          </div>
        ))}
      </CardContent>
    </Card>
  )
}
