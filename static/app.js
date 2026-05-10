// === LIMITS (mirror server-side caps) ===
const MAX_MESSAGE_LEN = 4000;
const MAX_TOPIC_LEN = 200;
const MAX_PDF_BYTES = 50 * 1024 * 1024;

// === GLOBAL ERROR HANDLER ===
function showErrorBanner(msg) {
  let banner = document.getElementById('error-banner');
  if (!banner) {
    banner = document.createElement('div');
    banner.id = 'error-banner';
    banner.innerHTML = '<span class="error-banner-msg"></span>' +
      '<button class="error-banner-reload" type="button">Reload</button>' +
      '<button class="error-banner-close" type="button" aria-label="Dismiss">&times;</button>';
    document.body.appendChild(banner);
    banner.querySelector('.error-banner-reload').addEventListener('click', function () { location.reload(); });
    banner.querySelector('.error-banner-close').addEventListener('click', function () { banner.classList.remove('visible'); });
  }
  banner.querySelector('.error-banner-msg').textContent = msg;
  banner.classList.add('visible');
}
window.addEventListener('error', function (e) {
  console.error('window.error', e.error || e.message);
  showErrorBanner('Something broke: ' + (e.message || 'unknown error'));
});
window.addEventListener('unhandledrejection', function (e) {
  console.error('unhandledrejection', e.reason);
  var reason = e.reason;
  var msg = reason && reason.message ? reason.message : String(reason);
  showErrorBanner('Network or runtime error: ' + msg);
});

// === apiFetch — retry + exponential backoff for idempotent GETs ===
// Non-GET methods are passed through with a single attempt to avoid
// duplicating writes. Pass { noRetry: true } to opt a GET out of retry.
async function apiFetch(url, opts) {
  opts = opts || {};
  const method = (opts.method || 'GET').toUpperCase();
  const retriable = !opts.noRetry && method === 'GET';
  const maxAttempts = retriable ? 3 : 1;
  let lastErr;
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const resp = await fetch(url, opts);
      if (resp.status >= 500 && attempt < maxAttempts) {
        lastErr = new Error('HTTP ' + resp.status);
      } else {
        return resp;
      }
    } catch (err) {
      if (err.name === 'AbortError') throw err;
      lastErr = err;
      if (attempt >= maxAttempts) throw err;
    }
    const delay = 200 * Math.pow(2, attempt - 1) + Math.random() * 100;
    await new Promise(function (r) { setTimeout(r, delay); });
  }
  throw lastErr;
}

let currentAssistantMsg = null;
let activeSessionId = null;
let allSessions = [];

const courseMeta = {
  'ce297': { name: 'CE-297 Safety', color: '#B45309' },
  'ddia': { name: 'DDIA', color: '#2563EB' },
  'dsa-interview': { name: 'DSA Interview', color: '#059669' },
  'software-arch': { name: 'Software Arch', color: '#7C3AED' },
  'thesis': { name: 'Thesis', color: '#DC2626' },
  '': { name: 'General', color: '#78716C' },
};

// === SESSIONS ===
async function loadSessions() {
  try {
    const resp = await apiFetch('/api/sessions');
    allSessions = await resp.json();
    renderSessionList();
  } catch (err) {
    console.error('Failed to load sessions', err);
  }
}

