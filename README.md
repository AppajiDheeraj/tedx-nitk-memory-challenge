# TEDxNITKSurathkal Memory Challenge

A production-ready memory game for TEDxNITKSurathkal. Players sign in with Google, choose a public username, clear Easy, Medium, and Hard in order, and compete on a server-verified leaderboard. Finishing Hard reveals this year's theme, **The Noise Paradox — The Quiet Signal**.

## What is included

- Google OpenID Connect sign-in; email stays private
- Case-insensitive, unique public usernames
- Server-generated boards and server-verified scores
- Sequential difficulty unlocks and one best result per player
- Hard-mode prize ranking with attempt counts and deterministic ties
- Responsive TEDx-branded interface and accessible keyboard interactions
- PostgreSQL migrations, rate limiting, secure cookies, and security headers
- Render Free deployment blueprint and a lightweight event load check

## Architecture

The application is one Go binary: Gin serves the static interface and JSON API, while PostgreSQL stores identities, sessions, games, and results. Google provides identity only; authorization and public usernames belong to this application.

This intentionally avoids WebAssembly. The game has little client-side computation, while authentication, score integrity, and the leaderboard require a server regardless. A small Go service is simpler to operate and fits the expected 150–180 sparse players.

## Local setup

Requirements: Go, Node.js 20+ for browser-script tests, PostgreSQL, and a Google OAuth web client.

```sh
cp .env.example .env
# Fill in .env, then export it into the shell:
set -a
source .env
set +a
go run .
```

Open `http://localhost:8082`. Configure this exact local Google redirect URI:

```text
http://localhost:8082/auth/google/callback
```

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

## Free deployment

`render.yaml` defines a Render Free web service. Create a free Neon PostgreSQL project, create a Google OAuth web client, deploy the repository as a Render Blueprint, and set the secrets listed in `.env.example`. The production redirect URI is:

```text
https://your-service.onrender.com/auth/google/callback
```

Free services can sleep and briefly take longer on the first request. Wake and verify the service before opening the game to attendees. Detailed event checks, prize export SQL, incident handling, and rollback steps are in [docs/operations.md](docs/operations.md).

## Privacy and prizes

Only the chosen username, score, difficulty, and attempt count are public. Google email and subject identifiers are never returned by public APIs. Organizers should validate winners using the private database export described in the runbook and handle that export as personal data.

See the [approved design](docs/superpowers/specs/2026-10-23-tedx-nitk-memory-challenge-design.md) and [implementation plan](docs/superpowers/plans/2026-10-23-tedx-nitk-memory-challenge.md) for the complete product contract.
