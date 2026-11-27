import { SectionHeading } from "@/components/section-heading"
import { PlusIcon } from "lucide-react"
import { Link, useNavigate, useSearchParams } from "react-router"

import { AccountBadges } from "@/components/account-badges"
import { ImportCashOperationsPanel } from "@/components/import-cash-operations-panel"
import { LoadedView } from "@/components/loaded-view"
import { StackedHead } from "@/components/stacked-head"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { useLoad } from "@/hooks/use-load"
import { fetchImports, type Account, type ImportSummary } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { formatPolishDate } from "@/lib/format"
import { paths } from "@/lib/routes"
import { cn } from "@/lib/utils"

const dateFormat = new Intl.DateTimeFormat(undefined, { dateStyle: "medium" })
const timeFormat = new Intl.DateTimeFormat(undefined, { timeStyle: "short" })

// ImportsPage shows the confirmed Imports as a table, in the order of the backend.
export function ImportsPage() {
  const { accounts, valuesVersion } = useAppState()
  const imports = useLoad("imports", valuesVersion, (signal) =>
    fetchImports(signal)
  )
  // The query "import" holds the id of the Import whose Cash Operations the panel shows.
  const [searchParams] = useSearchParams()
  const navigate = useNavigate()
  const selectedId = parseImportId(searchParams.get("import"))
  const selected =
    imports.status === "ok"
      ? (imports.data.find((item) => item.id === selectedId) ?? null)
      : null

  return (
    <Card className="min-w-0">
      <CardHeader>
        <SectionHeading
          title={
            <>
              Import history
              {imports.status === "ok" && (
                <Badge variant="secondary" className="num font-mono">
                  {imports.data.length}
                </Badge>
              )}
            </>
          }
          action={
            <Button asChild size="sm">
              <Link to={paths.import}>
                <PlusIcon data-icon="inline-start" />
                New Import
              </Link>
            </Button>
          }
        />
      </CardHeader>
      <CardContent className="min-w-0">
        <LoadedView loaded={imports} errorTitle="Cannot load the Imports">
          {(data) =>
            data.length === 0 ? (
              <p className="text-sm text-muted-foreground">
                There are no Imports yet.
              </p>
            ) : (
              <ImportsTable
                imports={data}
                accounts={accounts ?? []}
                onOpen={(id) => navigate(paths.importCashOperations(id))}
              />
            )
          }
        </LoadedView>
      </CardContent>
      {selected && (
        <ImportCashOperationsPanel
          item={selected}
          currency={
            accounts?.find((account) => account.id === selected.accountId)
              ?.currency ?? ""
          }
          onClose={() => navigate(paths.imports)}
        />
      )}
    </Card>
  )
}

type ImportsTableProps = {
  imports: ImportSummary[]
  // accounts give the type and the currency of the Account of each Import.
  accounts: Account[]
  // onOpen shows the Cash Operations of the Import with this id.
  onOpen: (id: number) => void
}

