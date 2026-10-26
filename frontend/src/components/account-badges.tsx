import { Badge } from "@/components/ui/badge"
import type { Account } from "@/lib/api"
import { cn } from "@/lib/utils"

type AccountBadgesProps = {
  account: Pick<Account, "type" | "currency">
  className?: string
}

// AccountBadges shows the IKZE tag of an IKZE Account and the badge of the Account currency.
export function AccountBadges({ account, className }: AccountBadgesProps) {
  return (
    <span className={cn("flex shrink-0 items-center gap-1", className)}>
      {account.type === "IKZE" && (
        <Badge
          variant="outline"
          className="h-4 rounded-sm border-positive/30 bg-positive/10 px-1 font-mono text-[10px] text-positive"
        >
          IKZE
        </Badge>
      )}
      <CurrencyBadge currency={account.currency} />
    </span>
  )
}

// CurrencyBadge shows a currency code as a small badge.
export function CurrencyBadge({ currency }: { currency: string }) {
  return (
    <Badge
      variant="outline"
      className="h-4 rounded-sm bg-muted px-1 font-mono text-[10px] text-muted-foreground"
    >
      {currency}
    </Badge>
  )
}
