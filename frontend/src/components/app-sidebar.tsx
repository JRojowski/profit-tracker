import {
  ChartLineIcon,
  HistoryIcon,
  LandmarkIcon,
  UploadIcon,
  WalletIcon,
  type LucideIcon,
} from "lucide-react"
import type { ReactNode } from "react"
import { Link, useLocation } from "react-router"

import { AccountBadges } from "@/components/account-badges"
import {
  Sidebar,
  SidebarContent,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarMenuSkeleton,
  SidebarSeparator,
  useSidebar,
} from "@/components/ui/sidebar"
import { useAppState } from "@/lib/app-state"
import { paths } from "@/lib/routes"

// AppSidebar shows "All Accounts", each Account, and the Import pages. On a narrow screen it is a sheet.
export function AppSidebar() {
  const { accounts } = useAppState()
  const { pathname } = useLocation()

  const allAccountsActive =
    pathname === paths.root || pathname === paths.allAccounts

  return (
    <Sidebar>
      <SidebarHeader>
        <div className="flex items-center gap-3 px-2 py-1.5">
          <div className="flex size-8 items-center justify-center rounded-lg border border-primary/30 bg-primary/15 text-primary">
            <ChartLineIcon className="size-4" />
          </div>
          <span className="text-sm font-semibold tracking-tight text-foreground">
            Profit Tracker
          </span>
        </div>
      </SidebarHeader>
      <SidebarContent>
        <SidebarGroup>
          <SidebarGroupLabel className="font-mono tracking-wider uppercase">
            Accounts
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              <NavItem
                to={paths.allAccounts}
                icon={LandmarkIcon}
                label="All Accounts"
                active={allAccountsActive}
              />
              {accounts === null ? (
                <SidebarMenuItem>
                  <SidebarMenuSkeleton />
                </SidebarMenuItem>
              ) : (
                accounts.map((account) => (
                  <NavItem
                    key={account.id}
                    to={paths.account(account.id)}
                    icon={WalletIcon}
                    label={account.name}
                    active={isUnder(pathname, paths.account(account.id))}
                  >
                    <AccountBadges account={account} className="ml-auto" />
                  </NavItem>
                ))
              )}
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
        <SidebarSeparator />
        <SidebarGroup>
          <SidebarGroupLabel className="font-mono tracking-wider uppercase">
            Data
          </SidebarGroupLabel>
          <SidebarGroupContent>
            <SidebarMenu>
              <NavItem
                to={paths.import}
                icon={UploadIcon}
                label="Import"
                active={pathname === paths.import}
              />
              <NavItem
                to={paths.imports}
                icon={HistoryIcon}
                label="Imports"
                active={pathname === paths.imports}
              />
            </SidebarMenu>
          </SidebarGroupContent>
        </SidebarGroup>
      </SidebarContent>
    </Sidebar>
  )
}

type NavItemProps = {
  to: string
  icon: LucideIcon
  label: string
  active: boolean
  // children is shown at the end of the item, for example the badges of an Account.
  children?: ReactNode
}

function NavItem({ to, icon: Icon, label, active, children }: NavItemProps) {
  const { setOpenMobile } = useSidebar()
  return (
    <SidebarMenuItem>
      <SidebarMenuButton
        asChild
        isActive={active}
        className="data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground data-active:ring-1 data-active:ring-sidebar-primary/25"
      >
        <Link
          to={to}
          aria-current={active ? "page" : undefined}
          onClick={() => setOpenMobile(false)}
        >
          <Icon />
          <span className="truncate">{label}</span>
          {children}
        </Link>
      </SidebarMenuButton>
    </SidebarMenuItem>
  )
}

// isUnder tells that the path is the base path or a page under it.
function isUnder(pathname: string, base: string): boolean {
  return pathname === base || pathname.startsWith(`${base}/`)
}
