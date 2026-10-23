import { useState, type FormEvent } from "react"
import { TriangleAlertIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardFooter,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { ToggleGroup, ToggleGroupItem } from "@/components/ui/toggle-group"
import {
  accountTypes,
  type AccountType,
  type ImportPreview,
  type NewAccount,
} from "@/lib/api"
import { formatAmount } from "@/lib/format"

type ImportPreviewCardProps = {
  preview: ImportPreview
  busy: boolean
  onConfirm: (account?: NewAccount) => void
  onCancel: () => void
}

export function ImportPreviewCard({
  preview,
  busy,
  onConfirm,
  onCancel,
}: ImportPreviewCardProps) {
  const isNewAccount = preview.account === null
  const hasOpenConflicts = preview.conflicts.some(
    (conflict) => conflict.decision === null
  )
  const [name, setName] = useState("")
  const [type, setType] = useState<AccountType>("regular")
  const [currency, setCurrency] = useState(preview.suggestedCurrency)
  const [submitted, setSubmitted] = useState(false)

  const nameError = name.trim() === "" ? "Enter a name." : null
  const currencyError = /^[A-Z]{3}$/.test(currency)
    ? null
    : "Enter a 3-letter currency code, for example USD."

  function handleSubmit(event: FormEvent) {
    event.preventDefault()
    if (!isNewAccount) {
      onConfirm()
      return
    }
    setSubmitted(true)
    if (nameError || currencyError) return
    onConfirm({ name: name.trim(), type, currency })
  }

  return (
    <Card>
      <form onSubmit={handleSubmit} className="flex flex-col gap-6">
        <CardHeader>
          <CardTitle>Preview</CardTitle>
          <CardDescription>{preview.fileName}</CardDescription>
        </CardHeader>
        <CardContent className="flex flex-col gap-6">
          <dl className="grid grid-cols-2 gap-2 text-sm">
            <dt className="text-muted-foreground">Account number</dt>
            <dd>{preview.accountNumber}</dd>
            <dt className="text-muted-foreground">Account</dt>
            <dd>{preview.account ? preview.account.name : "New Account"}</dd>
            <dt className="text-muted-foreground">New Cash Operations</dt>
            <dd>{preview.newCashOperations}</dd>
            <dt className="text-muted-foreground">Known Cash Operations</dt>
            <dd>{preview.skippedCashOperations}</dd>
            <dt className="text-muted-foreground">Import Conflicts</dt>
            <dd>{preview.conflicts.length}</dd>
          </dl>

          {preview.warnings.length > 0 && (
            <Alert>
              <TriangleAlertIcon />
              <AlertTitle>
                Transfers with no other side in your other Accounts
              </AlertTitle>
              <AlertDescription>
                <p>
                  The total for all Accounts can be wrong until you import the
                  other Account. You can still confirm this Import.
                </p>
                <ul className="flex flex-col gap-1">
                  {preview.warnings.map((warning) => (
                    <li key={warning.xtbId}>
                      {new Date(warning.time).toLocaleString()} ·{" "}
                      {warning.direction === "out"
                        ? "Transfer Out"
                        : "Transfer In"}{" "}
                      ·{" "}
                      {formatAmount(
                        warning.amount,
                        preview.account?.currency ?? currency
                      )}{" "}
                      · ID {warning.xtbId}
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          )}

          {isNewAccount && (
            <FieldGroup>
              <Field data-invalid={submitted && nameError !== null}>
                <FieldLabel htmlFor="account-name">Name</FieldLabel>
                <Input
                  id="account-name"
                  value={name}
                  disabled={busy}
                  onChange={(event) => setName(event.target.value)}
                />
                {submitted && nameError && <FieldError>{nameError}</FieldError>}
              </Field>
              <Field>
                <FieldLabel htmlFor="account-type">Type</FieldLabel>
                <ToggleGroup
                  id="account-type"
                  type="single"
                  variant="outline"
                  value={type}
                  disabled={busy}
                  onValueChange={(value) => {
                    // An empty value means that the user clicked the selected item again.
                    if (value) setType(value as AccountType)
                  }}
                >
                  {accountTypes.map((accountType) => (
                    <ToggleGroupItem key={accountType} value={accountType}>
                      {accountType}
                    </ToggleGroupItem>
                  ))}
                </ToggleGroup>
              </Field>
              <Field data-invalid={submitted && currencyError !== null}>
                <FieldLabel htmlFor="account-currency">Currency</FieldLabel>
                <Input
                  id="account-currency"
                  value={currency}
                  maxLength={3}
                  disabled={busy}
                  onChange={(event) =>
                    setCurrency(event.target.value.toUpperCase())
                  }
                />
                {preview.suggestedCurrency && (
                  <FieldDescription>
                    Suggested from the file name: {preview.suggestedCurrency}
                  </FieldDescription>
                )}
                {submitted && currencyError && (
                  <FieldError>{currencyError}</FieldError>
                )}
              </Field>
            </FieldGroup>
          )}
        </CardContent>
        <CardFooter className="flex justify-end gap-2">
          <Button
            type="button"
            variant="outline"
            disabled={busy}
            onClick={onCancel}
          >
            Cancel
          </Button>
          <Button type="submit" disabled={busy || hasOpenConflicts}>
            Confirm Import
          </Button>
        </CardFooter>
      </form>
    </Card>
  )
}
