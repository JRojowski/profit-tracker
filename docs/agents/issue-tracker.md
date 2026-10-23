# Issue tracker: GitHub Issues

Issues and specs for this repo live as GitHub Issues in `JRojowski/profit-tracker`. The repository is private. Use the `gh` CLI for all tracker operations.

## Conventions

- A spec is one issue with the label `spec`. Its title starts with `Spec:`.
- The spec body ends with a `## Tickets` task list. The list has one item per ticket, for example `- [ ] #7`.
- An implementation ticket is one issue. Never put two or more tickets in one issue.
- The first line of a ticket body is `**Parent spec:** #N`.
- A `**Blocked by:** #N, #N` line records the blockers. A ticket with no blockers has `**Blocked by:** None`.
- The triage state is a label. See `triage-labels.md` for the label strings.
- A done ticket is a closed issue with the reason "completed". When you close it, tick its item in the spec task list.
- Comments and conversation history are issue comments.

## When a skill says "publish to the issue tracker"

Create an issue with `gh issue create --title "<title>" --body-file <file> --label <label>`. Add the parent spec line and the blocker line to the body. Then add the new issue to the task list of its spec.

## When a skill says "fetch the relevant ticket"

Run `gh issue view <number> --comments`. The user normally gives the issue number.

## When a commit finishes a ticket

Put `Closes #<number>` in the commit message. GitHub closes the issue when the commit reaches `main`.

## Wayfinding operations

Used by `/wayfinder`. The **map** is one issue with one **child** issue per ticket.

- **Map**: an issue with the label `map`. Its body holds the Notes, Decisions-so-far and Fog sections.
- **Child ticket**: an issue with the question in the body. The first line of the body is `**Map:** #N`. A `Type:` line records the ticket type (`research`/`prototype`/`grilling`/`task`).
- **Blocking**: a `Blocked by: #N, #N` line near the top. A ticket is unblocked when each issue in that line is closed.
- **Frontier**: run `gh issue list --search "Map: #N in:body" --state open`. Select the issues that are unblocked and have no assignee. The lowest number wins.
- **Claim**: before any work, run `gh issue edit <number> --add-assignee @me`.
- **Resolve**: add the answer as a comment that starts with `## Answer`. Close the issue. Then add a context pointer (gist + link) to the Decisions-so-far section of the map.
