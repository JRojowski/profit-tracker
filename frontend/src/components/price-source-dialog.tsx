import { useState, type FormEvent } from "react"
import { PencilIcon } from "lucide-react"

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
import {
  errorMessage,
  setPriceTicker,
  setQuoteCurrency,
  type Holding,
} from "@/lib/api"

type PriceSourceDialogProps = {
  holding: Holding
  // onChanged runs after the tool stored a change. The caller gets new prices.
  onChanged: () => void
}

// PriceSourceDialog lets the user set the price ticker and the quote currency of the Instrument of a Holding.
export function PriceSourceDialog({
  holding,
  onChanged,
}: PriceSourceDialogProps) {
  const [open, setOpen] = useState(false)

  return (
    <Dialog open={open} onOpenChange={setOpen}>
      <DialogTrigger asChild>
        <Button
          variant="ghost"
          size="icon-sm"
          aria-label={`Edit the price source of ${holding.ticker}`}
        >
          <PencilIcon data-icon="inline-start" />
        </Button>
      </DialogTrigger>
      <DialogContent>
        <DialogHeader>
          <DialogTitle>Price source of {holding.ticker}</DialogTitle>
          <DialogDescription>
            Set the values when the default mapping gives a wrong price.
          </DialogDescription>
        </DialogHeader>
        {open && (
          <PriceSourceForm
            holding={holding}
            onChanged={onChanged}
            onDone={() => setOpen(false)}
          />
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
function PriceSourceForm({ holding, onChanged, onDone }: PriceSourceFormProps) {
  const initialTicker = holding.priceTickerManual ? holding.priceTicker : ""
  const initialCurrency = holding.quoteCurrencyManual
    ? (holding.quoteCurrency ?? "")
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
      setError("Enter the quote currency as three letters, for example USD.")
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
        await setPriceTicker(holding.ticker, ticker)
        changed = true
      }
      if (currency !== initialCurrency) {
        await setQuoteCurrency(holding.ticker, currency)
        changed = true
      }
      onDone()
      onChanged()
    } catch (err) {
      setBusy(false)
      setError(errorMessage(err))
      // The dialog stays open with the error, but the Holdings must show the stored change.
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
            placeholder={holding.priceTickerManual ? "" : holding.priceTicker}
            disabled={busy}
            onChange={(event) => setPriceTickerValue(event.target.value)}
          />
          <FieldDescription>
            The ticker at Yahoo Finance, for example VUSA.L. Leave it empty to
            use the default mapping.
          </FieldDescription>
        </Field>
        <Field>
          <FieldLabel htmlFor="quote-currency">Quote currency</FieldLabel>
          <Input
            id="quote-currency"
            value={quoteCurrency}
            placeholder={
              holding.quoteCurrencyManual ? "" : (holding.quoteCurrency ?? "")
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
