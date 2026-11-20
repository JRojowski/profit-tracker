import { ChevronsUpDownIcon, LandmarkIcon, WalletIcon } from "lucide-react"
import { useLocation, useNavigate } from "react-router"

import { AccountBadges } from "@/components/account-badges"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuLabel,
  DropdownMenuRadioGroup,
  DropdownMenuRadioItem,
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
import type { SidebarView } from "@/hooks/use-sidebar-view"
import type { Account } from "@/lib/api"
import { switchViewPath, type ViewTarget } from "@/lib/routes"

type ViewSwitchProps = {
  // accounts is null while the Accounts load.
  accounts: Account[] | null
  // view is the view that the sidebar shows.
  view: SidebarView
}

// ViewSwitch shows the selected view, All Accounts or one Account, and lists each view in a menu. A selection
// opens the same page type in the new view when it exists there, otherwise the overview.
export function ViewSwitch({ accounts, view }: ViewSwitchProps) {
  const { pathname } = useLocation()
  const navigate = useNavigate()
  const { setOpenMobile } = useSidebar()

  // selected is the Account of the view, or null for All Accounts and for an unknown Account.
  const selected = view.state === "view" ? view.account : null
  // selectedValue is the value of the checked menu item. An unknown Account checks no item.
  const selectedValue =
    view.state === "unknown" ? "" : selected ? String(selected.id) : "all"

  // select opens the page of the new view. The menu closes and nothing opens when the view stays the same.
  function select(value: string) {
    setOpenMobile(false)
    if (value === selectedValue) return
    const account = accounts?.find((a) => String(a.id) === value)
    const target: ViewTarget = account
      ? { kind: "account", account }
      : { kind: "all" }
    navigate(switchViewPath(pathname, target))
  }

  return (
    <SidebarMenu>
      <SidebarMenuItem>
        {accounts === null || view.state === "loading" ? (
          <SidebarMenuSkeleton showIcon />
        ) : (
          <DropdownMenu>
            <DropdownMenuTrigger asChild>
              <SidebarMenuButton
                size="lg"
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
                <span className="sr-only">Change the view</span>
                <ChevronsUpDownIcon
                  aria-hidden
                  className="ml-auto text-muted-foreground"
                />
              </SidebarMenuButton>
            </DropdownMenuTrigger>
            <DropdownMenuContent align="start" className="min-w-60">
              <DropdownMenuLabel>View</DropdownMenuLabel>
              <DropdownMenuRadioGroup
                value={selectedValue}
                onValueChange={select}
              >
                <DropdownMenuRadioItem value="all">
                  <LandmarkIcon />
                  <span className="flex-1 truncate">All Accounts</span>
                </DropdownMenuRadioItem>
                <DropdownMenuSeparator />
                {accounts.map((account) => (
                  <DropdownMenuRadioItem
                    key={account.id}
                    value={String(account.id)}
                  >
                    <WalletIcon />
                    <span className="flex-1 truncate">{account.name}</span>
                    <AccountBadges account={account} />
                  </DropdownMenuRadioItem>
                ))}
              </DropdownMenuRadioGroup>
            </DropdownMenuContent>
          </DropdownMenu>
        )}
      </SidebarMenuItem>
    </SidebarMenu>
  )
}
