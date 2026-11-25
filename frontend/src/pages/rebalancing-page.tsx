import { PencilIcon } from "lucide-react"
import { useState } from "react"
import { useSearchParams } from "react-router"

import { LoadedView } from "@/components/loaded-view"
import { SectionHeading } from "@/components/section-heading"
import { TiersEditor } from "@/components/tiers-editor"
import { RebalancingView } from "@/components/rebalancing-view"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Skeleton } from "@/components/ui/skeleton"
import { useLoad } from "@/hooks/use-load"
import { fetchTiers, type Tiers } from "@/lib/api"
import { useAppState } from "@/lib/app-state"

// RebalancingPage shows the Rebalancing table of the view of all Accounts and the "Edit targets" mode.
export function RebalancingPage() {
  const { accounts, accountsError, valuesVersion } = useAppState()
  const loaded = useLoad("tiers", valuesVersion, fetchTiers)
  const [searchParams] = useSearchParams()
  // The link "?edit=1" opens the "Edit targets" mode.
  const [editing, setEditing] = useState(searchParams.get("edit") === "1")
  // saved is the configuration of the last save. It shows until the next load.
  const [saved, setSaved] = useState<Tiers | null>(null)

  if (accountsError) {
    return (
      <Alert variant="destructive">
        <AlertTitle>Cannot load the Accounts</AlertTitle>
        <AlertDescription>{accountsError}</AlertDescription>
      </Alert>
    )
  }
  if (accounts === null) return <Skeleton className="h-24 w-full" />

  return (
    <LoadedView loaded={loaded} errorTitle="Cannot load the Tiers">
      {(loadedTiers) => {
        const tiers = saved ?? loadedTiers
        return (
          <div className="flex min-w-0 flex-col gap-4">
            <SectionHeading
              label="Rebalancing"
              title={editing ? "Edit targets" : "Rebalancing table"}
              description={
                editing
                  ? "The tool saves all Tiers in one step when you save the targets."
                  : "Each part shows how far it is from its Target Share and what to buy with the new cash."
              }
              action={
                !editing && (
                  <Button variant="outline" onClick={() => setEditing(true)}>
                    <PencilIcon data-icon="inline-start" />
                    Edit targets
                  </Button>
                )
              }
            />
            {editing ? (
              <TiersEditor
                saved={tiers}
                onSaved={(next) => {
                  setSaved(next)
                  setEditing(false)
                }}
                onCancel={() => setEditing(false)}
              />
            ) : tiers.tiers.length === 0 ? (
              <div className="flex flex-col items-start gap-3">
                <p className="text-sm text-muted-foreground">
                  There are no Tiers yet. Create Tiers and give each a Target
                  Share to start Rebalancing.
                </p>
                <Button onClick={() => setEditing(true)}>
                  Edit targets to create the first Tier
                </Button>
              </div>
            ) : (
              <RebalancingView />
            )}
          </div>
        )
      }}
    </LoadedView>
  )
}
