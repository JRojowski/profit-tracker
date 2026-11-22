import { expect, test, type Page } from "@playwright/test"

import { installFakeBackend } from "./fake-backend"
import { fixedNow, type DataSet } from "./fixtures"

// Each case is one page of the tool. "empty" is the data set of its empty state, or null when the page has none.
type PageCase = {
  name: string
  url: string
  empty: DataSet | null
  // charts tells that the populated page draws a chart, so the test waits for it.
  charts: boolean
  // panel tells that the page opens the Instrument Overview panel.
  panel?: boolean
  // populatedSet is the data set of the populated state. The default is "populated".
  populatedSet?: DataSet
}

const pageCases: PageCase[] = [
  {
    name: "all-accounts",
    url: "/accounts/all",
    empty: "noAccounts",
    charts: true,
  },
  {
    name: "account-overview",
    url: "/accounts/1",
    empty: "emptyAccounts",
    charts: true,
  },
  {
    // The last values of the Value History are within 1 percent. The two end tags of the chart must move apart.
    name: "account-overview-near-equal",
    url: "/accounts/1",
    empty: null,
    charts: true,
    populatedSet: "nearEqualEnd",
  },
  {
    name: "holdings",
    url: "/accounts/1/holdings",
    empty: "emptyAccounts",
    charts: false,
  },
  {
    name: "holdings-instrument-overview",
    url: "/accounts/1/holdings?overview=BETA.US",
    empty: null,
    charts: true,
    panel: true,
  },
  {
    name: "closed-positions",
    url: "/accounts/1/closed-positions",
    empty: "emptyAccounts",
    charts: false,
  },
  {
    name: "benchmarks",
    url: "/accounts/1/benchmarks",
    empty: "emptyAccounts",
    charts: false,
  },
  {
    name: "ikze",
    url: "/accounts/2/ikze",
    empty: "emptyAccounts",
    charts: false,
  },
  { name: "import", url: "/import", empty: null, charts: false },
  { name: "imports", url: "/imports", empty: "emptyAccounts", charts: false },
  { name: "not-found", url: "/no-such-page", empty: null, charts: false },
]

const viewports = [
  { name: "1440", size: { width: 1440, height: 900 } },
  { name: "375", size: { width: 375, height: 812 } },
] as const

const themes = ["light", "dark"] as const

// waitForRenderedPage waits until the page shows data: no skeleton, the fonts loaded and the charts drawn.
async function waitForRenderedPage(
  page: Page,
  pageCase: PageCase,
  populated: boolean
) {
  await page.waitForLoadState("networkidle")
  await expect(page.locator("h1").first()).toBeVisible()
  await expect(page.locator('[data-slot="skeleton"]')).toHaveCount(0)
  if (pageCase.panel && populated) {
    await expect(page.getByRole("dialog")).toBeVisible()
  }
  if (pageCase.charts && populated) {
    await expect(page.locator(".recharts-surface").first()).toBeVisible()
  }
  await page.evaluate(() => document.fonts.ready)
}

// expectNoHorizontalScroll fails when the page scrolls sideways.
async function expectNoHorizontalScroll(page: Page) {
  const fits = await page.evaluate(
    () =>
      document.documentElement.scrollWidth <=
      document.documentElement.clientWidth
  )
  expect(fits, "the page scrolls horizontally").toBe(true)
}

for (const pageCase of pageCases) {
  for (const state of ["populated", "empty"] as const) {
    const dataSet: DataSet | null =
      state === "populated"
        ? (pageCase.populatedSet ?? "populated")
        : pageCase.empty
    if (dataSet === null) continue
    for (const theme of themes) {
      for (const viewport of viewports) {
        const name = `${pageCase.name}-${state}-${theme}-${viewport.name}`
        test(name, async ({ page }) => {
          await page.setViewportSize(viewport.size)
          await page.addInitScript((value) => {
            localStorage.setItem("theme", value)
          }, theme)
          await page.clock.setFixedTime(fixedNow)
          const failures = await installFakeBackend(page, dataSet)

          await page.goto(pageCase.url)
          await waitForRenderedPage(page, pageCase, state === "populated")

          await expectNoHorizontalScroll(page)
          await expect(page).toHaveScreenshot(`${name}.png`, { fullPage: true })
          expect(failures).toEqual([])
        })
      }
    }
  }
}

