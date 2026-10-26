import { Link } from "react-router"

import { Button } from "@/components/ui/button"
import { paths } from "@/lib/routes"

// NotFoundPage shows a URL that is not a view of the tool.
export function NotFoundPage() {
  return (
    <div className="flex flex-col items-start gap-3">
      <p className="text-sm text-muted-foreground">
        The tool has no page at this URL.
      </p>
      <Button asChild variant="outline">
        <Link to={paths.allAccounts}>Open All Accounts</Link>
      </Button>
    </div>
  )
}
