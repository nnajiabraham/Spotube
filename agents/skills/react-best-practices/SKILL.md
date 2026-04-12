---
name: react-best-practices
description: React and Next.js performance optimization guidance focused on framework-level best practices. Use this skill when writing, reviewing, or refactoring React/Next.js code for correctness and performance.
---

# React Best Practices

Comprehensive React and Next.js optimization guide adapted for local use in this repository.

## Local Documentation (No External Skill Fetch Required)

Use these local files instead of external skill links:

- Full guide: `agents/skills/react-best-practices/AGENTS.md`
- Individual rules: `agents/skills/react-best-practices/rules/{rule-name}.md`
- Rule index/sections: `agents/skills/react-best-practices/rules/_sections.md`

## When to Apply

Use this skill when:
- Writing new React components or Next.js pages
- Implementing data fetching patterns (client or server)
- Reviewing code for performance bottlenecks
- Refactoring React/Next.js code for render or bundle efficiency
- Optimizing bundle size, hydration behavior, or interaction responsiveness

## Category Priority

1. Eliminating Waterfalls (`async-`) — critical latency reduction
2. Bundle Size Optimization (`bundle-`) — critical startup improvements
3. Server-Side Performance (`server-`) — high impact server/runtime efficiency
4. Client-Side Data Fetching (`client-`) — medium-high network efficiency
5. Re-render Optimization (`rerender-`) — medium UI responsiveness
6. Rendering Performance (`rendering-`) — medium browser rendering cost
7. JavaScript Performance (`js-`) — low-medium hot-path improvements
8. Advanced Patterns (`advanced-`) — targeted advanced guidance

## Usage Workflow

1. Start with `AGENTS.md` for broad review and prioritization.
2. Open specific local rule files for exact before/after patterns.
3. Apply the minimal change that resolves the performance or correctness issue.
4. Prefer React/Next.js platform-agnostic guidance over hosting-provider specifics.

## References

- [https://react.dev](https://react.dev)
- [https://nextjs.org](https://nextjs.org)
- [https://www.npmjs.com/package/swr](https://www.npmjs.com/package/swr)
