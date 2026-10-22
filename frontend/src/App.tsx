import { useEffect, useState } from "react"

import { Badge } from "@/components/ui/badge"
import { Tabs, TabsContent, TabsList, TabsTrigger } from "@/components/ui/tabs"
import { fetchHealth } from "@/lib/api"
import { ImportPage } from "@/pages/import-page"
import { ImportsPage } from "@/pages/imports-page"

type BackendState = "checking" | "ok" | "unavailable"

type Page = "import" | "imports"

export function App() {
  const [backend, setBackend] = useState<BackendState>("checking")
  const [page, setPage] = useState<Page>("import")

  useEffect(() => {
    const controller = new AbortController()
    fetchHealth(controller.signal)
      .then(() => setBackend("ok"))
      .catch(() => {
        if (!controller.signal.aborted) setBackend("unavailable")
      })
    return () => controller.abort()
  }, [])

  return (
    <div className="flex min-h-svh flex-col">
      <header className="flex items-center justify-between border-b px-6 py-4">
        <h1 className="text-lg font-medium">Profit Tracker</h1>
        <Badge
          variant={backend === "unavailable" ? "destructive" : "secondary"}
        >
          Backend: {backend}
        </Badge>
      </header>
      <main className="flex flex-1 flex-col gap-4 p-6">
        <Tabs value={page} onValueChange={(value) => setPage(value as Page)}>
          <TabsList>
            <TabsTrigger value="import">Import</TabsTrigger>
            <TabsTrigger value="imports">Imports</TabsTrigger>
          </TabsList>
          <TabsContent value="import">
            <ImportPage onImported={() => setPage("imports")} />
          </TabsContent>
          <TabsContent value="imports">
            <ImportsPage />
          </TabsContent>
        </Tabs>
      </main>
    </div>
  )
}

export default App
