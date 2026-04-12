---
name: learning-oriented-project-plan
description: Design architecture and a phased implementation plan for building a real project while learning a new language or framework. Use when the user has a spec and wants docs-first planning with learning goals, TDD, and Makefile workflows.
---

# Learning-oriented project plan

Produce two documents: an **architecture** doc and a **phased implementation & learning** doc, tuned for learning-by-building.

## When to use

- The user names their existing languages and a **new** language or framework they are learning.
- They attach or reference a project specification.

## Inputs to collect

- **Experience:** Languages/frameworks they already know (e.g. TypeScript, Python).
- **Target:** Language or framework being learned (e.g. Rust).
- **Spec:** Path to the spec (e.g. `docs/project-mvp-spec.md`).
- **Project slug** for filenames (e.g. `my-cli-tool`).

## Workflow

1. Read the spec. Ask **clarifying questions** before locking architecture.
2. Propose and iterate on architecture; use **Mermaid** for diagrams with conservative syntax (avoid parentheses in labels and raw `\n` escapes in labels).
3. After alignment, write:

### Deliverable 1: `docs/[project]-architecture.md`

- System overview with diagrams
- Storage and data model choices with tradeoffs
- Concurrency / safety notes appropriate to scope
- Right-sized module layout (avoid enterprise overkill for small tools)
- References (docs, articles)

### Deliverable 2: `docs/[project]-design-and-phases.md`

- Overview table mapping **language concepts** to phases (adapt heading if not Rust — e.g. “Concepts by phase”).
- For each phase:
  - **Learning goals** — `[ ]` checkboxes for concepts
  - **Implementation tasks** — `[ ]` file/module-level tasks
  - **Test cases** — `[ ]` unit and integration tests as the learning harness
  - **Review checklist** — `[ ]` what to study in the diff
- TDD: tests prove correctness per phase
- Suggested libraries with links; ground recommendations in **official documentation** and the versions you propose
- **Definition of Done** checklist for the whole project

### Cross-cutting guidelines

- Keep MVP scope tight; skip unnecessary abstractions.
- Phases should be reviewable in one sitting.
- Integration tests use **temporary directories**; never touch real user config.
- Include a **Makefile** with common targets: build, test, lint, format, clean, install (names may match project norms).
- Where useful, prefer structured logging (canonical log lines / wide events) for observability.

## Example opening

User: experienced in TypeScript, learning Rust, spec at `docs/mvp.md`. Output: `docs/acme-architecture.md` and `docs/acme-design-and-phases.md` with Rust-oriented concept tables.
