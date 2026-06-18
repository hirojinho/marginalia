const FOCUS_SECS = 25 * 60;
const BREAK_SECS = 5 * 60;

const pad2 = (n) => String(n).padStart(2, '0');
const fmt = (s) => pad2(Math.floor(s / 60)) + ':' + pad2(s % 60);

export function initPomodoro() {
  const btn = document.getElementById('pomodoro-btn');

  let state = 'idle'; // idle | running | paused | done
  let phase = 'focus'; // focus | break
  let remaining = FOCUS_SECS;
  let timer = null;

  function render() {
    btn.textContent = fmt(remaining);
    btn.classList.toggle('active', state === 'running');
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

  btn.addEventListener('click', function () {
    if (state === 'idle' || state === 'done') {
      state = 'running';
      timer = setInterval(tick, 1000);
    } else if (state === 'running') {
      clearInterval(timer);
      timer = null;
      state = 'paused';
    } else if (state === 'paused') {
      state = 'running';
      timer = setInterval(tick, 1000);
    }
    render();
  });

  btn.addEventListener('dblclick', function (e) {
    e.stopPropagation();
    clearInterval(timer);
    timer = null;
    state = 'idle';
    phase = 'focus';
    remaining = FOCUS_SECS;
    render();
  });

  render();
}
