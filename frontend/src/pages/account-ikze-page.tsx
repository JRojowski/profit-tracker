import { IkzeLimitCard } from "@/components/ikze-limit-card"
import { useAccount } from "@/pages/account-layout"

// AccountIkzePage shows the IKZE Contributions of an IKZE Account against the IKZE Limit.
export function AccountIkzePage() {
  const account = useAccount()
  return <IkzeLimitCard accountId={account.id} />
}
