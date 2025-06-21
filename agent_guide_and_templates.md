# Agent Development Guide: RFC-Driven Workflow

This guide outlines a development workflow leveraging Request for Comments (RFC) documents to ensure clarity, track progress, and maintain consistency in software projects developed with AI agent assistance.

## Writing a Product Requirements Document (PRD)

A PRD provides a high-level overview of the product, its goals, features, and technical foundation. It serves as a central reference point for understanding the "what" and "why" of the project, acting as a living document that reflects the current state. While the specific content evolves, maintaining an up-to-date PRD is valuable, especially when working with development agents.

Consider including the following sections in your PRD:

### PRD Template Structure

**Title:** [Project Name] - Product Requirements Document

**(Optional) Version/Status:** *(e.g., Version 1.0, MVP, Updated as of YYYY-MM-DD)*

**1. Introduction**
    *   *Briefly describe the product: What is it?*
    *   *Who is it for (target audience)?*
    *   *What is the main problem it solves or need it fulfills?*
    *   *What is the scope being documented (e.g., Minimum Viable Product, specific version)?*
    *   *Example: "This document outlines the MVP requirements for 'Project Phoenix', a web platform for teams to manage shared documents."*

**2. Goals / Core Value Proposition**
    *   *What are the primary business or user objectives for this product/version?*
    *   *State the core value proposition concisely: What is the main unique benefit for the user?*
    *   *Example: "Goal: Provide a simple, real-time collaborative document editing experience. Value Prop: Edit documents together seamlessly without version conflicts."*

**3. Key Features**
    *   *List the major user-facing features included in the current scope.*
    *   *Describe each feature briefly from the user's perspective. Focus on *what* the user can do.*
    *   *Example Features:*
        *   *User Authentication: Users can sign up, log in, and log out.*
        *   *Document Creation: Users can create new documents.*
        *   *Real-time Editing: Multiple users can edit the same document simultaneously and see changes live.*
        *   *Sharing: Users can share documents with specific collaborators.*

**4. Technical Stack**
    *   *List the key technologies, languages, frameworks, databases, and major services chosen for the project.*
    *   *Categorize them for clarity (e.g., Frontend, Backend, Database, Deployment, Monitoring).*
    *   *Example Stack:*
        *   *Backend: Python (Flask), PostgreSQL*
        *   *Frontend: React (Next.js), TypeScript, Tailwind CSS*
        *   *Real-time: WebSockets (Socket.IO)*
        *   *Deployment: Docker, AWS (ECS, RDS)*
        *   *Monitoring: Prometheus, Grafana*

**5. Architecture Overview**
    *   *Describe the high-level system architecture.*
    *   *How are the main components (e.g., frontend web server, backend API, database, real-time service, external services) structured and how do they interact?*
    *   *Mention key architectural patterns (e.g., Monolith, Microservices, Service-Oriented Architecture, SPA + API, Serverless).*
    *   *A simple diagram can be very helpful here if possible.*
    *   *Example: "The system uses a decoupled SPA frontend communicating via REST API with a monolithic backend. A separate WebSocket service handles real-time updates."*

**6. Feature Development History / Link to RFCs (Optional but Recommended)**
    *   *To track how the product reached its current state, list or link to the major features or RFCs that have been implemented.*
    *   *This connects the high-level requirements to the detailed implementation plans.*
    *   *Example: "Current state reflects completion of RFC-001 (Auth), RFC-002 (Document Model), RFC-004 (Real-time Setup)."*

**7. Deployment Strategy**
    *   *How is the application built? (e.g., Build tools, commands like `npm run build`)*
    *   *How is it packaged? (e.g., Docker images, static asset bundles)*
    *   *Where and how is it deployed for different environments (dev, staging, production)? (e.g., Cloud provider, platform like Kubernetes/Fly.io, CI/CD pipeline tool)*
    *   *Example: "Backend is containerized using Docker and deployed to Fly.io via GitHub Actions. Frontend is built as static assets and deployed to Vercel."*

