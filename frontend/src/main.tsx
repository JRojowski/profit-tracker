import { StrictMode } from "react"
import { createRoot } from "react-dom/client"

import "./index.css"
import App from "./App.tsx"
import { ThemeProvider } from "@/components/theme-provider.tsx"
import { TooltipProvider } from "@/components/ui/tooltip.tsx"
import { AppStateProvider } from "@/lib/app-state.tsx"

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <ThemeProvider defaultTheme="dark">
      <TooltipProvider>
        <AppStateProvider>
          <App />
        </AppStateProvider>
      </TooltipProvider>
    </ThemeProvider>
  </StrictMode>
)
