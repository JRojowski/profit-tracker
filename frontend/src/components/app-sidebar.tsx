import {
  BriefcaseIcon,
  ChartLineIcon,
  HistoryIcon,
  LayoutDashboardIcon,
  MessageSquareIcon,
  PiggyBankIcon,
  ReceiptIcon,
  ScaleIcon,
  TargetIcon,
  UploadIcon,
  type LucideIcon,
} from "lucide-react"
import { Link, useLocation } from "react-router"

import { ViewSwitch } from "@/components/view-switch"
import {
  Sidebar,
  SidebarContent,
  SidebarFooter,
  SidebarGroup,
  SidebarGroupContent,
  SidebarGroupLabel,
  SidebarHeader,
  SidebarMenu,
  SidebarMenuButton,
  SidebarMenuItem,
  SidebarSeparator,
  useSidebar,
} from "@/components/ui/sidebar"
import { useSidebarView } from "@/hooks/use-sidebar-view"
import { useAppState } from "@/lib/app-state"
import { useAssistant } from "@/lib/assistant-state"
import { isKraken } from "@/lib/provider"
import { paths } from "@/lib/routes"

type NavEntry = { to: string; icon: LucideIcon; label: string }

// AppSidebar shows the view switch, the pages of the selected view, the Import pages and, when the Assistant is on,
// a button for the Assistant. It collapses to icons. On a narrow screen it is a sheet behind the trigger of the top bar.
export function AppSidebar() {
  const { accounts } = useAppState()
  const { pathname } = useLocation()
  const assistant = useAssistant()
  const { setOpenMobile } = useSidebar()
  const view = useSidebarView(accounts)
  const account = view.state === "view" ? view.account : null

  const viewPages: NavEntry[] = account
    ? [
        {
          to: paths.account(account.id),
          icon: LayoutDashboardIcon,
          label: "Overview",
        },
        {
          to: paths.holdings(account.id),
          icon: BriefcaseIcon,
          label: "Holdings",
        },
        // An Account with no Lots, as at Kraken, has no Closed Positions. The page is not in the sidebar.
        ...(isKraken(account.provider)
          ? []
          : [
              {
                to: paths.closedPositions(account.id),
                icon: ReceiptIcon,
                label: "Closed Positions",
              },
            ]),
        {
          to: paths.benchmarks(account.id),
          icon: ScaleIcon,
          label: "Benchmarks",
        },
        ...(account.type === "IKZE"
          ? [{ to: paths.ikze(account.id), icon: PiggyBankIcon, label: "IKZE" }]
          : []),
      ]
    : [
        {
          to: paths.allAccounts,
          icon: LayoutDashboardIcon,
          label: "Overview",
        },
        {
          to: paths.allBenchmarks,
          icon: ScaleIcon,
          label: "Benchmarks",
        },
        {
          to: paths.rebalancing,
          icon: TargetIcon,
          label: "Rebalancing",
        },
      ]
  const importPages: NavEntry[] = [
    { to: paths.import, icon: UploadIcon, label: "Import" },
    { to: paths.imports, icon: HistoryIcon, label: "Import history" },
  ]
  // The root URL shows All Accounts, and a trailing slash opens the same page.
  const current =
    pathname === paths.root ? paths.allAccounts : trimSlash(pathname)

  return (
    <Sidebar collapsible="icon">
      <SidebarHeader>
        <div className="flex items-center gap-3 px-2 py-1.5 group-data-[collapsible=icon]:hidden">
          <div className="flex size-8 items-center justify-center rounded-lg border border-primary/30 bg-primary/15 text-primary">
            <ChartLineIcon className="size-4" />
          </div>
          <span className="text-sm font-semibold tracking-tight text-foreground">
            Profit Tracker
          </span>
        </div>
        <ViewSwitch accounts={accounts} view={view} />
      </SidebarHeader>
      <SidebarContent>
        {view.state !== "loading" && (
          <>
            <NavGroup label="Pages" entries={viewPages} current={current} />
            <SidebarSeparator />
          </>
        )}
        <NavGroup label="Data" entries={importPages} current={current} />
      </SidebarContent>
      {assistant.available && (
        <SidebarFooter>
          <SidebarMenu>
            <SidebarMenuItem>
              <SidebarMenuButton
                tooltip="Assistant"
                onClick={() => {
                  setOpenMobile(false)
                  assistant.setOpen(true)
                }}
              >
                <MessageSquareIcon />
                <span className="truncate">Assistant</span>
              </SidebarMenuButton>
            </SidebarMenuItem>
          </SidebarMenu>
        </SidebarFooter>
      )}
    </Sidebar>
  )
}

type NavGroupProps = { label: string; entries: NavEntry[]; current: string }

// NavGroup shows a labeled group of navigation items. The item of the open page is marked as current.
function NavGroup({ label, entries, current }: NavGroupProps) {
  const { setOpenMobile } = useSidebar()
  return (
    <SidebarGroup>
      <SidebarGroupLabel className="font-mono tracking-wider uppercase">
        {label}
      </SidebarGroupLabel>
      <SidebarGroupContent>
        <SidebarMenu>
          {entries.map(({ to, icon: Icon, label: text }) => {
            const active = current === to
            return (
              <SidebarMenuItem key={to}>
                <SidebarMenuButton
                  asChild
                  isActive={active}
                  tooltip={text}
                  className="data-active:bg-sidebar-accent data-active:text-sidebar-accent-foreground data-active:ring-1 data-active:ring-sidebar-primary/25"
                >
                  <Link
                    to={to}
                    aria-current={active ? "page" : undefined}
                    onClick={() => setOpenMobile(false)}
                  >
                    <Icon />
                    <span className="truncate">{text}</span>
                  </Link>
                </SidebarMenuButton>
              </SidebarMenuItem>
            )
          })}
        </SidebarMenu>
      </SidebarGroupContent>
    </SidebarGroup>
  )
}

// trimSlash removes the trailing slashes of a path.
function trimSlash(path: string): string {
  return path.replace(/\/+$/, "")
}