**8. Non-Functional Requirements / Design Considerations (Optional)**
    *   *List key considerations beyond specific features that influence design and implementation.*
    *   *Examples: Performance goals (e.g., API response time < 200ms), Security requirements (e.g., data encryption at rest), Scalability needs (e.g., handle 1000 concurrent editors), Accessibility standards (e.g., WCAG 2.1 AA compliance).*

---

Keep the PRD concise and focused on the current scope. It should be treated as a living document, updated periodically to reflect significant changes in requirements, architecture, or technology choices as the project evolves through RFCs.

## The RFC-Driven Workflow

All significant features, changes, or architectural decisions should first be defined in an RFC document, typically stored in a dedicated `docs/rfcs/` directory within the project. Each RFC serves as the specification and plan for a unit of work.

1.  **Define the Work:** Create an RFC using the standard template (see below). Clearly articulate the goals and technical approach.
2.  **Define Test Cases:** For each implementation task in the RFC's checklist, define a sub-checklist of specific test cases. These tests serve as the acceptance criteria for the task. This ensures quality and clarifies the requirements for the implementing agent.
3.  **Implement Incrementally:** Follow the RFC's technical design to complete each checklist item sequentially. After implementing a task, update its 'Implementation Notes / Summary' section with details about the changes (e.g., file paths, configuration updates, key decisions). This provides crucial context for other agents working on dependent RFCs.
4.  **Track Progress:** As each checklist item is completed, **edit the RFC file** to mark the item as done (`[X]`). This provides visibility into the progress.
5.  **🚨 VALIDATE COMPLETION:** Before marking an RFC as "Completed", **ALWAYS** run the full test suite to ensure no regressions were introduced. An RFC is not truly complete until all existing functionality remains intact.

### Sequential RFC Implementation Workflow

When implementing an RFC, follow this **strict sequential process** for each checklist item:

1. **Implement** the specific checklist item by making the necessary code/configuration changes
2. **Test** that the implementation works as expected 
3. **Check off** the item in the RFC file by changing `[ ]` to `[X]`
4. **Update Implementation Notes/Summary** section with detailed information about what was changed, including:
   - Specific file paths that were modified or created
   - Configuration changes made (environment variables, config files, etc.)
   - Database schema updates or migrations run
   - Key implementation decisions and any deviations from the original plan
   - Dependencies installed or updated
   - Commands executed
   - Any issues encountered and how they were resolved
5. **Only then proceed** to the next checklist item

**DO NOT** move to the next checklist item until the current item is both checked off AND the Implementation Notes section has been updated with comprehensive details. This ensures that future agents working on dependent RFCs have complete context about what was implemented and how.

### Test Validation Requirements

Before marking any RFC or checklist item as complete, you must:

1. **Run ALL Tests** to ensure no regression:
   - Backend: Run `go test ./...` and ensure ALL tests pass (not just new ones)
   - Frontend: Run `npm test` for unit tests and `npm run test:e2e` for E2E tests
   - Document any test failures and fixes in the Implementation Notes

2. **Follow the Validation Sequence**:
   - First: Run all unit tests
   - Second: Run all E2E tests  
   - Third: For user-facing features, validate the real application using Playwright MCP tool

3. **Use Playwright MCP for Real App Validation**:
   - Start the development servers (backend and frontend)
   - Use the Playwright MCP tool to interact with the application as a real user would
   - Verify all user flows work as expected
   - Document any issues found during real app validation

4. **Frontend Test Isolation**:
   - Always use MSW (Mock Service Worker) for API mocking in frontend tests
   - Tests should never depend on real backend connections
   - This ensures tests are fast, reliable, and can run in isolation

