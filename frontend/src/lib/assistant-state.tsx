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
  askAssistant,
  fetchAssistantAvailable,
  type AssistantMessage,
  type AssistantStep,
} from "@/lib/api"

// AssistantTurn is one question of the user and what the Assistant did with it.
export type AssistantTurn = {
  id: number
  question: string
  // status is "pending" while the Assistant works. A "failed" turn has no answer.
  status: "pending" | "answered" | "failed"
  answer: string
  steps: AssistantStep[]
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
  // ask sends one question. It does nothing when the text is empty or another question waits.
  ask: (question: string) => void
}

const AssistantContext = createContext<AssistantState | undefined>(undefined)

// AssistantProvider holds the conversation with the Assistant above the routes. The conversation stays
// while the user moves between pages, and it ends when the tab reloads.
export function AssistantProvider({ children }: { children: ReactNode }) {
  const [available, setAvailable] = useState(false)
  const [open, setOpen] = useState(false)
  const [turns, setTurns] = useState<AssistantTurn[]>([])
  const nextId = useRef(1)
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

  const ask = useCallback(
    (question: string) => {
      const text = question.trim()
      if (text === "" || asking) return
      const id = nextId.current++
      setTurns((list) => [
        ...list,
        { id, question: text, status: "pending", answer: "", steps: [] },
      ])
      const finish = (change: Partial<AssistantTurn>) =>
        setTurns((list) =>
          list.map((turn) => (turn.id === id ? { ...turn, ...change } : turn))
        )
      askAssistant(messagesFor(text))
        .then((result) =>
          finish({
            status: "answered",
            answer: result.answer,
            steps: result.steps,
          })
        )
        .catch(() => finish({ status: "failed" }))
    },
    [asking]
  )

  const value = useMemo(
    () => ({ available, open, setOpen, turns, asking, ask }),
    [available, open, turns, asking, ask]
  )

  return (
    <AssistantContext.Provider value={value}>
      {children}
    </AssistantContext.Provider>
  )
}

// messagesFor gives the messages that go to the backend for a new question. Today it is the question alone.
// A later ticket adds the earlier questions and answers here.
function messagesFor(question: string): AssistantMessage[] {
  return [{ role: "user", content: question }]
}

// useAssistant gives the conversation with the Assistant.
export function useAssistant(): AssistantState {
  const state = useContext(AssistantContext)
  if (state === undefined) {
    throw new Error("Use useAssistant only inside an AssistantProvider.")
  }
  return state
}
