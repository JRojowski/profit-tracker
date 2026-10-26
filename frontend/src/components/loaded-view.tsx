import type { ReactNode } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Skeleton } from "@/components/ui/skeleton"
import type { Loaded } from "@/hooks/use-load"

type LoadedViewProps<T> = {
  loaded: Loaded<T>
  // errorTitle is the title of the alert when the load fails, for example "Cannot load the Holdings".
  errorTitle: string
  children: (data: T) => ReactNode
}

// LoadedView shows a skeleton while the data loads, an alert when the load fails, and else the children.
export function LoadedView<T>({
  loaded,
  errorTitle,
  children,
}: LoadedViewProps<T>) {
  if (loaded.status === "loading") {
    return (
      <div className="flex flex-col gap-4">
        <Skeleton className="h-24 w-full" />
        <Skeleton className="h-48 w-full" />
      </div>
    )
  }
  if (loaded.status === "error") {
    return (
      <Alert variant="destructive">
        <AlertTitle>{errorTitle}</AlertTitle>
        <AlertDescription>{loaded.error}</AlertDescription>
      </Alert>
    )
  }
  return <>{children(loaded.data)}</>
}
