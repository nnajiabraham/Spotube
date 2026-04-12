---
name: implement-rfc
description: Implement an RFC from the repo sequentially using the Makefile, tests as gates, and browser MCP for client UI. Update the RFC checklist and implementation notes after each task. Use when the user points at an RFC and wants execution through completion.
---

# Implement RFC

Turn an RFC’s checklist into working code while keeping the RFC document the source of truth for progress and context for other agents.

## When to use

- The user references `@RFC-...md` (or equivalent) and wants implementation continued or started from a planned task list.
- The project expects builds, tests, and lint via **Makefile** targets.

## Inputs to collect

- Path to the RFC (e.g. `docs/rfcs/RFC-010-example.md`).
- Optional: task ID or description to start from; if omitted, infer from checked items and **Implementation notes**.
- Dev server base URL for UI work if not obvious (see this repo’s `AGENTS.md` or `docs/agents/dev-workflow.md`, or ask the user).

## Workflow

### 1) Orient

1. Read the RFC: summary, checklist, implementation notes, and any linked PRD sections.
2. **Do not trust existing checkmarks** until you verify behavior in code and tests; fix or uncheck stale items.
3. Run the full test suite (via Makefile) and scan project layout, schemas, and public APIs relevant to the RFC so you know current state before changing behavior.

### 2) Execute task-by-task

For each checklist item in order:

1. **Research** enough of the codebase that the change fits existing patterns and interfaces.
2. Implement the smallest coherent slice. Prefer **tests first** for new behavior: add or extend tests before marking the item complete.
3. Use **only** `make` targets for build, test, lint, format, serve, etc. (e.g. `make test`, `make lint`), not ad hoc commands, unless the Makefile is missing a needed target and the user approves an exception.
4. **Do not advance** to the next checklist item until **tests pass** for the current scope.
5. For **client-rendered UI** (SPA, React, etc.), verify behavior with the **browser MCP available in the environment** (e.g. Chrome DevTools MCP: navigate, snapshot, screenshot). Do not rely on `curl` for HTML that requires JavaScript.
6. For external libraries, rely on the repo’s declared versions, lockfiles, and **official project documentation** (vendor docs, package READMEs) when API details matter.

### 3) Update the RFC after each task

After completing a task (or a tight sub-task if the RFC splits that way):

1. Check off the item only when implementation **and** tests match the stated acceptance.
2. Append or update the **Implementation notes** section with: what changed, files touched, commands run, links to PRs/issues, and anything the next agent must know.

### 4) Debugging failing tests

- Run tests with verbose output when useful.
- Add focused logging if it helps; if logs are truncated, redirect full output to a file such as `debug.log` (overwrite each run) and inspect there.

### 5) Tracking

- Maintain an internal todo list mirroring RFC tasks and substeps; keep it in sync with the RFC checklist.

## Confirmation checkpoints

- Before large or irreversible changes, summarize the plan and align with the RFC scope.
- Before marking an item complete, confirm tests for that item are green.

## Error handling

- **Tests fail:** Fix implementation or tests; do not skip tests to proceed.
- **Checklist contradicts code:** Prefer truth in code; update RFC to reflect reality or file gaps explicitly.
