import { PencilIcon } from "lucide-react"
import { useState } from "react"

import { RenameAccountForm } from "@/components/rename-account-form"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import type { Account } from "@/lib/api"
import { useAppState } from "@/lib/app-state"

// RenameAccountDialog opens the form that renames the Account.
export function RenameAccountDialog({ account }: { account: Account }) {
  const { replaceAccount } = useAppState()
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label="Rename the Account"
          title="Rename the Account"
        >
          <PencilIcon data-icon="inline-start" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Rename the Account</DialogTitle>
          <DialogDescription>
            The name shows only in the tool. The broker account does not change.
          </DialogDescription>
        </DialogHeader>
        <RenameAccountForm
          key={account.id}
          account={account}
          onRenamed={(renamed) => {
            replaceAccount(renamed)
            setOpen(false)
          }}
          onCancel={() => setOpen(false)}
        />
      </DialogContent>
    </Dialog>
  )
}
