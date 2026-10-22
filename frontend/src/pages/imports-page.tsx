import { useEffect, useState } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from "@/components/ui/table"
import { Skeleton } from "@/components/ui/skeleton"
import { errorMessage, fetchImports, type ImportSummary } from "@/lib/api"

const dateFormat = new Intl.DateTimeFormat(undefined, {
  dateStyle: "medium",
  timeStyle: "short",
})

export function ImportsPage() {
  const [imports, setImports] = useState<ImportSummary[] | null>(null)
  const [error, setError] = useState<string | null>(null)

  useEffect(() => {
    const controller = new AbortController()
    fetchImports(controller.signal)
      .then(setImports)
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setError(errorMessage(err))
      })
    return () => controller.abort()
  }, [])

  if (error) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Cannot load the Imports</AlertTitle>
        <AlertDescription>{error}</AlertDescription>
      </Alert>
    )
  }

  if (imports === null) {
    return (
      <div className="flex flex-col gap-2">
        <Skeleton className="h-8 w-full" />
        <Skeleton className="h-8 w-full" />
      </div>
    )
  }

  if (imports.length === 0) {
    return (
      <p className="text-sm text-muted-foreground">There are no Imports yet.</p>
    )
  }

  return (
    <Table>
      <TableHeader>
        <TableRow>
          <TableHead>Date</TableHead>
          <TableHead>File</TableHead>
          <TableHead>Account</TableHead>
          <TableHead className="text-right">New</TableHead>
          <TableHead className="text-right">Known</TableHead>
        </TableRow>
      </TableHeader>
      <TableBody>
        {imports.map((item) => (
          <TableRow key={item.id}>
            <TableCell>
              {dateFormat.format(new Date(item.importedAt))}
            </TableCell>
            <TableCell>{item.fileName}</TableCell>
            <TableCell>
              {item.accountName}{" "}
              <span className="text-muted-foreground">
                ({item.accountNumber})
              </span>
            </TableCell>
            <TableCell className="text-right">
              {item.newCashOperations}
            </TableCell>
            <TableCell className="text-right">
              {item.skippedCashOperations}
            </TableCell>
          </TableRow>
        ))}
      </TableBody>
    </Table>
  )
}
