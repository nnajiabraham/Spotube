---
name: alternative-recommendation-review
description: Compare a proposed alternative to the agent's prior recommendation (or shared ideation) using criteria that fit the situation. Use when the user pastes a counter-proposal and wants an explicit agree/disagree and an updated stance.
---

# Alternative recommendation review

The user supplies **alternative research or a recommendation**. Compare it to **your current recommendation** or the **plan you and the user have been developing** (restate that stance briefly if the thread does not already make it obvious).

## When to use

- User pastes third-party advice, another model’s output, or a competing design and asks how it compares.

## Inputs to collect

- **Baseline:** Your prior recommendation, or the emerging consensus from earlier messages (approach, key choices, tradeoffs).
- **Alternative:** Full paste or linked content.
- **Comparison criteria:** Derive from, in order of precedence:
  1. Criteria the user **states in the same message** that invokes this skill (or in a follow-up).
  2. Criteria already established in the **conversation** (goals, constraints, values, non-goals).
  3. If still underspecified, infer a **small set** of dimensions that actually matter for *this* decision (e.g. a pure API shape discussion may not need an “operational cost” section; a security-sensitive flow may center risk and compliance).

Do **not** assume every comparison must include ease of integration, security, or operational cost—use them only when they are relevant to the user’s context, criteria, or the nature of the decision.

## Workflow

1. Restate both options in one short paragraph each, neutrally.
2. List the **criteria you will use** (bullet list), and note where each came from (user prompt, prior chat, or inferred—and if inferred, say why briefly).
3. Compare option A vs option B **against those criteria only**. Skip dimensions that do not apply; do not pad with generic sections.
4. State clearly: **agree**, **disagree**, or **partially agree** with adopting the alternative *relative to the baseline*.
5. Explain **how your stance changes**, if at all: adopt, reject, or hybridize—and what evidence would flip you.
6. If the alternative or baseline lacks information needed for a criterion that matters, say what is missing instead of inventing it.

## Output format

Use headings and bullets that match the **criteria you actually used**. The following is only an illustration when integration and security happen to matter:

- **Verdict:** [agree / disagree / partial] — [one line]
- **Criteria used:** [bullets]
- **Integration analysis:** [short] *(example dimension—omit if not relevant)*
- **Security analysis:** [short] *(example dimension—omit if not relevant)*
- **Other dimensions:** [only as needed, e.g. UX, cost, time-to-ship, team familiarity]
- **Updated recommendation:** [what you would do now]

## Example

Prior: managed Postgres + migrations in-repo. Alternative: SQLite per tenant. User cares about multi-tenant isolation and backup. Verdict: partial — SQLite wins local simplicity but fails the stated isolation/backup bar unless augmented; would not adopt as-is.
