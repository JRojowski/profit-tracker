import {
  CheckIcon,
  ChevronsUpDownIcon,
  LandmarkIcon,
  WalletIcon,
} from "lucide-react"
import { useLocation, useNavigate } from "react-router"

import { AccountBadges } from "@/components/account-badges"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuItem,
  DropdownMenuLabel,
  DropdownMenuSeparator,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import {
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  useSidebar,
} from "@/components/ui/sidebar"
import type { Account } from "@/lib/api"
import { switchViewPath, type ViewTarget } from "@/lib/routes"

type ViewSwitchProps = {
  // accounts is null while the Accounts load.
  accounts: Account[] | null
  // selected is the Account of the open view, or null for All Accounts.
  selected: Account | null
}

// ViewSwitch shows the selected view, All Accounts or one Account, and lists each view in a menu. A selection
// opens the same page type in the new view when it exists there, otherwise the overview.
export function ViewSwitch({ accounts, selected }: ViewSwitchProps) {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { setOpenMobile } = useSidebar()

  function select(target: ViewTarget) {
    setOpenMobile(false)
    navigate(switchViewPath(pathname, target))
  }

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        {accounts === null ? (
          <SidebarMenuSkeleton showIcon />
        ) : (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <SidebarMenuButton
                size="lg"
                aria-label="Change the view"
                tooltip={selected?.name ?? "All Accounts"}
                className="data-open:bg-sidebar-accent"
              >
                <span className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-primary/30 bg-primary/15 text-primary group-data-[collapsible=icon]:size-6">
                  {selected ? <WalletIcon /> : <LandmarkIcon />}
                </span>
                <span className="flex min-w-0 flex-1 flex-col gap-0.5 text-left">
                  <span className="truncate text-sm leading-none font-medium text-foreground">
                    {selected?.name ?? "All Accounts"}
                  </span>
                  {selected && (
                    <AccountBadges account={selected} className="self-start" />
                  )}
                </span>
                <ChevronsUpDownIcon className="ml-auto text-muted-foreground" />
              </SidebarMenuButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="min-w-60">
              <DropdownMenuLabel>View</DropdownMenuLabel>
              <DropdownMenuItem onSelect={() => select({ kind: "all" })}>
                <LandmarkIcon />
                <span className="flex-1 truncate">All Accounts</span>
                {selected === null && <CheckIcon />}
              </DropdownMenuItem>
              <DropdownMenuSeparator />
              {accounts.map((account) => (
                <DropdownMenuItem
                  key={account.id}
                  onSelect={() => select({ kind: "account", account })}
                >
                  <WalletIcon />
                  <span className="flex-1 truncate">{account.name}</span>
                  <AccountBadges account={account} />
                  {selected?.id === account.id && <CheckIcon />}
                </DropdownMenuItem>
              ))}
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