async function loadActiveSession() {
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

async function switchSession(id) {
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

async function loadSessionMessages() {
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
    scrollToBottom();
  } catch (err) {
    console.error('Failed to load messages', err);
  }
}

function renderContent(content) {
  try { return marked.parse(content || ''); } catch(e) { return escapeHtml(content || ''); }
}

function renderMarkdown(text) {
  try { return marked.parse(text || ''); } catch(e) { return escapeHtml(text || ''); }
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

function openSessionModal() {
  document.getElementById('session-modal-overlay').classList.add('open');
  document.getElementById('session-topic').value = '';
  document.getElementById('session-course').value = '';
  document.getElementById('session-topic').focus();
}

function closeSessionModal() {
  document.getElementById('session-modal-overlay').classList.remove('open');
}

async function createSession() {
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

async function deleteSession(id) {
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

// === EVENT DELEGATION ===
// Single document-level click listener dispatches actions defined via
// data-action attributes. Replaces inline onclick handlers throughout
// templates so they're inspectable, debuggable, and CSP-friendly.
document.addEventListener('click', function (e) {
  const el = e.target.closest('[data-action]');
  if (!el) return;
  const action = el.dataset.action;
  switch (action) {
    case 'switch-session': switchSession(parseInt(el.dataset.sessionId, 10)); break;
    case 'delete-session': e.stopPropagation(); deleteSession(parseInt(el.dataset.sessionId, 10)); break;
    case 'open-full-plan': openFullPlan(el.dataset.courseId); break;
    case 'open-pdf-from-drawer': openPdfFromDrawer(parseInt(el.dataset.pdfId, 10)); break;
    case 'toggle-topic': toggleTopic(el.dataset.courseId, parseInt(el.dataset.idx, 10)); break;
    case 'open-pdf': openPdf(parseInt(el.dataset.pdfId, 10)); break;
    case 'trigger-upload': triggerUpload(); break;
    case 'switch-pdf': switchPdf(parseInt(el.dataset.pdfId, 10)); break;
  }
});

// Static buttons that previously used inline onclick.
document.getElementById('session-pill').addEventListener('click', openDrawer);
document.getElementById('pdf-upload-zone').addEventListener('click', function () {
  document.getElementById('pdf-file-input').click();
});
document.getElementById('session-modal-cancel').addEventListener('click', closeSessionModal);
document.getElementById('session-modal-create').addEventListener('click', createSession);

document.getElementById('new-session-btn').addEventListener('click', openSessionModal);
document.getElementById('session-course').addEventListener('change', function() {
  const topicInput = document.getElementById('session-topic');
  if (!topicInput.value) {
    const meta = courseMeta[this.value] || courseMeta[''];
    topicInput.placeholder = meta.name + ' topic...';
  }
});

document.getElementById('sidebar-toggle').addEventListener('click', function() {
  document.getElementById('session-sidebar').classList.toggle('collapsed');
});

// Close modal on overlay click
document.getElementById('session-modal-overlay').addEventListener('click', function(e) {
  if (e.target === this) closeSessionModal();
});

// Enter key in modal creates session
document.getElementById('session-topic').addEventListener('keydown', function(e) {
  if (e.key === 'Enter') { e.preventDefault(); createSession(); }
});

// Load on startup
(async function initSessions() {
  await loadSessions();
  await loadActiveSession();
  if (activeSessionId) {
    await loadSessionMessages();
  }
})();

document.getElementById('chat-form').addEventListener('submit', async function(e) {
  e.preventDefault();
  if (!activeSessionId) {
    openSessionModal();
    return;
  }
  const input = document.getElementById('message-input');
  const msg = input.value.trim();
  if (!msg) return;
  if (msg.length > MAX_MESSAGE_LEN) {
    showErrorBanner('Message is too long (' + msg.length + '/' + MAX_MESSAGE_LEN + ' characters).');
    return;
  }
  input.value = '';
  document.getElementById('send-btn').disabled = true;

  const formData = new FormData();
  formData.append('message', msg);
  formData.append('session_id', activeSessionId.toString());

  const userDiv = document.createElement('div');
  userDiv.className = 'msg msg-user';
  userDiv.innerHTML = '<div class="msg-label">You</div><div class="msg-content">' + escapeHtml(msg) + '</div>';
  document.getElementById('messages').appendChild(userDiv);

  const assistantDiv = document.createElement('div');
  assistantDiv.className = 'msg msg-assistant';
  assistantDiv.innerHTML = '<div class="msg-label">Claw</div><div class="msg-content"><div class="thinking-block" style="display:none;"><details><summary>Thinking</summary><div class="thinking-content"></div></details></div><div class="answer-content"></div></div>';
  document.getElementById('messages').appendChild(assistantDiv);
  currentAssistantMsg = assistantDiv.querySelector('.msg-content');
  const thinkingBlock = currentAssistantMsg.querySelector('.thinking-block');
  const thinkingContent = currentAssistantMsg.querySelector('.thinking-content');
  const answerContent = currentAssistantMsg.querySelector('.answer-content');
  currentAssistantMsg.classList.add('token-cursor');
  let thinkingActive = false;

  try {
    const resp = await fetch('/chat', { method: 'POST', body: formData });
    if (!resp.ok) throw new Error('HTTP ' + resp.status);

    const reader = resp.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let eventType = '';
    let rawAnswer = '';
    let rawThinking = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const parts = buffer.split('\n');
      buffer = parts.pop() || '';

      for (const line of parts) {
        if (line.startsWith('event: ')) {
          eventType = line.slice(7).trim();
          continue;
        }
        if (line.startsWith('data: ')) {
          const data = line.slice(6);
          const token = JSON.parse(data);
          if (eventType === 'reasoning') {
            if (!thinkingActive) {
              thinkingActive = true;
              thinkingBlock.style.display = 'block';
            }
            rawThinking += token;
            thinkingContent.innerHTML = renderMarkdown(rawThinking);
            scrollToBottom();
          } else if (eventType === 'token' && answerContent) {
            currentAssistantMsg.classList.remove('token-cursor');
            rawAnswer += token;
            answerContent.innerHTML = renderMarkdown(rawAnswer);
            currentAssistantMsg.classList.add('token-cursor');
            scrollToBottom();
          } else if (eventType === 'done') {
            if (currentAssistantMsg) currentAssistantMsg.classList.remove('token-cursor');
            currentAssistantMsg = null;
            scrollToBottom();
          }
        }
      }
    }
  } catch (err) {
    if (currentAssistantMsg) {
      currentAssistantMsg.classList.remove('token-cursor');
      answerContent.innerHTML = 'Error: ' + escapeHtml(err.message);
    }
  } finally {
    document.getElementById('send-btn').disabled = false;
    input.focus();
  }
});

// === DRAWER ===
const drawer = document.getElementById('drawer');
const overlay = document.getElementById('drawer-overlay');
const drawerBody = document.getElementById('drawer-body');
const drawerHeader = document.getElementById('drawer-header');
let drawerState = 'list';
let currentCourseId = null;

document.getElementById('plan-btn').addEventListener('click', openDrawer);
document.getElementById('drawer-close').addEventListener('click', closeDrawer);
overlay.addEventListener('click', closeDrawer);

function openDrawer() {
  overlay.classList.add('open');
  drawer.classList.add('open');
  if (drawerState === 'list') {
    fetchPlanList();
  } else if (currentCourseId) {
    fetchFullPlan(currentCourseId);
  }
}

function closeDrawer() {
  overlay.classList.remove('open');
  drawer.classList.remove('open');
}

function showBackButton() {
  drawerHeader.innerHTML = '<button id="drawer-back" style="font-size:11px;font-weight:600;cursor:pointer;color:var(--text-secondary);text-transform:uppercase;letter-spacing:0.8px;padding:5px 10px;border-radius:var(--radius-sm);border:none;background:none;font-family:inherit;transition:color 0.15s,background 0.15s;">&larr; Courses</button><button id="drawer-close" style="font-size:13px;cursor:pointer;padding:4px 10px;border-radius:var(--radius-sm);color:var(--text-secondary);border:none;background:none;font-family:inherit;transition:color 0.15s,background 0.15s;">Close</button>';
  document.getElementById('drawer-back').addEventListener('click', showPlanList);
  document.getElementById('drawer-close').addEventListener('click', closeDrawer);
}

function showPlanHeader() {
  drawerHeader.innerHTML = '<h2>Study Plan</h2><button id="drawer-close" style="font-size:13px;cursor:pointer;padding:4px 10px;border-radius:var(--radius-sm);color:var(--text-secondary);border:none;background:none;font-family:inherit;transition:color 0.15s,background 0.15s;">Close</button>';
  document.getElementById('drawer-close').addEventListener('click', closeDrawer);
}

async function fetchPlanList() {
  drawerState = 'list';
  currentCourseId = null;
  showPlanHeader();
  drawerBody.innerHTML = '<div style="text-align:center;color:var(--text-tertiary);padding:48px 0;">Loading...</div>';
  try {
    const resp = await apiFetch('/api/plan');
    const data = await resp.json();
    renderCourseList(data);
  } catch (err) {
    drawerBody.innerHTML = '<div style="text-align:center;color:#DC2626;padding:48px 0;">Failed to load plans</div>';
  }
}

function renderCourseList(courses) {
  if (!courses || courses.length === 0) {
    drawerBody.innerHTML = '<div class="empty-plan">No courses found</div>';
    return;
  }
  let html = '';
  for (const c of courses) {
    const status = c.hasPlan ? (c.total === c.done ? 'All done' : c.done + '/' + c.total + ' done') : 'No plan';
    html += `
      <div class="course-card" style="cursor:pointer;" data-action="open-full-plan" data-course-id="${escapeHtmlAttr(c.id)}">
        <div class="course-card-header" style="cursor:pointer;">
          <div>
            <h3>${escapeHtml(c.name)}</h3>
            <div class="next-task">${escapeHtml(status)}</div>
          </div>
          <span class="chevron" style="font-size:16px;">&#x2192;</span>
        </div>
      </div>`;
  }
  drawerBody.innerHTML = html;
}

function openFullPlan(courseId) {
  drawerState = 'plan';
  currentCourseId = courseId;
  showBackButton();
  drawerBody.innerHTML = '<div style="text-align:center;color:var(--text-tertiary);padding:48px 0;">Loading...</div>';
  fetchFullPlan(courseId);
}

function showPlanList() {
  drawerState = 'list';
  currentCourseId = null;
  showPlanHeader();
  fetchPlanList();
}

async function fetchFullPlan(courseId) {
  try {
    const [planResp, pdfResp] = await Promise.all([
      apiFetch('/api/plan?view=full&id=' + encodeURIComponent(courseId)),
      apiFetch('/pdf/list')
    ]);
    const plan = await planResp.json();
    const allPdfs = await pdfResp.json();
    const coursePdfs = allPdfs.filter(p => p.course_id === courseId);
    renderFullPlan(plan, coursePdfs);
  } catch (err) {
    drawerBody.innerHTML = '<div style="text-align:center;color:#DC2626;padding:48px 0;">Failed to load plan</div>';
  }
}

function renderFullPlan(plan, coursePdfs) {
  if (!plan || !plan.phases || plan.phases.length === 0) {
    drawerBody.innerHTML = '<div class="empty-plan">No plan data</div>';
    return;
  }

  let html = '<div class="plan-content" style="overflow-wrap:break-word;word-break:break-word;">';
  let checkboxIdx = 0;

  for (const phase of plan.phases) {
    html += `<h2 style="font-size:13px;font-weight:700;margin:20px 0 8px;text-transform:uppercase;letter-spacing:0.3px;color:var(--text-secondary);">${escapeHtml(phase.title)}</h2>`;

    if (phase.tasks && phase.tasks.length > 0) {
      for (const task of phase.tasks) {
        const idx = checkboxIdx++;
        html += renderTaskRow(task, idx, plan.id);
      }
    }

    if (phase.clusters) {
      for (const cluster of phase.clusters) {
        html += `<h3 style="font-size:12px;font-weight:600;margin:12px 0 6px;color:var(--text-secondary);">${escapeHtml(cluster.title)}</h3>`;
        if (cluster.tasks) {
          for (const task of cluster.tasks) {
            const idx = checkboxIdx++;
            html += renderTaskRow(task, idx, plan.id);
          }
        }
      }
    }
  }

  if (plan.sessions && plan.sessions.length > 0) {
    html += `<h2 style="font-size:13px;font-weight:700;margin:24px 0 8px;text-transform:uppercase;letter-spacing:0.3px;color:var(--text-secondary);">Session Log</h2>`;
    html += `<table style="width:100%;font-size:12px;border-collapse:collapse;margin:8px 0;">
      <tr><th style="border:1px solid var(--border);padding:4px 8px;background:var(--bg-sunken);text-align:left;font-weight:600;">Date</th>
      <th style="border:1px solid var(--border);padding:4px 8px;background:var(--bg-sunken);text-align:left;font-weight:600;">Topic</th>
      <th style="border:1px solid var(--border);padding:4px 8px;background:var(--bg-sunken);text-align:left;font-weight:600;">Time</th></tr>`;
    for (const s of plan.sessions) {
      html += `<tr><td style="border:1px solid var(--border);padding:4px 8px;">${escapeHtml(s.date)}</td>
        <td style="border:1px solid var(--border);padding:4px 8px;">${escapeHtml(s.topic)}</td>
        <td style="border:1px solid var(--border);padding:4px 8px;">${escapeHtml(s.time)}</td></tr>`;
    }
    html += '</table>';
  }

  if (coursePdfs && coursePdfs.length > 0) {
    html += `<h2 style="font-size:13px;font-weight:700;margin:24px 0 8px;text-transform:uppercase;letter-spacing:0.3px;color:var(--text-secondary);">PDFs</h2>`;
    for (const pdf of coursePdfs) {
      const progress = pdf.last_page > 1 ? 'p.' + pdf.last_page + '/' + pdf.pages : pdf.pages + ' pages';
      html += `<div class="drawer-pdf-item" data-action="open-pdf-from-drawer" data-pdf-id="${pdf.id}">
        <span class="dpf-name">${escapeHtml(pdf.original_name.replace(/\.pdf$/i, ''))}</span>
        <span class="dpf-progress">${escapeHtml(progress)}</span>
      </div>`;
    }
  }

  html += '</div>';
  drawerBody.innerHTML = html;

  requestAnimationFrame(() => {
    const doneChecks = drawerBody.querySelectorAll('.topic-checkbox.done');
    if (doneChecks.length > 0) {
      const lastDone = doneChecks[doneChecks.length - 1];
      const row = lastDone.closest('.topic-row');
      if (row) {
        row.scrollIntoView({ behavior: 'smooth', block: 'center' });
        row.classList.add('task-highlight');
        setTimeout(() => row.classList.remove('task-highlight'), 1500);
      }
    }
  });
}

function renderTaskRow(task, idx, courseId) {
  const priority = task.priority ? {high:'!',medium:'\u00B7',low:'\u223C'}[task.priority]||'' : '';
  const renderedTitle = marked.parseInline(task.title) || escapeHtml(task.title);
  const notes = task.notes ? `<div style="font-size:11px;color:var(--text-secondary);margin:2px 0 6px 26px;line-height:1.4;">${marked.parse(task.notes)}</div>` : '';
  return `<div class="topic-row" data-action="toggle-topic" data-course-id="${escapeHtmlAttr(courseId)}" data-idx="${idx}">
    <div class="topic-checkbox ${task.done ? 'done' : ''}">${task.done ? '&#x2713;' : ''}</div>
    <div class="topic-title ${task.done ? 'done' : ''}">
      ${priority ? '<span style="color:var(--text-tertiary);margin-right:4px;">' + escapeHtml(priority) + '</span>' : ''}${renderedTitle}
    </div>
  </div>${notes}`;
}

async function toggleTopic(courseId, idx) {
  const formData = new FormData();
  formData.append('course', courseId);
  formData.append('index', idx);
  try {
    await fetch('/api/plan/toggle', { method: 'POST', body: formData });
    fetchFullPlan(courseId);
  } catch (err) {
    console.error('toggle failed', err);
  }
}

function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

function escapeHtmlAttr(str) {
  return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

function scrollToBottom() {
  const msgs = document.getElementById('messages');
  msgs.scrollTop = msgs.scrollHeight;
}

function openPdfFromDrawer(pdfId) {
  closeDrawer();
  currentPdfId = pdfId;
  openPdf(pdfId);
  showView('split');
}

// === SPLITTER ===
const splitter = document.getElementById('splitter');
let isDragging = false;

splitter.addEventListener('mousedown', function(e) {
  e.preventDefault();
  isDragging = true;
  splitter.classList.add('dragging');
  document.body.style.cursor = 'col-resize';
  document.body.style.userSelect = 'none';
  document.addEventListener('mousemove', onSplitterDrag);
  document.addEventListener('mouseup', onSplitterRelease);
});

function onSplitterDrag(e) {
  if (!isDragging) return;
  const container = document.getElementById('main-content');
  const rect = container.getBoundingClientRect();
  const chatPanel = document.getElementById('chat-panel');
  const totalWidth = rect.width - 5;
  const chatWidth = e.clientX - rect.left;
  const pct = Math.max(20, Math.min(80, (chatWidth / totalWidth) * 100));
  chatPanel.style.flex = '0 0 ' + pct + '%';
}

function onSplitterRelease() {
  isDragging = false;
  splitter.classList.remove('dragging');
  document.body.style.cursor = '';
  document.body.style.userSelect = '';
  document.removeEventListener('mousemove', onSplitterDrag);
  document.removeEventListener('mouseup', onSplitterRelease);
  if (pdfDoc) {
    const viewer = document.getElementById('pdf-viewer');
    const saved = viewer.scrollTop;
    if (viewMode === 'scroll') renderAllPages();
    else renderPage(currentPage);
    viewer.scrollTop = saved;
  }
}

// touch support for splitter
splitter.addEventListener('touchstart', function(e) {
  e.preventDefault();
  isDragging = true;
  splitter.classList.add('dragging');
  document.addEventListener('touchmove', onSplitterTouchDrag, { passive: false });
  document.addEventListener('touchend', onSplitterTouchRelease);
});

function onSplitterTouchDrag(e) {
  if (!isDragging) return;
  e.preventDefault();
  const touch = e.touches[0];
  const container = document.getElementById('main-content');
  const rect = container.getBoundingClientRect();
  const chatPanel = document.getElementById('chat-panel');
  const totalWidth = rect.width - 5;
  const chatWidth = touch.clientX - rect.left;
  const pct = Math.max(20, Math.min(80, (chatWidth / totalWidth) * 100));
  chatPanel.style.flex = '0 0 ' + pct + '%';
}

function onSplitterTouchRelease() {
  isDragging = false;
  splitter.classList.remove('dragging');
  document.removeEventListener('touchmove', onSplitterTouchDrag);
  document.removeEventListener('touchend', onSplitterTouchRelease);
  if (pdfDoc) {
    const viewer = document.getElementById('pdf-viewer');
    const saved = viewer.scrollTop;
    if (viewMode === 'scroll') renderAllPages();
    else renderPage(currentPage);
    viewer.scrollTop = saved;
  }
}

// === VIEW MANAGEMENT ===
let currentView = 'chat';
let currentPdfId = null;
let pdfDoc = null;
let currentPage = 1;
let totalPages = 0;
let scale = 1.0;
let currentScale = 1.0;
let viewMode = 'scroll';
let pdfjsReady = false;
let renderedPages = null;
let pageObserver = null;

window.addEventListener('pdfjs-ready', () => { pdfjsReady = true; });

const knownPdfCourses = [
  { id: 'ce297', name: 'Safety Models and Techniques (CE-297)' },
  { id: 'ddia', name: 'Designing Data-Intensive Applications' },
  { id: 'dsa-interview', name: 'DSA Interview Prep' },
  { id: 'software-arch', name: 'Software Architecture' },
  { id: 'thesis', name: 'Thesis — Phase 1 Survey' },
];

function showView(view) {
  currentView = view;
  const chatPanel = document.getElementById('chat-panel');
  const pdfPanel = document.getElementById('pdf-panel');
  const splitterEl = document.getElementById('splitter');

  chatPanel.classList.remove('hidden');
  chatPanel.style.flex = '';
  chatPanel.style.minWidth = '';
  chatPanel.style.opacity = '';
  pdfPanel.classList.remove('visible');
  pdfPanel.style.flex = '';
  pdfPanel.style.minWidth = '';
  splitterEl.classList.remove('visible');

  if (view === 'chat') {
    chatPanel.style.flex = '1';
  } else if (view === 'pdf') {
    chatPanel.classList.add('hidden');
    pdfPanel.classList.add('visible');
    pdfPanel.style.flex = '1';
  } else if (view === 'split') {
    pdfPanel.classList.add('visible');
    splitterEl.classList.add('visible');
    chatPanel.style.flex = '0 0 38%';
    pdfPanel.style.flex = '1';
    chatPanel.style.minWidth = '280px';
    pdfPanel.style.minWidth = '300px';
  }
}

document.getElementById('pdf-btn').addEventListener('click', function() {
  if (currentView === 'chat') {
    if (currentPdfId) {
      showView('split');
    } else {
      showView('split');
      const pdfPanel = document.getElementById('pdf-panel');
      pdfPanel.classList.add('panel-enter');
      loadPdfEmptyState();
      setTimeout(() => pdfPanel.classList.remove('panel-enter'), 300);
    }
  } else {
    showView('chat');
  }
  this.classList.toggle('active', currentView !== 'chat');
});

// PDF upload
const uploadZone = document.getElementById('pdf-upload-zone');
const fileInput = document.getElementById('pdf-file-input');

uploadZone.addEventListener('dragover', function(e) { e.preventDefault(); uploadZone.classList.add('dragover'); });
uploadZone.addEventListener('dragleave', function() { uploadZone.classList.remove('dragover'); });
uploadZone.addEventListener('drop', function(e) {
  e.preventDefault();
  uploadZone.classList.remove('dragover');
  const files = e.dataTransfer.files;
  if (files.length > 0 && files[0].type === 'application/pdf') { uploadPdf(files[0]); }
});
fileInput.addEventListener('change', function() {
  if (fileInput.files.length > 0) { uploadPdf(fileInput.files[0]); }
});

async function uploadPdf(file) {
  const isPdf = file.type === 'application/pdf' || /\.pdf$/i.test(file.name);
  if (!isPdf) {
    showErrorBanner('Only PDF files are supported.');
    return;
  }
  if (file.size > MAX_PDF_BYTES) {
    const mb = (file.size / 1024 / 1024).toFixed(1);
    showErrorBanner('PDF too large: ' + mb + ' MB (max ' + (MAX_PDF_BYTES / 1024 / 1024) + ' MB).');
    return;
  }
  const courseSelect = document.getElementById('pdf-course-select');
  const formData = new FormData();
  formData.append('file', file);
  formData.append('course_id', courseSelect.value);

  const zone = document.getElementById('pdf-upload-zone');
  zone.innerHTML = '<h3>Uploading...</h3><p>' + escapeHtml(file.name) + '</p>';

  try {
    const resp = await fetch('/pdf/upload', { method: 'POST', body: formData });
    if (!resp.ok) throw new Error('Upload failed: ' + resp.status);
    const pdf = await resp.json();
    currentPdfId = pdf.id;
    zone.innerHTML = '<h3>Drop PDF here or click to upload</h3><p>Supported: .pdf files</p>';
    await openPdf(pdf.id);
  } catch (err) {
    zone.innerHTML = '<h3>Drop PDF here or click to upload</h3><p>Supported: .pdf files</p>';
    alert('Upload failed: ' + err.message);
  }
}

async function loadPdfEmptyState() {
  const container = document.getElementById('pdf-list-container');
  container.innerHTML = '<div style="text-align:center;color:var(--text-tertiary);">Loading...</div>';
  try {
    const resp = await apiFetch('/pdf/list');
    const pdfs = await resp.json();
    renderPdfEmptyList(pdfs);
  } catch (err) {
    container.innerHTML = '<div style="text-align:center;color:#DC2626;">Failed to load PDFs</div>';
  }
}

function renderPdfEmptyList(pdfs) {
  const container = document.getElementById('pdf-list-container');
  if (!pdfs || pdfs.length === 0) {
    container.innerHTML = '<div style="text-align:center;color:var(--text-tertiary);font-size:13px;">No PDFs uploaded yet</div>';
    return;
  }

  const groups = {};
  for (const pdf of pdfs) {
    const key = pdf.course_id || 'library';
    if (!groups[key]) groups[key] = { name: pdf.course_name || 'Library', pdfs: [] };
    groups[key].pdfs.push(pdf);
  }

  let html = '';
  for (const [key, group] of Object.entries(groups)) {
    const courseInfo = knownPdfCourses.find(c => c.id === key);
    const groupName = courseInfo ? courseInfo.name : 'Library';
    html += '<div class="pdf-list-section"><h4>' + escapeHtml(groupName) + '</h4>';
    for (const pdf of group.pdfs) {
      const progress = pdf.last_page > 1 ? 'p.' + pdf.last_page + ' / ' + pdf.pages : 'Not started';
      html += '<div class="pdf-list-item" data-action="open-pdf" data-pdf-id="' + pdf.id + '">' +
        '<span class="pdf-name">' + escapeHtml(pdf.original_name.replace(/\.pdf$/i, '')) + '</span>' +
        '<span class="pdf-progress">' + escapeHtml(progress) + '</span></div>';
    }
    html += '</div>';
  }
  container.innerHTML = html;
}

async function openPdf(id) {
  if (!window.pdfjsLib) {
    alert('PDF viewer is still loading. Please try again in a moment.');
    return;
  }

  try {
    const resp = await apiFetch('/pdf/list');
    const pdfs = await resp.json();
    const pdf = pdfs.find(p => p.id === id);
    if (!pdf) throw new Error('PDF not found');

    currentPdfId = id;
    currentPage = pdf.last_page || 1;
    scale = 1.0;
    currentScale = 1.0;
    document.getElementById('pdf-zoom-level').textContent = '100%';
    document.getElementById('pdf-viewer').style.transform = 'none';

    fetch('/pdf/progress/' + id, { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ page: currentPage }) });

    const loadingTask = window.pdfjsLib.getDocument('/pdf/file/' + id);
    pdfDoc = await loadingTask.promise;
    totalPages = pdfDoc.numPages;

    document.getElementById('pdf-empty-state').style.display = 'none';
    document.getElementById('pdf-open-state').style.display = 'flex';
    document.getElementById('pdf-filename').textContent = pdf.original_name.replace(/\.pdf$/i, '');

    if (viewMode === 'scroll') {
      renderAllPages();
      const targetCanvas = document.getElementById('pdf-canvas-' + currentPage);
      if (targetCanvas) {
        targetCanvas.scrollIntoView({ behavior: 'auto', block: 'start' });
      }
    } else {
      renderPage(currentPage);
    }

    showView(currentView === 'chat' ? 'split' : currentView);
    document.getElementById('pdf-btn').classList.add('active');
  } catch (err) {
    console.error('Failed to open PDF:', err);
    alert('Failed to open PDF: ' + err.message);
  }
}

function renderAllPages() {
  const viewer = document.getElementById('pdf-viewer');
  const counter = document.getElementById('pdf-page-counter');
  if (pageObserver) { pageObserver.disconnect(); pageObserver = null; }
  viewer.innerHTML = '';
  viewer.appendChild(counter);
  renderedPages = new Set();

  for (let i = 1; i <= totalPages; i++) {
    const canvas = document.createElement('canvas');
    canvas.className = 'pdf-page-canvas';
    canvas.id = 'pdf-canvas-' + i;
    canvas.dataset.pageNum = i;
    viewer.appendChild(canvas);
  }

  let pdfSaveTimer = null;
  let mostVisiblePage = currentPage;

  pageObserver = new IntersectionObserver((entries) => {
    let bestRatio = 0;
    let bestPage = 0;
    for (const entry of entries) {
      if (entry.isIntersecting) {
        const pageNum = parseInt(entry.target.dataset.pageNum);
        if (!renderedPages.has(pageNum)) {
          renderedPages.add(pageNum);
          renderPageToCanvas(pageNum, entry.target);
        }
        if (entry.intersectionRatio > bestRatio) {
          bestRatio = entry.intersectionRatio;
          bestPage = pageNum;
        }
      }
    }
    if (bestPage > 0 && bestPage !== mostVisiblePage) {
      mostVisiblePage = bestPage;
      clearTimeout(pdfSaveTimer);
      pdfSaveTimer = setTimeout(() => {
        if (currentPdfId) {
          currentPage = mostVisiblePage;
          updatePageCounter();
          fetch('/pdf/progress/' + currentPdfId, {
            method: 'PUT',
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify({ page: mostVisiblePage })
          }).catch(() => {});
        }
      }, 2000);
    }
  }, { root: viewer, rootMargin: '200px', threshold: [0, 0.1, 0.3, 0.6, 0.9] });

  viewer.querySelectorAll('.pdf-page-canvas').forEach(canvas => {
    pageObserver.observe(canvas);
  });

  updatePageCounter();
}

async function renderPageToCanvas(pageNum, canvas) {
  try {
    const page = await pdfDoc.getPage(pageNum);
    const containerWidth = document.getElementById('pdf-viewer').clientWidth - 48;
    const viewport = page.getViewport({ scale: 1 });
    const fitScale = (containerWidth / viewport.width) * scale;
    const scaledViewport = page.getViewport({ scale: fitScale });

    canvas.width = scaledViewport.width;
    canvas.height = scaledViewport.height;

    const ctx = canvas.getContext('2d');
    await page.render({ canvasContext: ctx, viewport: scaledViewport }).promise;
  } catch (err) {
    console.error('Error rendering page', pageNum, err);
  }
}

function renderPage(pageNum) {
  const viewer = document.getElementById('pdf-viewer');
  const counter = document.getElementById('pdf-page-counter');
  if (pageObserver) { pageObserver.disconnect(); pageObserver = null; }
  renderedPages = null;
  viewer.innerHTML = '';
  viewer.appendChild(counter);

  const canvas = document.createElement('canvas');
  canvas.className = 'pdf-page-canvas';
  viewer.appendChild(canvas);

  renderPageToCanvas(pageNum, canvas);
  updatePageCounter();
}

function updatePageCounter() {
  const counter = document.getElementById('pdf-page-counter');
  if (counter) { counter.textContent = currentPage + ' / ' + totalPages; }
}

let scrollTimeout = null;
document.getElementById('pdf-viewer')?.addEventListener('scroll', function() {
  if (viewMode !== 'scroll') return;
  clearTimeout(scrollTimeout);
  scrollTimeout = setTimeout(() => {
    const viewer = document.getElementById('pdf-viewer');
    const canvases = viewer.querySelectorAll('.pdf-page-canvas');
    const viewerRect = viewer.getBoundingClientRect();
    for (const canvas of canvases) {
      const rect = canvas.getBoundingClientRect();
      if (rect.top >= viewerRect.top && rect.top <= viewerRect.bottom) {
        const pageNum = parseInt(canvas.id.replace('pdf-canvas-', ''));
        if (pageNum && pageNum !== currentPage) {
          currentPage = pageNum;
          updatePageCounter();
          savePdfProgress(currentPage);
        }
        break;
      }
    }
  }, 200);
});

function saveProgress() {
  if (!currentPdfId) return;
  fetch('/pdf/progress/' + currentPdfId, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ page: currentPage })
  }).catch(err => console.error('Save progress failed:', err));
}

