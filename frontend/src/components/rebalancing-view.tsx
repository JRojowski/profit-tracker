import { useEffect, useState } from "react"

import { LoadedView } from "@/components/loaded-view"
import { RebalancingTable } from "@/components/rebalancing-table"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import {
  Field,
  FieldDescription,
  FieldError,
  FieldLabel,
} from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import { useLoad } from "@/hooks/use-load"
import {
  fetchRebalancing,
  type Rebalancing,
  type RebalancingAccount,
} from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import { formatAmount, formatDate, formatRate } from "@/lib/format"

// typingDelayMs is the time without typing after which the page sends a new request.
const typingDelayMs = 500

// amountPattern is a non-negative decimal amount that the backend accepts.
const amountPattern = /^\d+([.,]\d+)?$/

// RebalancingView shows the cash fields, the banners and the Rebalancing table of the Tier Accounts.
// The cash fields start with the free cash of each Account. A change sends a new request after the user stops typing.
// The page does not store the cash. The old table stays on the screen while the new table loads.
export function RebalancingView() {
  const { valuesVersion } = useAppState()
  // typed holds the text that the user typed, by Account id. An Account with no entry shows its free cash.
  const [typed, setTyped] = useState<Record<number, string>>({})
  // sent holds the valid amounts of the last pause in typing.
  const [sent, setSent] = useState<Record<number, string>>({})
  useEffect(() => {
    const timer = setTimeout(() => {
      const valid: Record<number, string> = {}
      for (const [id, text] of Object.entries(typed)) {
        if (amountPattern.test(text.trim())) {
          valid[Number(id)] = text.trim().replace(",", ".")
        }
      }
      setSent(valid)
    }, typingDelayMs)
    return () => clearTimeout(timer)
  }, [typed])

  const loaded = useLoad(
    `rebalancing:${JSON.stringify(sent)}`,
    valuesVersion,
    (signal) => fetchRebalancing(sent, signal)
  )
  // shown is the last table that loaded. It stays on the screen while a new table loads.
  const [shown, setShown] = useState<Rebalancing | null>(null)
  if (loaded.status === "ok" && loaded.data !== shown) setShown(loaded.data)

  return (
    <div className="flex min-w-0 flex-col gap-4">
      {shown === null ? (
        <LoadedView loaded={loaded} errorTitle="Cannot load the Rebalancing">
          {() => null}
        </LoadedView>
      ) : (
        <>
          {loaded.status === "error" && (
            <Alert variant="destructive">
              <AlertTitle>Cannot load the Rebalancing</AlertTitle>
              <AlertDescription>{loaded.error}</AlertDescription>
            </Alert>
          )}
          <Banners data={shown} />
          {shown.accounts.length > 0 && (
            <div className="grid gap-3 md:grid-cols-2">
              {shown.accounts.map((account) => (
                <CashField
                  key={account.accountId}
                  account={account}
                  text={typed[account.accountId]}
                  onChange={(text) =>
                    setTyped((previous) => ({
                      ...previous,
                      [account.accountId]: text,
                    }))
                  }
                />
              ))}
            </div>
          )}
          {shown.available && <RebalancingTable data={shown} />}
          {shown.exchangeRate && (
            <p className="text-sm text-muted-foreground">
              Exchange Rate of the USD Account:{" "}
              <span className="num">
                1 USD = {formatRate(shown.exchangeRate.rate)} PLN
              </span>
              , table of{" "}
              <span className="num">{formatDate(shown.exchangeRate.date)}</span>
              . Amounts of the parts are in PLN.
            </p>
          )}
        </>
      )}
    </div>
  )
}

type CashFieldProps = {
  account: RebalancingAccount
  // text is the typed text, or undefined when the user did not type.
  text: string | undefined
  onChange: (text: string) => void
}

// CashField shows the cash for Rebalancing of one Account and its unused cash.
function CashField({ account, text, onChange }: CashFieldProps) {
  const id = `cash-${account.accountId}`
  const invalid = text !== undefined && !amountPattern.test(text.trim())
  return (
    <Field data-invalid={invalid || undefined}>
      <FieldLabel htmlFor={id}>
        Cash for Rebalancing on {account.name} ({account.currency})
      </FieldLabel>
      <Input
        id={id}
        inputMode="decimal"
        className="num"
        aria-invalid={invalid}
        value={text ?? account.freeCash}
        onChange={(e) => onChange(e.target.value)}
      />
      {invalid ? (
        <FieldError>Enter an amount of zero or more.</FieldError>
      ) : (
        <FieldDescription>
          Free cash on the Account:{" "}
          <span className="num">
            {formatAmount(account.freeCash, account.currency)}
          </span>
          . Unused cash after the Planned Purchases:{" "}
          <span className="num">
            {formatAmount(account.unusedCash, account.currency)}
          </span>
          .
        </FieldDescription>
      )}
    </Field>
  )
}

// Banners shows the missing Accounts, the warnings and the Holdings that the table cannot place or value.
function Banners({ data }: { data: Rebalancing }) {
  const noTier = data.noTier?.holdings ?? []
  return (
    <>
      {data.missingAccounts.map((currency) => (
        <Alert key={currency}>
          <AlertTitle>No {currency} Account</AlertTitle>
          <AlertDescription>
            The tool has no regular XTB Account in {currency}. The Planned
            Purchases for this Account stay empty.
          </AlertDescription>
        </Alert>
      ))}
      {data.warnings.map((warning) => (
        <Alert key={warning}>
          <AlertTitle>The table is not available</AlertTitle>
          <AlertDescription>{warning}</AlertDescription>
        </Alert>
      ))}
      {noTier.length > 0 && (
        <Alert>
          <AlertTitle>Holdings with no Tier</AlertTitle>
          <AlertDescription>
            Their value counts in the base of the Actual Shares. Edit the
            targets to give them a Tier:{" "}
            <span className="font-mono">
              {noTier.map((h) => h.ticker).join(", ")}
            </span>
            .
          </AlertDescription>
        </Alert>
      )}
      {data.unpricedHoldings.length > 0 && (
        <Alert>
          <AlertTitle>Holdings with no price</AlertTitle>
          <AlertDescription>
            The table leaves them out of all values, shares and Gaps:{" "}
            <span className="font-mono">
              {[...new Set(data.unpricedHoldings.map((h) => h.ticker))].join(
                ", "
              )}
            </span>
            .
          </AlertDescription>
        </Alert>
      )}
    </>
  )
}
