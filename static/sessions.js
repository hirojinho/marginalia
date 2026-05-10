// Session list, create/load/switch/delete, session pill, modal.
import { apiFetch } from './apiFetch.js';
import { showErrorBanner } from './errorBanner.js';
import { escapeHtml, renderContent } from './dom.js';

const MAX_TOPIC_LEN = 200;

export const courseMeta = {
  'ce297': { name: 'CE-297 Safety', color: '#B45309' },
  'ddia': { name: 'DDIA', color: '#2563EB' },
  'dsa-interview': { name: 'DSA Interview', color: '#059669' },
  'software-arch': { name: 'Software Arch', color: '#7C3AED' },
  'thesis': { name: 'Thesis', color: '#DC2626' },
  '': { name: 'General', color: '#78716C' },
};

let activeSessionId = null;
let allSessions = [];

export function getActiveSessionId() { return activeSessionId; }
export function setActiveSessionId(id) { activeSessionId = id; }

export async function loadSessions() {
  try {
    const resp = await apiFetch('/api/sessions');
    allSessions = await resp.json();
    renderSessionList();
  } catch (err) {
    console.error('Failed to load sessions', err);
  }
}

export async function loadActiveSession() {
  try {
    const resp = await apiFetch('/api/sessions/active');
    const data = await resp.json();
    if (data && data.id) {
      activeSessionId = data.id;
      updateSessionPill(data);
    } else {
      activeSessionId = null;
      updateSessionPill(null);
    }
    highlightActiveSession();
  } catch (err) {
    activeSessionId = null;
    updateSessionPill(null);
  }
}

function renderSessionList() {
  const container = document.getElementById('session-list');
  if (!allSessions || allSessions.length === 0) {
    container.innerHTML = '<div style="text-align:center;color:var(--text-tertiary);font-size:13px;padding:24px 12px;">No sessions yet.<br>Click <strong>+ New</strong> to start.</div>';
    return;
  }

  const groups = {};
  const groupOrder = [];
  for (const s of allSessions) {
    const key = s.course_id || '';
    if (!groups[key]) {
      groups[key] = [];
      groupOrder.push(key);
    }
    groups[key].push(s);
  }

  let html = '';
  for (const key of groupOrder) {
    const meta = courseMeta[key] || { name: 'General', color: '#78716C' };
    html += `<div class="session-group-title">${escapeHtml(meta.name)}</div>`;
    for (const s of groups[key]) {
      const isActive = s.id === activeSessionId;
      const timeAgo = formatTimeAgo(s.updated_at);
      html += `<div class="session-item${isActive ? ' active' : ''}" data-session-id="${s.id}" data-action="switch-session">
        <div style="display:flex;justify-content:space-between;align-items:start;">
          <div class="session-topic">${escapeHtml(s.topic || 'General')}</div>
          <span class="session-delete" data-action="delete-session" data-session-id="${s.id}" title="Delete">&#x2715;</span>
        </div>
        <div class="session-meta">${timeAgo}${s.pdf_name ? ' &middot; ' + escapeHtml(s.pdf_name.replace(/\.pdf$/i,'')) : ''}</div>
      </div>`;
    }
  }
  container.innerHTML = html;
}

function formatTimeAgo(isoStr) {
  if (!isoStr) return '';
  const diff = Date.now() - new Date(isoStr).getTime();
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return mins + 'm ago';
  const hrs = Math.floor(mins / 60);
  if (hrs < 24) return hrs + 'h ago';
  const days = Math.floor(hrs / 24);
  return days + 'd ago';
}

export async function switchSession(id) {
  try {
    await fetch('/api/sessions/active', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: id })
    });
    activeSessionId = id;
    highlightActiveSession();
    updateSessionPill(allSessions.find(s => s.id === id) || { id: id, topic: 'Session' });
    await loadSessionMessages();
    await loadSessions();
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
        div.innerHTML = '<div class="msg-label">You</div><div class="msg-content">' + renderContent(m.content) + '</div>';
        container.appendChild(div);
      } else if (m.role === 'assistant') {
        const div = document.createElement('div');
        div.className = 'msg msg-assistant';
        div.innerHTML = '<div class="msg-label">Claw</div><div class="msg-content">' + renderContent(m.content) + '</div>';
        container.appendChild(div);
      }
    }
    const msgsEl = document.getElementById('messages');
    msgsEl.scrollTop = msgsEl.scrollHeight;
  } catch (err) {
    console.error('Failed to load messages', err);
  }
}

function highlightActiveSession() {
  document.querySelectorAll('.session-item').forEach(el => {
    el.classList.toggle('active', parseInt(el.dataset.sessionId) === activeSessionId);
  });
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

export function openSessionModal() {
  document.getElementById('session-modal-overlay').classList.add('open');
  document.getElementById('session-topic').value = '';
  document.getElementById('session-course').value = '';
  document.getElementById('session-topic').focus();
}

export function closeSessionModal() {
  document.getElementById('session-modal-overlay').classList.remove('open');
}

export async function createSession() {
  const courseId = document.getElementById('session-course').value;
  const topic = document.getElementById('session-topic').value.trim() || 'General';
  if (topic.length > MAX_TOPIC_LEN) {
    showErrorBanner('Topic is too long (max ' + MAX_TOPIC_LEN + ' characters).');
    return;
  }
  const createBtn = document.getElementById('session-modal-create');
  createBtn.disabled = true;
  createBtn.textContent = 'Creating…';
  try {
    const resp = await fetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ course_id: courseId, topic: topic })
    });
    if (!resp.ok) throw new Error('HTTP ' + resp.status);
    const session = await resp.json();
    activeSessionId = session.id;
    await loadActiveSession();
    await loadSessions();
    document.getElementById('messages').innerHTML = '';
    closeSessionModal();
  } catch (err) {
    console.error('Failed to create session', err);
    showErrorBanner('Failed to create session: ' + err.message);
  } finally {
    createBtn.disabled = false;
    createBtn.textContent = 'Create';
  }
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
    await loadSessions();
    await loadActiveSession();
  } catch (err) {
    console.error('Failed to delete session', err);
  }
}

export function initSessionsUI() {
  document.getElementById('session-modal-cancel').addEventListener('click', closeSessionModal);
  document.getElementById('session-modal-create').addEventListener('click', createSession);
  document.getElementById('new-session-btn').addEventListener('click', openSessionModal);
  document.getElementById('session-course').addEventListener('change', function () {
    const topicInput = document.getElementById('session-topic');
    if (!topicInput.value) {
      const meta = courseMeta[this.value] || courseMeta[''];
      topicInput.placeholder = meta.name + ' topic...';
    }
  });
  document.getElementById('session-modal-overlay').addEventListener('click', function (e) {
    if (e.target === this) closeSessionModal();
  });
  document.getElementById('session-topic').addEventListener('keydown', function (e) {
    if (e.key === 'Enter') { e.preventDefault(); createSession(); }
  });
}
