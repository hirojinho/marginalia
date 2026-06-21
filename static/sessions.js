// Session state + data ops: course metadata, active-session tracking, message
// loading, the session pill, switch/delete. The left-rail navigator lives in
// rail.js (ADR 0011); this module no longer renders a session list.
import { apiFetch } from './apiFetch.js';
import { escapeHtml, renderContent } from './dom.js';

export const courseMeta = {
  biology: { name: 'Biology', color: '#B45309' },
  cs101: { name: 'CS 101', color: '#2563EB' },
  algorithms: { name: 'Algorithms', color: '#059669' },
  history: { name: 'History', color: '#7C3AED' },
  research: { name: 'Research', color: '#DC2626' },
  '': { name: 'General', color: '#78716C' },
};

// Colors for courses that exist in the DB but aren't in the hardcoded map above
// (e.g. created via the agent / POST /api/courses). Cycled by discovery order.
const FALLBACK_COURSE_COLORS = ['#0D9488', '#DB2777', '#CA8A04', '#4F46E5', '#0891B2'];

let activeSessionId = null;

// Merge courses from the backend into courseMeta so agent-created courses
// (which aren't in the hardcoded map) show up in the rail switcher.
export async function loadCourses() {
  try {
    const resp = await apiFetch('/api/courses');
    const courses = await resp.json();
    let extra = 0;
    for (const c of courses || []) {
      if (!c || !c.id || c.id.startsWith('postship-smoke')) continue;
      if (!courseMeta[c.id]) {
        courseMeta[c.id] = {
          name: c.name || c.id,
          color: FALLBACK_COURSE_COLORS[extra % FALLBACK_COURSE_COLORS.length],
        };
        extra++;
      }
    }
  } catch (err) {
    console.error('Failed to load courses', err);
  }
}

export function getActiveSessionId() {
  return activeSessionId;
}
export function setActiveSessionId(id) {
  activeSessionId = id;
}

// loadActiveSession restores the active session id + pill and RETURNS the
// active session object (or null) so callers can act on it (e.g. restore
// reading on page load).
export async function loadActiveSession() {
  try {
    const resp = await apiFetch('/api/sessions/active');
    const data = await resp.json();
    if (data && data.id) {
      activeSessionId = data.id;
      updateSessionPill(data);
      return data;
    }
    activeSessionId = null;
    updateSessionPill(null);
    return null;
  } catch {
    activeSessionId = null;
    updateSessionPill(null);
    return null;
  }
}

export async function switchSession(id) {
  try {
    await fetch('/api/sessions/active', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: id }),
    });
    activeSessionId = id;
    await loadSessionMessages();
    await loadActiveSession();
  } catch (err) {
    console.error('Failed to switch session', err);
  }
}

export async function loadSessionMessages() {
  if (!activeSessionId) {
    document.getElementById('messages').innerHTML = '';
    return;
  }
  try {
    const resp = await apiFetch('/api/sessions/messages?session_id=' + activeSessionId);
    const msgs = await resp.json();
    const container = document.getElementById('messages');
    container.innerHTML = '';
    for (const m of msgs) {
      if (m.role === 'user') {
        const div = document.createElement('div');
        div.className = 'msg msg-user';
        div.innerHTML =
          '<div class="msg-label">You</div><div class="msg-content">' +
          escapeHtml(m.content) +
          '</div>';
        container.appendChild(div);
      } else if (m.role === 'assistant') {
        const div = document.createElement('div');
        div.className = 'msg msg-assistant';
        let inner = '<div class="msg-label">marginalia</div><div class="msg-content">';
        if (m.reasoning) {
          inner +=
            '<details class="thinking-inline"><summary>Thinking</summary>' +
            '<div class="thinking-content">' +
            renderContent(m.reasoning) +
            '</div></details>';
        }
        inner += renderContent(m.content) + '</div>';
        div.innerHTML = inner;
        container.appendChild(div);
      }
    }
    const msgsEl = document.getElementById('messages');
    msgsEl.scrollTop = msgsEl.scrollHeight;
  } catch (err) {
    console.error('Failed to load messages', err);
  }
}

// clearWorkspace empties the chat pane (used when opening an as-yet-empty task).
export function clearWorkspace() {
  document.getElementById('messages').innerHTML = '';
}

function updateSessionPill(session) {
  const pill = document.getElementById('session-pill');
  if (!session || !session.id) {
    pill.style.display = 'none';
    return;
  }
  const meta = courseMeta[session.course_id || ''] || courseMeta[''];
  pill.style.display = 'inline-flex';
  pill.innerHTML = `<span class="pill-dot" style="background:${meta.color}"></span>${escapeHtml(session.topic || meta.name)}`;
}

export async function deleteSession(id) {
  if (!confirm('Delete this session and its chat history?')) return;
  try {
    await fetch('/api/sessions?id=' + id, { method: 'DELETE' });
    if (activeSessionId === id) {
      activeSessionId = null;
      document.getElementById('messages').innerHTML = '';
      updateSessionPill(null);
    }
    await loadActiveSession();
  } catch (err) {
    console.error('Failed to delete session', err);
  }
}
