// Plan-spine rail: the selected course's plan (phases → tasks) is the left-rail
// navigator. Tasks resolve to Sessions via /api/sessions/for-task (lazy).
import { apiFetch } from './apiFetch.js';
import { courseMeta, setActiveSessionId, loadSessionMessages, clearWorkspace } from './sessions.js';
import { escapeHtml } from './dom.js';

const SELECTED_COURSE_KEY = 'claw-study:railCourse';

let selectedCourse = localStorage.getItem(SELECTED_COURSE_KEY) || 'ce297';
let currentPlan = null; // JSONPlan for selectedCourse, or null
let sessionsByTaskId = {}; // task_id -> session (has-work lookup)
let otherSessions = []; // task_id-less sessions (Scratch stub)
let pendingTask = null; // {courseId, taskId, title} when a task is open with no session yet

export function getSelectedCourse() {
  return selectedCourse;
}
export function getPendingTask() {
  return pendingTask;
}
export function clearPendingTask() {
  pendingTask = null;
}

// loadRailData fetches the selected course's full plan and the sessions list,
// then indexes sessions by task_id (for has-work dots and task→session resolution).
export async function loadRailData() {
  try {
    const [planResp, sessResp] = await Promise.all([
      apiFetch('/api/plan?view=full&id=' + encodeURIComponent(selectedCourse)),
      apiFetch('/api/sessions'),
    ]);
    currentPlan = planResp.ok ? await planResp.json() : null;
    const sessions = await sessResp.json();
    sessionsByTaskId = {};
    otherSessions = [];
    for (const s of sessions || []) {
      if (s.task_id) sessionsByTaskId[s.task_id] = s;
      else otherSessions.push(s);
    }
  } catch (err) {
    console.error('Failed to load rail data', err);
    currentPlan = null;
  }
}

// Linear task index across phases+clusters — matches the backend's toggle index
// (handler/plan.go toggleTaskAt walks phases' tasks then each cluster's tasks).
function walkTasks(plan, fn) {
  let idx = 0;
  for (const phase of plan.phases || []) {
    fn({ kind: 'phase', title: phase.title });
    for (const task of phase.tasks || []) fn({ kind: 'task', task, idx: idx++ });
    for (const cluster of phase.clusters || []) {
      fn({ kind: 'cluster', title: cluster.title });
      for (const task of cluster.tasks || []) fn({ kind: 'task', task, idx: idx++ });
    }
  }
}

function firstUndoneTaskId(plan) {
  let found = null;
  walkTasks(plan, (n) => {
    if (found) return;
    if (n.kind === 'task' && !n.task.done) found = n.task.id;
  });
  return found;
}

export function renderRail() {
  const container = document.getElementById('session-list');
  const switcher = renderCourseSwitcher();
  if (!currentPlan || !currentPlan.phases || currentPlan.phases.length === 0) {
    container.innerHTML =
      switcher + '<div class="rail-empty">No plan for this course yet.</div>' + renderOther();
    return;
  }
  const nextId = firstUndoneTaskId(currentPlan);
  let html = switcher + '<div class="rail-plan">';
  walkTasks(currentPlan, (n) => {
    if (n.kind === 'phase') {
      html += `<div class="rail-phase">${escapeHtml(n.title)}</div>`;
    } else if (n.kind === 'cluster') {
      html += `<div class="rail-cluster">${escapeHtml(n.title)}</div>`;
    } else {
      const t = n.task;
      const hasWork = !!sessionsByTaskId[t.id];
      const isNext = t.id === nextId;
      html += `<div class="rail-task${t.done ? ' done' : ''}${isNext ? ' next' : ''}" data-action="open-task" data-task-id="${escapeHtml(t.id)}" data-idx="${n.idx}">
        <span class="rail-check${t.done ? ' done' : ''}" data-action="toggle-task" data-idx="${n.idx}">${t.done ? '&#x2713;' : ''}</span>
        <span class="rail-task-title">${escapeHtml(t.title)}</span>
        <span class="rail-work-dot" style="visibility:${hasWork ? 'visible' : 'hidden'}" title="Has work"></span>
      </div>`;
    }
  });
  html += '</div>' + renderOther();
  container.innerHTML = html;
}

function renderCourseSwitcher() {
  let opts = '';
  for (const id of Object.keys(courseMeta)) {
    if (id === '') continue; // skip the "General" pseudo-course in the switcher
    const sel = id === selectedCourse ? ' selected' : '';
    opts += `<option value="${escapeHtml(id)}"${sel}>${escapeHtml(courseMeta[id].name)}</option>`;
  }
  return `<select id="rail-course-select" class="rail-course-select" data-action="noop">${opts}</select>`;
}

function renderOther() {
  if (!otherSessions.length) return '';
  let html = '<div class="rail-other"><div class="rail-other-label">Other chats</div>';
  for (const s of otherSessions) {
    html += `<div class="rail-other-item" data-action="switch-session" data-session-id="${s.id}">${escapeHtml(s.topic || 'Untitled')}</div>`;
  }
  return html + '</div>';
}

export function initRail() {
  // Course switcher: change event (not a click data-action) swaps the rail.
  document.getElementById('session-list').addEventListener('change', (e) => {
    const sel = e.target.closest('#rail-course-select');
    if (!sel) return;
    selectCourse(sel.value);
  });
}

export async function selectCourse(courseId) {
  selectedCourse = courseId;
  localStorage.setItem(SELECTED_COURSE_KEY, courseId);
  await loadRailData();
  renderRail();
}

// loadRail is the public entry: load data for the selected course, then render.
export async function loadRail() {
  await loadRailData();
  renderRail();
}

// openTask resolves the Session for a task. If one exists, it is activated and
// its chat loaded. If not, a pending-task workspace is shown (lazy: the row is
// created on the first message — see chat.js).
export async function openTask(taskId) {
  const title = taskTitleById(taskId);
  markActiveTask(taskId);
  try {
    const resp = await apiFetch(
      '/api/sessions/for-task?course_id=' +
        encodeURIComponent(selectedCourse) +
        '&task_id=' +
        encodeURIComponent(taskId),
    );
    const data = await resp.json();
    if (data && data.id) {
      pendingTask = null;
      await fetch('/api/sessions/active', {
        method: 'PUT',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ id: data.id }),
      });
      setActiveSessionId(data.id);
      await loadSessionMessages();
      setBanner(title);
    } else {
      // No session yet — hold a pending task; the workspace is empty until the
      // first message creates the row.
      pendingTask = { courseId: selectedCourse, taskId, title };
      setActiveSessionId(null);
      clearWorkspace();
      setBanner(title + '  ·  new — your first message starts it');
    }
  } catch (err) {
    console.error('Failed to open task', err);
  }
}

function taskTitleById(taskId) {
  let title = '';
  if (currentPlan) {
    walkTasks(currentPlan, (n) => {
      if (n.kind === 'task' && n.task.id === taskId) title = n.task.title;
    });
  }
  return title;
}

function markActiveTask(taskId) {
  document.querySelectorAll('.rail-task').forEach((el) => {
    el.classList.toggle('active', el.dataset.taskId === taskId);
  });
}

function setBanner(text) {
  const el = document.getElementById('workspace-banner');
  if (!text) {
    el.style.display = 'none';
    el.textContent = '';
    return;
  }
  el.style.display = 'block';
  el.textContent = text;
}