let saveProgressTimer = null;
function savePdfProgress(pageNum) {
  if (!currentPdfId) return;
  clearTimeout(saveProgressTimer);
  saveProgressTimer = setTimeout(() => {
    fetch('/pdf/progress/' + currentPdfId, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ page: pageNum })
    }).catch(() => {});
  }, 2000);
}

// Toolbar handlers
document.getElementById('pdf-close-btn')?.addEventListener('click', function() {
  currentPdfId = null;
  pdfDoc = null;
  document.getElementById('pdf-open-state').style.display = 'none';
  document.getElementById('pdf-empty-state').style.display = '';
  loadPdfEmptyState();
  showView('chat');
  document.getElementById('pdf-btn').classList.remove('active');
});

document.getElementById('pdf-view-toggle')?.addEventListener('click', function() {
  if (viewMode === 'scroll') {
    viewMode = 'single';
    this.textContent = 'Page';
    if (pdfDoc) renderPage(currentPage);
  } else {
    viewMode = 'scroll';
    this.textContent = 'Scroll';
    if (pdfDoc) renderAllPages();
  }
});

function applyZoom(newScale) {
  currentScale = Math.max(0.25, Math.min(3, newScale));
  document.getElementById('pdf-zoom-level').textContent = Math.round(currentScale * 100) + '%';
  const viewer = document.getElementById('pdf-viewer');
  viewer.style.transformOrigin = 'top center';
  if (currentScale > 2.0 && pdfDoc) {
    scale = currentScale;
    viewer.style.transform = 'none';
    const saved = viewer.scrollTop;
    if (viewMode === 'scroll') renderAllPages();
    else renderPage(currentPage);
    viewer.scrollTop = saved;
  } else {
    viewer.style.transform = `scale(${currentScale})`;
  }
}

