'use strict';

function formatTime(value) {
  const seconds = Math.max(0, Number.isFinite(Number(value)) ? Math.floor(Number(value)) : 0);
  return `${Math.floor(seconds / 60)}:${String(seconds % 60).padStart(2, '0')}`;
}

function validUsername(value) {
  return /^[A-Za-z0-9_]{3,24}$/.test(String(value || ''));
}

function validEmail(value) {
  return /^[^\s@<>]+@[^\s@<>]+\.[^\s@<>]+$/.test(String(value || '').trim());
}

function canPlayDifficulty(progress, difficulty) {
  return Boolean(progress && progress[difficulty]);
}

if (typeof module !== 'undefined' && module.exports) {
  module.exports = { formatTime, validUsername, validEmail, canPlayDifficulty };
}

if (typeof window !== 'undefined') {
  window.addEventListener('DOMContentLoaded', boot);
}

function boot() {
  const $ = (id) => document.getElementById(id);
  const symbols = {
    apple: '◉', banana: '◒', grapes: '✦', orange: '◐', strawberry: '⌁',
    watermelon: '◎', cherry: '◈', kiwi: '◑', mango: '✺', pineapple: '◇',
    pear: '◌', peach: '⌂', lemon: '✶', blueberry: '◍', coconut: '△'
  };
  const state = { me: null, game: null, difficulty: 'easy', busy: false, seconds: 300, deadline: 0, timer: null, first: -1, shown: [], pairs: new Set(), hideTimer: null, queue: Promise.resolve(), cardEls: [], openDialog: null, previousFocus: null };

  async function api(path, options = {}) {
    let response;
    try {
      response = await fetch(path, { ...options, headers: { 'Content-Type': 'application/json', ...(options.headers || {}) } });
    } catch (_) {
      const error = new Error('The server is waking up or you are offline. Please try again.');
      error.status = 0;
      throw error;
    }
    const data = await response.json().catch(() => ({}));
    if (!response.ok) {
      const error = new Error(data.error || `Request failed (${response.status})`);
      error.status = response.status;
      throw error;
    }
    return data;
  }

  function showEntry(message = '') {
    state.me = null;
    state.game = null;
    stopTimer();
    $('entryPanel').hidden = false;
    $('gameShell').hidden = true;
    document.body.classList.remove('is-playing');
    $('playerMenu').hidden = true;
    $('entryError').textContent = message;
  }

  function showGame() {
    $('headerUsername').textContent = state.me.username;
    $('entryPanel').hidden = true;
    $('gameShell').hidden = false;
    $('playerMenu').hidden = false;
    document.body.classList.add('is-playing');
    updateDifficultyButtons();
    fitBoard();
  }

  async function loadSession() {
    try {
      state.me = await api('/me');
      showGame();
      if (!state.game) setMessage('Pick a level, or press New game to begin.');
    } catch (error) {
      showEntry(error.status === 401 ? '' : error.message);
    }
  }

  function updateDifficultyButtons() {
    document.querySelectorAll('[data-difficulty]').forEach((button) => {
      const difficulty = button.dataset.difficulty;
      const enabled = canPlayDifficulty(state.me && state.me.progress, difficulty);
      button.disabled = !enabled || state.busy;
      button.classList.toggle('is-active', state.difficulty === difficulty);
      button.classList.toggle('is-locked', !enabled);
      button.setAttribute('aria-pressed', String(state.difficulty === difficulty));
      button.title = enabled ? `Play ${difficulty}` : 'Complete the previous level to unlock';
    });
  }

  function setMessage(text, kind = '') {
    $('gameMessage').textContent = text;
    $('gameMessage').className = `game-message ${kind}`;
  }

  // Size the board so every card fits in the space left on screen.
  function fitBoard() {
    const current = state.game;
    const board = $('board');
    if (!current || $('gameShell').hidden) return;
    const wrap = $('boardWrap');
    const gap = parseFloat(getComputedStyle(board).columnGap) || 8;
    const cell = Math.max(28, Math.min(
      (wrap.clientWidth - gap * (current.columns - 1)) / current.columns,
      (wrap.clientHeight - gap * (current.rows - 1)) / current.rows
    ));
    board.style.width = `${Math.floor(cell * current.columns + gap * (current.columns - 1))}px`;
  }

  // The board is built once per game; flips only toggle classes so the
  // browser animates transforms instead of rebuilding the DOM.
  function buildBoard() {
    const current = state.game;
    const board = $('board');
    board.style.gridTemplateColumns = `repeat(${current.columns}, minmax(0, 1fr))`;
    state.cardEls = current.cards.map((card) => {
      const button = document.createElement('button');
      button.type = 'button';
      button.className = 'memory-card';
      button.dataset.position = String(card.position);
      button.innerHTML = '<span class="card-inner"><span class="card-face card-back" aria-hidden="true"></span><span class="card-face card-front" aria-hidden="true"></span></span>';
      return button;
    });
    board.replaceChildren(...state.cardEls);
    state.cardEls.forEach((_, position) => paintCard(position));
    fitBoard();
  }

  function paintCard(position) {
    const card = state.game.cards[position];
    const el = state.cardEls[position];
    if (!card || !el) return;
    const symbol = symbols[card.value] || '';
    el.classList.toggle('is-flipped', card.revealed || card.matched);
    el.classList.toggle('is-matched', card.matched);
    el.classList.toggle('is-loading', card.revealed && !card.value);
    el.querySelector('.card-front').textContent = symbol;
    el.setAttribute('aria-label', card.matched ? `Matched ${symbol}` : card.revealed ? (card.value ? `Revealed ${symbol}` : 'Turning card') : `Hidden card ${position + 1}`);
  }

  function paintStats() {
    const current = state.game;
    if (!current) return;
    $('moves').textContent = String(current.moves);
    $('pairs').textContent = `${current.pairs_found} / ${current.total_pairs}`;
    paintTime();
  }

  function paintTime() {
    $('time').textContent = formatTime(state.seconds);
    $('timerCard').classList.toggle('is-urgent', state.seconds <= 30 && Boolean(state.game) && state.game.status === 'in_progress');
  }

  function loadGame(data) {
    state.game = { ...data, cards: data.cards.map((card) => ({ position: card.position, value: card.value || '', revealed: card.revealed, matched: card.matched })) };
    const open = state.game.cards.filter((card) => card.revealed && !card.matched).map((card) => card.position);
    clearTimeout(state.hideTimer);
    state.hideTimer = null;
    state.pairs = new Set();
    state.first = open.length === 1 ? open[0] : -1;
    state.shown = open.length === 2 ? open : [];
    if (state.shown.length) state.hideTimer = setTimeout(closeShown, 400);
    setDeadline(data.time_left_seconds);
    buildBoard();
    paintStats();
  }

  async function startGame(difficulty = state.difficulty) {
    if (!canPlayDifficulty(state.me && state.me.progress, difficulty) || state.busy) return;
    state.busy = true;
    state.difficulty = difficulty;
    stopTimer();
    updateDifficultyButtons();
    $('newGame').disabled = true;
    setMessage('Shuffling the signal…');
    try {
      loadGame(await api('/games', { method: 'POST', body: JSON.stringify({ difficulty }) }));
      setMessage('Choose two cards. Remember what you see.');
      startTimer();
    } catch (error) {
      if (error.status === 401) await loadSession();
      else setMessage(error.message, 'is-error');
    } finally {
      state.busy = false;
      $('newGame').disabled = false;
      updateDifficultyButtons();
    }
  }

  // Re-read the authoritative board after an error or when the tab returns.
  async function resync() {
    if (!state.game) return;
    const id = state.game.id;
    const data = await api(`/games/${id}`);
    if (!state.game || state.game.id !== id) return;
    loadGame(data);
    if (data.status === 'in_progress') startTimer();
    else if (data.status === 'completed') await completeLevel();
    else showExpired();
  }

  function showExpired() {
    stopTimer();
    if (state.game) state.game.status = 'expired';
    state.seconds = 0;
    paintTime();
    setMessage('Time slipped away. Press New game and listen closer.', 'is-error');
  }

  // Flip a shown mismatch back, optionally keeping one card open.
  function closeShown(keep = -1) {
    clearTimeout(state.hideTimer);
    state.hideTimer = null;
    state.shown.forEach((position) => {
      if (position === keep) return;
      state.game.cards[position].revealed = false;
      paintCard(position);
    });
    state.shown = [];
  }

  // Taps are never dropped: the card turns at once and its request joins an
  // ordered queue. The server applies flips in the same order, hiding a
  // mismatched pair itself when the next card arrives.
  function onBoardClick(event) {
    const el = event.target.closest('.memory-card');
    const current = state.game;
    if (!el || !current || current.status !== 'in_progress' || state.busy) return;
    const position = Number(el.dataset.position);
    const card = current.cards[position];
    if (!card || card.matched) return;
    let pair = null;
    if (state.shown.includes(position)) {
      // Tapping a card of the shown mismatch keeps it open as a new first card.
      closeShown(position);
      state.first = position;
    } else {
      if (card.revealed) return;
      closeShown();
      state.pairs.forEach((pending) => { pending.movedOn = true; });
      card.revealed = true;
      paintCard(position);
      if (state.first === -1) {
        state.first = position;
      } else {
        pair = { a: state.first, b: position, movedOn: false };
        state.pairs.add(pair);
        state.first = -1;
      }
    }
    const gameId = current.id;
    state.queue = state.queue.then(() => sendFlip(gameId, position, pair));
  }

  async function sendFlip(gameId, position, pair) {
    let result;
    try {
      result = await api(`/games/${gameId}/flip`, { method: 'POST', body: JSON.stringify({ position }) });
    } catch (error) {
      if (!state.game || state.game.id !== gameId) return;
      if (error.status === 401) return loadSession();
      if (/expired/.test(error.message)) return showExpired();
      setMessage(error.message, 'is-error');
      try { await resync(); } catch (_) { /* the next tap retries */ }
      return;
    }
    const current = state.game;
    if (!current || current.id !== gameId) return;
    const card = current.cards[position];
    card.value = result.value;
    current.moves = result.moves;
    current.pairs_found = result.pairs_found;
    current.status = result.status;
    if (result.status === 'in_progress') setDeadline(result.time_left_seconds);
    paintCard(position);
    if (pair) state.pairs.delete(pair);
    if (result.result === 'match') {
      const other = current.cards[result.related_position];
      other.matched = card.matched = true;
      paintCard(position);
      paintCard(other.position);
      setMessage(result.status === 'completed' ? 'Signal complete.' : 'A clear match.', 'is-success');
    } else if (result.result === 'mismatch') {
      const both = [result.related_position, position];
      if (pair && pair.movedOn) {
        both.forEach((p) => { current.cards[p].revealed = false; paintCard(p); });
      } else {
        state.shown = both;
        state.hideTimer = setTimeout(closeShown, 800);
        setMessage('Hold that pattern…');
      }
    } else if (!state.pairs.size) {
      setMessage('Now find its pair.');
    }
    paintStats();
    if (result.status === 'completed') {
      stopTimer();
      state.seconds = result.time_left_seconds;
      paintTime();
      setTimeout(completeLevel, 550);
    }
  }

  // The countdown runs against a fixed deadline, so it never drifts; every
  // flip response re-anchors it to the server clock.
  function setDeadline(secondsLeft) {
    state.deadline = Date.now() + Math.max(0, secondsLeft) * 1000;
    state.seconds = Math.max(0, secondsLeft);
  }

  function startTimer() {
    stopTimer();
    state.timer = setInterval(() => {
      if (!state.game || state.game.status !== 'in_progress') return stopTimer();
      const seconds = Math.max(0, Math.ceil((state.deadline - Date.now()) / 1000));
      if (seconds !== state.seconds) {
        state.seconds = seconds;
        paintTime();
      }
      if (seconds === 0) {
        stopTimer();
        state.queue = state.queue.then(() => resync().catch(showExpired));
      }
    }, 250);
  }

  function stopTimer() {
    if (state.timer) clearInterval(state.timer);
    state.timer = null;
  }

  async function completeLevel() {
    stopTimer();
    try { state.me = await api('/me'); } catch (_) { /* completion still renders */ }
    updateDifficultyButtons();
    const difficulty = state.game.difficulty;
    const next = difficulty === 'easy' ? 'medium' : difficulty === 'medium' ? 'hard' : '';
    $('completionCard').classList.add('is-reveal');
    if (next) {
      $('completionCopy').textContent = `${next[0].toUpperCase() + next.slice(1)} is now unlocked. Meanwhile, meet this year's confirmed speakers.`;
      $('continueLevel').hidden = false;
      $('continueLevel').dataset.next = next;
      $('continueLevel').textContent = `Continue to ${next}`;
    } else {
      $('completionCopy').textContent = 'Hard is complete. These are the voices of The Noise Paradox.';
      $('continueLevel').hidden = true;
      $('continueLevel').dataset.next = '';
    }
    openDialog($('completionDialog'));
  }

  async function loadLeaderboard(difficulty = 'hard') {
    document.querySelectorAll('[data-leader-difficulty]').forEach((tab) => {
      const active = tab.dataset.leaderDifficulty === difficulty;
      tab.classList.toggle('is-active', active);
      tab.setAttribute('aria-selected', String(active));
    });
    const target = $('leaderboard');
    target.textContent = 'Listening for scores…';
    try {
      const data = await api(`/leaderboard?difficulty=${encodeURIComponent(difficulty)}&limit=20`);
      if (!data.leaderboard.length) {
        const empty = document.createElement('p');
        empty.className = 'empty-state';
        empty.textContent = 'No completed runs yet. The first signal could be yours.';
        target.replaceChildren(empty);
        return;
      }
      const list = document.createElement('ol');
      list.className = 'leader-list';
      data.leaderboard.forEach((entry) => {
        const item = document.createElement('li');
        const rank = document.createElement('span'); rank.className = 'leader-rank'; rank.textContent = String(entry.rank).padStart(2, '0');
        const person = document.createElement('span'); person.className = 'leader-person'; person.textContent = entry.username;
        const score = document.createElement('strong'); score.textContent = `${entry.moves} moves · ${formatTime(entry.elapsed_seconds)}`;
        item.append(rank, person, score);
        list.append(item);
      });
      target.replaceChildren(list);
    } catch (error) {
      target.textContent = error.message;
    }
  }

  function openDialog(backdrop) {
    state.previousFocus = document.activeElement;
    state.openDialog = backdrop;
    backdrop.hidden = false;
    document.body.classList.add('dialog-open');
    const focusable = backdrop.querySelector('button, a, input');
    if (focusable) focusable.focus();
  }

  function closeDialog() {
    if (!state.openDialog) return;
    state.openDialog.hidden = true;
    state.openDialog = null;
    document.body.classList.remove('dialog-open');
    if (state.previousFocus && state.previousFocus.focus) state.previousFocus.focus();
  }

  function trapDialogKey(event) {
    if (!state.openDialog) return;
    if (event.key === 'Escape') return closeDialog();
    if (event.key !== 'Tab') return;
    const items = [...state.openDialog.querySelectorAll('button:not(:disabled), a[href], input:not(:disabled)')];
    if (!items.length) return;
    const first = items[0], last = items[items.length - 1];
    if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
    else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
  }

  $('entryForm').addEventListener('submit', async (event) => {
    event.preventDefault();
    const email = $('email').value.trim();
    const username = $('username').value.trim();
    $('entryError').textContent = '';
    if (!validEmail(email)) {
      $('entryError').textContent = 'Enter a valid email address.';
      $('email').focus();
      return;
    }
    if (!validUsername(username)) {
      $('entryError').textContent = 'Use 3–24 letters, numbers, or underscores for your username.';
      $('username').focus();
      return;
    }
    $('entrySubmit').disabled = true;
    try {
      state.me = await api('/auth/enter', { method: 'POST', body: JSON.stringify({ email, username }) });
      await loadSession();
      const next = state.me && state.me.progress && state.me.progress.hard ? 'hard' : state.me && state.me.progress && state.me.progress.medium ? 'medium' : 'easy';
      state.difficulty = next;
      startGame(next);
    } catch (error) {
      $('entryError').textContent = error.message;
    } finally {
      $('entrySubmit').disabled = false;
    }
  });
  $('signOut').addEventListener('click', async () => { try { await api('/auth/logout', { method: 'POST', body: '{}' }); } finally { showEntry(); $('email').focus(); } });
  $('newGame').addEventListener('click', () => startGame(state.difficulty));
  document.querySelectorAll('[data-difficulty]').forEach((button) => button.addEventListener('click', () => startGame(button.dataset.difficulty)));
  document.querySelectorAll('[data-open-leaderboard]').forEach((button) => button.addEventListener('click', () => { openDialog($('leaderboardDialog')); loadLeaderboard('hard'); }));
  document.querySelectorAll('[data-open-help]').forEach((button) => button.addEventListener('click', () => openDialog($('helpDialog'))));
  window.addEventListener('resize', fitBoard);
  $('board').addEventListener('click', onBoardClick);
  document.addEventListener('visibilitychange', () => {
    if (document.visibilityState === 'visible' && state.game && state.game.status === 'in_progress') {
      state.queue = state.queue.then(() => resync().catch(() => {}));
    }
  });
  document.querySelectorAll('[data-leader-difficulty]').forEach((button) => button.addEventListener('click', () => loadLeaderboard(button.dataset.leaderDifficulty)));
  document.querySelectorAll('[data-close-dialog]').forEach((button) => button.addEventListener('click', closeDialog));
  document.querySelectorAll('.dialog-backdrop').forEach((backdrop) => backdrop.addEventListener('click', (event) => { if (event.target === backdrop) closeDialog(); }));
  $('continueLevel').addEventListener('click', () => {
    const next = $('continueLevel').dataset.next;
    closeDialog();
    if (next) startGame(next);
  });
  document.addEventListener('keydown', trapDialogKey);
  window.addEventListener('online', () => setMessage('Back online.'));
  window.addEventListener('offline', () => setMessage('You are offline. Your current board is safe on the server.', 'is-error'));
  loadSession();
}
