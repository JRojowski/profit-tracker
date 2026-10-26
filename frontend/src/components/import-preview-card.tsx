import { useState, type FormEvent, type ReactNode } from "react"
import { CircleCheckIcon, TriangleAlertIcon } from "lucide-react"

import { AccountBadges } from "@/components/account-badges"
import { FigureLabel } from "@/components/figure-label"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Field,
  FieldDescription,
  FieldError,
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
import { cn } from "@/lib/utils"

type ImportPreviewCardProps = {
  preview: ImportPreview
  // formId is the id of the confirm form. The Confirm button of the page submits it.
  formId: string
  busy: boolean
  onConfirm: (account?: NewAccount) => void
}

// ImportPreviewCard shows the figures and the warnings of an Import before the confirmation.
// It shows the known Account, or the form of the new Account. The Confirm button of the page submits its form.
export function ImportPreviewCard({
  preview,
  formId,
  busy,
  onConfirm,
}: ImportPreviewCardProps) {
  const isNewAccount = preview.account === null
  const [name, setName] = useState("")
  const [type, setType] = useState<AccountType>("regular")
  const [currency, setCurrency] = useState(preview.suggestedCurrency)
  const [submitted, setSubmitted] = useState(false)

  const nameError = name.trim() === "" ? "Enter a name." : null
  const currencyError = /^[A-Z]{3}$/.test(currency)
    ? null
    : "Enter a 3-letter currency code, for example USD."
  const shownCurrency = preview.account?.currency ?? currency
  const conflictCount = preview.conflicts.length

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
    <Card className="min-w-0">
      <CardHeader>
        <CardTitle>Preview</CardTitle>
        <CardDescription className="font-mono break-all">
          {preview.fileName}
        </CardDescription>
      </CardHeader>
      <CardContent>
        <form
          id={formId}
          onSubmit={handleSubmit}
          className="flex flex-col gap-4"
        >
          <dl className="grid grid-cols-2 gap-3 md:grid-cols-3 lg:grid-cols-5">
            <Figure label="Account number">{preview.accountNumber}</Figure>
            <Figure label="Currency">
              <span className="text-primary">{shownCurrency || "—"}</span>
            </Figure>
            <Figure label="New Cash Operations">
              <span
                className={cn(preview.newCashOperations > 0 && "text-positive")}
              >
                {preview.newCashOperations}
              </span>
            </Figure>
            <Figure label="Known Cash Operations">
              {preview.skippedCashOperations}
            </Figure>
            <Figure label="Import Conflicts" highlight={conflictCount > 0}>
              <span className={cn(conflictCount > 0 && "text-warning")}>
                {conflictCount}
              </span>
            </Figure>
          </dl>

          {preview.warnings.length > 0 && (
            <Alert className="border-warning/40 bg-warning/10 *:[svg]:text-warning">
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
                    <li key={warning.xtbId} className="break-words">
                      <span className="num">
                        {new Date(warning.time).toLocaleString()}
                      </span>{" "}
                      ·{" "}
                      {warning.direction === "out"
                        ? "Transfer Out"
                        : "Transfer In"}{" "}
                      ·{" "}
                      <span className="num font-medium text-foreground">
                        {formatAmount(warning.amount, shownCurrency)}
                      </span>{" "}
                      · ID <span className="num">{warning.xtbId}</span>
                    </li>
                  ))}
                </ul>
              </AlertDescription>
            </Alert>
          )}

          {preview.account ? (
            <div className="flex min-w-0 flex-wrap items-center gap-x-3 gap-y-1 rounded-lg border p-3 text-sm">
              <CircleCheckIcon className="size-4 shrink-0 text-positive" />
              <span className="text-muted-foreground">Known Account:</span>
              <span className="min-w-0 font-medium break-words">
                {preview.account.name}
              </span>
              <AccountBadges account={preview.account} />
              <span className="text-xs text-muted-foreground md:ml-auto">
                The Import adds the new Cash Operations to its Ledger.
              </span>
            </div>
          ) : (
            <div className="flex flex-col gap-4 rounded-lg border p-4">
              <div className="flex flex-col gap-1">
                <FigureLabel>New Account</FigureLabel>
                <p className="text-sm text-muted-foreground">
                  The account number is new. Enter the data of the new Account.
                </p>
              </div>
              <div className="grid gap-4 md:grid-cols-3">
                <Field data-invalid={submitted && nameError !== null}>
                  <FieldLabel htmlFor="account-name">Name</FieldLabel>
                  <Input
                    id="account-name"
                    value={name}
                    disabled={busy}
                    onChange={(event) => setName(event.target.value)}
                  />
                  {submitted && nameError && (
                    <FieldError>{nameError}</FieldError>
                  )}
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
                      <ToggleGroupItem
                        key={accountType}
                        value={accountType}
                        className="data-[state=on]:bg-primary/15 data-[state=on]:text-primary"
                      >
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
                    className="font-mono"
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
              </div>
            </div>
          )}
        </form>
      </CardContent>
    </Card>
  )
}

type FigureProps = {
  label: string
  // highlight marks a figure that needs the attention of the user.
  highlight?: boolean
  children: ReactNode
}

// Figure shows one figure of the preview in a small tile.
function Figure({ label, highlight = false, children }: FigureProps) {
  return (
    <div
      className={cn(
        "flex min-w-0 flex-col gap-1.5 rounded-lg border p-3",
        highlight && "border-warning/50 bg-warning/5"
      )}
    >
      <dt>
        <FigureLabel>{label}</FigureLabel>
      </dt>
      <dd className="num text-lg font-semibold">{children}</dd>
    </div>
  )
}
