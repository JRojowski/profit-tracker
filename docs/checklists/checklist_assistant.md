# Manual test checklist: Assistant

Spec: #63. Ticket: #70.

No automatic test calls the real model, and a free model can select a wrong function or a wrong period. This checklist is the only check of the answers of the real model.

Run this checklist again after each change of the model: a change of `PROFIT_TRACKER_ASSISTANT_MODEL`, a change of the default model, or a change of the instructions.

Warning: the Assistant sends amounts, Instruments, dates and Account names to the provider. For a first run, use `make demo`. It has synthetic data. Never use the files in `exports/` for the demo.

## 0. Setup

- [ ] Get an API key from OpenRouter. Put it in the environment of the shell only. Never write it in a file of the repository.

  ```bash
  export PROFIT_TRACKER_OPENROUTER_KEY=<key>
  make demo
  ```

- [ ] To use another model, add `PROFIT_TRACKER_ASSISTANT_MODEL=<model ID>` before `make demo`. Write the model ID in the result table at the end of this file.
- [ ] Open the URL that `make demo` prints. The top bar and the sidebar have the control "Assistant".
- [ ] Stop the tool. Start it again without the key (`env -u PROFIT_TRACKER_OPENROUTER_KEY`). No "Assistant" control shows on any page.
- [ ] A free model allows 20 requests per minute and 50 requests per day. One question uses two or more requests. Plan at most 20 questions for each day.

## 1. Questions

For each question: open the Assistant, ask the question in English, and compare each number of the answer with the screen in the list. Open the Account that the question names. A step under the answer must name the function that the question needs.

Check these rules for each answer:

- [ ] Each number agrees with the screen, to the last digit that the screen shows.
- [ ] Each amount has its currency. No sum joins two currencies.
- [ ] The names are the names of the screens: "Total Result", "Holding", "Closed Position". No "P/L", "position" or "portfolio".
- [ ] The answer holds no HTML tag and no markdown table.
- [ ] The steps under the answer show the Account, the Instrument and the period that you asked for.

| # | Question | Step | Screen with the expected numbers |
|---|---|---|---|
| 1 | "What is my Total Result, and what are its parts?" | Results | Overview of the Account: the cards "Total Result" and "Annual Return", and the waterfall of the Total Result. |
| 2 | "What are my Simple Return, my Annual Return and my Time-Weighted Return?" | Results | Overview of the Account: the cards, and the Time-Weighted Return of the Value History card. For a return that is not available, the answer gives the reason that the screen gives. It gives no number. |
| 3 | "What was my Total Result in 2025?" (use a year that the Account has) | Results with a period | Value History card: the value at the end of 2024 and at the end of 2025. The Total Result of the year is the change of the value, minus the Net Deposits of the year. |
| 4 | "What is the Unrealized Profit of my Holdings?" | Holdings | Holdings page: the column "Unrealized Profit" and the row "Total". A Holding with no known value is named as such. |
| 5 | "Which three Closed Positions were the worst?" | Closed Positions with "worst 3" | Closed Positions page: sort by "Realized Profit", ascending. |
| 6 | "What was my Realized Profit on `<ticker>` in 2025?" | Closed Positions with the Instrument and the period | Closed Positions page: the filter by Instrument and by period, and the sum of the rows. |
| 7 | "How much did I get in net dividends on `<ticker>` in 2025, and what are my fees?" | Income and fees | The export file of the Account, opened in a spreadsheet: sum the dividend rows of the Instrument and the fee rows of the period. For the whole time, the waterfall on the Overview page shows the totals. |
| 8 | "What is my Allocation?" (ask it on the page "All Accounts") | Allocation and Net Deposits | The Allocation card of "All Accounts". The answer keeps the sums in PLN. |
| 9 | "How much of the IKZE Limit is left in 2025, and what are my Net Deposits?" (ask it on the IKZE Account) | Allocation and Net Deposits with the year | IKZE page: the IKZE Limit card. Overview: Net Deposits. |
| 10 | "Should I buy more `<ticker>`?" | None | No screen. The Assistant says that it gives no advice about a purchase or a sale. It names no price target and gives no estimate. |

## 2. Conversation and page context

- [ ] Ask question 6 for one Instrument. Then ask "And in 2024?". The answer uses the same Instrument. The steps show the new period.
- [ ] Open the Instrument Overview of an Instrument. Open the Assistant with the control "Ask the Assistant about this Instrument" in the header of the Instrument Overview. Under the input, the line "Open page" names the Account and the Instrument. Ask "How much did I earn on this Instrument?". The first step shows the ticker of the open Instrument.
- [ ] Ask the same question on a page with no Account ("All Accounts"). The first step shows "All Accounts", or the Assistant asks which Account you mean. The step never names an Account that you did not name.
- [ ] Ask a question in Polish. The answer is in Polish. The terms of the tool ("Total Result", "Holding") stay in English.
- [ ] Ask "Which Instrument will grow most?". The Assistant says that it cannot answer. It gives no number.
- [ ] Move to another page with the Assistant open. The conversation stays. Reload the page. The conversation is gone and the example questions show.

## 3. Errors

Use the fake provider or stop the network for these steps. Do not use up the daily limit of your key.

- [ ] Start the tool with a wrong key. The question gives the message "The provider does not answer" or "The provider refuses the question". The question stays on the screen and the control "Send again" shows. No message and no line of `server.log` holds the key.
- [ ] Stop the tool during a question. The message "The tool is not reachable" shows. Start the tool and click "Send again". The answer shows.
- [ ] After the daily limit of a free model, the message says that the daily limit of the free model can be the cause.

## 4. Result of the run

Copy this table below for each run.

| Date | Model ID | Questions that failed | Notes |
|---|---|---|---|
| | | | |
