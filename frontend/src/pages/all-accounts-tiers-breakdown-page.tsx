import { PencilIcon, TriangleAlertIcon } from "lucide-react"
import { Link } from "react-router"

import { LoadedView } from "@/components/loaded-view"
import { SectionHeading } from "@/components/section-heading"
import { TiersBreakdownTable } from "@/components/tiers-breakdown-table"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader } from "@/components/ui/card"
import { Skeleton } from "@/components/ui/skeleton"
import { useLoad } from "@/hooks/use-load"
import { fetchAllAccountsTiersBreakdown } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { paths } from "@/lib/routes"

// AllAccountsTiersBreakdownPage shows the result of each Instrument of all Tier Accounts together, grouped by Tier, in PLN.
// When an Exchange Rate is missing, it shows the warnings in place of the table.
export function AllAccountsTiersBreakdownPage() {
  const { accounts, accountsError, valuesVersion } = useAppState()
  const breakdown = useLoad(
    "all",
    valuesVersion,
    fetchAllAccountsTiersBreakdown
  )

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
              description="The result of each Instrument of all Tier Accounts together, grouped by Tier, in PLN."
            />
          </CardHeader>
          <CardContent className="flex min-w-0 flex-col gap-4">
            {data.available ? (
              <TiersBreakdownTable breakdown={data} view="all" />
            ) : (
              <>
                <p className="text-sm text-muted-foreground">
                  The tool cannot convert the amounts of all Accounts to PLN.
                </p>
                {data.warnings.length > 0 && (
                  <Alert className="border-warning/40 bg-warning/10 *:[svg]:text-warning">
                    <TriangleAlertIcon />
                    <AlertTitle>
                      The tool cannot show the Tiers Breakdown
                    </AlertTitle>
                    <AlertDescription>
                      <ul className="list-inside list-disc">
                        {data.warnings.map((warning) => (
                          <li key={warning}>{warning}</li>
                        ))}
                      </ul>
                    </AlertDescription>
                  </Alert>
                )}
              </>
            )}
          </CardContent>
        </Card>
      )}
    </LoadedView>
  )
}
