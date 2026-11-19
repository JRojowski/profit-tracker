import { RotateCcw } from "lucide-react"

import { Button } from "@/components/ui/button"

// ResetColumnsButton gives the default column order and the default sort of a table.
// It is disabled while the view is the default view.
export function ResetColumnsButton({
  isDefault,
  onReset,
}: {
  isDefault: boolean
  onReset: () => void
}) {
  return (
    <Button
      type="button"
      variant="outline"
      size="sm"
      disabled={isDefault}
      onClick={onReset}
    >
      <RotateCcw data-icon="inline-start" />
      Reset columns
    </Button>
  )
}
