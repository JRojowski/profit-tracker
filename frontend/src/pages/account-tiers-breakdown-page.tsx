import { PencilIcon } from "lucide-react"
import { Link } from "react-router"

import { LoadedView } from "@/components/loaded-view"
import { SectionHeading } from "@/components/section-heading"
import { TiersBreakdownTable } from "@/components/tiers-breakdown-table"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { useLoad } from "@/hooks/use-load"
import { fetchTiersBreakdown } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { paths } from "@/lib/routes"
import { useAccount } from "@/pages/account-layout"

// AccountTiersBreakdownPage shows the result of each Instrument of a Tier Account, grouped by Tier.
export function AccountTiersBreakdownPage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const breakdown = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchTiersBreakdown(account.id, signal)
  )

  return (
    <LoadedView loaded={breakdown} errorTitle="Cannot load the Tiers Breakdown">
      {(data) => (
        <Card className="min-w-0">
          <CardHeader>
            <SectionHeading
              action={
                <Button variant="outline" asChild>
                  <Link to={`${paths.rebalancing}?edit=1`}>
                    <PencilIcon data-icon="inline-start" />
                    Edit targets
                  </Link>
                </Button>
              }
              title="Tiers Breakdown"
              description="The result of each Instrument, grouped by Tier, in the currency of the Account."
            />
          </CardHeader>
          <CardContent className="min-w-0">
            <TiersBreakdownTable breakdown={data} view={String(account.id)} />
          </CardContent>
        </Card>
      )}
    </LoadedView>
  )
}
