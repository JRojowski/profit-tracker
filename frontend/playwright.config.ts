import { defineConfig } from "@playwright/test"

// The screenshot tests run against the built frontend, which `vite preview` serves.
// The fake backend of the tests answers each /api request, so the Go program does not run.
// SCREENSHOT_PORT sets another port, so that two worktrees can run the tests at the same time.
const port = Number(process.env.SCREENSHOT_PORT ?? 4173)

export default defineConfig({
  testDir: "./screenshots",
  // The stored images depend on the operating system, because the fonts render differently.
  // The project records them on macOS and runs the tests locally.
  snapshotPathTemplate: "{testDir}/images/{arg}{ext}",
  outputDir: "./test-results",
  reporter: [
    ["list"],
    ["html", { open: "never", outputFolder: "playwright-report" }],
  ],
  fullyParallel: true,
  workers: 4,
  retries: 0,
  expect: {
    toHaveScreenshot: {
      animations: "disabled",
      caret: "hide",
      scale: "css",
      // The default threshold of 0.2 hides small changes of a surface. The value 0 is unstable: one anti-aliased pixel flickers between runs. The value 0.01 is stable.
      threshold: 0.01,
      // Some anti-aliased pixels still flicker between runs. Without a tolerance, an update run rewrites unchanged images.
      // A change of one character changes more than 20 pixels, so the tests still find it.
      maxDiffPixels: 20,
    },
  },
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    locale: "en-US",
    timezoneId: "Europe/Warsaw",
    deviceScaleFactor: 1,
    reducedMotion: "reduce",
    serviceWorkers: "block",
  },
  webServer: {
    command: `npm run preview -- --host 127.0.0.1 --port ${port} --strictPort`,
    url: `http://127.0.0.1:${port}`,
    reuseExistingServer: false,
  },
})
