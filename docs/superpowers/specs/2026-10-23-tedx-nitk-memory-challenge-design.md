# TEDxNITKSurathkal Memory Challenge Design

## Intent

Ship a public, polished memory game for TEDxNITKSurathkal on a zero-cost stack. Any Google user can sign in, choose a public username, play progressively harder boards, appear on a server-verified leaderboard, and reveal the current event theme after completing the challenge. The result must be deployable from a new repository owned by `AppajiDheeraj` and usable by roughly 150–180 sparse players.

## Goals

- Accept any Google account through OpenID Connect and keep email addresses private.
- Require a unique public username before play; use Google's immutable `sub` claim as account identity.
- Move progression, game state, timing, and leaderboard qualification to the server.
- Show separate Easy, Medium, and Hard leaderboards; use each player's best Hard completion for prize ranking.
- Reveal “The Noise Paradox — The Quiet Signal” after the player completes Hard, using the event site's approved wording and visual direction.
- Meet core mobile, keyboard, accessibility, error-state, and responsive-design expectations.
- Deploy the Go application on Render Free and PostgreSQL on Neon Free from a public GitHub repository.

## Non-goals

- Password, email-link, institutional-domain, or non-Google login.
- Converting the game to TinyGo, WebAssembly, Next.js, or Better Auth.
- A general CMS, admin dashboard, social sharing system, analytics platform, or multiple event themes.
- Claiming perfect bot prevention in a browser game; final prize eligibility remains subject to organizer review.
- Paid hosting, a custom domain, or uptime guarantees.

## Anti-goals

- Do not expose player email addresses or Google identifiers in HTML, APIs, logs, or leaderboard responses.
- Do not trust usernames, difficulty unlocks, elapsed time, card layouts, or scores supplied by the browser.
- Do not copy last year's local-storage identity or client-submitted score design.
- Do not recreate or distort the TEDx event logo. Use an approved event-owned asset when available; otherwise use compliant text branding until it is supplied.

## Constraints

- Hosting and managed database must remain on free plans.
- Render Free can sleep after 15 idle minutes and may take about one minute to wake.
- Neon Free scales to zero after five minutes and supplies 1 GB storage per project; this is ample for the expected audience.
- Google Cloud OAuth configuration requires authorized local and production redirect URIs.
- TED red is `#EB0028`; supporting colors are black, white, and restrained neutral tones.
- The public event name is `TEDxNITKSurathkal`.

## Approaches Considered

### Selected: Go/Gin + Google OIDC + Neon + Render

Keep the existing server-authoritative Go application, add a small OIDC/session implementation, use hosted PostgreSQL, and serve the static frontend from the same process. This preserves the working game transaction logic and produces one deployable service. The accepted tradeoff is free-tier cold starts.

### Rejected: Next.js + Better Auth + Neon + Vercel

Better Auth fits TypeScript well, but adopting it would require rewriting the existing Go handlers, transactions, and game service. That is more code and regression risk without a capacity or cost benefit for this event.

### Rejected: Supabase Auth/Postgres + Edge Functions

This integrates auth and data but still requires porting the Go backend to a different runtime and designing database policies. It adds platform-specific surface area that the current application does not need.

## Architecture

One Go/Gin process serves the frontend, OAuth endpoints, authenticated game API, leaderboard API, and health checks. Neon PostgreSQL stores players, sessions, games, and cards. Google handles authentication only; authorization, usernames, progression, and game eligibility remain in this application.

The browser receives only the cards currently revealed or matched. It never receives pair identifiers, hidden card values, authoritative timestamps, private identity fields, or a writable score. Session cookies are opaque, `HttpOnly`, `Secure` in production, and `SameSite=Lax`.

## Authentication and Player Identity

The authorization-code flow uses Google OIDC with `openid email profile`, state validation, PKCE, exact redirect URIs, and ID-token validation. On callback, the server upserts a player by Google `sub`, updates verified email/profile fields, creates a random session, stores only the session-token hash, and redirects to the game.

Authenticated users without a username see a one-step onboarding form. Usernames are 3–24 ASCII letters, digits, or underscores, are unique case-insensitively, and are the only identity shown publicly. Sign-out deletes the server session and clears the cookie. Sessions expire after seven days.

## Data Model

