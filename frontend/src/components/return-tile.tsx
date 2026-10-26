import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import type { Returns } from "@/lib/api"
import { formatPercent } from "@/lib/format"

type ReturnTileProps = {
  returns: Returns
}

// ReturnTile shows the Simple Return and the Annual Return. A return that is not available shows "—" and its note.
export function ReturnTile({ returns }: ReturnTileProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Return</CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <ReturnLine
          label="Simple Return"
          percent={returns.simpleReturn}
          note={returns.simpleReturnNote}
        />
        <ReturnLine
          label="Annual Return"
          percent={returns.annualReturn}
          note={returns.annualReturnNote}
        />
      </CardContent>
    </Card>
  )
}

type ReturnLineProps = {
  label: string
  percent: string | null
  note: string | null
}

function ReturnLine({ label, percent, note }: ReturnLineProps) {
  return (
    <div className="flex flex-col gap-1">
      <p className="text-sm text-muted-foreground">{label}</p>
      <p className="text-xl font-semibold tabular-nums">
        {percent === null ? "—" : formatPercent(percent)}
      </p>
      {percent === null && note && (
        <p className="text-sm text-muted-foreground">{note}</p>
      )}
    </div>
  )
}
