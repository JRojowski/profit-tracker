import { IkzeLimitCards } from "@/components/ikze-limit-card"
import { LoadedView } from "@/components/loaded-view"
import { useLoad } from "@/hooks/use-load"
import { fetchIkzeLimit } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { useAccount } from "@/pages/account-layout"

// AccountIkzePage shows the IKZE Contributions of an IKZE Account against the IKZE Limit.
export function AccountIkzePage() {
  const account = useAccount()
  const { valuesVersion } = useAppState()
  const limit = useLoad(String(account.id), valuesVersion, (signal) =>
    fetchIkzeLimit(account.id, signal)
  )

  return (
    <LoadedView loaded={limit} errorTitle="Cannot load the IKZE Limit">
      {(data) => <IkzeLimitCards limit={data} />}
    </LoadedView>
  )
}
