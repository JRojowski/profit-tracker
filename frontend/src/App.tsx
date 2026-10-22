import { useEffect, useState } from "react"

import { Badge } from "@/components/ui/badge"
import { fetchHealth } from "@/lib/api"

type BackendState = "checking" | "ok" | "unavailable"

export function App() {
  const [backend, setBackend] = useState<BackendState>("checking")

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
      <main className="flex flex-1 flex-col gap-4 p-6" />
    </div>
  )
}

export default App
