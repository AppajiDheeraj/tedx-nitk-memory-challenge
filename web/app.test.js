const test = require('node:test');
const assert = require('node:assert/strict');

const { formatTime, validUsername, validEmail, canPlayDifficulty } = require('./app.js');

test('formatTime clamps invalid or negative values', () => {
  assert.equal(formatTime(65), '1:05');
  assert.equal(formatTime(-4), '0:00');
  assert.equal(formatTime('bad'), '0:00');
});

test('validUsername mirrors the server public-name boundary', () => {
  assert.equal(validUsername('signal_7'), true);
  assert.equal(validUsername('ab'), false);
  assert.equal(validUsername('has space'), false);
  assert.equal(validUsername('abcdefghijklmnopqrstuvwxy'), false);
});

test('canPlayDifficulty uses only server progress', () => {
  const progress = { easy: true, medium: true, hard: false };
  assert.equal(canPlayDifficulty(progress, 'easy'), true);
  assert.equal(canPlayDifficulty(progress, 'medium'), true);
  assert.equal(canPlayDifficulty(progress, 'hard'), false);
  assert.equal(canPlayDifficulty({}, 'easy'), false);
});

test('validEmail accepts ordinary addresses only', () => {
  assert.equal(validEmail('person@example.com'), true);
  assert.equal(validEmail(' a.b+c@nitk.edu.in '), true);
  assert.equal(validEmail('person@localhost'), false);
  assert.equal(validEmail('no-at-sign'), false);
  assert.equal(validEmail('two words@example.com'), false);
});
