# Spec overlay

This file extends the `to-spec` template. Read it each time you write a spec.

## Rules

- Only add. Do not change or remove a `to-spec` section. Keep User Stories. Keep file paths out. Keep the `ready-for-agent` label.
- Add the sections of the list below. Put `## Overview` first, before `## Problem Statement`. Put the other added sections after `## Implementation Decisions`.
- If an enabled section does not apply, write `N/A — <reason>`. The reason is mandatory.
- Write the spec in English, in ASD-STE100 style: active voice, one instruction per sentence, max 25 words per sentence.
- If the task came as issue `#N`, publish the spec as a GitHub sub-issue of `#N`. Do not edit the text of `#N`.
- Run `/pocock-harness:verify-spec <issue>` after you publish.

## Enabled sections

- Overview
- Screens
- Code changes
- Contracts
- Data model
- Configuration
- Invariants and failure modes

## Section formats

Each section is a `## <name>` heading. `verify-spec` checks the formats below.

### Overview

Write one or two Mermaid diagrams, then a legend. Or write `none — <reason>`.

- Slice map: use `flowchart LR` for 3 or more changes, or for an HTTP, DB or queue boundary. One node per module or external system. Quote each label: `["..."]`. Mark a changed node `:::change` and an external system `:::ext`. Label an edge with its boundary: `-- HTTP -->`. Max 15 nodes.
- Flow: use `sequenceDiagram` when the runtime behaviour changes. End a new message with `(new)` and a changed message with `(changed)`. Write a removed step as `Note over <participant>: removed - <step>`. Max 12 messages.
- Label text has no `;`, `#`, `"` or brackets. Declare `classDef change stroke:#e8590c,stroke-width:3px` and `classDef ext stroke-dasharray:5 5` once, at the end of the flowchart. Do not use fill.
- Draw only what the spec text states.

### Screens

One `###` heading per screen. List its states and its user actions. Do not use pixel details.

### Code changes

Group the changes by module. Use one `### <module>` heading per group, then one bullet per change. Mark a new name with `(new)`. Do not write file paths.

### Contracts

Write one `### <name>` heading per contract, with these three lines:

```
### <name>

Request: <method and path, or message shape>
Response: <success shape>
Errors:
- 404: <case>
- status: <value>: <case, for a non-HTTP contract>
```

`Errors:` lists each case with a status. Use an HTTP code (`404`) or `status: <value>`.

### Data model

List each new or changed entity, with its fields, types and constraints. Mark a new name with `(new)`.

### Configuration

List each setting by name, and where the value comes from: an env var, Key Vault, App Configuration. Never write a secret value. Write `<placeholder>` or the name of the source.

```
- ORDER_API_TOKEN — Key Vault secret `order-api-token`
- ORDER_TIMEOUT_S — env var, default 30
```

### Invariants and failure modes

List the invariants as `- INV-1: <text>`. Then write one table with one row per invariant id:

```
| Invariant | Failure | Detection | Response |
|-----------|---------|-----------|----------|
| INV-1 | <what breaks> | <how we see it> | <what the system does> |
```
