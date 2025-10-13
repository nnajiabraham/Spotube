# EBJoy Migration Reference - PocketBase to Echo

## 1. Migration Overview

EBJoy successfully migrated from PocketBase to Echo framework through a systematic RFC-driven approach (RFC-020 through RFC-029). The migration preserved all functionality while modernizing the tech stack.

## 2. Technology Stack Transformation

### 2.1 Backend Stack Changes
| Component | PocketBase | Echo Migration |
|-----------|------------|----------------|
| Web Framework | PocketBase (built-in) | Echo v4 |
| Database | SQLite (PocketBase managed) | SQLite (modernc.org/sqlite) |
| Migrations | PocketBase migrations | Goose v3 |
| ORM/Query Builder | PocketBase DAO | Jet v2 (typed SQL) |
| Authentication | PocketBase Auth | gorilla/sessions + bcrypt |
| Session Store | PocketBase | Cookie-based (gorilla/sessions) |
| File Storage | PocketBase managed | Custom file system implementation |
| Logging | PocketBase logs | Zerolog (structured) |
| Environment | .env loading | joho/godotenv |

### 2.2 Frontend Stack (Unchanged)
- React 19 + TypeScript + Vite
- TanStack Router & Query
- Tailwind CSS v4
- Changed: PocketBase SDK → Custom HTTP client with fetch

## 3. Architecture Patterns

### 3.1 Project Structure
```
ebjoy/
├── Makefile                 # Root orchestrator
├── backend/
│   ├── Makefile            # Backend-specific targets
│   ├── cmd/
│   │   ├── migrate/        # Migration CLI tool
│   │   └── server/         # Main server entry
│   ├── internal/
│   │   ├── auth/          # Auth helpers
│   │   ├── config/        # Configuration
│   │   ├── db/            # Jet generated models
│   │   ├── handlers/      # HTTP handlers
│   │   ├── http/          # Server setup
│   │   ├── logging/       # Logger setup
│   │   ├── migrate/       # Migration helpers
│   │   ├── services/      # Business logic
│   │   └── sqliteconn/    # SQLite connection
│   └── migrations/        # SQL migration files
└── frontend/
    ├── Makefile           # Frontend-specific targets
    └── src/
        └── lib/
            └── api/       # HTTP client
```

### 3.2 Key Design Decisions

#### Database & Schema
- Pure SQL migrations with Goose (no ORM migrations)
- Jet for type-safe SQL generation from schema
- Unix epoch seconds (INTEGER) for all timestamps
- SQLite with production-ready PRAGMAs:
  ```sql
  PRAGMA journal_mode = WAL;
  PRAGMA synchronous = NORMAL;
  PRAGMA busy_timeout = 5000;
  PRAGMA foreign_keys = ON;
  ```

#### API Conventions
- RESTful endpoints replacing PocketBase collection APIs
- Standardized response shapes:
  ```json
  // Lists
  { "data": [...], "meta": { "page": 1, "per_page": 20, "total": 100 } }
  
  // Errors
  { "error": { "code": "validation_failed", "message": "..." } }
  ```
- Query parameters (Option A only):
  - Pagination: `page`, `per_page`
  - Sorting: `sort`, `order` (no `-field` syntax)
  - Filters: Flat parameters (no brackets)

#### Authentication
- Cookie-based sessions (HTTPOnly, Secure in production)
- CSRF protection with Echo middleware
- bcrypt for password hashing
- Session verification on protected routes

## 4. Makefile Build System

### 4.1 Hierarchical Structure
```makefile
# Root Makefile includes sub-makefiles
-include backend/Makefile
-include frontend/Makefile

# Parallel execution for dev
dev: ## Start both servers
    @$(MAKE) -j2 backend/dev frontend/dev
```