// Each sidebar case opens the Holdings page of the standard Account and puts the sidebar in one state.
const sidebarCases = [
  {
    name: "sidebar-collapsed",
    viewport: viewports[0],
    prepare: async (page: Page) => {
      await page.getByRole("button", { name: "Toggle Sidebar" }).click()
      await expect(page.locator('[data-slot="sidebar"]')).toHaveAttribute(
        "data-state",
        "collapsed"
      )
    },
  },
  {
    name: "view-switch-open",
    viewport: viewports[0],
    prepare: async (page: Page) => {
      await page.getByRole("button", { name: /Change the view/ }).click()
      await expect(page.getByRole("menu")).toBeVisible()
    },
  },
  {
    name: "sidebar-sheet",
    viewport: viewports[1],
    prepare: async (page: Page) => {
      await page.getByRole("button", { name: "Toggle Sidebar" }).click()
      await expect(page.getByRole("dialog")).toBeVisible()
    },
  },
]

for (const sidebarCase of sidebarCases) {
  for (const theme of themes) {
    const name = `${sidebarCase.name}-${theme}-${sidebarCase.viewport.name}`
    test(name, async ({ page }) => {
      await page.setViewportSize(sidebarCase.viewport.size)
      await page.addInitScript((value) => {
        localStorage.setItem("theme", value)
      }, theme)
      await page.clock.setFixedTime(fixedNow)
      const failures = await installFakeBackend(page, "populated")

      await page.goto("/accounts/1/holdings")
      await waitForRenderedPage(
        page,
        { name, url: "", empty: null, charts: false },
        true
      )
      await sidebarCase.prepare(page)
      // The sidebar slides and the sheet fades in, so wait for the settled state.
      await page.waitForTimeout(400)

      await expectNoHorizontalScroll(page)
      await expect(page).toHaveScreenshot(`${name}.png`, { fullPage: true })
      expect(failures).toEqual([])
    })
  }
}

// Each Import case uploads a file and shows the preview that the fake backend answers with.
const previewCases = [
  { name: "import-preview-known", preview: "known" },
  { name: "import-preview-new", preview: "new" },
] as const

for (const previewCase of previewCases) {
  for (const theme of themes) {
    for (const viewport of viewports) {
      const name = `${previewCase.name}-${theme}-${viewport.name}`
      test(name, async ({ page }) => {
        await page.setViewportSize(viewport.size)
        await page.addInitScript((value) => {
          localStorage.setItem("theme", value)
        }, theme)
        await page.clock.setFixedTime(fixedNow)
        const failures = await installFakeBackend(page, "populated", {
          preview: previewCase.preview,
        })

        await page.goto("/import")
        await page.locator('input[type="file"]').setInputFiles({
          name: "export.xlsx",
          mimeType:
            "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet",
          buffer: Buffer.from("synthetic"),
        })
        await page.getByRole("button", { name: "Preview", exact: true }).click()
        await expect(
          page.getByText(
            previewCase.preview === "known" ? "Known Account:" : "New Account",
            { exact: true }
          )
        ).toBeVisible()
        await page.waitForLoadState("networkidle")
        await page.evaluate(() => document.fonts.ready)

        await expectNoHorizontalScroll(page)
        await expect(page).toHaveScreenshot(`${name}.png`, { fullPage: true })
        expect(failures).toEqual([])
      })
    }
  }
}