5. **🚨 CRITICAL: Full Regression Testing After RFC Completion**:
   - **ALWAYS** run the complete test suite after marking an RFC as "Completed"
   - **RFC cannot be considered truly complete until ALL existing tests still pass**
   - This ensures your RFC implementation doesn't break existing functionality
   - Document any test failures and their resolution in the RFC Implementation Notes
   - **Zero tolerance for regressions** - fix any failing tests before finalizing the RFC

## Standard RFC Template

Use this template for all new RFCs to ensure consistency and clarity.

```markdown
# RFC-XXX: [Descriptive Title]

**Status:** Draft | Active | Done
**Branch:** `rfc/XXX-[short-description]`  *(e.g., `rfc/042-user-profile-api`)*
**(Optional) Related Issues:** #[Issue Number]

## 1. Goal

*   *Concisely state the primary objective of this RFC. What should be achieved?*
*   *Example: Implement a REST API endpoint for creating new user profiles.*

## 2. Background & Context

*   *Explain the "why" behind this RFC. What problem does it solve? What is the current situation? Why is this change needed now? Link to relevant previous RFCs if applicable.*
*   *Example: Currently, users can only be created via the admin panel. This RFC adds a public API endpoint to allow self-registration.*

## 3. Technical Design

*   *Provide a detailed breakdown of the proposed solution. Use sub-sections for different components (e.g., Backend API, Frontend UI, Database Schema, Configuration).*
*   *Specify file paths, function names, API endpoints, data structures, environment variables, etc., using representative examples.*
*   *Include configuration snippets or pseudo-code where helpful.*
*   *Mention any significant architectural decisions or trade-offs.*

### 3.1 Component A (e.g., Backend API)

*   *Details specific to Component A...*
*   *Endpoints:*
    *   `POST /api/users` - Creates a new user. Expects `email`, `password` in the request body. Returns `user_id`.
*   *Files:*
    *   `src/handlers/user_handler.go` (or `.ts`, `.py`, etc.)
    *   `src/models/user.go`
*   *Config:*
    *   Environment variable: `AUTH_SECRET_KEY`

### 3.2 Component B (e.g., Database Schema)

*   *Details specific to Component B...*
*   *Table: `users`*
    *   `id`: INT PRIMARY KEY
    *   `email`: VARCHAR UNIQUE NOT NULL
    *   `password_hash`: VARCHAR NOT NULL
    *   `created_at`: TIMESTAMP

## 4. Dependencies

*   *List any new external libraries, tools, services, or significant internal dependencies introduced or heavily relied upon by this RFC.*
*   *NPM Package: `bcrypt` for password hashing.*
*   *Go Module: `github.com/golang-jwt/jwt/v5` for token generation.*
*   *Requires configuration for: SMTP service for welcome emails.*

## 5. Checklist

*   *Break down the implementation into small, verifiable steps. For each task, define a sub-checklist of test cases that must pass for the task to be considered complete.*
*   [ ] **Task 1: Add `users` table migration script.**
    *   **Test Cases**:
        *   [ ] Test that migration `up` creates the table with the correct schema and constraints.
        *   [ ] Test that migration `down` successfully removes the table.
*   [ ] **Task 2: Implement `POST /api/users` handler function.**
    *   **Test Cases**:
        *   [ ] Test successful user creation returns a 201 status.
        *   [ ] Test request with a missing `email` field returns a 400 error.
        *   [ ] Test request with an existing email returns a 409 conflict error.
*   [ ] **Task 3: Add input validation for email and password.**
    *   **Test Cases**:
        *   [ ] Test that an invalid email format is rejected.
        *   [ ] Test that a password shorter than the required length is rejected.
*   [ ] **Task 4: Write unit tests for the user creation logic.**
    *   **Test Cases**:
        *   [ ] Test user creation logic is covered by unit tests.
        *   [ ] Test input validation logic is covered by unit tests.
*   [ ] **Task 5: Add `AUTH_SECRET_KEY` to environment configuration files.**
    *   **Test Cases**:
        *   [ ] Test that the application fails to start if the secret is missing in production mode.

## 6. Definition of Done

*   *Clearly define the criteria that must be met for this RFC to be considered complete.*
*   *All checklist items are completed.*
*   *The `POST /api/users` endpoint is functional and deployed.*
*   *Automated tests covering the new endpoint pass.*
*   *Manual verification confirms a user can be created via the API.*

## Implementation Notes / Summary

*   *Add any extra context, links to documentation, or potential challenges.*
*   *Example: Password complexity rules will be handled in a separate RFC.*
*   *After completion, this section can be used to summarize the key changes made.*
*   ***CRITICAL:** This section serves as context for other implementer agents working on dependent RFCs. Update this section after each checklist item with detailed information including: specific file paths that were modified, configuration changes made, database schema updates, environment variables added, and any important implementation decisions that future agents will need to understand.*
```

