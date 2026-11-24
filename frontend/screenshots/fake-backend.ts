import type { Page, Route } from "@playwright/test"

import {
  fixtureFor,
  importPreviews,
  tiersRejected,
  unsupportedEntriesError,
  type ApiFixture,
  type DataSet,
} from "./fixtures"

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
): { body: unknown; status?: number; failed?: boolean } | undefined {
  if (method !== "GET") return undefined
  const fixed: Record<string, unknown> = {
    "/api/health": { status: "ok" },
    "/api/assistant": { available: false },
    "/api/accounts": fixture.accounts,
    "/api/all-accounts/summary": fixture.allSummary,
    "/api/all-accounts/allocation": fixture.allAllocation,
    "/api/all-accounts/value-history": fixture.allHistory,
    "/api/all-accounts/benchmarks": fixture.allBenchmarks,
    "/api/imports": fixture.imports,
    "/api/tiers": fixture.tiers,
    "/api/all-accounts/rebalancing": fixture.rebalancing,
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
      benchmarks: fixture.benchmarks,
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
      ? { body: { error: "Instrument not found." }, status: 404, failed: true }
      : { body }
  }
  return undefined
}

// installFakeBackend answers each /api request from the fixture of the data set. The backend does not run.
// A request that the fixture cannot answer, or a request to another origin, makes the page fail.
// An answer with an error status that the case does not expect, a script error and a console error make the page fail too.
// The option holdAccounts keeps the answer of /api/accounts until the promise resolves, to show the loading state.
// The option preview answers an upload with a preview.
// The option unsupportedEntries answers an upload with the HTTP 400 error of a Kraken export with unsupported entries.
// It returns the list of the failures. The test checks that the list is empty.
export async function installFakeBackend(
  page: Page,
  dataSet: DataSet,
  options: {
    holdAccounts?: Promise<void>
    // preview is the answer of an upload.
    preview?: keyof typeof importPreviews
    unsupportedEntries?: boolean
    // tiersRejected answers a save of the Tiers with the 422 error.
    tiersRejected?: boolean
  } = {}
): Promise<string[]> {
  const fixture = fixtureFor(dataSet)
  const failures: string[] = []
  page.on("pageerror", (error) => failures.push(`page error: ${error.message}`))
  page.on("console", (message) => {
    if (message.type() !== "error") return
    // The browser logs the expected HTTP 400 answer of the upload as an error. That error is part of the case.
    const expected =
      options.unsupportedEntries &&
      new URL(message.location().url, "http://127.0.0.1").pathname ===
        "/api/imports"
    const expectedTiers =
      options.tiersRejected &&
      new URL(message.location().url, "http://127.0.0.1").pathname ===
        "/api/tiers"
    if (!expected && !expectedTiers)
      failures.push(`console error: ${message.text()}`)
  })
  await page.route("**/*", async (route) => {
    const url = new URL(route.request().url())
    if (url.hostname !== "127.0.0.1") {
      failures.push(`external request: ${url.href}`)
      return route.abort()
    }
    if (!url.pathname.startsWith("/api/")) return route.continue()
    if (
      options.preview &&
      route.request().method() === "POST" &&
      url.pathname === "/api/imports"
    ) {
      return json(route, importPreviews[options.preview], 201)
    }
    if (
      options.unsupportedEntries &&
      route.request().method() === "POST" &&
      url.pathname === "/api/imports"
    ) {
      return json(route, unsupportedEntriesError, 400)
    }
    if (
      options.tiersRejected &&
      route.request().method() === "PUT" &&
      url.pathname === "/api/tiers"
    ) {
      return json(route, tiersRejected, 422)
    }
    const found = answer(fixture, route.request().method(), url.pathname)
    if (found === undefined) {
      failures.push(`unanswered: ${route.request().method()} ${url.pathname}`)
      return json(route, { error: "No fixture for this request." }, 404)
    }
    if (found.failed) {
      failures.push(`error answer: ${url.pathname}`)
    }
    if (url.pathname === "/api/accounts" && options.holdAccounts) {
      await options.holdAccounts
    }
    return json(route, found.body, found.status)
  })
  return failures
}