// The sidebar states apart from a known view. Each case checks the sidebar and stores one screenshot.
for (const theme of themes) {
  test(`sidebar-loading-${theme}-1440`, async ({ page }) => {
    await page.setViewportSize(viewports[0].size)
    await page.addInitScript((value) => {
      localStorage.setItem("theme", value)
    }, theme)
    await page.clock.setFixedTime(fixedNow)
    // The skeleton of the sidebar has a random width. A fixed value keeps the image stable.
    await page.addInitScript(() => {
      Math.random = () => 0.5
    })
    let release: () => void = () => {}
    const held = new Promise<void>((resolve) => {
      release = resolve
    })
    const failures = await installFakeBackend(page, "populated", {
      holdAccounts: held,
    })

    await page.goto("/accounts/1/holdings")
    const sidebar = page.locator('[data-slot="sidebar"]')
    await expect(
      sidebar.locator('[data-slot="sidebar-menu-skeleton"]')
    ).toHaveCount(1)
    await expect(sidebar.getByText("Pages", { exact: true })).toHaveCount(0)
    await expect(sidebar.getByRole("link", { name: "Overview" })).toHaveCount(0)
    // The page body loads in parts, so the image shows the sidebar only.
    await expect(sidebar).toHaveScreenshot(`sidebar-loading-${theme}-1440.png`)
    release()
    await expect(sidebar.getByRole("link", { name: "Holdings" })).toBeVisible()
    expect(failures).toEqual([])
  })

  test(`sidebar-unknown-account-${theme}-1440`, async ({ page }) => {
    await page.setViewportSize(viewports[0].size)
    await page.addInitScript((value) => {
      localStorage.setItem("theme", value)
    }, theme)
    await page.clock.setFixedTime(fixedNow)
    const failures = await installFakeBackend(page, "populated")

    await page.goto("/accounts/999")
    await expect(page.locator("h1").first()).toHaveText("Unknown Account")
    await page.waitForLoadState("networkidle")
    const sidebar = page.locator('[data-slot="sidebar"]')
    await expect(
      sidebar.getByRole("button", { name: /All Accounts/ })
    ).toBeVisible()
    await expect(sidebar.locator('[aria-current="page"]')).toHaveCount(0)
    await sidebar.getByRole("button", { name: /Change the view/ }).click()
    await expect(page.getByRole("menuitemradio")).toHaveCount(3)
    await expect(
      page.locator('[role="menuitemradio"][aria-checked="true"]')
    ).toHaveCount(0)
    await page.keyboard.press("Escape")
    await page.evaluate(() => document.fonts.ready)
    await expect(page).toHaveScreenshot(
      `sidebar-unknown-account-${theme}-1440.png`,
      { fullPage: true }
    )
    expect(failures).toEqual([])
  })
}

test("the sidebar keeps the last view on the Import pages", async ({
  page,
}) => {
  await page.setViewportSize(viewports[0].size)
  await page.clock.setFixedTime(fixedNow)
  const failures = await installFakeBackend(page, "populated")

  await page.goto("/accounts/2/holdings")
  const sidebar = page.locator('[data-slot="sidebar"]')
  await sidebar.getByRole("link", { name: "Import", exact: true }).click()
  await expect(page).toHaveURL("/import")
  await expect(
    sidebar.getByRole("button", { name: /Retirement/ })
  ).toBeVisible()
  await expect(sidebar.getByRole("link", { name: "IKZE" })).toBeVisible()
  // Only the item of the open page is current. No item of the group "Pages" is.
  await expect(sidebar.locator('[aria-current="page"]')).toHaveText("Import")
  await sidebar.getByRole("button", { name: /Change the view/ }).click()
  await expect(
    page.locator('[role="menuitemradio"][aria-checked="true"]')
  ).toContainText("Retirement")
  expect(failures).toEqual([])
})

test("a Radix trigger button is a pill", async ({ page }) => {
  await page.setViewportSize(viewports[0].size)
  await page.clock.setFixedTime(fixedNow)
  const failures = await installFakeBackend(page, "populated")

  await page.goto("/accounts/1/closed-positions")
  for (const name of ["From date", "To date", "Instrument filter"]) {
    const button = page.getByRole("button", { name: new RegExp(`^${name}`) })
    await expect(button).toBeVisible()
    const radius = await button.evaluate(
      (element) => getComputedStyle(element).borderTopLeftRadius
    )
    expect(radius, `${name} is a pill`).not.toBe("0px")
    expect(parseFloat(radius)).toBeGreaterThan(100)
  }
  expect(failures).toEqual([])
})
