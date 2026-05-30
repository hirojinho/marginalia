// settings.js — Course Steering settings modal (ADR 0010/0016).
// Reads GET /api/courses/settings?course_id=, writes PUT on save.
import { apiFetch } from './apiFetch.js';
import { escapeHtml } from './dom.js';

export async function openCourseSettings(courseId) {
  if (document.querySelector('.settings-overlay')) return;
  let s;
  try {
    const resp = await apiFetch(`/api/courses/settings?course_id=${encodeURIComponent(courseId)}`);
    if (!resp.ok) throw new Error('HTTP ' + resp.status);
    s = await resp.json();
  } catch (err) {
    alert('Could not load course settings: ' + err.message);
    return;
  }
  renderModal(courseId, s);
}

function renderModal(courseId, s) {
  const overlay = document.createElement('div');
  overlay.className = 'settings-overlay';
  overlay.innerHTML = `
    <div class="settings-modal" role="dialog" aria-modal="true">
      <h2>Course settings — ${escapeHtml(courseId)}</h2>
      <label class="settings-field">Framing / goal
        <textarea id="set-framing" rows="3" placeholder="e.g. exam-prep first, conceptual exam">${escapeHtml(s.framing)}</textarea>
      </label>
      <label class="settings-field">Exam style
        <textarea id="set-exam" rows="2" placeholder="e.g. conceptual oral, problem sets">${escapeHtml(s.exam_style)}</textarea>
      </label>
      <label class="settings-field">Reading chunk size (pages)
        <input id="set-chunk" type="number" min="3" max="30" value="${Number(s.chunk_pages)}">
      </label>
      <label class="settings-check"><input id="set-stop" type="checkbox" ${s.stop_after_task ? 'checked' : ''}> Stop after each task</label>
      <label class="settings-check"><input id="set-inter" type="checkbox" ${s.interleaving ? 'checked' : ''}> Interleave older tasks at session open</label>
      <div id="set-error" class="settings-error"></div>
      <div class="settings-actions">
        <button id="set-cancel">Cancel</button>
        <button id="set-save" class="primary">Save</button>
      </div>
    </div>`;
  document.body.appendChild(overlay);

  const onKey = (e) => {
    if (e.key === 'Escape') close();
  };
  const close = () => {
    overlay.remove();
    document.removeEventListener('keydown', onKey);
  };
  document.addEventListener('keydown', onKey);
  overlay.querySelector('#set-cancel').addEventListener('click', close);
  overlay.addEventListener('click', (e) => {
    if (e.target === overlay) close();
  });

  overlay.querySelector('#set-save').addEventListener('click', async () => {
    const payload = {
      course_id: courseId,
      framing: overlay.querySelector('#set-framing').value,
      exam_style: overlay.querySelector('#set-exam').value,
      chunk_pages: parseInt(overlay.querySelector('#set-chunk').value, 10),
      stop_after_task: overlay.querySelector('#set-stop').checked,
      interleaving: overlay.querySelector('#set-inter').checked,
    };
    try {
      const r = await apiFetch('/api/courses/settings', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(payload),
      });
      if (!r.ok) {
        const e = await r.json().catch(() => ({}));
        overlay.querySelector('#set-error').textContent =
          e.error || 'Save failed (HTTP ' + r.status + ')';
        return;
      }
      close();
    } catch (err) {
      overlay.querySelector('#set-error').textContent = 'Save failed: ' + err.message;
    }
  });
}