- `players`: one row per Google account with unique `google_subject`, private verified email, optional profile fields, unique public username, and timestamps.
- `sessions`: hashed token, player foreign key, expiry, and creation timestamp; expired sessions are rejected and opportunistically deleted.
- `games`: existing authoritative attempt data plus authenticated player ownership and server timestamps.
- `game_cards`: existing hidden board values and reveal/match state.

Production starts with a fresh Neon database; existing anonymous development rows are not migrated because they cannot be safely linked to Google identities. Migrations remain ordered SQL files and run as an explicit deployment command.

## Game and Ranking Rules

- Easy is initially available; completing Easy unlocks Medium; completing Medium unlocks Hard.
- The server rejects attempts for locked difficulties.
- One account can have only one in-progress game at a time; starting another expires the old attempt.
- The existing five-minute limit remains server-authoritative.
- Leaderboards are separate by difficulty and show one best completed run per player.
- Ranking is fewest moves, then shortest server-measured elapsed time, then earliest completion.
- The prize view uses Hard only and never exposes email; organizers identify a potential winner privately by looking up the stored account after the event.
- Completed attempts are unlimited; only each player's best Hard result is prize-ranked, and attempt counts remain available for organizer review.
- Basic per-IP and per-player request limits protect auth, game creation, flips, and leaderboard endpoints. Database transactions continue to serialize game actions.

## User Experience

The landing state introduces the memory challenge and offers “Continue with Google.” After sign-in, first-time users choose a public username. Returning users see their username, progress, and a sign-out action.

The existing board interaction remains, but difficulty locks come from the API instead of `localStorage`. Loading, wake-up, expired-session, duplicate-username, offline, timer-expiry, and retry states are visible and recoverable. Dialog focus is trapped and restored; cards and controls work with keyboard and touch; reduced-motion preferences disable nonessential animation.

Completing Hard opens a full completion reveal: “The Noise Paradox,” the subtitle “The Quiet Signal,” and the approved manifesto line “In a world that never stops, what truly matters often whispers.” The reveal links to `https://www.tedxnitksurathkal.in/` and reuses the event-owned `tedx-white.png` logo and `quiet-signal/threshold-1600.webp` theme artwork published by that site without redrawing either asset. Easy and Medium completions show compact progression messages rather than the final theme.

## Repository and Deployment

Create public repository `AppajiDheeraj/tedx-nitk-memory-challenge` with description:

> A polished TEDxNITKSurathkal memory challenge with Google sign-in, server-verified leaderboards, and a themed completion reveal.

Topics: `tedx`, `tedx-nitk-surathkal`, `memory-game`, `golang`, `gin`, `postgresql`, `google-oauth`, `leaderboard`, `web-game`.

The repository contains the cleaned current code, tests, migrations, `.env.example`, Render blueprint, deployment/runbook documentation, and no secrets. Render receives the Neon connection string, Google credentials, public base URL, and a random session-secret value as environment variables. Production OAuth is configured only after Render provides the final hostname.

## Verification and Operations

- Unit tests cover username validation, session expiry, progression, timer calculation, board generation, and leaderboard ordering.
- Handler/integration tests cover unauthenticated access, ownership, OAuth callback failure, hidden-card secrecy, locked levels, duplicate usernames, and expired games.
- Migration smoke test runs against PostgreSQL.
- Browser smoke tests cover sign-in redirect, onboarding, one full game flow, leaderboard, mobile layout, keyboard use, and final reveal.
- A small concurrent-load check exercises 200 signed-in-style sessions against game creation, flips, and leaderboard reads without relying on production Google login.
- Deployment verification checks health, TLS, security headers, OAuth redirect, database persistence, and a fresh incognito session.
- Before the event, an organizer wakes the free service and runs the smoke checklist. Leaderboard data is exported after the event before any free-plan cleanup.

## Calls Made by the Builder

- Use direct Google OIDC instead of Better Auth because the retained backend is Go.
- Use a public username to avoid exposing Google names or emails.
- Treat Hard as the prize leaderboard so different board sizes are not compared unfairly.
- Use the current official site theme, “The Noise Paradox — The Quiet Signal,” as the final reveal.
- Accept free-tier cold starts instead of adding paid infrastructure or a keep-alive workaround.

