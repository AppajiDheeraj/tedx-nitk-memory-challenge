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
  const state = { me: null, game: null, difficulty: 'easy', busy: false, seconds: 300, timer: null, syncTicks: 0, openDialog: null, previousFocus: null };

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

  function renderGame() {
    const current = state.game;
    if (!current) {
      $('board').replaceChildren();
      return;
    }
    $('moves').textContent = String(current.moves);
    $('pairs').textContent = `${current.pairs_found} / ${current.total_pairs}`;
    $('time').textContent = formatTime(state.seconds);
    $('timerCard').classList.toggle('is-urgent', state.seconds <= 30 && current.status === 'in_progress');
    const board = $('board');
    board.style.gridTemplateColumns = `repeat(${current.columns}, minmax(0, 1fr))`;
    fitBoard();
    board.replaceChildren(...current.cards.map((card) => {
      const visible = card.revealed || card.matched;
      const button = document.createElement('button');
      button.type = 'button';
      button.className = `memory-card ${card.matched ? 'is-matched' : visible ? 'is-visible' : ''}`;
      button.disabled = state.busy || current.status !== 'in_progress' || visible;
      button.setAttribute('aria-label', card.matched ? `Matched ${symbols[card.value] || card.value}` : visible ? `Revealed ${symbols[card.value] || card.value}` : `Hidden card ${card.position + 1}`);
      const face = document.createElement('span');
      face.textContent = visible ? (symbols[card.value] || '●') : '·';
      button.append(face);
      button.addEventListener('click', () => flipCard(card.position));
      return button;
    }));
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
      state.game = await api('/games', { method: 'POST', body: JSON.stringify({ difficulty }) });
      state.seconds = state.game.time_left_seconds;
      setMessage('Choose two cards. Remember what you see.');
      startTimer();
    } catch (error) {
      if (error.status === 401) await loadSession();
      else setMessage(error.message, 'is-error');
    } finally {
      state.busy = false;
      $('newGame').disabled = false;
      updateDifficultyButtons();
      renderGame();
    }
  }

  async function refreshGame() {
    if (!state.game) return;
    state.game = await api(`/games/${state.game.id}`);
    state.seconds = state.game.time_left_seconds;
    renderGame();
    if (state.game.status === 'completed') await completeLevel();
    if (state.game.status === 'expired') {
      stopTimer();
      setMessage('Time slipped away. Start a new game and listen closer.', 'is-error');
    }
  }

  async function flipCard(position) {
    if (!state.game || state.busy || state.game.status !== 'in_progress') return;
    state.busy = true;
    renderGame();
    try {
      const result = await api(`/games/${state.game.id}/flip`, { method: 'POST', body: JSON.stringify({ position }) });
      await refreshGame();
      if (result.result === 'mismatch' && state.game.status === 'in_progress') {
        setMessage('Hold that pattern…');
        await new Promise((resolve) => setTimeout(resolve, 720));
        await api(`/games/${state.game.id}/resolve`, { method: 'POST', body: '{}' });
        await refreshGame();
      } else if (result.result === 'match') {
        setMessage(state.game.status === 'completed' ? 'Signal complete.' : 'A clear match.', 'is-success');
      }
    } catch (error) {
      if (error.status === 401) await loadSession();
      else {
        setMessage(error.message, 'is-error');
        try { await refreshGame(); } catch (_) { /* next action retries */ }
      }
    } finally {
      state.busy = false;
      renderGame();
    }
  }

  function startTimer() {
    stopTimer();
    state.syncTicks = 0;
    state.timer = setInterval(async () => {
      if (!state.game || state.game.status !== 'in_progress') return stopTimer();
      state.seconds = Math.max(0, state.seconds - 1);
      state.syncTicks++;
      $('time').textContent = formatTime(state.seconds);
      $('timerCard').classList.toggle('is-urgent', state.seconds <= 30);
      if (state.seconds === 0 || state.syncTicks % 15 === 0) {
        try { await refreshGame(); } catch (_) { /* local countdown continues */ }
      }
    }, 1000);
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
    const finalReveal = difficulty === 'hard';
    $('standardCompletion').hidden = finalReveal;
    $('themeReveal').hidden = !finalReveal;
    $('completionCard').classList.toggle('is-theme', finalReveal);
    if (!finalReveal) {
      const next = difficulty === 'easy' ? 'medium' : 'hard';
      $('completionCopy').textContent = `${next[0].toUpperCase() + next.slice(1)} is now unlocked. The signal gets quieter from here.`;
      $('continueLevel').dataset.next = next;
      $('continueLevel').textContent = `Continue to ${next}`;
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
  document.querySelectorAll('[data-leader-difficulty]').forEach((button) => button.addEventListener('click', () => loadLeaderboard(button.dataset.leaderDifficulty)));
  document.querySelectorAll('[data-close-dialog]').forEach((button) => button.addEventListener('click', closeDialog));
  document.querySelectorAll('.dialog-backdrop').forEach((backdrop) => backdrop.addEventListener('click', (event) => { if (event.target === backdrop) closeDialog(); }));
  $('continueLevel').addEventListener('click', () => { const next = $('continueLevel').dataset.next; closeDialog(); startGame(next); });
  document.addEventListener('keydown', trapDialogKey);
  window.addEventListener('online', () => setMessage('Back online.'));
  window.addEventListener('offline', () => setMessage('You are offline. Your current board is safe on the server.', 'is-error'));
  loadSession();
}
