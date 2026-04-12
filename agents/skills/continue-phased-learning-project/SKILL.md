---
name: continue-phased-learning-project
description: Resume a phased learning project in a fresh chat from a given phase and task. Use when the user points at spec, architecture, and plan docs and wants continuation without re-explaining the full workflow.
---

# Continue phased learning project

Pick up implementation from a named phase and task using the same rules as **implement-phased-learning-project**, with minimal preamble.

## When to use

- New chat or handoff: user says “continue from phase X, task …” with doc paths.

## Inputs to collect

- **Project name**
- **Phase number** (or identifier) and **task description** to resume from
- Paths: **Spec**, **Architecture**, **Plan**

## Workflow

1. Read the plan doc; locate completed `[ ]` items and the first incomplete work.
2. Confirm the user’s “phase X / task Y” matches the doc; reconcile if not.
3. Follow:
   - TDD: write or verify tests first
   - `make test` / `make lint` after changes
   - Check off items in the plan as they complete
   - **Official library documentation** when API details matter
4. After each task, tell the user what to review for learning (short summary).

## Note

For full pre-flight, end-of-phase learning guide updates, and reporting structure, use **implement-phased-learning-project** when starting a large stretch of work; use this skill for compact continuation.
