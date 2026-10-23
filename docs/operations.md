# Event operations runbook

This runbook is for the TEDxNITKSurathkal organizer operating the free Render and Neon deployment.

## Before the event

1. Confirm the Render deployment is green and the Neon project is active.
2. Confirm `BASE_URL` exactly matches the public `https://…onrender.com` URL.
3. Confirm the Google OAuth web client has the exact `/auth/google/callback` redirect URI.
4. Sign in with a normal Google account, set a username, and complete a test game.
5. Check Easy, Medium, and Hard leaderboard tabs on both a phone and a laptop.
6. Wake the service 15 minutes before play, then run:

```sh
node scripts/load-smoke.js https://your-service.onrender.com
```

A passing check has zero failed requests and public-route p95 below five seconds after warm-up. Render Free may cold-start; the script allows up to 90 seconds for that first wake-up.

## Required production settings

| Variable | Expected value |
| --- | --- |
| `DATABASE_URL` | Neon pooled PostgreSQL URL with TLS required |
| `BASE_URL` | Exact public Render origin, no trailing slash |
| `GOOGLE_CLIENT_ID` | Google OAuth web client ID |
| `GOOGLE_CLIENT_SECRET` | Matching Google OAuth client secret |
| `SESSION_SECRET` | Render-generated random value |
| `COOKIE_SECURE` | `true` |

Never paste secret values into issues, chat, screenshots, or the repository.

## During play

- Keep the Render logs and Neon monitoring pages open.
- If sign-in fails for everyone, check the Google redirect URI and Render environment variables before changing code.
- If the first request is slow, wait for the service to wake and retry; do not ask players to create duplicate usernames.
- If the database is unavailable, pause the contest. Scores are deliberately not accepted offline because they could not be verified.
- Do not change ranking rules after play begins.

## Winner verification

Run this in the Neon SQL editor after entries close. Keep the result private because it includes email addresses.

```sql
WITH ranked AS (
  SELECT
    p.username,
    p.email,
    g.moves,
    g.completion_time,
    g.completed_at,
    COUNT(*) OVER (PARTITION BY g.player_id, g.difficulty) AS attempt_count,
    ROW_NUMBER() OVER (
      PARTITION BY g.player_id, g.difficulty
      ORDER BY g.moves, g.completion_time, g.completed_at, g.id
    ) AS player_rank
  FROM games g
  JOIN players p ON p.id = g.player_id
  WHERE g.status = 'completed' AND g.difficulty = 'hard'
)
SELECT username, email, moves, completion_time, completed_at, attempt_count
FROM ranked
WHERE player_rank = 1
ORDER BY moves, completion_time, completed_at, username
LIMIT 20;
```

The first two or three rows are the prize candidates. Contact them using the private email only after checking the public username and result.

## Rollback and recovery

- For an application regression, use Render's deploy history to redeploy the last known-good commit. Database migrations in this release are additive and remain compatible with that release line.
- Before manual database work, export the winner query and the affected rows.
- Never delete players or games during the event. If a result needs investigation, record its game ID and preserve it.
- If a secret is exposed, rotate it in its source system, update Render, and redeploy. Invalidating `SESSION_SECRET` signs everyone out.

## After the event

1. Export and securely store the final winner list.
2. Announce winners only after identity verification.
3. Remove any local exports containing emails once handoff is complete.
4. Keep the public repository free of production credentials.

Free tiers have no uptime guarantee. The practical mitigation for this event size is to wake the app early, run the included load check, keep one organizer watching logs, and retain the verified export before announcing prizes.