### 4.2 Backend Make Targets
```makefile
backend/dev         # Runs db/up → db/gen → server
backend/test        # Runs tests with gotestsum
backend/build       # Builds binary
backend/lint        # go fmt + go vet
backend/db/create   # Create migration (NAME=xxx)
backend/db/up       # Apply migrations
backend/db/down     # Rollback one migration
backend/db/gen      # Run Jet codegen
```

### 4.3 Development Workflow
1. `make install` - Install all dependencies
2. `make backend/db/up` - Apply migrations
3. `make backend/db/gen` - Generate Jet models
4. `make dev` - Start development servers
5. `make test` - Run all tests

## 5. Migration Implementation Phases

### Phase 0: Setup & Health (RFC-022)
- Basic Echo server setup
- Health endpoint
- Middleware stack (RequestID, Logger, Recovery, CORS)
- Environment configuration

### Phase 1: Migrations & Codegen (RFC-023)
- Goose migration setup
- Initial schema (users, events, entries, entry_files)
- Jet code generation
- SQLite connection with PRAGMAs

### Phase 2: Authentication (RFC-024)
- Session-based auth with gorilla/sessions
- Signup/Login/Logout endpoints
- Password hashing with bcrypt
- CSRF protection

### Phase 3: Events CRUD (RFC-025)
- Owner-only event management
- Soft delete pattern
- Auto-generation of link_id and passwords
- Query parameter standardization

### Phase 4: Guest Access (RFC-026)
- Public event access by link
- Password verification
- Access window enforcement
- Public entries listing

### Phase 5: Entries & Files (RFC-027)
- Multipart file upload
- File storage organization
- Secure file serving
- Media type validation

### Phase 6: Moderation (RFC-028)
- Guest reporting
- Owner moderation tools
- Flagging system

### Phase 7: Frontend Integration (RFC-029)
- API client migration
- Timestamp handling (epoch seconds)
- CSRF token management
- Response shape alignment

## 6. Key Implementation Patterns

### 6.1 Handler Structure
```go
type HandlerDeps struct {
    DB           *sql.DB
    SessionStore sessions.Store
    SessionName  string
    Logger       zerolog.Logger
}

func SomeHandler(deps HandlerDeps) echo.HandlerFunc {
    return func(c echo.Context) error {
        // Implementation
    }
}
```

### 6.2 Database Queries with Jet
```go
// Type-safe query generation
stmt := SELECT(
    Events.ID,
    Events.Name,
    Events.CreatedAt,
).FROM(
    Events,
).WHERE(
    Events.UserID.EQ(String(userID)).
    AND(Events.DeletedAt.IS_NULL()),
).ORDER_BY(Events.CreatedAt.DESC())
```

### 6.3 Session Authentication
```go
// Middleware to check session
session, _ := deps.SessionStore.Get(c.Request(), deps.SessionName)
userID, ok := session.Values["user_id"].(string)
if !ok || userID == "" {
    return echo.NewHTTPError(http.StatusUnauthorized)
}
```

### 6.4 Error Handling
```go
// Centralized error responses
type ErrorResponse struct {
    Error ErrorDetail `json:"error"`
}

type ErrorDetail struct {
    Code    string `json:"code"`
    Message string `json:"message"`
}

// Sanitized errors (no internal details)
return echo.NewHTTPError(http.StatusBadRequest, ErrorResponse{
    Error: ErrorDetail{
        Code:    "validation_failed",
        Message: "Invalid input",
    },
})
```

## 7. Testing Approach

### 7.1 Backend Testing
```go
// Integration tests with real SQLite
func setupTestDB(t *testing.T) *sql.DB {
    tmpfile, _ := os.CreateTemp("", "test-*.db")
    db, _ := sql.Open("sqlite", tmpfile.Name())
    
    // Run migrations
    goose.Up(db, "../../migrations")
    
    t.Cleanup(func() {
        db.Close()
        os.Remove(tmpfile.Name())
    })
    
    return db
}

// HTTP testing with Echo
e := echo.New()
req := httptest.NewRequest(http.MethodGet, "/api/health", nil)
rec := httptest.NewRecorder()
e.ServeHTTP(rec, req)
```

