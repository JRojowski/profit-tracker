import { useState, type FormEvent } from "react"

import { Button } from "@/components/ui/button"
import { Field, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { errorMessage, renameAccount, type Account } from "@/lib/api"

type RenameAccountFormProps = {
  account: Account
  onRenamed: (account: Account) => void
  onCancel: () => void
}

export function RenameAccountForm({
  account,
  onRenamed,
  onCancel,
}: RenameAccountFormProps) {
  const [name, setName] = useState(account.name)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (name.trim() === "") {
      setError("Enter a name.")
      return
    }
    setBusy(true)
    setError(null)
    try {
      onRenamed(await renameAccount(account.id, name.trim()))
    } catch (err) {
      setError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex items-end gap-2">
      <Field data-invalid={error !== null}>
        <FieldLabel htmlFor="rename-account">Account name</FieldLabel>
        <Input
          id="rename-account"
          value={name}
          disabled={busy}
          onChange={(event) => setName(event.target.value)}
        />
        {error && <FieldError>{error}</FieldError>}
      </Field>
      <Button
        type="button"
        variant="outline"
        disabled={busy}
        onClick={onCancel}
      >
        Cancel
      </Button>
      <Button type="submit" disabled={busy}>
        Save
      </Button>
    </form>
  )
}
