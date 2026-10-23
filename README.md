# TEDxNITKSurathkal Memory Challenge

A production-ready memory game for TEDxNITKSurathkal. Players enter their email and a public username, clear Easy, Medium, and Hard in order, and compete on a server-verified leaderboard. Finishing Hard reveals this year's theme, **The Noise Paradox — The Quiet Signal**.

## What is included

- No third-party sign-in: email + username entry; email stays private
- Returning players resume with the same email and username
- Case-insensitive, unique public usernames
- Server-generated boards and server-verified scores
- Sequential difficulty unlocks and one best result per player
- Hard-mode prize ranking with attempt counts and deterministic ties
- Responsive TEDx-branded interface and accessible keyboard interactions
- PostgreSQL migrations, rate limiting, secure cookies, and security headers
- Render Free deployment blueprint and a lightweight event load check

## Architecture

The application is one Go binary: Gin serves the static interface and JSON API, while PostgreSQL stores players, sessions, games, and results. There is no external identity provider: a player is identified by email, and their session is an HttpOnly cookie.

This intentionally avoids WebAssembly. The game has little client-side computation, while authentication, score integrity, and the leaderboard require a server regardless. A small Go service is simpler to operate and fits the expected 150–180 sparse players.

## Local setup

Requirements: Go, Node.js 20+ for browser-script tests, and PostgreSQL.

```sh
cp .env.example .env
# Fill in .env, then export it into the shell:
set -a
source .env
set +a
go run .
```

Open `http://localhost:8082`.

The server applies embedded SQL migrations during startup.

## Verification

```sh
go test -race ./...
go vet ./...
go build ./...
node --test web/app.test.js scripts/load-smoke.test.js
node --check web/app.js
node --check scripts/load-smoke.js
```

After deployment, wake the free service and exercise its public read paths:

```sh
node scripts/load-smoke.js https://your-service.onrender.com
```

## Free deployment (Render + Neon)

1. In Neon, open the project → **Connect** and copy the connection string (pooled or direct both work).
2. In Render, open the web service → **Environment** and set `DATABASE_URL` to that string. Nothing else is required.
3. Deploy. The server creates and migrates the tables on startup; `/health` turns green when it is serving.

`render.yaml` describes the same service if you prefer creating it as a Blueprint. Free services sleep after inactivity and take up to a minute to wake — open the site 15 minutes before play.  Event checks, prize export SQL, and rollback steps are in [docs/operations.md](docs/operations.md).

## Privacy and prizes

Only the chosen username, score, difficulty, and attempt count are public. Emails are never returned by any API. Organizers should validate winners using the private database export described in the runbook and handle that export as personal data.

See the [approved design](docs/superpowers/specs/2026-10-23-tedx-nitk-memory-challenge-design.md) and [implementation plan](docs/superpowers/plans/2026-10-23-tedx-nitk-memory-challenge.md) for the complete product contract.
