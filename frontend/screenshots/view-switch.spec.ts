import { expect, test } from "@playwright/test"

import { switchViewPath, type ViewTarget } from "../src/lib/routes"

const all: ViewTarget = { kind: "all" }
const standard: ViewTarget = {
  kind: "account",
  account: { id: 1, type: "regular" },
}
const ike: ViewTarget = { kind: "account", account: { id: 3, type: "IKE" } }
const ikze: ViewTarget = { kind: "account", account: { id: 2, type: "IKZE" } }

// Each case is one change of the view: the page that is open, the new view and the page that must open.
const cases: {
  from: string
  search?: string
  to: ViewTarget
  expected: string
}[] = [
  { from: "/accounts/1", to: all, expected: "/accounts/all" },
  { from: "/accounts/1/holdings", to: all, expected: "/accounts/all" },
  { from: "/accounts/1/closed-positions", to: all, expected: "/accounts/all" },
  { from: "/accounts/2/ikze", to: all, expected: "/accounts/all" },
  { from: "/accounts/all", to: standard, expected: "/accounts/1" },
  { from: "/", to: ikze, expected: "/accounts/2" },
  { from: "/accounts/1", to: ikze, expected: "/accounts/2" },
  { from: "/accounts/1/holdings", to: ikze, expected: "/accounts/2/holdings" },
  {
    from: "/accounts/2/closed-positions",
    to: standard,
    expected: "/accounts/1/closed-positions",
  },
  // The Benchmarks page stays the Benchmarks page between Accounts.
  {
    from: "/accounts/1/benchmarks",
    to: ikze,
    expected: "/accounts/2/benchmarks",
  },
  {
    from: "/accounts/2/benchmarks",
    to: standard,
    expected: "/accounts/1/benchmarks",
  },
  {
    from: "/accounts/1/benchmarks",
    to: all,
    expected: "/accounts/all/benchmarks",
  },
  {
    from: "/accounts/all/benchmarks",
    to: standard,
    expected: "/accounts/1/benchmarks",
  },
  {
    from: "/accounts/all/benchmarks",
    to: ikze,
    expected: "/accounts/2/benchmarks",
  },
  // The period of the query string stays on the Benchmarks page.
  {
    from: "/accounts/1/benchmarks",
    search: "?period=1Y",
    to: all,
    expected: "/accounts/all/benchmarks?period=1Y",
  },
  {
    from: "/accounts/all/benchmarks",
    search: "?period=3M",
    to: ikze,
    expected: "/accounts/2/benchmarks?period=3M",
  },
  // Other pages drop the query string.
  {
    from: "/accounts/1/closed-positions",
    search: "?period=1Y",
    to: all,
    expected: "/accounts/all",
  },
  { from: "/accounts/all", to: all, expected: "/accounts/all" },
  // The IKZE page exists only in an IKZE Account.
  { from: "/accounts/2/ikze", to: standard, expected: "/accounts/1" },
  { from: "/accounts/1/", to: ikze, expected: "/accounts/2" },
  // The IKZE page does not exist in an IKE Account.
  { from: "/accounts/2/ikze", to: ike, expected: "/accounts/3" },
  { from: "/accounts/1/holdings", to: ike, expected: "/accounts/3/holdings" },
  // The pages of the Import group are not pages of a view.
  { from: "/import", to: standard, expected: "/accounts/1" },
  { from: "/imports", to: all, expected: "/accounts/all" },
  { from: "/no-such-page", to: ikze, expected: "/accounts/2" },
]

for (const { from, search, to, expected } of cases) {
  test(`view switch from ${from}${search ?? ""} to ${JSON.stringify(to)}`, () => {
    expect(switchViewPath(from, to, search)).toBe(expected)
  })
}
