import {
  createContext,
  useCallback,
  useContext,
  useEffect,
  useMemo,
  useState,
  type ReactNode,
} from "react"

import {
  errorMessage,
  fetchAccounts,
  fetchHealth,
  fetchRefreshProgress,
  refreshPrices,
  type Account,
  type PriceRefresh,
  type RefreshProgress,
} from "@/lib/api"

// progressInterval is the time in milliseconds between two reads of the progress of a running price refresh.
const progressInterval = 1000

// PriceRefreshState holds the result of the last refresh of the prices and the Exchange Rates.
export type PriceRefreshState = {
  refreshing: boolean
  // progress is the last known progress of the running refresh, or null.
  progress: RefreshProgress | null
  // result is the result of the last price refresh, or null before the first refresh.
  result: PriceRefresh | null
  // refreshedAt is the time of the last successful price refresh. A repeated refresh with the same prices
  // changes only this time, so the user sees that the refresh ran.
  refreshedAt: Date | null
  // error is the error of the last price refresh, or null.
  error: string | null
}

type AppState = {
  // accounts is null until the first load ends.
  accounts: Account[] | null
  accountsError: string | null
  // backendUnavailable is true when the health check fails.
  backendUnavailable: boolean
  // valuesVersion changes after each price refresh and each Import. A view that shows values loads them again
  // when it changes.
  valuesVersion: number
  priceRefresh: PriceRefreshState
  reloadAccounts: () => void
  replaceAccount: (account: Account) => void
  refreshAllPrices: () => void
}

const AppStateContext = createContext<AppState | undefined>(undefined)

// AppStateProvider holds the state that the app shell and all pages share.
export function AppStateProvider({ children }: { children: ReactNode }) {
  const [accounts, setAccounts] = useState<Account[] | null>(null)
  const [accountsError, setAccountsError] = useState<string | null>(null)
  // accountsVersion changes when the list of Accounts must load again, for example after an Import.
  const [accountsVersion, setAccountsVersion] = useState(0)
  const [backendUnavailable, setBackendUnavailable] = useState(false)
  const [valuesVersion, setValuesVersion] = useState(0)
  const [priceRefresh, setPriceRefresh] = useState<PriceRefreshState>({
    refreshing: false,
    progress: null,
    result: null,
    refreshedAt: null,
    error: null,
  })

  useEffect(() => {
    const controller = new AbortController()
    fetchHealth(controller.signal)
      .then(() => setBackendUnavailable(false))
      .catch(() => {
        if (!controller.signal.aborted) setBackendUnavailable(true)
      })
    return () => controller.abort()
  }, [])

  useEffect(() => {
    const controller = new AbortController()
    fetchAccounts(controller.signal)
      .then((list) => {
        setAccounts(list)
        setAccountsError(null)
      })
      .catch((err: unknown) => {
        if (controller.signal.aborted) return
        setAccountsError(errorMessage(err))
      })
    return () => controller.abort()
  }, [accountsVersion])

  // reloadAccounts loads the Accounts and all values again, for example after an Import.
  const reloadAccounts = useCallback(() => {
    setAccountsVersion((version) => version + 1)
    setValuesVersion((version) => version + 1)
  }, [])

  const replaceAccount = useCallback((renamed: Account) => {
    setAccounts(
      (list) =>
        list
          ?.map((account) => (account.id === renamed.id ? renamed : account))
          .sort(byNameThenId) ?? null
    )
  }, [])

  const refreshAllPrices = useCallback(() => {
    setPriceRefresh((state) => ({ ...state, refreshing: true, progress: null }))
    // While the refresh waits, the tool reads its progress about once each second.
    const polling = new AbortController()
    const readProgress = () => {
      fetchRefreshProgress(polling.signal)
        .then((progress) => {
          if (polling.signal.aborted) return
          setPriceRefresh((state) =>
            state.refreshing ? { ...state, progress } : state
          )
        })
        .catch(() => {
          // A failed read of the progress is not important. The next read can work.
        })
    }
    const timer = window.setInterval(readProgress, progressInterval)
    readProgress()
    const stopPolling = () => {
      window.clearInterval(timer)
      polling.abort()
    }
    refreshPrices()
      .then((result) => {
        stopPolling()
        setPriceRefresh({
          refreshing: false,
          progress: null,
          result,
          refreshedAt: new Date(),
          error: null,
        })
        setValuesVersion((version) => version + 1)
      })
      .catch((err: unknown) => {
        stopPolling()
        setPriceRefresh({
          refreshing: false,
          progress: null,
          result: null,
          refreshedAt: null,
          error: errorMessage(err),
        })
      })
  }, [])

  const value = useMemo(
    () => ({
      accounts,
      accountsError,
      backendUnavailable,
      valuesVersion,
      priceRefresh,
      reloadAccounts,
      replaceAccount,
      refreshAllPrices,
    }),
    [
      accounts,
      accountsError,
      backendUnavailable,
      valuesVersion,
      priceRefresh,
      reloadAccounts,
      replaceAccount,
      refreshAllPrices,
    ]
  )

  return (
    <AppStateContext.Provider value={value}>
      {children}
    </AppStateContext.Provider>
  )
}

// useAppState gives the state that the app shell and all pages share.
export function useAppState(): AppState {
  const state = useContext(AppStateContext)
  if (state === undefined) {
    throw new Error("Use useAppState only inside an AppStateProvider.")
  }
  return state
}

// byNameThenId sorts the Accounts in the order of GET /api/accounts.
function byNameThenId(a: Account, b: Account): number {
  if (a.name !== b.name) return a.name < b.name ? -1 : 1
  return a.id - b.id
}
