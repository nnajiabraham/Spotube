# RFC-012: Comprehensive E2E Testing Suite

**Status:** Draft  
**Branch:** `rfc/012-e2e-testing`  
**Depends On:** All previous RFCs, as it tests the integrated user flow.

---

## 1. Goal

Define and structure a comprehensive End-to-End (E2E) testing suite using **Playwright** and **Mock Service Worker (MSW)**. This suite will validate critical user journeys from start to finish, ensuring all frontend components and backend integrations (as mocked by MSW) work together as a cohesive application.

## 2. Background & Context

Previous RFCs have defined unit and integration tests for individual features in isolation. This RFC ties everything together by simulating complete user stories. As per the user's direction, this is not about re-testing individual components but about verifying the full application flow. We will use MSW to create a stable, predictable mock server, allowing us to test complex frontend application states without depending on a live backend.

## 3. Technical Design

### 3.1 E2E Test Structure
E2E tests will live in `frontend/tests/e2e/`. We will create separate spec files for each major user journey.

```
frontend/
└── tests/
    ├── e2e/
    │   ├── setup.spec.ts
    │   ├── auth.spec.ts
    │   ├── mapping.spec.ts
    │   └── sync.spec.ts
    └── msw/
        ├── handlers.ts
        └── server.ts
```

### 3.2 MSW Mock Server Setup
We will create a set of MSW handlers in `frontend/tests/msw/handlers.ts` that cover all API endpoints used by the application. This allows us to control the "backend" state for our Playwright tests.

**Example MSW Handler**:
```typescript
// frontend/tests/msw/handlers.ts
import { http, HttpResponse } from 'msw';

export const handlers = [
  // Mock the setup status endpoint
  http.get('/api/setup/status', () => {
    return HttpResponse.json({ required: true });
  }),

  // Mock a successful login
  http.post('/api/auth/spotify/login', () => {
    // In a real test, this would likely redirect, which Playwright can handle
    return new HttpResponse(null, { status: 302, headers: { 'Location': '/dashboard?spotify=connected' }});
  }),
  
  // Mock playlist fetching
  http.get('/api/spotify/playlists', () => {
      return HttpResponse.json([
          { id: 'sp_playlist_1', name: 'Spotify Playlist 1', track_count: 10 }
      ]);
  }),
  
  // ... other handlers for YouTube, mappings, logs, etc.
];
```
The Playwright tests will be configured to use these MSW handlers.

### 3.3 E2E User Journeys (Test Cases)

#### 3.3.1 `setup.spec.ts`: First-Time Setup Wizard
1.  **Story**: A new user starts the app for the first time.
2.  **Steps**:
    *   Navigate to the root URL.
    *   **Assert**: The setup wizard is visible.
    *   Fill in all four credential fields with valid (dummy) data.
    *   Click "Save".
    *   **Assert**: MSW handler for `POST /api/setup` was called.
    *   **Assert**: The user is redirected to the dashboard or login page.
    *   Reload the page.
    *   **Assert**: The setup wizard is **not** visible (MSW handler for `/api/setup/status` now returns `{ required: false }`).

#### 3.3.2 `auth.spec.ts`: OAuth Connection Flow
1.  **Story**: A user connects their Spotify and YouTube accounts.
2.  **Steps**:
    *   Start on the dashboard.
    *   **Assert**: "Connect Spotify" and "Connect YouTube" cards are visible.
    *   Click "Connect Spotify".
    *   **Assert**: The page navigates to the (mocked) Spotify login flow.
    *   Simulate a successful callback from Spotify.
    *   **Assert**: The user is redirected back to the dashboard.
    *   **Assert**: A "Spotify connected" toast message appears.
    *   **Assert**: The "Connect Spotify" card is replaced with a "View Playlists" card.
    *   Repeat the flow for YouTube.

#### 3.3.3 `mapping.spec.ts`: Playlist Mapping Flow
1.  **Story**: An authenticated user creates a new playlist mapping.
2.  **Steps**:
    *   Start on the dashboard, with both services connected.
    *   Navigate to the "Mappings" page.
    *   Click "Create New Mapping".
    *   Select a Spotify playlist from the dropdown (populated by MSW).
    *   Select a YouTube playlist from the dropdown (populated by MSW).
    *   Configure sync options.
    *   Click "Save".
    *   **Assert**: The new mapping appears in the data table on the mappings page.
    *   Click the "Delete" button for the new mapping.
    *   **Assert**: The mapping is removed from the table.

#### 3.3.4 `sync.spec.ts`: Sync Status Visualization
1.  **Story**: A user observes the status of a sync job on the dashboard.
2.  **Steps**:
    *   Start on the dashboard.
    *   Use MSW to return a dashboard stats payload indicating there are "5 Pending" items in the queue.
    *   **Assert**: The "Queue - Pending" card shows "5".
    *   Update the MSW handler to return "0 Pending" and "5 Done" items.
    *   Wait for TanStack Query's `refetchInterval`.
    *   **Assert**: The "Queue - Pending" card updates to "0" and a "Done" card might appear or update.

## 4. Dependencies
*   `playwright` and `@playwright/test` (already defined in project)
*   `msw` (already defined in project)

## 5. Checklist
- [ ] **E2E1** Set up the MSW handler structure in `frontend/tests/msw/`.
- [ ] **E2E2** Configure Playwright to use MSW for its test runs.
- [ ] **E2E3** Implement the `setup.spec.ts` test case for the first-run wizard.
- [ ] **E2E4** Implement the `auth.spec.ts` test case for connecting external services.
- [ ] **E2E5** Implement the `mapping.spec.ts` test case for creating and managing mappings.
- [ ] **E2E6** Implement the `sync.spec.ts` test case for visualizing queue status.
- [ ] **E2E7** Ensure all E2E tests can be run via a single command (e.g., `npm run test:e2e`).
- [ ] **E2E8** Update `README.md` with instructions on how to run the E2E test suite.

## 6. Definition of Done
*   The E2E test suite covers the critical user paths of setup, authentication, mapping, and status visualization.
*   Tests are stable and run against a mocked backend, ensuring they are fast and reliable.
*   The `npm run test:e2e` command successfully executes the entire Playwright suite.

## Resources & References
*   Playwright Documentation – https://playwright.dev/docs/intro
*   Mock Service Worker (MSW) Docs – https://mswjs.io/docs/
*   Playwright with MSW Integration Patterns – Often found in community blogs and examples. A common pattern is to use MSW's `setupServer` for Node.js environments like Playwright's test runner.

---

*End of RFC-012* 