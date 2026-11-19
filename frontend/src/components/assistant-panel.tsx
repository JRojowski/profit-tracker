import {
  LoaderCircleIcon,
  MessageSquarePlusIcon,
  RotateCwIcon,
  SendIcon,
} from "lucide-react"
import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from "react"

import { AssistantMarkdown } from "@/components/assistant-markdown"
import { AssistantSteps } from "@/components/assistant-steps"
import { Alert, AlertDescription, AlertTitle } from "@/components/ui/alert"
import { Button } from "@/components/ui/button"
import {
  Sheet,
  SheetContent,
  SheetDescription,
  SheetHeader,
  SheetTitle,
} from "@/components/ui/sheet"
import { Textarea } from "@/components/ui/textarea"
import { useOverviewUrl } from "@/hooks/use-overview-url"
import { useRouteAccount } from "@/hooks/use-route-account"
import type { AssistantContext } from "@/lib/api"
import { useAppState } from "@/lib/app-state"
import {
  maxQuestionLength,
  useAssistant,
  type AssistantErrorKind,
  type AssistantTurn,
} from "@/lib/assistant-state"

// failureTexts is the message for each kind of failure. The panel shows it in place of the answer.
// The text for a rate limit names the daily limit of the free model. The text for a failure of the provider
// tells that the data of the user is not the cause.
const failureTexts: Record<
  AssistantErrorKind,
  { title: string; description: string }
> = {
  not_configured: {
    title: "The Assistant is not set up",
    description:
      "The tool has no API key for the model provider. Set the key and start the tool again.",
  },
  invalid_request: {
    title: "The question is not valid",
    description:
      "The tool cannot send this conversation. Shorten the question or start a new conversation.",
  },
  rate_limited: {
    title: "The provider refuses the question",
    description:
      "The provider limits how many questions it answers. The daily limit of the free model can be the cause. Try again later.",
  },
  provider_failure: {
    title: "The provider does not answer",
    description:
      "The model provider does not answer or answers too slowly. Your data is not the cause. Send the question again.",
  },
  network: {
    title: "The tool is not reachable",
    description:
      "The question did not reach the tool. Check that the tool runs and that the network works, then send the question again.",
  },
  unknown: {
    title: "The Assistant cannot answer now",
    description: "Something went wrong. Send the question again.",
  },
}

// AssistantPanel shows the conversation with the Assistant in a panel at the right side. Like the Instrument
// Overview panel, it does not block the page: it has no overlay, and a click on the page does not close it.
// It stands above the Instrument Overview panel. Escape closes it first, because it is the layer that opened
// last, or because the Instrument Overview panel passes the key on to it.
export function AssistantPanel() {
  const {
    available,
    open,
    setOpen,
    turns,
    asking,
    ask,
    retry,
    newConversation,
  } = useAssistant()
  const { accounts } = useAppState()
  const account = useRouteAccount(accounts)
  const overview = useOverviewUrl().state
  const [draft, setDraft] = useState("")
  const listEnd = useRef<HTMLDivElement>(null)

  // The page context comes from the URL, so it stays correct while the Assistant covers the Instrument Overview.
  const accountId = account?.id
  const instrument = overview?.ticker
  const context = useMemo<AssistantContext>(
    () => ({
      ...(accountId !== undefined && { accountId }),
      ...(instrument !== undefined && { instrument }),
    }),
    [accountId, instrument]
  )

  // The list shows the newest turn.
  useEffect(() => {
    listEnd.current?.scrollIntoView({ block: "end" })
  }, [turns, open])

  if (!available) return null

  const send = () => {
    if (draft.trim() === "" || asking) return
    ask(draft, context)
    setDraft("")
  }
  // Enter sends the question. Shift+Enter makes a new line. A key that confirms a text composition does neither.
  const onKeyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key !== "Enter" || event.shiftKey) return
    if (event.nativeEvent.isComposing) return
    event.preventDefault()
    send()
  }
  const lastTurn = turns[turns.length - 1]

  return (
    <Sheet open={open} onOpenChange={setOpen} modal={false}>
      <SheetContent
        data-assistant-panel=""
        className="z-60 data-[side=right]:w-full data-[side=right]:sm:max-w-xl"
        // A click on the page behind the panel must not close it. The panel closes with its own control and with Escape.
        onInteractOutside={(event) => event.preventDefault()}
      >
        <SheetHeader className="gap-1.5 border-b pr-12">
          <SheetTitle>Assistant</SheetTitle>
          <SheetDescription>
            Ask a question about your Accounts, results and Holdings.
          </SheetDescription>
          {turns.length > 0 && (
            <Button
              variant="ghost"
              size="sm"
              className="absolute top-3 right-12"
              onClick={newConversation}
            >
              <MessageSquarePlusIcon data-icon="inline-start" />
              New conversation
            </Button>
          )}
        </SheetHeader>
        <div className="flex min-h-0 flex-1 flex-col gap-6 overflow-y-auto px-4">
          {turns.length === 0 && (
            <EmptyConversation
              instrument={instrument}
              onPick={(question) => ask(question, context)}
            />
          )}
          {turns.map((turn) => (
            <TurnView
              key={turn.id}
              turn={turn}
              onRetry={
                turn === lastTurn && !asking ? () => retry(context) : null
              }
            />
          ))}
          <div ref={listEnd} />
        </div>
        <div className="flex flex-col gap-2 border-t p-4">
          <ContextLine accountName={account?.name} instrument={instrument} />
          <Textarea
            aria-label="Question for the Assistant"
            placeholder="Ask about your results, Holdings or Allocation"
            value={draft}
            rows={2}
            maxLength={maxQuestionLength}
            onChange={(event) => setDraft(event.target.value)}
            onKeyDown={onKeyDown}
            className="max-h-40 min-h-10"
          />
          <div className="flex items-center justify-between gap-2">
            <span className="text-xs text-muted-foreground">
              Enter sends. Shift+Enter makes a new line.
            </span>
            <Button
              onClick={send}
              disabled={draft.trim() === "" || asking}
              aria-label="Send the question"
            >
              <SendIcon data-icon="inline-start" />
              Send
            </Button>
          </div>
        </div>
      </SheetContent>
    </Sheet>
  )
}

