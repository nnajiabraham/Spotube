# Draft prompts

Reusable copy-paste prompts. Prefer loading the matching skill under `agents/skills/<name>/SKILL.md` when available.

---

**Title:** Implement RFC

**Purpose:** Continue or start RFC execution with Makefile, tests, browser MCP for UI, and RFC doc updates.

**Prompt:**

```
Continue implementing RFC-XXX-XXX.md from Task X:XXXXX.

Orient using the RFC checklist and Implementation notes; do not trust existing checkmarks until verified in code and tests.

Before changing behavior: run the full test suite via the Makefile and understand current project state (layout, schemas, APIs relevant to this RFC).

For each checklist item in order:
- Research the codebase enough to integrate cleanly.
- Prefer tests first for new behavior; do not move to the next item until tests pass for the current scope.
- Use only Makefile targets for build, test, lint, format, serve, etc.
- For client-rendered UI, verify with the browser MCP available in this environment (e.g. Chrome DevTools: navigate, snapshot, screenshot)—not curl alone.
- For external libraries, use official documentation and the versions pinned in this repo.

After each task: check off the item when implementation and tests match acceptance; update Implementation notes with what changed, files touched, commands, and links for other agents.

If tests fail: use verbose runs, focused logging, or full logs in a rewritten debug.log as needed.
```

---

**Title:** Design and plan project with learning goals

**Purpose:** Produce architecture + phased learning plan docs from a spec (new language/framework).

**Prompt:**

```
I am an experienced software engineer with production experience in [YOUR_LANGUAGES e.g. JS/TS, Python, Java] learning [NEW_LANGUAGE e.g. Rust] by building a real project.

I have a project specification: @[PATH_TO_SPEC_DOC]

Help me design and plan this project with the goal of learning [NEW_LANGUAGE] while building it. We'll work together to create:

1. **Architecture document** (`docs/[project]-architecture.md`):
   - System overview with Mermaid diagrams (use conservative syntax: no parentheses in labels, no `\n` escapes)
   - Storage model decisions with tradeoffs explained (e.g., JSONL vs JSON, why)
   - Concurrency/safety considerations appropriate for the project scope
   - "Right-sized" code structure (avoid over-engineering; this is a single-user/simple tool, not enterprise scale)
   - References section with relevant docs/articles

2. **Phased implementation & learning plan** (`docs/[project]-design-and-phases.md`):
   - "[NEW_LANGUAGE] concepts by phase" overview table
   - Each phase broken down with:
     - **Learning goals**: specific language concepts to focus on (with `[ ]` checkboxes)
     - **Implementation tasks**: granular file/module-level tasks (with `[ ]` checkboxes)
     - **Test cases**: explicit unit + integration tests serving as harness for both me and the agent (with `[ ]` checkboxes)
     - **Review checklist**: what I should look at in the diff to learn (with `[ ]` checkboxes)
   - TDD approach: tests validate implementation correctness
   - Suggested crates/libraries with docs links
   - "Definition of Done" summary checklist

Guidelines:
- Keep MVP scope tight; avoid abstractions that aren't directly needed
- Phases should be small enough to review in one sitting
- Integration tests must use temp dirs and never touch real user config
- Include a Makefile with common dev commands (build, test, lint, format, clean, install)
- Use canonical log lines / wide events pattern for observability where appropriate
- Ground library recommendations in official documentation and explicit versions

Start by reading the spec, then ask clarifying questions before proposing architecture. Include Mermaid diagrams for visual alignment. After we agree on architecture, create the phased plan document.
```

---

**Title:** Implement phased learning project

**Purpose:** Execute phases from spec + architecture + plan with TDD and Makefile.

**Prompt:**

```
I am learning [NEW_LANGUAGE e.g. Rust] by building [PROJECT_NAME].

Project documents:
- Spec: @[PATH_TO_SPEC e.g. docs/project-mvp-spec.md]
- Architecture: @[PATH_TO_ARCH e.g. docs/project-architecture.md]
- Implementation plan: @[PATH_TO_PLAN e.g. docs/project-design-and-phases.md]

Current status: [Starting Phase 0 | Continuing from Phase X task Y | etc.]
Phases to implement: [n e.g. 1, 3, or "all"]

Pre-implementation checks:
1. Verify `.gitignore` includes build/test artifacts (e.g., `target/`, `*.lock`, coverage files)
2. Check official toolchain installation requirements for [NEW_LANGUAGE]
3. Report all required toolchains and how to install them before proceeding

Implementation workflow:
1. Read the plan document to understand where we are (check completed items)
2. For the current phase:
   - Implement tasks in order, checking off `[ ]` items as completed
   - Write tests FIRST (TDD) - they serve as the verification harness
   - Run `make test` and `make lint` after each significant change
   - Log what was done in the plan doc's checklist
3. After completing a phase:
   - Verify all tests pass (`make test`, `make lint`, `make format-check`)
   - Update the plan doc (check off completed items)
   - Update `docs/learning_guide.md` (create if not exists) with:
     - Concepts encountered in this phase
     - Language-specific syntax and terminologies
     - How concepts are implemented in the project (with code references)
     - Short notes for learning review
   - Commit all relevant changes (ensure build artifacts are excluded)
   - If more phases remain (n > 1), proceed to next phase

Guidelines:
- Verify third-party APIs against official docs and pinned versions before writing code
- Use the Makefile for all build/test/lint commands
- Never use `unwrap()` in production code paths (tests may use it)
- Integration tests must use temp dirs (never touch real user config)
- If a test fails, fix the implementation, don't skip the test
- Keep commits/changes small and reviewable
- If you're unsure about a design decision, ask before implementing

After each phase, tell me:
- What was implemented
- What [NEW_LANGUAGE] concepts I should pay attention to in the diff
- Any gotchas or patterns worth noting

Let's begin.
```

---

**Title:** Continue phased learning project

**Purpose:** Short handoff prompt for a new chat.

**Prompt:**

```
Continue implementing [PROJECT_NAME] from Phase [X], Task: [TASK_DESCRIPTION].

Docs:
- Spec: @[PATH_TO_SPEC]
- Architecture: @[PATH_TO_ARCH]
- Plan: @[PATH_TO_PLAN]

Read the plan doc to see completed items and pick up where we left off. Follow the implementation workflow:
- TDD: write/verify tests first
- Use `make test` and `make lint` after changes
- Check off completed items in the plan doc
- Use official library documentation when API details matter
- After each task, summarize what I should review to learn

Begin.
```

---

**Title:** Alternative recommendation review

**Purpose:** Compare a pasted alternative to your recommendation using criteria that fit the decision (from this message, prior chat, or what clearly matters for the topic).

**Prompt:**

```
Here's an alternative research and recommendation. Weigh it against ours (baseline from this thread). Use comparison criteria from this message if I list any; otherwise use what we already established in the chat, or infer only the dimensions that actually matter for this decision—not every topic needs integration, security, or ops analysis. Tell me if you agree or disagree, or how it changes your recommendation and stance.

"""

[PASTE_ALTERNATIVE_HERE]

"""
```

---
