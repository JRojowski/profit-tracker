import { createBrowserRouter, RouterProvider, useNavigate } from "react-router"

import { AppShell } from "@/components/app-shell"
import { useAppState } from "@/lib/app-state"
import { accountPattern, paths } from "@/lib/routes"
import { AccountHoldingsPage } from "@/pages/account-holdings-page"
import { AccountIkzePage } from "@/pages/account-ikze-page"
import { AccountLayout } from "@/pages/account-layout"
import { AccountOverviewPage } from "@/pages/account-overview-page"
import { AccountSalesPage } from "@/pages/account-sales-page"
import { AllAccountsPage } from "@/pages/all-accounts-page"
import { ImportPage } from "@/pages/import-page"
import { ImportsPage } from "@/pages/imports-page"
import { NotFoundPage } from "@/pages/not-found-page"

// The backend serves index.html for each of these paths, so a reload opens the same view.
const router = createBrowserRouter([
  {
    element: <AppShell />,
    children: [
      { path: paths.root, element: <AllAccountsPage /> },
      { path: paths.allAccounts, element: <AllAccountsPage /> },
      {
        path: accountPattern,
        element: <AccountLayout />,
        children: [
          { index: true, element: <AccountOverviewPage /> },
          { path: "holdings", element: <AccountHoldingsPage /> },
          { path: "sales", element: <AccountSalesPage /> },
          { path: "ikze", element: <AccountIkzePage /> },
        ],
      },
      { path: paths.import, element: <ImportRoute /> },
      { path: paths.imports, element: <ImportsPage /> },
      { path: "*", element: <NotFoundPage /> },
    ],
  },
])

// ImportRoute opens the Imports after a confirmed Import. A new Account shows in the sidebar at once.
function ImportRoute() {
  const navigate = useNavigate()
  const { reloadAccounts } = useAppState()
  return (
    <ImportPage
      onImported={() => {
        reloadAccounts()
        void navigate(paths.imports)
      }}
    />
  )
}

// App shows the view of the URL.
export function App() {
  return <RouterProvider router={router} />
}

export default App
