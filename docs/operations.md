# Event operations runbook

This runbook is for the TEDxNITKSurathkal organizer operating the free Render and Neon deployment.

## Before the event

1. Confirm the Render deployment is green and the Neon project is active.
2. Confirm `DATABASE_URL` is set on the Render service.
3. Open the site, enter a test email and username, and complete a test game.
4. Check Easy, Medium, and Hard leaderboard tabs on both a phone and a laptop.
5. Wake the service 15 minutes before play, then run:

```sh
node scripts/load-smoke.js https://your-service.onrender.com
```

A passing check has zero failed requests and public-route p95 below five seconds after warm-up. Render Free may cold-start; the script allows up to 90 seconds for that first wake-up.

## Required production settings

| Variable | Expected value |
| --- | --- |
| `DATABASE_URL` | Neon PostgreSQL connection string (`sslmode=require`) |
| `COOKIE_SECURE` | Optional; defaults to `true` on Render |

Never paste secret values into issues, chat, screenshots, or the repository.

## During play

- Keep the Render logs and Neon monitoring pages open.
- If nobody can start a game, check the Render logs and that `DATABASE_URL` is set and the Neon project is active.
- If the first request is slow, wait for the service to wake and retry; players who return use the same email and username to resume.
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
- If the database password is exposed, reset it in Neon, update `DATABASE_URL` in Render, and redeploy.

## After the event

1. Export and securely store the final winner list.
2. Announce winners only after identity verification.
3. Remove any local exports containing emails once handoff is complete.
4. Keep the public repository free of production credentials.

Free tiers have no uptime guarantee. The practical mitigation for this event size is to wake the app early, run the included load check, keep one organizer watching logs, and retain the verified export before announcing prizes.
