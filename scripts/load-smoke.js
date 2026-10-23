'use strict';

function buildPaths(total, { cookie, gameID }) {
  const publicPaths = ['/health', '/leaderboard?difficulty=hard&limit=20'];
  const paths = [];
  for (let index = 0; index < total; index++) {
    if (cookie && gameID && index % 3 === 2) paths.push(`/games/${encodeURIComponent(gameID)}`);
    else paths.push(publicPaths[index % publicPaths.length]);
  }
  return paths;
}

function summarize(results) {
  const durations = results.map((result) => result.duration).sort((a, b) => a - b);
  const errors = results.filter((result) => !result.ok).length;
  const p95Index = Math.max(0, Math.ceil(durations.length * 0.95) - 1);
  return {
    total: results.length,
    errors,
    errorRate: results.length ? errors / results.length : 0,
    p95: durations[p95Index] || 0,
    max: durations[durations.length - 1] || 0,
  };
}

async function run() {
  const baseURL = String(process.argv[2] || process.env.BASE_URL || '').replace(/\/$/, '');
  if (!/^https?:\/\//.test(baseURL)) throw new Error('Usage: node scripts/load-smoke.js https://your-service.example');
  const total = Number(process.env.LOAD_TEST_REQUESTS || 200);
  const concurrency = Number(process.env.LOAD_TEST_CONCURRENCY || 20);
  const cookie = process.env.LOAD_TEST_COOKIE || '';
  const gameID = process.env.LOAD_TEST_GAME_ID || '';
  const paths = buildPaths(total, { cookie, gameID });

  const warm = await fetch(`${baseURL}/health`, { signal: AbortSignal.timeout(90_000) });
  if (!warm.ok) throw new Error(`Warm-up failed with ${warm.status}`);

  const results = new Array(paths.length);
  let cursor = 0;
  async function worker() {
    while (cursor < paths.length) {
      const index = cursor++;
      const started = performance.now();
      try {
        const headers = cookie && paths[index].startsWith('/games/') ? { Cookie: cookie } : {};
        const response = await fetch(`${baseURL}${paths[index]}`, { headers, signal: AbortSignal.timeout(15_000) });
        results[index] = { ok: response.ok, duration: Math.round(performance.now() - started), status: response.status };
        await response.arrayBuffer();
      } catch (error) {
        results[index] = { ok: false, duration: Math.round(performance.now() - started), status: 0, error: error.message };
      }
    }
  }
  await Promise.all(Array.from({ length: Math.min(concurrency, total) }, worker));
  const summary = summarize(results);
  console.log(JSON.stringify(summary, null, 2));
  if (summary.errors > 0 || summary.p95 > 5_000) process.exitCode = 1;
}

module.exports = { buildPaths, summarize };
if (require.main === module) run().catch((error) => { console.error(error.message); process.exitCode = 1; });
