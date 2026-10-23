const test = require('node:test');
const assert = require('node:assert/strict');

const { buildPaths, summarize } = require('./load-smoke.js');

test('buildPaths includes only public routes without authenticated inputs', () => {
  const paths = buildPaths(4, {});
  assert.deepEqual(paths, ['/health', '/leaderboard?difficulty=hard&limit=20', '/health', '/leaderboard?difficulty=hard&limit=20']);
});

test('buildPaths adds owned game reads only when cookie and game id exist', () => {
  const paths = buildPaths(3, { cookie: 'tedx_session=x', gameID: '12' });
  assert.deepEqual(paths, ['/health', '/leaderboard?difficulty=hard&limit=20', '/games/12']);
});

test('summarize computes errors and nearest-rank p95', () => {
  const summary = summarize([
    { ok: true, duration: 10 },
    { ok: true, duration: 20 },
    { ok: false, duration: 30 },
    { ok: true, duration: 40 },
  ]);
  assert.deepEqual(summary, { total: 4, errors: 1, errorRate: 0.25, p95: 40, max: 40 });
});
