# TEDxNITKSurathkal Memory Challenge Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Ship the existing memory game as a free public TEDxNITKSurathkal experience with Google sign-in, private identities, public usernames, server-verified progression and leaderboards, the current theme reveal, and a verified live deployment.

**Architecture:** A single Go/Gin service serves static assets, Google OIDC/session endpoints, and the authenticated game API. Neon PostgreSQL stores players, hashed sessions, games, and cards; Render runs the service and startup migrations.

**Tech Stack:** Go 1.26+, Gin, `database/sql` + PostgreSQL, `coreos/go-oidc`, `golang.org/x/oauth2`, HTML/CSS/JavaScript, Neon Free, Render Free, Google OpenID Connect.

**Spec:** `docs/superpowers/specs/2026-10-23-tedx-nitk-memory-challenge-design.md`

## Global Constraints

- Use only free plans; accept documented Render and Neon cold starts.
- Any Google account may sign in; Google `sub` is identity and email remains private.
- Public usernames are 3–24 ASCII letters, digits, or underscores and unique case-insensitively.
- Gameplay state, progression, timers, ownership, and ranking remain server-authoritative.
- Hard is the prize leaderboard; retries are unlimited and each player's best completion ranks.
- Use TED red `#EB0028`, `TEDxNITKSurathkal`, and official event-owned reveal assets without redrawing the logo.
- Do not add a framework, CMS, admin dashboard, analytics platform, or WebAssembly build.

## Review Focus

- Forged or missing session cookies must return 401 without leaking whether a Google account exists (Task 2 tests).
- A signed-in player must never read or mutate another player's game by changing an ID (Task 3 tests).
- Concurrent flips and duplicate game creation must preserve one legal game state and one active attempt (Task 3 tests).
- Usernames differing only by case must conflict and unsafe characters must fail without changing the prior username (Task 2 tests).
- OAuth/database/free-tier wake-up failures must leave a retryable UI instead of a blank or partially authenticated page (Tasks 4 and 6 checks).

---

### Task 1: Production Schema and Startup Configuration

**Files:**
- Create: `db/migrations/000005_add_authenticated_players.up.sql`
- Create: `db/migrations/000005_add_authenticated_players.down.sql`
- Create: `internal/store/migrations.go`
- Create: `internal/store/migrations_test.go`
- Modify: `internal/store/database.go`
- Modify: `main.go`

**Interfaces:**
- Produces: `store.Migrate(ctx context.Context, db *sql.DB) error`; final `players`, `sessions`, `games`, and `game_cards` schema.
- Produces environment contract: `DATABASE_URL`, `PORT`, `BASE_URL`, `GOOGLE_CLIENT_ID`, `GOOGLE_CLIENT_SECRET`, `SESSION_SECRET`, `COOKIE_SECURE`.

- [ ] **Step 1: Write failing migration tests** that assert sorted one-time application, rollback-on-error, and idempotent reruns using a test executor.
- [ ] **Step 2: Run `go test ./internal/store -run TestMigrate -v`** and confirm failure because `Migrate` is undefined.
- [ ] **Step 3: Add the auth/session migration**: nullable-to-onboarding `players.username`, unique `google_subject`, private verified email/profile columns, case-insensitive username index, hashed-token `sessions`, and authenticated ownership constraints.
- [ ] **Step 4: Implement `Migrate`** with embedded `.up.sql` files, a `schema_migrations` table, transaction-per-file behavior, and a PostgreSQL advisory lock; configure bounded DB pooling.
- [ ] **Step 5: Update startup** to migrate before serving and bind to `PORT` with `8082` as local default.
- [ ] **Step 6: Run `go test ./internal/store ./...`** and expect PASS.
- [ ] **Step 7: Commit** with `feat: add production database migrations`.

### Task 2: Google OIDC, Sessions, and Usernames

**Files:**
- Create: `internal/auth/service.go`
- Create: `internal/auth/service_test.go`
- Create: `internal/handlers/auth_handler.go`
- Create: `internal/handlers/auth_handler_test.go`
- Modify: `main.go`
- Modify: `go.mod`
- Modify: `go.sum`

