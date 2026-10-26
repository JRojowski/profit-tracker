---
name: frontend-standard
description: Frontend conventions for the profit-tracker web app (React, TypeScript, Vite, Tailwind, shadcn/ui, Recharts). Use when you write, review, or refactor code in frontend/, or when you check a UI change in the browser.
---

# Frontend standard

The frontend is one Vite app in `frontend/`. The Go binary embeds the build and serves `index.html` for each client route.

## Completion criterion

Your frontend work is done only when both checks pass:

1. `make frontend-check` passes with zero errors. It runs the typecheck, eslint, the prettier check and the build.
2. The browser check in "Check in the browser" passes for each flow that you changed.

To format the code, run `cd frontend && npm run format`. If `npx <tool>` fails with "could not determine executable", run the tool from `frontend/node_modules/.bin/`.

## Stack

| Concern | Use |
|---|---|
| UI | React 19 with TypeScript |
| Build | Vite |
| Styling | Tailwind CSS v4. The tokens are in `src/index.css`. |
| Components | shadcn/ui, style `radix-nova`, in `src/components/ui/` |
| Routing | `react-router` with `createBrowserRouter` in `src/App.tsx` |
| Charts | The shadcn `chart` component, which uses Recharts |
| Icons | `lucide-react` |
| Fonts | Geist for text. Geist Mono for numbers, badges and small section labels. |

Add a new package only when the current stack cannot do the job in a few lines. Write the reason in the commit message.

## Layout

- `src/components/ui/`: shadcn components. Do not edit them by hand.
- `src/components/`: business components, one topic for each file.
- `src/pages/`: one component for each route.
- `src/hooks/`: React hooks.
- `src/lib/`: the API client (`api.ts`), the shared state (`app-state.tsx`), the routes (`routes.ts`), the formatting (`format.ts`).

## shadcn components

1. Before you import a component, make sure that its file is in `src/components/ui/`.
2. To add a component, run `yes n | npx shadcn@latest add <name>` in `frontend/`. The `yes n` keeps the components that already exist. Without it, the command stops at the overwrite question.
3. Use the parts of a component in their structure: `Card` with `CardHeader`, `CardTitle` and `CardContent`. Give each `Dialog` and `Sheet` a title.
4. Add `data-icon="inline-start"` or `data-icon="inline-end"` to each icon in a `Button`.
5. Use `Badge` for a status or a tag, `Separator` for a divider, `Skeleton` for a load, and `Alert` for a warning.

The files in `ui/` import `cn` from the package `"cn"`. Business code imports `cn` from `@/lib/utils`. Use `cn()` only to merge class names.

## Styling

1. Use only the semantic tokens of `src/index.css`. Never write a colour value or a palette class such as `bg-sky-500` in a component.
2. Do not use the `dark:` variant. When a colour is wrong in one theme, fix the token in `src/index.css` for both themes.
3. Use `flex` or `grid` with `gap-*`. Do not use `space-x-*` or `space-y-*`.
4. Give each number the `num` class. It sets Geist Mono, tabular figures and no line break.
5. `CardTitle` has the class `font-heading`, and that class wins over `num`. For a number in a `CardTitle`, write `num font-mono`.

The project tokens in addition to the shadcn tokens:

| Token | Use |
|---|---|
| `positive` | A profit, a result above zero, the IKZE tag |
| `negative` | A loss, a result below zero |
| `primary` | The accent: the selected item, the Total Result bar |
| `chart-1` to `chart-8` | The parts of a chart, in order of value |
| `chart-cash` | The cash part of a chart |
| `chart-other` | The "Other" part of a chart |

## Numbers and words

1. Show each amount with `formatAmount(amount, currency)` and each percentage with `formatPercent(percent)` from `src/lib/format.ts`.
2. For a profit, a loss or a result, add `{ signed: true }`. Then a value above zero shows "+" and a value below zero shows "−" (U+2212).
3. Never show colour as the only signal of a profit or a loss. The sign must also show.
4. Never calculate with the result of `Number(amount)`. The backend sends exact decimal text. The frontend rounds only for display.
5. Write only the terms of `GLOSSARY.md` on the screen. Do not use "TWR", "MWR", "IRR", "FX Rate", "Final Net", "P/L", "position" or "portfolio".
6. Write all screen text in English.

## Data loading

1. Load backend data with `useLoad(scope, version, load)` from `src/hooks/use-load.ts`. The scope must name all inputs of `load`, for example the Account id.
2. Use `valuesVersion` from `useAppState()` as the version. It changes after each price refresh and each Import. Then the hook loads again and keeps the old data on the screen until the new data arrives.
3. Show the result with `LoadedView`. It shows a skeleton while the data loads and an alert when the load fails.
4. Do not write a `useEffect` that fetches data in a page.
5. Declare all hooks before the first early return.
6. After a change of a price source, call `refreshAllPrices()`. After an Import, call `reloadAccounts()`.

## Routes

1. Build each URL with `paths` from `src/lib/routes.ts`. Do not write a URL as text in a component.
2. A tab page of an Account gets its Account from `useAccount()`.
3. Use `Link` or `NavLink` for navigation, so that the back button works.

## Layout and narrow screens

1. The layout must work from a width of 375 px. The page must not scroll horizontally.
2. Give `min-w-0` to a flex child that holds a wide table or a long text.
3. On a narrow screen, stack the cards in one column. The breakpoint `md` (768 px) separates the narrow layout from the wide layout.
4. The mockups in `docs/design/v3/` show each page. They also contain words and defects that spec #19 tells you not to copy.

## Charts

1. Use the shadcn `chart` component. Give the chart colours through the `ChartConfig` with the tokens of the table above.
2. Each chart has a text alternative: a legend with exact values, or a row of exact amounts.

## Comments

Write a comment above each exported component, hook, function and type. Write it in full sentences, in active voice, at most 25 words for each sentence. Match the density of the comments in the files around it.

## Check in the browser

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
