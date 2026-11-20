import type { Page, Route } from "@playwright/test"

import { fixtureFor, type ApiFixture, type DataSet } from "./fixtures"

// json answers a request with a JSON body.
function json(route: Route, body: unknown, status = 200): Promise<void> {
  return route.fulfill({
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  })
}

// answer finds the response for one API path, or undefined when the fixture has no answer.
function answer(
  fixture: ApiFixture,
  method: string,
  path: string
): { body: unknown; status?: number } | undefined {
  if (method !== "GET") return undefined
  const fixed: Record<string, unknown> = {
    "/api/health": { status: "ok" },
    "/api/assistant": { available: false },
    "/api/accounts": fixture.accounts,
    "/api/all-accounts/summary": fixture.allSummary,
    "/api/all-accounts/allocation": fixture.allAllocation,
    "/api/all-accounts/value-history": fixture.allHistory,
    "/api/imports": fixture.imports,
    "/api/prices/refresh/progress": {
      running: false,
      done: 0,
      total: 0,
      ticker: null,
    },
  }
  if (path in fixed) return { body: fixed[path] }

  const account = /^\/api\/accounts\/(\d+)\/([a-z-]+)$/.exec(path)
  if (account) {
    const id = Number(account[1])
    const byId: Record<string, Record<number, unknown>> = {
      summary: fixture.summary,
      allocation: fixture.allocation,
      "value-history": fixture.history,
      holdings: fixture.holdings,
      "closed-positions": fixture.closedPositions,
      "ikze-limit": fixture.ikzeLimit,
    }
    const body = byId[account[2]]?.[id]
    return body === undefined ? undefined : { body }
  }

  const overview =
    /^\/api\/accounts\/(\d+)\/instruments\/([^/]+)\/overview$/.exec(path)
  if (overview) {
    const body =
      fixture.overview[`${overview[1]}:${decodeURIComponent(overview[2])}`]
    return body === undefined
      ? { body: { error: "Instrument not found." }, status: 404 }
      : { body }
  }
  return undefined
}

// installFakeBackend answers each /api request from the fixture of the data set. The backend does not run.
// A request that the fixture cannot answer, or a request to another origin, makes the page fail.
// It returns the list of the failures. The test checks that the list is empty.
export async function installFakeBackend(
  page: Page,
  dataSet: DataSet
): Promise<string[]> {
  const fixture = fixtureFor(dataSet)
  const failures: string[] = []
  await page.route("**/*", (route) => {
    const url = new URL(route.request().url())
    if (url.hostname !== "127.0.0.1") {
      failures.push(`external request: ${url.href}`)
      return route.abort()
    }
    if (!url.pathname.startsWith("/api/")) return route.continue()
    const found = answer(fixture, route.request().method(), url.pathname)
    if (found === undefined) {
      failures.push(`unanswered: ${route.request().method()} ${url.pathname}`)
      return json(route, { error: "No fixture for this request." }, 404)
    }
    return json(route, found.body, found.status)
  })
  return failures
}