## Version Control & Commit Messages

All changes should be committed using Git. Follow the **Conventional Commits** format for clear and automated history tracking:

```
type(scope): message
```

*   **type:** Describes the kind of change (e.g., `feat`, `fix`, `refactor`, `chore`, `test`, `docs`, `perf`, `ci`, `build`).
*   **scope (Important!):** Identifies the part of the codebase or the specific RFC the commit relates to. **Use the RFC number** (if applicable) or a relevant module/feature name.
*   **message:** A concise description of the change in the imperative mood.

**Examples:**

*   `feat(rfc-042): Add user creation endpoint`
*   `fix(auth): Prevent login with inactive accounts`
*   `chore(deps): Update web framework to v2.1`
*   `refactor(api): Simplify database query logic in product handler`
*   `test(ui): Add tests for password validation component`

Linking commits to RFCs via the scope is crucial for tracing the implementation history of features and changes.

## Effective Tool Usage for RFC Implementation

By following the RFC process, using the standard template, adhering to commit conventions, and utilizing tools effectively, development agents can contribute to projects in a structured, traceable, and efficient manner.

---

## Full RFC Example Template

Here is a blank template structure for reference when creating a new RFC:

```markdown
# RFC-XXX: [Your Feature or Change Title]

**Status:** Draft
**Branch:** `rfc/XXX-[your-branch-name]`
**Related Issues:** #[Link to relevant issue(s) if any]

## 1. Goal

*   *Clearly state what this RFC aims to accomplish.*

## 2. Background & Context

*   *Provide context. Why is this needed? What is the current state?*

## 3. Technical Design

*   *Describe the technical approach. Break it down into components if necessary.*

### 3.1 Component/Area 1 (e.g., API Changes)

*   *Detail changes for this area: endpoints, data models, logic, file paths, etc.*

### 3.2 Component/Area 2 (e.g., UI Changes)

*   *Detail changes for this area: routes, components, state management, file paths, etc.*

### 3.3 Component/Area 3 (e.g., Database/Config)

*   *Detail changes for this area: schema, migrations, environment variables, etc.*

## 4. Dependencies

*   *List any new libraries, services, or critical dependencies.*

## 5. Checklist

*   *Break down the work into specific, actionable tasks. For each task, define a sub-checklist of test cases.*
*   [ ] **Task 1: ...**
    *   **Test Cases**:
        *   [ ] Test case 1a.
        *   [ ] Test case 1b.
*   [ ] **Task 2: ...**
    *   **Test Cases**:
        *   [ ] Test case 2a.
*   [ ] **Task 3: ...**

## 6. Definition of Done

*   *Define the criteria for when this RFC is considered complete.*
*   *Example: All checklist items done, tests passing, feature verified.*

## Implementation Notes / Summary

*   *Add any relevant notes, links, or potential issues during implementation.*
*   *After completion, summarize the key changes and outcomes here.*
*   ***CRITICAL:** Update this section after each checklist item with detailed information for future agents: specific file paths modified, configuration changes, database schema updates, environment variables, and implementation decisions.*
```

---