**Interfaces:**
- Produces: `auth.NewService(db, oauth2Config, verifier, baseURL, secureCookies) *Service`.
- Produces: `Service.BeginLogin`, `Service.CompleteLogin`, `Service.CurrentPlayer`, `Service.SetUsername`, `Service.Logout`, and `auth.RequirePlayer` Gin middleware.
- Produces endpoints: `GET /auth/google`, `GET /auth/google/callback`, `POST /auth/logout`, `GET /me`, `PUT /me/username`.

- [ ] **Step 1: Write failing tests** asserting `ValidUsername("signal_7") == true`, invalid whitespace/punctuation/lengths are false, expired/unknown token hashes return no player, case-only duplicate usernames conflict, and valid OIDC claims persist only private identity fields.
- [ ] **Step 2: Run `go test ./internal/auth ./internal/handlers -run 'Test(Auth|Username|Session)' -v`** and confirm missing-package failures.
- [ ] **Step 3: Implement authorization-code OIDC** with state, PKCE, exact callback URI, verified ID token, verified-email requirement, opaque 32-byte session token, SHA-256 database hash, seven-day expiry, and secure cookie flags.
- [ ] **Step 4: Implement handlers and middleware** with generic auth errors, JSON `me` responses containing only player ID, username, picture, and progress, plus conflict-safe username updates.
- [ ] **Step 5: Wire routes and dependencies in `main.go`** and add only `coreos/go-oidc` and `golang.org/x/oauth2`.
- [ ] **Step 6: Run auth/handler tests and `go test ./...`**; expect PASS.
- [ ] **Step 7: Commit** with `feat: add Google sign-in and player sessions`.

### Task 3: Authenticated Game Ownership, Progression, and Rankings

**Files:**
- Modify: `internal/game/models.go`
- Modify: `internal/game/service.go`
- Create: `internal/game/service_test.go`
- Modify: `internal/handlers/game_handler.go`
- Create: `internal/handlers/game_handler_test.go`
- Create: `internal/handlers/rate_limit.go`
- Create: `internal/handlers/rate_limit_test.go`
- Modify: `main.go`

**Interfaces:**
- Consumes: authenticated player ID from Task 2 middleware.
- Produces: `CreateGame(playerID int64, difficulty string)`, `GetGame(playerID, gameID int64)`, `FlipCard(playerID, gameID int64, position int)`, `ResolveMismatch(playerID, gameID int64)`, and `GetLeaderboard(difficulty string, limit int)`.
- Produces leaderboard response with rank, username, difficulty, moves, elapsed seconds, completed time, and attempt count only.

- [ ] **Step 1: Write failing service/handler tests** for locked Medium/Hard, unlock progression, cross-player 404s, one active attempt, hidden values, concurrent duplicate flips, per-difficulty best-run ranking, and no private fields.
- [ ] **Step 2: Run targeted game/handler tests** and confirm signature/behavior failures.
- [ ] **Step 3: Change game queries to require player ownership**, expire a prior active attempt in the create transaction, and derive unlocks solely from completed database games.
- [ ] **Step 4: Replace leaderboard query** with one best completion per player for the requested difficulty, ordered by moves, elapsed time, then completion time; default the public modal to Hard.
- [ ] **Step 5: Add a small fixed-window limiter** for auth starts, game creation, flips, and leaderboard reads with bounded cleanup and `429 Retry-After` responses.
- [ ] **Step 6: Update handlers** to remove `player_name`, consume the authenticated player, validate difficulty/limits, and preserve non-disclosing ownership errors.
- [ ] **Step 7: Run `go test -race ./...`** and expect PASS.
- [ ] **Step 8: Commit** with `feat: secure game progression and leaderboards`.

### Task 4: Polished Authenticated Frontend and Theme Reveal

**Files:**
- Modify: `web/index.html`
- Create: `web/styles.css`
- Create: `web/app.js`
- Create: `web/assets/tedx-white.png`
- Create: `web/assets/threshold-1600.webp`
- Modify: `main.go`

**Interfaces:**
- Consumes: `/me`, auth endpoints, username endpoint, authenticated game API, and difficulty-filtered leaderboard from Tasks 2–3.
- Produces: signed-out landing, username onboarding, game shell, server-driven progression, leaderboard tabs, and final reveal.

