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
import { ImportPreviewCard } from "@/components/import-preview-card"
import {
  confirmImport,
  errorMessage,
  uploadImport,
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

  async function handleUpload(event: FormEvent) {
    event.preventDefault()
    if (!file) return
    setBusy(true)
    setError(null)
    try {
      setPreview(await uploadImport(file))
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  async function handleConfirm(account?: NewAccount) {
    if (!preview) return
    setBusy(true)
    setError(null)
    try {
      const summary = await confirmImport(preview.id, account)
      reset()
      onImported(summary)
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
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

      {preview && (
        <ImportPreviewCard
          key={preview.id}
          preview={preview}
          busy={busy}
          onConfirm={handleConfirm}
          onCancel={reset}
        />
      )}
    </div>
  )
}
