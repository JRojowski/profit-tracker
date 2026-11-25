import { PlusIcon, Trash2Icon } from "lucide-react"
import { useState } from "react"

import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Field, FieldError, FieldLabel } from "@/components/ui/field"
import { Input } from "@/components/ui/input"
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "@/components/ui/select"
import {
  errorMessage,
  saveTiers,
  TiersRejected,
  type TierError,
  type TierInstrument,
  type Tiers,
} from "@/lib/api"
import {
  addTier,
  draftFromTiers,
  instrumentsOfTier,
  moveInstrument,
  removeTier,
  sortedInstruments,
  toRequest,
  validateOrders,
  type DraftTier,
  type TiersDraft,
} from "@/lib/tiers-draft"

type TiersEditorProps = {
  saved: Tiers
  // onSaved gets the configuration that the backend saved.
  onSaved: (saved: Tiers) => void
  onCancel: () => void
}

// noTier is the value of the Tier selector for an Instrument with no Tier.
const noTier = "none"

// TiersEditor is the "Edit targets" mode. It edits a local copy of the configuration and sends it with one PUT on
// save. A 422 answer shows each message next to its Tier, and nothing is saved.
export function TiersEditor({ saved, onSaved, onCancel }: TiersEditorProps) {
  const [draft, setDraft] = useState<TiersDraft>(() => draftFromTiers(saved))
  const [errors, setErrors] = useState<TierError[]>([])
  const [failure, setFailure] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)
  const [deleting, setDeleting] = useState<DraftTier | null>(null)

  const instruments = sortedInstruments(saved.instruments)
  const generalErrors = errors.filter((e) => e.tierIndex === null)

  async function save() {
    setBusy(true)
    setErrors([])
    setFailure(null)
    const invalid = validateOrders(draft)
    if (invalid.length > 0) {
      setErrors(invalid)
      setBusy(false)
      return
    }
    try {
      onSaved(await saveTiers(toRequest(draft)))
    } catch (err) {
      if (err instanceof TiersRejected) setErrors(err.errors)
      else setFailure(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  function requestDelete(tier: DraftTier) {
    if (instrumentsOfTier(draft, tier.key).length > 0) setDeleting(tier)
    else setDraft(removeTier(draft, tier.key))
  }

  function updateTier(key: number, change: Partial<DraftTier>) {
    setDraft({
      ...draft,
      tiers: draft.tiers.map((t) => (t.key === key ? { ...t, ...change } : t)),
    })
  }

  return (
    <div className="flex flex-col gap-4">
      {(generalErrors.length > 0 || failure !== null) && (
        <Alert variant="destructive">
          <AlertTitle>The tool did not save the Tiers</AlertTitle>
          <AlertDescription>
            <ul className="flex flex-col gap-1">
              {generalErrors.map((e) => (
                <li key={e.message}>{e.message}</li>
              ))}
              {failure !== null && <li>{failure}</li>}
            </ul>
          </AlertDescription>
        </Alert>
      )}

      <div className="grid gap-4 md:grid-cols-2">
        {draft.tiers.map((tier, index) => (
          <TierFields
            key={tier.key}
            tier={tier}
            errors={errors.filter((e) => e.tierIndex === index)}
            disabled={busy}
            onChange={(change) => updateTier(tier.key, change)}
            onDelete={() => requestDelete(tier)}
          />
        ))}
      </div>

      <div>
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onClick={() => setDraft(addTier(draft))}
        >
          <PlusIcon data-icon="inline-start" />
          Add a Tier
        </Button>
      </div>

      <InstrumentAssignment
        instruments={instruments}
        draft={draft}
        disabled={busy}
        onDraft={setDraft}
      />

      <div className="flex flex-wrap justify-end gap-2">
        <Button
          type="button"
          variant="outline"
          disabled={busy}
          onClick={onCancel}
        >
          Cancel
        </Button>
        <Button type="button" disabled={busy} onClick={() => void save()}>
          Save targets
        </Button>
      </div>

      <Dialog
        open={deleting !== null}
        onOpenChange={(open) => {
          if (!open) setDeleting(null)
        }}
      >
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Delete the Tier?</DialogTitle>
            <DialogDescription>
              {deleting &&
                `The Tier ${deleting.order}. ${deleting.name} has ${instrumentsOfTier(draft, deleting.key).length} Instruments. They get no Tier. The tool saves the change only when you save the targets.`}
            </DialogDescription>
          </DialogHeader>
          <DialogFooter>
            <Button variant="outline" onClick={() => setDeleting(null)}>
              Keep the Tier
            </Button>
            <Button
              variant="destructive"
              onClick={() => {
                if (deleting) setDraft(removeTier(draft, deleting.key))
                setDeleting(null)
              }}
            >
              Delete the Tier
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}

type TierFieldsProps = {
  tier: DraftTier
  errors: TierError[]
  disabled: boolean
  onChange: (change: Partial<DraftTier>) => void
  onDelete: () => void
}

// TierFields edits the order number, the name, the Target Share and the Shared Part name of one Tier. The 422
// messages of the Tier show under the fields.
function TierFields({
  tier,
  errors,
  disabled,
  onChange,
  onDelete,
}: TierFieldsProps) {
  const id = `tier-${tier.key}`
  return (
    <Card data-invalid={errors.length > 0 || undefined}>
      <CardHeader>
        <CardTitle className="flex items-center justify-between gap-2">
          <span>Tier {tier.order}</span>
          <Button
            type="button"
            variant="ghost"
            size="icon-sm"
            disabled={disabled}
            aria-label={`Delete the Tier ${tier.order}`}
            onClick={onDelete}
          >
            <Trash2Icon data-icon="inline-start" />
          </Button>
        </CardTitle>
      </CardHeader>
      <CardContent className="flex flex-col gap-3">
        <div className="grid grid-cols-2 gap-3">
          <Field>
            <FieldLabel htmlFor={`${id}-order`}>Order number</FieldLabel>
            <Input
              id={`${id}-order`}
              inputMode="numeric"
              className="num"
              value={tier.order}
              disabled={disabled}
              onChange={(e) => onChange({ order: e.target.value })}
            />
          </Field>
          <Field>
            <FieldLabel htmlFor={`${id}-share`}>Target Share (%)</FieldLabel>
            <Input
              id={`${id}-share`}
              inputMode="decimal"
              className="num"
              value={tier.targetShare}
              disabled={disabled}
              onChange={(e) => onChange({ targetShare: e.target.value })}
            />
          </Field>
        </div>
        <Field>
          <FieldLabel htmlFor={`${id}-name`}>Name</FieldLabel>
          <Input
            id={`${id}-name`}
            value={tier.name}
            disabled={disabled}
            onChange={(e) => onChange({ name: e.target.value })}
          />
        </Field>
        <Field>
          <FieldLabel htmlFor={`${id}-shared`}>Shared Part name</FieldLabel>
          <Input
            id={`${id}-shared`}
            value={tier.sharedPartName}
            placeholder="For example Blue Chips"
            disabled={disabled}
            onChange={(e) => onChange({ sharedPartName: e.target.value })}
          />
        </Field>
        {errors.map((e) => (
          <FieldError key={`${e.ticker}-${e.message}`}>{e.message}</FieldError>
        ))}
      </CardContent>
    </Card>
  )
}

type InstrumentAssignmentProps = {
  instruments: TierInstrument[]
  draft: TiersDraft
  disabled: boolean
  onDraft: (draft: TiersDraft) => void
}

// InstrumentAssignment lists each Instrument for assignment with a Tier selector and an optional Target Share.
// A Target Share makes the Instrument an Instrument Target. Without it the Instrument is in the Shared Part.
function InstrumentAssignment({
  instruments,
  draft,
  disabled,
  onDraft,
}: InstrumentAssignmentProps) {
  return (
    <Card>
      <CardHeader>
        <CardTitle>Instruments</CardTitle>
      </CardHeader>
      <CardContent>
        {instruments.length === 0 ? (
          <p className="text-sm text-muted-foreground">
            No Instrument has a Fill on a Tier Account yet.
          </p>
        ) : (
          <ul className="flex flex-col divide-y">
            {instruments.map((instrument) => {
              const assignment = draft.assignments[instrument.ticker]
              return (
                <li
                  key={instrument.ticker}
                  className="flex flex-wrap items-center justify-between gap-x-4 gap-y-2 py-2"
                >
                  <div className="flex min-w-0 flex-wrap items-center gap-2 text-sm">
                    <span className="font-mono">{instrument.ticker}</span>
                    <span className="min-w-0 truncate text-muted-foreground">
                      {instrument.name}
                    </span>
                    {!instrument.hasOpenHolding && <NoOpenHoldingBadge />}
                  </div>
                  <div className="flex items-center gap-2">
                    <Select
                      value={assignment ? String(assignment.tierKey) : noTier}
                      disabled={disabled}
                      onValueChange={(value) =>
                        onDraft(
                          moveInstrument(
                            draft,
                            instrument.ticker,
                            value === noTier ? null : Number(value)
                          )
                        )
                      }
                    >
                      <SelectTrigger
                        className="w-44"
                        aria-label={`Tier of ${instrument.ticker}`}
                      >
                        <SelectValue />
                      </SelectTrigger>
                      <SelectContent>
                        <SelectItem value={noTier}>No Tier</SelectItem>
                        {draft.tiers.map((tier) => (
                          <SelectItem key={tier.key} value={String(tier.key)}>
                            {tier.order}. {tier.name || "Unnamed"}
                          </SelectItem>
                        ))}
                      </SelectContent>
                    </Select>
                    <Input
                      className="w-28 num"
                      inputMode="decimal"
                      placeholder="Shared"
                      aria-label={`Target Share of ${instrument.ticker} (%)`}
                      value={assignment?.targetShare ?? ""}
                      disabled={disabled || !assignment}
                      onChange={(e) =>
                        onDraft({
                          ...draft,
                          assignments: {
                            ...draft.assignments,
                            [instrument.ticker]: {
                              ...assignment,
                              targetShare: e.target.value,
                            },
                          },
                        })
                      }
                    />
                  </div>
                </li>
              )
            })}
          </ul>
        )}
      </CardContent>
    </Card>
  )
}

// NoOpenHoldingBadge marks an Instrument that has no open Holding.
function NoOpenHoldingBadge() {
  return <Badge variant="outline">No open Holding</Badge>
}