- [ ] **Step 1: Split the existing inline UI** into semantic HTML, focused CSS, and plain JavaScript while retaining the board interaction and API error handling.
- [ ] **Step 2: Add auth/onboarding states** with “Continue with Google,” username validation feedback, profile/sign-out controls, and no email rendering.
- [ ] **Step 3: Replace localStorage progression** with `/me` progress, authenticated game creation, and retryable expired-session/offline/cold-start handling.
- [ ] **Step 4: Implement accessible interaction**: focus-managed dialogs, Escape close, keyboard/touch cards, visible focus, live status, reduced motion, and mobile layouts.
- [ ] **Step 5: Add official assets from `tedxnitksurathkal.in`** and the Hard-completion reveal for “The Noise Paradox — The Quiet Signal,” linking to the canonical event site.
- [ ] **Step 6: Serve CSS/JS/assets with correct content types and cache policy**, then run `go test ./...` and a local browser smoke check.
- [ ] **Step 7: Commit** with `feat: deliver polished TEDx game experience`.

### Task 5: Deployment, Operations, and Repository Documentation

**Files:**
- Create: `.env.example`
- Create: `render.yaml`
- Create: `.gitignore`
- Modify: `README.md`
- Create: `docs/operations.md`
- Create: `scripts/load-smoke.js`

**Interfaces:**
- Consumes: environment contract and health endpoint from Tasks 1–4.
- Produces: reproducible local setup, Render deploy, Neon migration/startup, Google redirect setup, event-day checklist, export procedure, and a dependency-free load smoke command.

- [ ] **Step 1: Add deployment configuration** using Render's free Go web service, health check `/health`, `go build`, and secret environment placeholders.
- [ ] **Step 2: Add safe configuration examples and ignore rules** with no real credentials or connection strings.
- [ ] **Step 3: Replace the empty README and add the runbook** covering local setup, OAuth redirect URIs, cold starts, pre-event wake-up, result export, rollback, and free-tier limitations.
- [ ] **Step 4: Add a Node standard-library load smoke script** that sends 200 sparse concurrent health/leaderboard requests and, when `LOAD_TEST_COOKIE` is supplied, authenticated game reads; fail on non-2xx responses or documented latency/error thresholds without adding a production test mode.
- [ ] **Step 5: Run `go test -race ./...`, `go vet ./...`, `go build ./...`, and the documented local smoke path**; expect all PASS.
- [ ] **Step 6: Commit** with `docs: add free deployment and event runbook`.

### Task 6: GitHub, Neon, Google, Render, and Live Verification

**Files:**
- Modify only deployment metadata or documentation if live verification exposes a concrete defect.

**Interfaces:**
- Consumes: verified repository from Tasks 1–5.
- Produces: public GitHub repository, Neon project, Google OAuth web client, Render service, and final public HTTPS URL.

- [ ] **Step 1: Create `AppajiDheeraj/tedx-nitk-memory-challenge`** as public, set the approved description/topics, push `main`, and verify no secrets with `git grep` plus GitHub's file view.
- [ ] **Step 2: Create a Neon Free project/database**, capture the pooled TLS connection string as a Render secret, and verify migrations/startup against it.
- [ ] **Step 3: Create/configure Google OAuth consent and web credentials** for localhost plus the final Render callback, limiting scopes to `openid email profile`.
- [ ] **Step 4: Create the Render Free service** from the repository, set secrets, deploy, and verify healthy build/startup logs.
- [ ] **Step 5: Run production checks** for HTTPS/security headers, signed-out page, Google redirect, authenticated onboarding, Easy→Medium→Hard progression, leaderboard privacy, final reveal, mobile viewport, cold wake-up, and database persistence.
- [ ] **Step 6: Patch and redeploy only concrete failures**, repeat the affected check plus the full test/build suite, and commit each fix.
- [ ] **Step 7: Record the live URL and deployment date in `README.md`**, push, and confirm the final deployment matches that commit.
- [ ] **Step 8: Commit** with `docs: record production deployment`.