document.getElementById('pdf-zoom-in')?.addEventListener('click', function() {
  applyZoom(currentScale + 0.25);
});

document.getElementById('pdf-zoom-out')?.addEventListener('click', function() {
  applyZoom(currentScale - 0.25);
});

// Keyboard shortcuts
document.addEventListener('keydown', function(e) {
  if (!currentPdfId || !pdfDoc) return;
  if (document.activeElement === document.getElementById('message-input')) return;

  switch(e.key) {
    case 'j':
    case 'ArrowDown':
      e.preventDefault();
      if (viewMode === 'single') {
        currentPage = Math.min(totalPages, currentPage + 1);
        renderPage(currentPage);
        saveProgress();
      } else {
        document.getElementById('pdf-viewer').scrollBy(0, 100);
      }
      break;
    case 'k':
    case 'ArrowUp':
      e.preventDefault();
      if (viewMode === 'single') {
        currentPage = Math.max(1, currentPage - 1);
        renderPage(currentPage);
        saveProgress();
      } else {
        document.getElementById('pdf-viewer').scrollBy(0, -100);
      }
      break;
    case 's':
      e.preventDefault();
      if (currentView === 'pdf') showView('split');
      else if (currentView === 'split') showView('chat');
      else showView('split');
      break;
    case 'q':
      e.preventDefault();
      document.getElementById('pdf-close-btn').click();
      break;
    case '+':
    case '=':
      e.preventDefault();
      document.getElementById('pdf-zoom-in').click();
      break;
    case '-':
      e.preventDefault();
      document.getElementById('pdf-zoom-out').click();
      break;
    case '0':
      e.preventDefault();
      applyZoom(1.0);
      break;
  }
});

