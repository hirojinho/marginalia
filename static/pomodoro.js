const FOCUS_SECS = 25 * 60;
const BREAK_SECS = 5 * 60;

const pad2 = (n) => String(n).padStart(2, '0');

export function initPomodoro() {
  const style = document.createElement('style');
  style.textContent = `
#pomodoro-widget {
  position: fixed;
  bottom: 20px;
  right: 20px;
  background: var(--bg-secondary, #1e1e2e);
  color: var(--text-primary, #cdd6f4);
  border: 1px solid var(--border, #313244);
  border-radius: 10px;
  padding: 10px 14px;
  text-align: center;
  font-family: inherit;
  font-size: 13px;
  z-index: 1000;
  min-width: 90px;
  user-select: none;
}
#pomodoro-time {
  font-size: 22px;
  font-weight: 600;
  letter-spacing: 1px;
  margin: 4px 0;
}
#pomodoro-label {
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.5px;
  opacity: 0.7;
}
#pomodoro-btn {
  margin-top: 6px;
  padding: 3px 12px;
  border: 1px solid var(--border, #313244);
  border-radius: 6px;
  background: transparent;
  color: inherit;
  font-size: 12px;
  cursor: pointer;
  width: 100%;
}
#pomodoro-btn:hover {
  background: var(--bg-hover, #313244);
}`;
  document.head.appendChild(style);

  const widget = document.createElement('div');
  widget.id = 'pomodoro-widget';
  widget.innerHTML = `
    <div id="pomodoro-label">Focus</div>
    <div id="pomodoro-time">25:00</div>
    <button id="pomodoro-btn">Start</button>`;
  document.body.appendChild(widget);

  const labelEl = document.getElementById('pomodoro-label');
  const timeEl = document.getElementById('pomodoro-time');
  const btnEl = document.getElementById('pomodoro-btn');

  let state = 'idle';
  let phase = 'focus';
  let remaining = FOCUS_SECS;
  let timer = null;

  function render() {
    labelEl.textContent = phase === 'focus' ? 'Focus' : 'Break';
    timeEl.textContent = pad2(Math.floor(remaining / 60)) + ':' + pad2(remaining % 60);
    const labels = { idle: 'Start', running: 'Pause', paused: 'Resume', done: 'Next' };
    btnEl.textContent = labels[state];
  }

  function tick() {
    remaining--;
    if (remaining <= 0) {
      clearInterval(timer);
      timer = null;
      phase = phase === 'focus' ? 'break' : 'focus';
      remaining = phase === 'focus' ? FOCUS_SECS : BREAK_SECS;
      state = 'done';
    }
    render();
  }

  btnEl.addEventListener('click', function () {
    if (state === 'idle') {
      state = 'running';
      timer = setInterval(tick, 1000);
    } else if (state === 'running') {
      clearInterval(timer);
      timer = null;
      state = 'paused';
    } else if (state === 'paused') {
      state = 'running';
      timer = setInterval(tick, 1000);
    } else if (state === 'done') {
      state = 'running';
      timer = setInterval(tick, 1000);
    }
    render();
  });

  render();
}
