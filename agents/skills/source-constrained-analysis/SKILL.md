---
name: source-constrained-analysis
description: Answer whether a single document is sufficient for a stated goal, with steps derived only from that source, or stop at the first explicit gap. Use when the user restricts reasoning to one URL or file and wants a sufficiency verdict without outside knowledge.
---

# Source-constrained analysis

Execute a task using **only** one designated source. No other documents, examples, conventions, or general knowledge beyond the stated baseline.

## When to use

- The user points to exactly one document (path or URL) and asks for a verdict on whether it is enough to complete a goal.
- The user explicitly forbids using training data or other references.

## Inputs to collect

- **Domain baseline**: Pretend you know nothing about the domain except what the user lists as allowed background.
- **Allowed knowledge**: Short list of facts the user permits (may be empty).
- **Source**: One URL or `@path/to/doc` — the only authoritative text.
- **Goal**: What “done” means for this exercise.

## Workflow

1. Read the source in full (or the user-provided excerpt if only a fragment is in scope).
2. Produce exactly this structure:

### Deliverable

1. **Sufficiency verdict**: Either `sufficient` or `insufficient`.
2. If **sufficient**: The exact steps to complete the goal, each step justified by a direct quote or paraphrase tied to explicit wording in the source.
3. If **insufficient**: Stop at the **first** unstated assumption, missing prerequisite, or unclear instruction. Name it precisely. Do not speculate beyond that point.

### Rules

- Derive every step only from what the source **explicitly** states.
- Do not use outside knowledge beyond the stated baseline.
- Do not use other docs, examples, conventions, or training data.
- Do not guess or fill in missing steps.

## Example

**Allowed knowledge:** “HTTP is request/response.” **Source:** Internal runbook section on deploy flags. **Goal:** “List the flags needed for a canary deploy.” If the runbook never defines “canary,” verdict is `insufficient` and the first gap is the missing definition or flag list for canary.
