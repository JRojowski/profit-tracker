import { useState, type FormEvent, type ReactNode } from "react"
import { CircleCheckIcon, PencilIcon, TriangleAlertIcon } from "lucide-react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { PriceWarningText } from "@/components/price-warning-text"
import { Button } from "@/components/ui/button"
import {
  Dialog,
  DialogClose,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
  DialogTrigger,
} from "@/components/ui/dialog"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldGroup,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { Separator } from "@/components/ui/separator"
import {
  errorMessage,
  setPriceConfirmation,
  setPriceTicker,
  setQuoteCurrency,
  type NamedPriceSource,
  type PriceWarning,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"

type PriceSourceDialogProps = {
  // instrument is the price source of the Instrument, from a Holding or from a Price Warning.
  instrument: NamedPriceSource
  // warnings are the Price Warnings of the Instrument. They can be empty.
  warnings: PriceWarning[]
  // onChanged runs after the tool stored a change of the price ticker or the Quote Currency. The caller gets new prices.
  // A change of the confirmation does not run it.
  onChanged: () => void
}

type PriceSourceDialogTriggerProps = PriceSourceDialogProps & {
  // trigger is the element that opens the dialog. It must accept a ref and a click handler, for example a button.
  // Without it, a small edit button opens the dialog.
  trigger?: ReactNode
}

// PriceSourceDialog lets the user set the price ticker and the Quote Currency of an Instrument.
// It shows the Price Warnings of the Instrument. For a price mismatch, the user can confirm the price source.
export function PriceSourceDialog({
  instrument,
  warnings,
  onChanged,
  trigger,
}: PriceSourceDialogTriggerProps) {
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        {trigger ?? (
          <Button
            variant="ghost"
            size="icon-sm"
            aria-label={`Edit the price source of ${instrument.ticker}`}
          >
            <PencilIcon data-icon="inline-start" />
          </Button>
        )}
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Price source of {instrument.ticker}</DialogTitle>
          <DialogDescription>
            {instrument.name}. Set the values when the default mapping gives a
            wrong price.
          </DialogDescription>
        </DialogHeader>
        {open && (
          <>
            {warnings.length > 0 && <DialogWarnings warnings={warnings} />}
            <PriceConfirmation
              instrument={instrument}
              warnings={warnings}
              onDone={() => setOpen(false)}
            />
            <PriceSourceForm
              instrument={instrument}
              warnings={warnings}
              onChanged={onChanged}
              onDone={() => setOpen(false)}
            />
          </>
        )}
      </DialogContent>
    </Dialog>
  )
}

type PriceSourceFormProps = PriceSourceDialogProps & {
  // onDone closes the dialog after the tool stored all changes.
  onDone: () => void
}