// ImportsTable shows each Import with its date, file, Account, period and counts of Cash Operations.
// A click on a row, or the file button with the keyboard, opens the Cash Operations of the Import.
// The table has a fixed layout, so a long file name wraps and the table never gets wider than its card.
function ImportsTable({ imports, accounts, onOpen }: ImportsTableProps) {
  const byId = new Map(accounts.map((account) => [account.id, account]))

  return (
    <Table className="table-fixed">
      <TableHeader>
        <TableRow>
          <TableHead className="hidden w-36 md:table-cell">Date</TableHead>
          <TableHead>
            <span className="hidden md:inline">File</span>
            {/* On a narrow screen, one column holds the file, the date and the Account. */}
            <StackedHead labels={["File", "Date", "Period", "Account"]} />
          </TableHead>
          <TableHead className="hidden w-48 md:table-cell">Account</TableHead>
          <TableHead className="hidden w-44 md:table-cell">Period</TableHead>
          <TableHead className="hidden w-28 text-right whitespace-normal md:table-cell">
            New Cash Operations
          </TableHead>
          <TableHead className="hidden w-28 text-right whitespace-normal md:table-cell">
            Known Cash Operations
          </TableHead>
          <TableHead className="w-24 text-right md:hidden">
            <StackedHead labels={["New", "Known"]} alignEnd />
          </TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {imports.map((item) => {
          const importedAt = new Date(item.importedAt)
          const account = byId.get(item.accountId)
          return (
            <TableRow
              key={item.id}
              className="cursor-pointer"
              onClick={() => onOpen(item.id)}
            >
              <TableCell className="hidden md:table-cell">
                <ImportDate time={importedAt} />
              </TableCell>
              <TableCell className="align-top whitespace-normal md:align-middle">
                <div className="flex min-w-0 flex-col gap-1">
                  {/* The button is the keyboard route to the row. A click on it reaches the click handler of the row. */}
                  <button
                    type="button"
                    className="w-fit max-w-full rounded-sm text-left font-mono text-xs break-all underline-offset-4 outline-none hover:underline focus-visible:ring-2 focus-visible:ring-ring"
                    aria-label={`Show the Cash Operations of ${item.fileName}`}
                  >
                    {item.fileName}
                  </button>
                  <span className="flex flex-col gap-1 md:hidden">
                    <ImportDate time={importedAt} />
                    <ImportPeriod item={item} />
                    <ImportAccount item={item} account={account} />
                  </span>
                </div>
              </TableCell>
              <TableCell className="hidden whitespace-normal md:table-cell">
                <ImportAccount item={item} account={account} />
              </TableCell>
              <TableCell className="hidden md:table-cell">
                <ImportPeriod item={item} />
              </TableCell>
              <TableCell className="hidden text-right md:table-cell">
                <NewCount count={item.newCashOperations} />
              </TableCell>
              <TableCell className="hidden text-right num text-muted-foreground md:table-cell">
                {item.skippedCashOperations}
              </TableCell>
              <TableCell className="text-right align-top md:hidden">
                <span className="flex flex-col items-end gap-1">
                  <NewCount count={item.newCashOperations} />
                  <span className="num text-muted-foreground">
                    {item.skippedCashOperations}
                  </span>
                </span>
              </TableCell>
            </TableRow>
          )
        })}
      </TableBody>
    </Table>
  )
}

// parseImportId gives the Import id of the query text, or null when the text is not an id.
function parseImportId(text: string | null): number | null {
  return text !== null && /^[1-9][0-9]*$/.test(text) ? Number(text) : null
}

// ImportPeriod shows the dates, in Polish time, of the first and the last Cash Operation of an Import, or "—" when it added none.
function ImportPeriod({ item }: { item: ImportSummary }) {
  if (item.periodFrom === null || item.periodTo === null) {
    return <span className="text-muted-foreground">—</span>
  }
  return (
    <span className="num text-xs">
      {formatPolishDate(item.periodFrom)} – {formatPolishDate(item.periodTo)}
    </span>
  )
}

// ImportDate shows the date and, in a muted colour, the time of an Import.
function ImportDate({ time }: { time: Date }) {
  return (
    <span className="flex flex-wrap gap-x-1.5 text-xs">
      <span className="num">{dateFormat.format(time)}</span>
      <span className="num text-muted-foreground">
        {timeFormat.format(time)}
      </span>
    </span>
  )
}

type ImportAccountProps = {
  item: ImportSummary
  // account is undefined while the Accounts load.
  account: Account | undefined
}

// ImportAccount shows the Account of an Import with its badges and its account number.
function ImportAccount({ item, account }: ImportAccountProps) {
  return (
    <span className="flex min-w-0 flex-wrap items-center gap-x-2 gap-y-0.5">
      <span className="min-w-0 font-medium break-words">
        {item.accountName}
      </span>
      {account && <AccountBadges account={account} />}
      <span className="num text-xs text-muted-foreground">
        {item.accountNumber}
      </span>
    </span>
  )
}

// NewCount shows the number of new Cash Operations. A number above zero has the positive colour.
function NewCount({ count }: { count: number }) {
  return (
    <span
      className={cn(
        "num",
        count > 0 ? "text-positive" : "text-muted-foreground"
      )}
    >
      {count}
    </span>
  )
}
