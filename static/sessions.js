// Session list, create/load/switch/delete, session pill, modal.
import { apiFetch } from './apiFetch.js';
import { showErrorBanner } from './errorBanner.js';
import { escapeHtml, renderContent } from './dom.js';

const MAX_TOPIC_LEN = 200;

export const courseMeta = {
  ce297: { name: 'CE-297 Safety', color: '#B45309' },
  ddia: { name: 'DDIA', color: '#2563EB' },
  'dsa-interview': { name: 'DSA Interview', color: '#059669' },
  'software-arch': { name: 'Software Arch', color: '#7C3AED' },
  thesis: { name: 'Thesis', color: '#DC2626' },
  '': { name: 'General', color: '#78716C' },
};

let activeSessionId = null;
let allSessions = [];

const EXPANDED_KEY = 'claw-study:expandedCourse';

function getExpandedCourse() {
  return localStorage.getItem(EXPANDED_KEY); // string courseId, '' for General, or null
}
function setExpandedCourse(courseId) {
  if (courseId === null) localStorage.removeItem(EXPANDED_KEY);
  else localStorage.setItem(EXPANDED_KEY, courseId);
}

export function getActiveSessionId() {
  return activeSessionId;
}
export function setActiveSessionId(id) {
  activeSessionId = id;
}

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
  } catch {
    activeSessionId = null;
    updateSessionPill(null);
  }
}

function renderSessionList() {
  const container = document.getElementById('session-list');

  // Group sessions by course, newest-first within each course.
  const byCourse = {};
  for (const s of allSessions) {
    const key = s.course_id || '';
    (byCourse[key] = byCourse[key] || []).push(s);
  }
  for (const key of Object.keys(byCourse)) {
    byCourse[key].sort((a, b) => new Date(b.updated_at) - new Date(a.updated_at));
  }

  // Which course is expanded: stored choice, else the active session's course,
  // else the first course with sessions.
  let expanded = getExpandedCourse();
  if (expanded === null) {
    const active = allSessions.find((s) => s.id === activeSessionId);
    if (active) expanded = active.course_id || '';
  }

  // Always show every defined course (so '+' can launch into any).
  const order = Object.keys(courseMeta);

  let html = '';
  for (const key of order) {
    const meta = courseMeta[key];
    const sessions = byCourse[key] || [];
    const isExpanded = expanded === key;
    html += `<div class="course-group${isExpanded ? ' expanded' : ''}" data-course="${key}">
      <div class="course-header" data-action="toggle-course" data-course="${key}">
        <span class="course-chevron">&#x25B6;</span>
        <span class="course-dot" style="background:${meta.color}"></span>
        <span class="course-name">${escapeHtml(meta.name)}</span>
        <span class="course-count">${sessions.length || ''}</span>
        <span class="course-add" data-action="create-in-course" data-course="${key}" title="New session">&#x2b;</span>
      </div>
      <div class="course-sessions">`;
    if (sessions.length === 0) {
      html += `<div class="course-empty-hint">No sessions yet — + to start</div>`;
    } else {
      for (const s of sessions) {
        const isActive = s.id === activeSessionId;
        html += `<div class="session-item${isActive ? ' active' : ''}" data-session-id="${s.id}" data-action="switch-session">
          <div style="display:flex;justify-content:space-between;align-items:center;">
            <div class="session-topic-wrap">
              <span class="session-topic" data-action="start-rename" data-session-id="${s.id}">${escapeHtml(s.topic || 'Untitled')}</span>
              <span class="session-rename-btn" data-action="start-rename" data-session-id="${s.id}" title="Rename">&#x270E;</span>
            </div>
            <span class="session-delete" data-action="delete-session" data-session-id="${s.id}" title="Delete">&#x2715;</span>
          </div>
        </div>`;
      }
    }
    html += `</div></div>`;
  }
  container.innerHTML = html;
}

export async function switchSession(id) {
  try {
    await fetch('/api/sessions/active', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ id: id }),
    });
    activeSessionId = id;
    highlightActiveSession();
    updateSessionPill(allSessions.find((s) => s.id === id) || { id: id, topic: 'Session' });
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
        div.innerHTML =
          '<div class="msg-label">You</div><div class="msg-content">' +
          renderContent(m.content) +
          '</div>';
        container.appendChild(div);
      } else if (m.role === 'assistant') {
        const div = document.createElement('div');
        div.className = 'msg msg-assistant';
        let inner = '<div class="msg-label">Claw</div><div class="msg-content">';
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

function highlightActiveSession() {
  document.querySelectorAll('.session-item').forEach((el) => {
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
      body: JSON.stringify({ course_id: courseId, topic: topic }),
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

export async function createSessionInCourse(courseId) {
  try {
    const resp = await fetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ course_id: courseId, topic: '' }),
    });
    if (!resp.ok) throw new Error('HTTP ' + resp.status);
    const session = await resp.json();
    setExpandedCourse(courseId); // keep the course we just created in open
    activeSessionId = session.id;
    await loadActiveSession();
    await loadSessions();
    document.getElementById('messages').innerHTML = '';
  } catch (err) {
    console.error('Failed to create session', err);
    showErrorBanner('Failed to create session: ' + err.message);
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

function startRenameSession(id) {
  const session = allSessions.find((s) => s.id === id);
  if (!session) return;
  const wrap = document.querySelector(
    `.session-topic-wrap [data-session-id="${id}"].session-topic`,
  );
  if (!wrap) return;
  const parent = wrap.closest('.session-topic-wrap');
  const input = document.createElement('input');
  input.type = 'text';
  input.className = 'session-topic-input';
  input.value = session.topic || '';
  input.maxLength = 200;
  parent.replaceWith(input);
  input.focus();
  input.select();

  async function commit() {
    const newTopic = input.value.trim();
    if (newTopic && newTopic !== session.topic) {
      try {
        await fetch('/api/sessions?id=' + id, {
          method: 'PATCH',
          headers: { 'Content-Type': 'application/json' },
          body: JSON.stringify({ topic: newTopic }),
        });
        session.topic = newTopic;
        if (id === activeSessionId) updateSessionPill(session);
      } catch (err) {
        console.error('Failed to rename session', err);
      }
    }
    renderSessionList();
  }

  input.addEventListener('keydown', function (e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      commit();
    }
    if (e.key === 'Escape') {
      renderSessionList();
    }
  });
  input.addEventListener('blur', commit);
}

export function initSessionsUI() {
  document.getElementById('session-list').addEventListener('click', function (e) {
    const renameEl = e.target.closest('[data-action="start-rename"]');
    if (renameEl) {
      e.stopPropagation();
      startRenameSession(parseInt(renameEl.dataset.sessionId, 10));
      return;
    }
    const addEl = e.target.closest('[data-action="create-in-course"]');
    if (addEl) {
      e.stopPropagation();
      createSessionInCourse(addEl.dataset.course);
      return;
    }
    // switch-session and delete-session are handled by the document-level
    // dispatcher in app.js — do NOT re-bind them here (would double-fire).
    const headerEl = e.target.closest('[data-action="toggle-course"]');
    if (headerEl) {
      const course = headerEl.dataset.course;
      setExpandedCourse(getExpandedCourse() === course ? null : course);
      renderSessionList();
      return;
    }
  });
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
    if (e.key === 'Enter') {
      e.preventDefault();
      createSession();
    }
  });
}
