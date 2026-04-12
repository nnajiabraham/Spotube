---
name: implement-phased-learning-project
description: Implement a phased learning project from existing spec, architecture, and plan docs using TDD and Makefile commands. Use when starting or continuing multi-phase build-a-project-to-learn work.
---

# Implement phased learning project

Execute phases from `docs/[project]-design-and-phases.md` while updating learning artifacts and keeping tests green.

## When to use

- Spec, architecture, and phased plan already exist.
- The user wants TDD, Makefile-driven commands, and a running **learning guide**.

## Inputs to collect

- **Language/framework** being learned.
- **Project name.**
- Paths: **Spec**, **Architecture**, **Plan** (`@` references).
- **Current status** (e.g. starting phase 0, or phase 2 task 3).
- **Phases to run** this session: a number, list, or `all`.

## Workflow

### Pre-flight

1. Confirm `.gitignore` excludes build artifacts, lockfiles if policy says so, coverage output, and other generated junk (adapt to language: e.g. `target/` for Rust).
2. Confirm toolchain and install steps for the target language from **official vendor or language documentation**; report what must be installed before coding.

### Per phase

1. Read the plan; find the first incomplete `[ ]` items.
2. For each implementation chunk:
   - Write or extend **tests first** when adding behavior.
   - Implement; run `make test` and `make lint` after substantive edits (use project’s Makefile targets).
   - Check off completed items in the plan doc.
3. Before relying on third-party API details, verify against **official library documentation** and the versions pinned in the project.

### After each phase

1. All tests and lint pass (`make test`, `make lint`, and `make format-check` if present).
2. Update **Implementation plan** checkboxes.
3. Update or create `docs/learning_guide.md` with:
   - Concepts hit this phase
   - Syntax and terminology notes
   - How concepts appear in the repo (file references)
   - Short review notes for the human
4. Commit with small, reviewable commits; ensure artifacts are not committed.

### Guidelines

- Use the Makefile for build/test/lint; avoid one-off commands unless Makefile is incomplete and user agrees.
- Avoid `unwrap()`-style shortcuts in **production** paths when the language is Rust (adapt for other languages: no silent failure in user paths).
- Integration tests: temp dirs only; no real user config.
- Failing test → fix code or test; do not delete tests to proceed.
- Uncertain design → ask before large implementation.

### End-of-phase report to the user

- What shipped
- What language concepts to study in the diff
- Gotchas and patterns worth remembering

## Example

Rust learner, plan at `docs/counter-design-and-phases.md`, status “phase 1 task 2”, phases `1` only: run pre-flight, finish phase 1 tasks in order, update `docs/learning_guide.md`, commit.
