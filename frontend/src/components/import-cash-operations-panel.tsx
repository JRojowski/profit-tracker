import { LoadedView } from "@/components/loaded-view"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useLoad } from "@/hooks/use-load"
import {
  fetchImportCashOperations,
  type ImportCashOperation,
  type ImportSummary,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import {
  amountSign,
  formatAmount,
  formatPolishDate,
  formatPolishTime,
  formatPrice,
  formatVolume,
  signColour,
} from "@/lib/format"
import { cn } from "@/lib/utils"

type ImportCashOperationsPanelProps = {
  item: ImportSummary
  // currency is the currency of the Account of the Import. It is empty when the tool does not know it yet.
  currency: string
  onClose: () => void
}

// ImportCashOperationsPanel lists the Cash Operations that one Import added, in a panel at the right side.
// It is open for as long as it is mounted. The Import history mounts it for the Import in the URL.
export function ImportCashOperationsPanel({
  item,
  currency,
  onClose,
}: ImportCashOperationsPanelProps) {
  const { valuesVersion } = useAppState()
  const loaded = useLoad(`import:${item.id}`, valuesVersion, (signal) =>
    fetchImportCashOperations(item.id, signal)
  )

  return (
    <Sheet
      open
      onOpenChange={(open) => {
        if (!open) onClose()
      }}
    >
      <SheetContent className="data-[side=right]:w-full data-[side=right]:sm:max-w-3xl">
        <SheetHeader className="gap-1.5 border-b pr-12">
          <SheetTitle>Cash Operations of the Import</SheetTitle>
          <SheetDescription className="font-mono text-xs break-all">
            {item.fileName}
          </SheetDescription>
        </SheetHeader>
        <div className="min-h-0 flex-1 overflow-y-auto px-4 pb-4">
          <LoadedView
            loaded={loaded}
            errorTitle="Cannot load the Cash Operations"
          >
            {(data) =>
              data.length === 0 ? (
                <div className="flex flex-col gap-1 py-6 text-sm text-muted-foreground">
                  <p className="font-medium text-foreground">
                    No new Cash Operations
                  </p>
                  <p>This Import added no Cash Operation.</p>
                </div>
              ) : (
                <OperationsTable operations={data} currency={currency} />
              )
            }
          </LoadedView>
        </div>
      </SheetContent>
    </Sheet>
  )
}

// OperationsTable shows each Cash Operation with its time, type, Instrument, volume, price, amount and comment.
// The time is in Polish time, as on the other pages. On a narrow screen the table scrolls inside the panel.
function OperationsTable({
  operations,
  currency,
}: {
  operations: ImportCashOperation[]
  currency: string
}) {
  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Time</TableHead>
          <TableHead>Type</TableHead>
          <TableHead>Ticker</TableHead>
          <TableHead className="text-right">Volume</TableHead>
          <TableHead className="text-right">Price</TableHead>
          <TableHead className="text-right">Amount</TableHead>
          <TableHead>Comment</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {operations.map((operation) => (
          <TableRow key={operation.xtbId}>
            <TableCell>
              <span className="flex flex-wrap gap-x-1.5 text-xs">
                <span className="num">{formatPolishDate(operation.time)}</span>
                <span className="num text-muted-foreground">
                  {formatPolishTime(operation.time)}
                </span>
              </span>
            </TableCell>
            <TableCell>{operation.type}</TableCell>
            <TableCell className="num">{operation.ticker || "—"}</TableCell>
            <TableCell className="text-right num">
              {operation.volume === null ? "—" : formatVolume(operation.volume)}
            </TableCell>
            <TableCell className="text-right num">
              {operation.price === null ? "—" : formatPrice(operation.price)}
            </TableCell>
            <TableCell
              className={cn(
                "text-right num",
                signColour(amountSign(operation.amount))
              )}
            >
              {formatAmount(operation.amount, currency, { signed: true })}
            </TableCell>
            <TableCell className="max-w-48 text-xs break-words whitespace-normal text-muted-foreground">
              {operation.comment || "—"}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