// ContextLine tells the user what the Assistant knows about the open page: the Account and the Instrument.
function ContextLine({
  accountName,
  instrument,
}: {
  accountName: string | undefined
  instrument: string | undefined
}) {
  const parts = [accountName, instrument].filter(
    (part): part is string => part !== undefined
  )
  if (parts.length === 0) return null
  return (
    <p className="text-xs break-words text-muted-foreground">
      Open page: <span className="num font-mono">{parts.join(" · ")}</span>
    </p>
  )
}

// EmptyConversation shows what the Assistant does and the example questions. A click on an example sends it.
// With an open Instrument Overview, the first example asks about that Instrument.
function EmptyConversation({
  instrument,
  onPick,
}: {
  instrument: string | undefined
  onPick: (question: string) => void
}) {
  const year = new Date().getFullYear() - 1
  const examples = [
    instrument === undefined
      ? "What is my Total Result, and what are its parts?"
      : `How much did I earn on ${instrument}, with the dividends?`,
    `What was my Total Result in ${year}?`,
    `Which Closed Position was the worst in ${year}?`,
    "How much of the IKZE Limit is left this year?",
  ]
  return (
    <div className="flex flex-col gap-4">
      <p className="text-sm text-muted-foreground">
        The Assistant answers with the numbers that the tool calculates. It does
        not recommend a purchase or a sale.
      </p>
      <ul aria-label="Example questions" className="flex flex-col gap-2">
        {examples.map((example) => (
          <li key={example}>
            <Button
              variant="outline"
              className="h-auto w-full justify-start py-2 text-left whitespace-normal"
              onClick={() => onPick(example)}
            >
              {example}
            </Button>
          </li>
        ))}
      </ul>
    </div>
  )
}

// TurnView shows one question, and below it the answer, the work in progress state or the error.
// onRetry is set for the last turn when it failed. It sends the question again.
function TurnView({
  turn,
  onRetry,
}: {
  turn: AssistantTurn
  onRetry: (() => void) | null
}) {
  return (
    <section className="flex flex-col gap-3" aria-label="Question and answer">
      <p className="max-w-[85%] self-end rounded-lg bg-muted px-3 py-2 text-sm break-words whitespace-pre-wrap">
        {turn.question}
      </p>
      {turn.status === "pending" && (
        <p
          role="status"
          className="flex items-center gap-2 text-sm text-muted-foreground"
        >
          <LoaderCircleIcon className="animate-spin" aria-hidden="true" />
          Working on the answer…
        </p>
      )}
      {turn.status === "failed" && (
        <FailureView kind={turn.error ?? "unknown"} onRetry={onRetry} />
      )}
      {turn.status === "answered" && <AnswerView turn={turn} />}
    </section>
  )
}

// FailureView shows the message of a failure in place of the answer, and a control that sends the question again.
function FailureView({
  kind,
  onRetry,
}: {
  kind: AssistantErrorKind
  onRetry: (() => void) | null
}) {
  const text = failureTexts[kind]
  return (
    <Alert variant="destructive">
      <AlertTitle>{text.title}</AlertTitle>
      <AlertDescription>
        <p>{text.description}</p>
        {onRetry && (
          <Button
            variant="outline"
            size="sm"
            className="mt-2 text-foreground"
            onClick={onRetry}
          >
            <RotateCwIcon data-icon="inline-start" />
            Send again
          </Button>
        )}
      </AlertDescription>
    </Alert>
  )
}

// AnswerView shows the answer as Markdown, and under it the steps of the model.
function AnswerView({ turn }: { turn: AssistantTurn }) {
  return (
    <div className="flex min-w-0 flex-col gap-3">
      <AssistantMarkdown>{turn.answer}</AssistantMarkdown>
      <AssistantSteps steps={turn.steps} />
    </div>
  )
}
