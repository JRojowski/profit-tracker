import { useState, type DragEvent, type FormEvent } from "react"
import {
  CircleCheckIcon,
  FileSpreadsheetIcon,
  UploadIcon,
  XIcon,
} from "lucide-react"

import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import { cn } from "@/lib/utils"

type ImportFileDropProps = {
  file: File | null
  // inputKey resets the file input after a cancel or a confirmation.
  inputKey: number
  busy: boolean
  // previewReady is true when the preview of the file is on the screen. Then the file cannot change.
  previewReady: boolean
  onFileChange: (file: File | null) => void
  onPreview: () => void
}

const sizeFormat = new Intl.NumberFormat(undefined, {
  maximumFractionDigits: 0,
})

// ImportFileDrop is the first step of an Import: the user drops an XTB export or selects it, and asks for the preview.
export function ImportFileDrop({
  file,
  inputKey,
  busy,
  previewReady,
  onFileChange,
  onPreview,
}: ImportFileDropProps) {
  const [dragging, setDragging] = useState(false)
  const locked = busy || previewReady

  function handleDragOver(event: DragEvent) {
    event.preventDefault()
    if (!locked) setDragging(true)
  }

  function handleDrop(event: DragEvent) {
    event.preventDefault()
    setDragging(false)
    if (locked) return
    const dropped = event.dataTransfer.files[0]
    if (dropped) onFileChange(dropped)
  }

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (file && !locked) onPreview()
  }

  return (
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>Broker export</CardTitle>
      </CardHeader>
      <CardContent>
        <form onSubmit={handleSubmit} className="flex flex-col gap-4">
          <label
            htmlFor="import-file"
            onDragOver={handleDragOver}
            onDragLeave={() => setDragging(false)}
            onDrop={handleDrop}
            className={cn(
              "flex flex-col items-center gap-2 rounded-lg border-2 border-dashed px-4 py-8 text-center transition-colors focus-within:border-ring",
              locked
                ? "cursor-not-allowed opacity-60"
                : "cursor-pointer hover:border-primary/50 hover:bg-primary/5",
              dragging && "border-primary bg-primary/10"
            )}
          >
            <span className="flex size-10 items-center justify-center rounded-full bg-muted text-muted-foreground">
              <UploadIcon className="size-5" />
            </span>
            <span className="text-sm font-medium">
              Drop an XTB export (<span className="font-mono">.xlsx</span>) here
              or choose a file
            </span>
            <span className="text-xs text-muted-foreground">
              The export of one Account. The tool saves nothing before you
              confirm the Import.
            </span>
            <input
              key={inputKey}
              id="import-file"
              type="file"
              accept=".xlsx"
              className="sr-only"
              disabled={locked}
              onChange={(event) =>
                onFileChange(event.target.files?.[0] ?? null)
              }
            />
          </label>

          {file && (
            <div className="flex min-w-0 flex-wrap items-center gap-3 rounded-lg border bg-muted/40 p-3">
              {previewReady ? (
                <CircleCheckIcon className="size-5 shrink-0 text-positive" />
              ) : (
                <FileSpreadsheetIcon className="size-5 shrink-0 text-muted-foreground" />
              )}
              <div className="flex min-w-0 flex-1 flex-col gap-0.5">
                <span className="font-mono text-sm break-all">{file.name}</span>
                <span className="num text-xs text-muted-foreground">
                  {sizeFormat.format(Math.max(1, file.size / 1024))} KB
                </span>
              </div>
              {previewReady ? (
                <Badge
                  variant="outline"
                  className="border-positive/30 bg-positive/10 text-positive"
                >
                  Preview ready
                </Badge>
              ) : (
                <div className="flex items-center gap-2">
                  <Button type="submit" disabled={busy}>
                    Preview
                  </Button>
                  <Button
                    type="button"
                    variant="ghost"
                    size="icon"
                    aria-label="Remove the file"
                    disabled={busy}
                    onClick={() => onFileChange(null)}
                  >
                    <XIcon />
                  </Button>
                </div>
              )}
            </div>
          )}
        </form>
      </CardContent>
    </Card>
  )
}
