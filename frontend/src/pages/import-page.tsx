import { useState, type FormEvent } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Field, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { ImportConflictsCard } from "@/components/import-conflicts-card"
import { ImportPreviewCard } from "@/components/import-preview-card"
import {
  cancelImport,
  confirmImport,
  decideConflict,
  errorMessage,
  uploadImport,
  type ConflictDecision,
  type ImportPreview,
  type ImportSummary,
  type NewAccount,
} from "@/lib/api"

type ImportPageProps = {
  onImported: (summary: ImportSummary) => void
}

export function ImportPage({ onImported }: ImportPageProps) {
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // The input key resets the file input after a cancel or a confirmation.
  const [inputKey, setInputKey] = useState(0)

  function reset() {
    setFile(null)
    setPreview(null)
    setError(null)
    setInputKey((key) => key + 1)
  }

  // run calls an API action. It blocks the page while the action runs and shows the error of a failed action.
  async function run(action: () => Promise<void>) {
    setBusy(true)
    setError(null)
    try {
      await action()
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  async function handleUpload(event: FormEvent) {
    event.preventDefault()
    if (!file) return
    await run(async () => setPreview(await uploadImport(file)))
  }

  async function handleConfirm(account?: NewAccount) {
    if (!preview) return
    await run(async () => {
      const summary = await confirmImport(preview.id, account)
      reset()
      onImported(summary)
    })
  }

  async function handleDecide(conflictId: number, decision: ConflictDecision) {
    if (!preview) return
    await run(async () => {
      const updated = await decideConflict(preview.id, conflictId, decision)
      setPreview({
        ...preview,
        conflicts: preview.conflicts.map((conflict) =>
          conflict.id === updated.id ? updated : conflict
        ),
      })
    })
  }

  async function handleCancel() {
    if (!preview) return
    await run(async () => {
      await cancelImport(preview.id)
      reset()
    })
  }

  return (
    <div className="flex max-w-xl flex-col gap-4">
      <Card>
        <CardHeader>
          <CardTitle>Upload an XTB export</CardTitle>
          <CardDescription>
            Select the xlsx file of one Account. The tool saves nothing before
            you confirm the Import.
          </CardDescription>
        </CardHeader>
        <CardContent>
          <form onSubmit={handleUpload} className="flex items-end gap-2">
            <Field>
              <FieldLabel htmlFor="import-file">Export file</FieldLabel>
              <Input
                key={inputKey}
                id="import-file"
                type="file"
                accept=".xlsx"
                disabled={busy || preview !== null}
                onChange={(event) => setFile(event.target.files?.[0] ?? null)}
              />
            </Field>
            <Button type="submit" disabled={!file || busy || preview !== null}>
              Preview
            </Button>
          </form>
        </CardContent>
      </Card>

      {error && (
        <Alert variant="destructive">
          <AlertTitle>The Import failed</AlertTitle>
          <AlertDescription>{error}</AlertDescription>
        </Alert>
      )}

      {preview && preview.conflicts.length > 0 && (
        <ImportConflictsCard
          conflicts={preview.conflicts}
          currency={preview.account?.currency ?? preview.suggestedCurrency}
          busy={busy}
          onDecide={handleDecide}
        />
      )}

      {preview && (
        <ImportPreviewCard
          key={preview.id}
          preview={preview}
          busy={busy}
          onConfirm={handleConfirm}
          onCancel={handleCancel}
        />
      )}
    </div>
  )
}
