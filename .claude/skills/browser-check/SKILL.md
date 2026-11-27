---
name: browser-check
description: Checks a frontend change in the running profit-tracker app with the Playwright MCP tools, in both themes and at two widths. Use after each change to a page in frontend/, and when you verify a UI ticket.
---

# Browser check

The screenshot tests (`make frontend-screenshots`) compare the look of each page with stored images. They do not test the behaviour. This check is the only check of the behaviour, so do not skip it.

Do this check after each change to a page.

1. Run `make demo`. It starts the tool on `127.0.0.1:8099` with a new test database and synthetic data. Never use the files in `exports/`. They hold personal data.
2. Optional: for fast reloads, run `PROFIT_TRACKER_ADDR=127.0.0.1:8099 make frontend-dev`.
3. Open each changed route with the Playwright MCP tools. Save each screenshot in `.playwright-mcp/` in the repository root. The Playwright tools do not write to other folders.
4. Check each route in four states: the dark theme and the light theme, at a width of 1440 px and at 375 px. The theme switch is the button at the right of the top bar.
5. At 375 px, run `document.documentElement.scrollWidth <= document.documentElement.clientWidth` in the page. The result must be `true`.
6. Read the console messages. There must be no errors, except the errors of a backend that you stopped on purpose.
7. Do each acceptance criterion of the ticket in the browser. Look at each screenshot and compare it with the mockup.
8. Click "Refresh prices" one time. The values must stay on the screen while the new values load.

Stop the demo server when you are done, with the command that `make demo` prints.
