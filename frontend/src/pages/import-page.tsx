import { useState } from "react"
import { CircleAlertIcon, UploadIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { ImportConflictsCard } from "@/components/import-conflicts-card"
import { ImportFileDrop } from "@/components/import-file-drop"
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

// confirmFormId is the id of the form of the preview. The Confirm button submits it.
const confirmFormId = "import-confirm"

type ImportPageProps = {
  onImported: (summary: ImportSummary) => void
}

// ImportPage leads the user through an Import: select a file, read the preview, decide each Import Conflict,
// and confirm or cancel. The tool saves nothing before the confirmation.
export function ImportPage({ onImported }: ImportPageProps) {
  const [file, setFile] = useState<File | null>(null)
  const [preview, setPreview] = useState<ImportPreview | null>(null)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  // The input key resets the file input after a cancel, a confirmation or a removed file.
  const [inputKey, setInputKey] = useState(0)

  const hasOpenConflicts =
    preview?.conflicts.some((conflict) => conflict.decision === null) ?? false

  function reset() {
    setFile(null)
    setPreview(null)
    setError(null)
    setInputKey((key) => key + 1)
  }

  function handleFileChange(next: File | null) {
    setFile(next)
    setError(null)
    if (next === null) setInputKey((key) => key + 1)
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

  async function handlePreview() {
    if (!file) return
    await run(async () => setPreview(await uploadImport(file)))
  }

  async function handleConfirm(account?: NewAccount) {
    // The Confirm button is disabled while a conflict has no decision. This check also stops the Enter key.
    if (!preview || hasOpenConflicts) return
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
    <div className="flex min-w-0 flex-col gap-6">
      <ImportFileDrop
        file={file}
        inputKey={inputKey}
        busy={busy}
        previewReady={preview !== null}
        onFileChange={handleFileChange}
        onPreview={handlePreview}
      />

      {error && (
        <Alert variant="destructive">
          <AlertTitle>The Import failed</AlertTitle>
          <AlertDescription className="break-words">{error}</AlertDescription>
        </Alert>
      )}

      {preview && (
        <ImportPreviewCard
          key={preview.id}
          preview={preview}
          formId={confirmFormId}
          busy={busy}
          onConfirm={handleConfirm}
        />
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
        <div className="flex flex-col gap-3 border-t pt-4 sm:flex-row sm:items-center sm:justify-between">
          {hasOpenConflicts ? (
            <p className="flex items-center gap-2 text-sm text-warning">
              <CircleAlertIcon className="size-4 shrink-0" />
              Decide each Import Conflict first.
            </p>
          ) : (
            <p className="text-sm text-muted-foreground">
              The tool saves the Import when you confirm it.
            </p>
          )}
          <div className="flex justify-end gap-2">
            <Button
              type="button"
              variant="outline"
              disabled={busy}
              onClick={handleCancel}
            >
              Cancel
            </Button>
            <Button
              type="submit"
              form={confirmFormId}
              disabled={busy || hasOpenConflicts}
            >
              <UploadIcon data-icon="inline-start" />
              Confirm Import
            </Button>
          </div>
        </div>
      )}
    </div>
  )
}
