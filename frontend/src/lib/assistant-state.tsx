import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useRef,
  useState,
  type ReactNode,
} from "react"

import {
  ApiError,
  askAssistant,
  fetchAssistantAvailable,
  type AssistantContext,
  type AssistantMessage,
  type AssistantStep,
} from "@/lib/api"

// maxQuestionLength is the longest message that the backend accepts, in characters.
export const maxQuestionLength = 4000

// maxMessages is the largest number of messages that the backend accepts in one request.
const maxMessages = 40

// AssistantErrorKind is the kind of failure of a question. Each kind has its own message in the panel.
// "network" is a request that did not reach the backend. "unknown" is each other failure.
export type AssistantErrorKind =
  | "not_configured"
  | "invalid_request"
  | "rate_limited"
  | "provider_failure"
  | "network"
  | "unknown"

// AssistantTurn is one question of the user and what the Assistant did with it.
export type AssistantTurn = {
  id: number
  question: string
  // status is "pending" while the Assistant works. A "failed" turn has no answer and has an error.
  status: "pending" | "answered" | "failed"
  answer: string
  steps: AssistantStep[]
  error: AssistantErrorKind | null
}

type AssistantState = {
  // available is false until the backend says that the Assistant is on.
  available: boolean
  open: boolean
  setOpen: (open: boolean) => void
  // turns holds the conversation in time order. It lives in memory and ends on a reload.
  turns: AssistantTurn[]
  // asking is true while one question waits for its answer. The Assistant takes one question at a time.
  asking: boolean
  // ask sends one question with the page context. It does nothing when the text is empty or another question waits.
  ask: (question: string, context: AssistantContext) => void
  // retry sends the last question again, when it failed. It uses the page context of the moment.
  retry: (context: AssistantContext) => void
  // newConversation removes all questions and answers. A question that waits is dropped.
  newConversation: () => void
}

const AssistantContextValue = createContext<AssistantState | undefined>(
  undefined
)

// AssistantProvider holds the conversation with the Assistant above the routes. The conversation stays
// while the user moves between pages, and it ends when the tab reloads.
export function AssistantProvider({ children }: { children: ReactNode }) {
  const [available, setAvailable] = useState(false)
  const [open, setOpen] = useState(false)
  const [turns, setTurns] = useState<AssistantTurn[]>([])
  const nextId = useRef(1)
  // request is the controller of the question that waits. A new conversation aborts it.
  const request = useRef<AbortController | null>(null)
  const asking = turns.some((turn) => turn.status === "pending")

  useEffect(() => {
    const controller = new AbortController()
    fetchAssistantAvailable(controller.signal)
      .then(setAvailable)
      .catch(() => {
        // A failed read means that the Assistant is not available. The screen shows no control.
        if (!controller.signal.aborted) setAvailable(false)
      })
    return () => controller.abort()
  }, [])

  // run sends the question of the turn with its id and writes the outcome into the turn.
  // history holds the turns that come before the question.
  const run = useCallback(
    (
      id: number,
      question: string,
      context: AssistantContext,
      history: AssistantTurn[]
    ) => {
      const controller = new AbortController()
      request.current = controller
      const finish = (change: Partial<AssistantTurn>) =>
        setTurns((list) =>
          list.map((turn) => (turn.id === id ? { ...turn, ...change } : turn))
        )
      askAssistant(messagesFor(history, question), context, controller.signal)
        .then((result) =>
          finish({
            status: "answered",
            answer: result.answer,
            steps: result.steps,
            error: null,
          })
        )
        .catch((err: unknown) => {
          if (controller.signal.aborted) return
          finish({ status: "failed", error: errorKind(err) })
        })
    },
    []
  )

  const ask = useCallback(
    (question: string, context: AssistantContext) => {
      const text = question.trim()
      if (text === "" || asking) return
      const id = nextId.current++
      setTurns((list) => [
        ...list,
        {
          id,
          question: text,
          status: "pending",
          answer: "",
          steps: [],
          error: null,
        },
      ])
      run(id, text, context, turns)
    },
    [asking, turns, run]
  )

  const retry = useCallback(
    (context: AssistantContext) => {
      const last = turns[turns.length - 1]
      if (last === undefined || last.status !== "failed" || asking) return
      setTurns((list) =>
        list.map((turn) =>
          turn.id === last.id
            ? { ...turn, status: "pending", error: null }
            : turn
        )
      )
      run(last.id, last.question, context, turns.slice(0, -1))
    },
    [asking, turns, run]
  )

  const newConversation = useCallback(() => {
    request.current?.abort()
    request.current = null
    setTurns([])
  }, [])

  const value = useMemo(
    () => ({
      available,
      open,
      setOpen,
      turns,
      asking,
      ask,
      retry,
      newConversation,
    }),
    [available, open, turns, asking, ask, retry, newConversation]
  )

  return (
    <AssistantContextValue.Provider value={value}>
      {children}
    </AssistantContextValue.Provider>
  )
}

// errorKind gives the kind of a failure of a question. A request that did not reach the backend throws a TypeError.
function errorKind(err: unknown): AssistantErrorKind {
  if (err instanceof ApiError) {
    switch (err.code) {
      case "not_configured":
      case "invalid_request":
      case "rate_limited":
      case "provider_failure":
        return err.code
      default:
        return "unknown"
    }
  }
  return err instanceof TypeError ? "network" : "unknown"
}

// messagesFor gives the messages that go to the backend for a new question: each answered turn before it, in
// order, and the question. A turn that failed has no answer and is left out. The backend takes at most 40 messages
// of at most 4000 characters. A longer conversation sends its newest messages, and the first of them is a question.
function messagesFor(
  history: AssistantTurn[],
  question: string
): AssistantMessage[] {
  const messages: AssistantMessage[] = []
  for (const turn of history) {
    if (turn.status !== "answered" || turn.answer.trim() === "") continue
    messages.push(
      { role: "user", content: clip(turn.question) },
      { role: "assistant", content: clip(turn.answer) }
    )
  }
  messages.push({ role: "user", content: clip(question) })
  const newest = messages.slice(-maxMessages)
  const firstQuestion = newest.findIndex((message) => message.role === "user")
  return newest.slice(firstQuestion)
}

// clip cuts a text to the length that the backend accepts. The length is in characters, not in code units.
function clip(text: string): string {
  if (text.length <= maxQuestionLength) return text
  return Array.from(text).slice(0, maxQuestionLength).join("")
}

// useAssistant gives the conversation with the Assistant.
export function useAssistant(): AssistantState {
  const state = useContext(AssistantContextValue)
  if (state === undefined) {
    throw new Error("Use useAssistant only inside an AssistantProvider.")
  }
  return state
}