// PriceSourceForm starts with the manual values of the Instrument. An empty field means the default value.
function PriceSourceForm({
  instrument,
  onChanged,
  onDone,
}: PriceSourceFormProps) {
  const initialTicker = instrument.priceTickerManual
    ? instrument.priceTicker
    : ""
  const initialCurrency = instrument.quoteCurrencyManual
    ? (instrument.quoteCurrency ?? "")
    : ""
  const [priceTicker, setPriceTickerValue] = useState(initialTicker)
  const [quoteCurrency, setQuoteCurrencyValue] = useState(initialCurrency)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)

  async function handleSubmit(event: FormEvent) {
    event.preventDefault()
    const ticker = priceTicker.trim()
    const currency = quoteCurrency.trim().toUpperCase()
    if (currency !== "" && !/^[A-Z]{3}$/.test(currency)) {
      setError("Enter the Quote Currency as three letters, for example USD.")
      return
    }
    if (ticker === initialTicker && currency === initialCurrency) {
      setError("Nothing changed.")
      return
    }
    setBusy(true)
    setError(null)
    // changed tells that the tool stored one of the two values. The second request can fail after the first.
    let changed = false
    try {
      if (ticker !== initialTicker) {
        await setPriceTicker(instrument.ticker, ticker)
        changed = true
      }
      if (currency !== initialCurrency) {
        await setQuoteCurrency(instrument.ticker, currency)
        changed = true
      }
      onDone()
      onChanged()
    } catch (err) {
      setBusy(false)
      setError(errorMessage(err))
      // The dialog stays open with the error, but the views must show the stored change.
      if (changed) onChanged()
    }
  }

  return (
    <form onSubmit={handleSubmit} className="flex flex-col gap-4">
      <FieldGroup>
        <Field>
          <FieldLabel htmlFor="price-ticker">Price ticker</FieldLabel>
          <Input
            id="price-ticker"
            value={priceTicker}
            placeholder={
              instrument.priceTickerManual ? "" : instrument.priceTicker
            }
            disabled={busy}
            onChange={(event) => setPriceTickerValue(event.target.value)}
          />
          <FieldDescription>
            The ticker at Yahoo Finance, for example VUSA.L. Leave it empty to
            use the default mapping.
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="quote-currency">Quote Currency</FieldLabel>
          <Input
            id="quote-currency"
            value={quoteCurrency}
            placeholder={
              instrument.quoteCurrencyManual
                ? ""
                : (instrument.quoteCurrency ?? "")
            }
            maxLength={3}
            disabled={busy}
            onChange={(event) => setQuoteCurrencyValue(event.target.value)}
          />
          <FieldDescription>
            The currency of the price at the exchange. Leave it empty to use the
            currency from the ticker suffix.
          </FieldDescription>
        </Field>
        {error && <FieldError>{error}</FieldError>}
      </FieldGroup>
      <DialogFooter>
        <DialogClose asChild>
          <Button type="button" variant="outline" disabled={busy}>
            Cancel
          </Button>
        </DialogClose>
        <Button type="submit" disabled={busy}>
          {busy ? "Saving…" : "Save"}
        </Button>
      </DialogFooter>
    </form>
  )
}

// DialogWarnings lists the Price Warnings of the Instrument, so that the user sees what to check.
function DialogWarnings({ warnings }: { warnings: PriceWarning[] }) {
  return (
    <Alert className="border-warning/40 bg-warning/10 *:[svg]:text-warning">
      <TriangleAlertIcon />
      <AlertTitle>
        {warnings.length === 1 ? "Price Warning" : "Price Warnings"}
      </AlertTitle>
      <AlertDescription>
        <ul className="flex flex-col gap-1">
          {warnings.map((warning) => (
            <li key={warning.kind} className="break-words">
              <PriceWarningText warning={warning} />
            </li>
          ))}
        </ul>
      </AlertDescription>
    </Alert>
  )
}

type PriceConfirmationProps = Omit<PriceSourceDialogProps, "onChanged"> & {
  // onDone closes the dialog after the tool stored the change.
  onDone: () => void
}

// PriceConfirmation lets the user confirm that the price ticker and the Quote Currency are correct, or remove
// the confirmation. It shows only for a price mismatch warning or for a confirmed Instrument.
// A confirmation changes no price, so the views load again with no price refresh.
function PriceConfirmation({
  instrument,
  warnings,
  onDone,
}: PriceConfirmationProps) {
  const { applyPriceConfirmation } = useAppState()
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<string | null>(null)
  const confirmable = warnings.some((warning) => warning.confirmable)
  const confirmed = instrument.priceConfirmed
  if (!confirmable && !confirmed) return null

  async function handleClick() {
    setBusy(true)
    setError(null)
    try {
      await setPriceConfirmation(instrument.ticker, !confirmed)
      onDone()
      applyPriceConfirmation(instrument.ticker, !confirmed)
    } catch (err) {
      setBusy(false)
      setError(errorMessage(err))
    }
  }

  return (
    <div className="flex flex-col gap-3">
      <p className="text-sm text-muted-foreground">
        {confirmed
          ? "You confirmed that the price ticker and the Quote Currency are correct. The tool shows no warning about a price mismatch for this Instrument."
          : "When the price ticker and the Quote Currency are correct, for example after a real price jump, confirm them. The confirmation removes only the warning about a price mismatch."}
      </p>
      <Button
        type="button"
        variant="outline"
        className="h-auto min-h-8 whitespace-normal"
        disabled={busy}
        onClick={handleClick}
      >
        {!confirmed && <CircleCheckIcon data-icon="inline-start" />}
        {busy
          ? "Saving…"
          : confirmed
            ? "Remove the confirmation"
            : "The price ticker and the Quote Currency are correct"}
      </Button>
      {error && <FieldError>{error}</FieldError>}
      <Separator />
    </div>
  )
}