// Text selection -> chat draft
document.getElementById('pdf-viewer')?.addEventListener('mouseup', function() {
  const selection = window.getSelection();
  const text = selection.toString().trim();
  if (!text) return;
  const prefix = '[p.' + currentPage + '] "' + text.substring(0, 120) + (text.length > 120 ? '...' : '') + '" ';
  const input = document.getElementById('message-input');
  if (input && !input.value.includes(prefix)) {
    input.value = input.value ? prefix + input.value : prefix;
    input.focus();
  }
});

// Debounced resize with scroll preservation
let resizeDebounce = null;
let lastViewerWidth = 0;
window.addEventListener('resize', () => {
  if (!pdfDoc || currentView === 'chat') return;
  clearTimeout(resizeDebounce);
  resizeDebounce = setTimeout(() => {
    const viewer = document.getElementById('pdf-viewer');
    const newWidth = viewer.clientWidth;
    if (Math.abs(newWidth - lastViewerWidth) < 5) return;
    lastViewerWidth = newWidth;
    const savedScroll = viewer.scrollTop;
    if (viewMode === 'scroll') renderAllPages();
    else renderPage(currentPage);
    viewer.scrollTop = savedScroll;
  }, 300);
});

// Auto-open last PDF on startup in split view
window.addEventListener('pdfjs-ready', async () => {
  try {
    const resp = await apiFetch('/pdf/last');
    const data = await resp.json();
    if (data && data.pdf) {
      currentPdfId = data.pdf.id;
      currentPage = data.pdf.last_page || 1;
      await openPdf(currentPdfId);
    }
  } catch (err) {
    // No last opened PDF, that's fine
  }
});

