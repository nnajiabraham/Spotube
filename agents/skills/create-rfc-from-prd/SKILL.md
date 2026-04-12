---
name: create-rfc-from-prd
description: Draft an implementation-ready RFC from the PRD and prior RFCs, with atomic checklist items, test cases, and research citations. Use when the user assigns a specific RFC from the roadmap and wants a new file under docs/rfcs/.
---

# Create RFC from PRD

Produce a **Draft** RFC that a developer can implement without guessing, aligned with the PRD and existing RFCs.

## When to use

- The user specifies an RFC number/title from the PRD roadmap (e.g. PRD section 8).
- You need one new file under `docs/rfcs/` that downstream work will follow.

## Inputs to collect

- Path or reference to the **PRD**.
- **RFC assignment**: number + title (and kebab-case slug for the filename).
- Optional: branch naming convention (default `rfc/[NUMBER]-[short-description]`).

## Workflow

### 1) Read the PRD

Focus on:

- Key features and functional requirements
- Technical stack (do not substitute technologies)
- Architecture overview and patterns
- RFC roadmap / phases
- Documented technical decisions and rationale
- File paths or module layout the PRD prescribes (e.g. section 7.2)

### 2) Read existing RFCs

- Read completed/draft RFCs in `docs/rfcs/`, **excluding** any archive folder.
- Note decisions, interfaces, and dependencies your RFC must respect.
- Record gaps between PRD and current repo reality; surface them in the new RFC.

### 3) Research

Before writing the body:

- Official docs for each technology in scope
- Best practices and patterns for the problem area
- Libraries and **exact versions** where the stack is chosen
- Security, performance, and operational considerations

### 4) Plan the implementation (in the RFC)

- File and directory changes
- API or data contracts
- Testing strategy and risks

### 5) Write the RFC file

Create `docs/rfcs/RFC-[NUMBER]-[kebab-case-title].md` with:

- **Status:** Draft
- **Suggested branch:** `rfc/[NUMBER]-[short-description]`
- **Problem / context** aligned with PRD
- **Proposal** and design (diagrams if helpful)
- **Atomic checklist items**, each independently verifiable, each with **test cases** (positive and negative where relevant)
- **Research** subsection with URLs and references
- **Implementation notes** for future agents (assumptions, open questions)
- Exact commands, config snippets, and dependency versions where they remove ambiguity

### 6) Process expectations

- If the roadmap is sequential: this RFC should build on prior RFCs; note prerequisites.
- Prefer one RFC per assignment; do not merge unrelated scope.

## RFC document template

Copy and fill the following structure (adapt headings if the repo uses a house style, but keep checklist + test-case pairing).

```markdown
# RFC-[NUMBER]: [Title]

| Field | Value |
|-------|--------|
| Status | Draft |
| Branch | rfc/[NUMBER]-[short-description] |

## Summary

[2–4 sentences: what changes and why.]

## Context

[Problem, constraints, link to PRD sections.]

## Goals

- [Measurable goal 1]
- [Measurable goal 2]

## Non-goals

- [Explicitly out of scope]

## Proposal

[Design, data flow, key algorithms, file layout.]

## Dependencies

- [Library@version, services, prior RFCs]

## Security & operations

[Threats, secrets, rollout, monitoring.]

## Checklist

### [Area 1]

- [ ] **[Checklist item ID]:** [Short imperative description]
  - **Tests:** [How to verify — commands, assertions, manual steps]
  - **Negative / edge:** [What could break; how tests cover it]

### [Area 2]

- [ ] ...

## Research & references

- [URL or doc] — [what it informed]

## Implementation notes (for agents)

[Files touched, Makefile targets, migration steps, follow-up RFCs.]

## Open questions

- [...]
```

## Quality bar

- Every checklist line has test instructions a stranger can run.
- Versions and commands are concrete, not “use latest X.”
- Assumptions are explicit; PRD conflicts are called out.
