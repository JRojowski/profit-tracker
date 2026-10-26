import { Link } from "react-router"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import { paths } from "@/lib/routes"

// AccountNotFound tells that the URL has no known Account, and links to "All Accounts".
export function AccountNotFound() {
  return (
    <Alert>
      <AlertTitle>The tool has no such Account</AlertTitle>
      <AlertDescription className="flex flex-col items-start gap-3">
        <p>
          The link is wrong, or the Account does not exist. Open "All Accounts"
          to see the Accounts of the tool.
        </p>
        <Button asChild variant="outline" className="no-underline">
          <Link to={paths.allAccounts}>Open All Accounts</Link>
        </Button>
      </AlertDescription>
    </Alert>
  )
}