// Filename dropdown
document.getElementById('pdf-filename')?.addEventListener('click', async function(e) {
  e.stopPropagation();
  const dropdown = document.getElementById('pdf-dropdown');
  if (dropdown.classList.contains('open')) {
    dropdown.classList.remove('open');
    return;
  }
  try {
    const resp = await apiFetch('/pdf/list');
    const pdfs = await resp.json();
    renderPdfDropdown(pdfs);
    dropdown.classList.add('open');
  } catch (err) {
    console.error('Failed to load PDF list', err);
  }
});

document.addEventListener('click', function(e) {
  const dropdown = document.getElementById('pdf-dropdown');
  if (dropdown && dropdown.classList.contains('open')) {
    if (!e.target.closest('#pdf-filename') && !e.target.closest('#pdf-dropdown')) {
      dropdown.classList.remove('open');
    }
  }
});

function renderPdfDropdown(pdfs) {
  const dropdown = document.getElementById('pdf-dropdown');
  const groups = {};
  for (const pdf of pdfs) {
    const key = pdf.course_id || 'library';
    if (!groups[key]) groups[key] = { name: pdf.course_name || 'Library', pdfs: [] };
    groups[key].pdfs.push(pdf);
  }

  let html = '<div class="dropdown-item" style="font-weight:600;" data-action="trigger-upload">\u2191 Upload PDF</div>';
  html += '<div style="height:1px;background:var(--border);margin:4px 0;"></div>';
  for (const [key, group] of Object.entries(groups)) {
    html += '<div class="dropdown-section"><div class="dropdown-section-title">' + escapeHtml(group.name) + '</div>';
    for (const pdf of group.pdfs) {
      const progress = pdf.last_page > 1 ? 'p.' + pdf.last_page + '/' + pdf.pages : pdf.pages + ' pages';
      html += '<div class="dropdown-item" data-action="switch-pdf" data-pdf-id="' + pdf.id + '">' +
        '<span class="item-name">' + escapeHtml(pdf.original_name.replace(/\.pdf$/i, '')) + '</span>' +
        '<span class="item-progress">' + escapeHtml(progress) + '</span></div>';
    }
    html += '</div>';
  }
  dropdown.innerHTML = html;
}

function triggerUpload() {
  document.getElementById('pdf-dropdown').classList.remove('open');
  document.getElementById('pdf-file-input').click();
}

async function switchPdf(id) {
  document.getElementById('pdf-dropdown').classList.remove('open');
  currentPdfId = id;
  await openPdf(id);
  showView(currentView);
}

// === HEALTH CHECK ===
const healthDot = document.getElementById('health-dot');
async function checkHealth() {
  healthDot.className = 'checking';
  try {
    const resp = await fetch('/debug/health', { signal: AbortSignal.timeout(5000) });
    if (resp.ok) {
      healthDot.className = 'ok';
      healthDot.title = 'Connected';
    } else {
      throw new Error('HTTP ' + resp.status);
    }
  } catch (err) {
    healthDot.className = 'error';
    healthDot.title = 'Connection error: ' + err.message;
  }
}
checkHealth();
setInterval(checkHealth, 30000);
