import { expect, test } from "@playwright/test"

import { switchViewPath, type ViewTarget } from "../src/lib/routes"

const all: ViewTarget = { kind: "all" }
const standard: ViewTarget = {
  kind: "account",
  account: { id: 1, type: "standard" },
}
const ikze: ViewTarget = { kind: "account", account: { id: 2, type: "IKZE" } }

// Each case is one change of the view: the page that is open, the new view and the page that must open.
const cases: { from: string; to: ViewTarget; expected: string }[] = [
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
  // The IKZE page exists only in an IKZE Account.
  { from: "/accounts/2/ikze", to: standard, expected: "/accounts/1" },
  { from: "/accounts/1/", to: ikze, expected: "/accounts/2" },
  // The pages of the Import group are not pages of a view.
  { from: "/import", to: standard, expected: "/accounts/1" },
  { from: "/imports", to: all, expected: "/accounts/all" },
  { from: "/no-such-page", to: ikze, expected: "/accounts/2" },
]

for (const { from, to, expected } of cases) {
  test(`view switch from ${from} to ${JSON.stringify(to)}`, () => {
    expect(switchViewPath(from, to)).toBe(expected)
  })
}