### 7.2 Frontend Testing
- MSW for API mocking (no real backend needed)
- Direct mocking of HTTP client functions
- Maintained existing test structure

## 8. Frontend Migration Strategy

### 8.1 HTTP Client Implementation
```typescript
// Replace PocketBase SDK
export const httpClient = {
  async request(path: string, options: RequestInit = {}) {
    const response = await fetch(`${API_URL}${path}`, {
      ...options,
      credentials: 'include', // For cookies
      headers: {
        'Content-Type': 'application/json',
        ...options.headers,
      },
    });
    
    if (!response.ok) {
      const error = await response.json();
      throw new ApiError(response.status, error.error.message);
    }
    
    return response.json();
  }
};
```

### 8.2 CSRF Token Handling
```typescript
// Automatic CSRF token management
async function getCsrfToken(): Promise<string> {
  // Try cookie first
  const cookie = document.cookie
    .split('; ')
    .find(row => row.startsWith('ebjoy_csrf='));
    
  if (cookie) {
    return cookie.split('=')[1];
  }
  
  // Fallback to API endpoint
  const response = await fetch('/api/csrf', {
    credentials: 'include'
  });
  const data = await response.json();
  return data.csrf;
}

// Attach to requests
const csrfToken = await getCsrfToken();
headers['X-CSRF-Token'] = csrfToken;
```

### 8.3 Timestamp Conversion
```typescript
// API returns epoch seconds, UI uses ISO strings
export function epochToISO(epochSeconds: number): string {
  return new Date(epochSeconds * 1000).toISOString();
}

export function isoToEpoch(isoString: string): number {
  return Math.floor(new Date(isoString).getTime() / 1000);
}
```

## 9. Production Considerations

### 9.1 Environment Configuration
```env
# Backend
APP_ENV=production
PORT=8091
DB_PATH=/data/prod.sqlite
CORS_ALLOW_ORIGINS=https://app.example.com
SESSION_SECURE=true
SESSION_COOKIE_NAME=app_session
SESSION_TTL_SECONDS=2592000
BCRYPT_COST=12

# Frontend
VITE_API_URL=https://api.example.com
```

### 9.2 Deployment Changes
- No more PocketBase binary
- Standard Go binary deployment
- Static file hosting for frontend
- Database file persistence
- File upload directory persistence

### 9.3 Monitoring & Logging
- Structured JSON logs with Zerolog
- Request ID tracking
- Access logs with latency
- Error aggregation ready

## 10. Lessons Learned

### 10.1 What Worked Well
- RFC-driven development with detailed checklists
- Incremental migration phases
- Maintaining API compatibility for frontend
- Comprehensive test coverage
- Makefile automation

### 10.2 Key Decisions
- Jet over GORM for type safety
- Cookie sessions over JWT
- Unix timestamps for consistency
- Option A query parameters only
- Keeping frontend changes minimal

### 10.3 Migration Tips
1. Start with health endpoint and basic setup
2. Implement auth early (many endpoints depend on it)
3. Keep response shapes identical to PocketBase
4. Test each phase thoroughly before moving on
5. Document API changes clearly
6. Use same field names as PocketBase collections
7. Implement CSRF protection early
8. Plan file storage structure upfront

## 11. Code Patterns to Reuse

### 11.1 Dependency Injection
- All handlers receive dependencies via structs
- Testable and mockable
- Clear dependency declaration

### 11.2 Middleware Composition
- Standard Echo middleware
- Custom auth middleware
- CORS and CSRF configuration

### 11.3 Database Patterns
- Connection pooling (MaxOpenConns=1 for SQLite)
- Transaction helpers
- Consistent error handling

### 11.4 Testing Patterns
- Real database for integration tests
- Temporary files cleaned up
- Parallel test execution
- Table-driven tests

This migration approach proved successful for EBJoy and can be adapted for Spotube's migration from PocketBase to Echo.
