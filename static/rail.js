// Plan-spine rail: the selected course's plan (phases → tasks) is the left-rail
// navigator. Tasks resolve to Sessions via /api/sessions/for-task (lazy).
import { apiFetch } from './apiFetch.js';
import {
  courseMeta,
  setActiveSessionId,
  loadSessionMessages,
  clearWorkspace,
  switchSession,
} from './sessions.js';
import { escapeHtml } from './dom.js';
import { openPdf, showView, setCurrentPdfId } from './pdf.js';
import { openCourseSettings } from './settings.js';
import { showErrorBanner } from './errorBanner.js';

const SELECTED_COURSE_KEY = 'claw-study:railCourse';

let selectedCourse = localStorage.getItem(SELECTED_COURSE_KEY) || 'ce297';
let currentPlan = null; // JSONPlan for selectedCourse, or null
let sessionsByTaskId = {}; // task_id -> session (has-work lookup)
let scratchSessions = []; // task_id-less, not archived (live Scratch)
let archivedSessions = []; // task_id-less, archived (pre-redesign — "Before the redesign")
let detachedSessions = []; // task_id set but absent from the current course's plan (orphaned)
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
    // Task ids present in the currently-selected course's plan.
    const planTaskIds = new Set();
    if (currentPlan) {
      walkTasks(currentPlan, (n) => {
        if (n.kind === 'task') planTaskIds.add(n.task.id);
      });
    }
    sessionsByTaskId = {};
    scratchSessions = [];
    archivedSessions = [];
    detachedSessions = [];
    for (const s of sessions || []) {
      if (s.task_id) {
        if (planTaskIds.has(s.task_id)) {
          sessionsByTaskId[s.task_id] = s; // has-work for a current plan task
        } else if (s.course_id === selectedCourse) {
          detachedSessions.push(s); // anchored to this course, but the task is gone
        }
        // task-anchored sessions of OTHER courses are not shown on this rail
      } else {
        // task-less = Scratch family; show global (no course) + this course's
        const inScope = !s.course_id || s.course_id === selectedCourse;
        if (!inScope) continue;
        if (s.archived) archivedSessions.push(s);
        else scratchSessions.push(s);
      }
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

// Scroll the rail so the last completed task is in view — where the learner
// left off. Falls back to the next (first incomplete) task, then does nothing
// if the plan is empty. Positions the target about a third down the list so the
// next task below it is also visible. Called when the sidebar is opened, not on
// every render (so toggling a task doesn't yank the scroll).
export function scrollRailToLastChecked() {
  const container = document.getElementById('session-list');
  if (!container) return;
  const done = container.querySelectorAll('.rail-task.done');
  const target = done.length ? done[done.length - 1] : container.querySelector('.rail-task.next');
  if (!target) return;
  const containerRect = container.getBoundingClientRect();
  const targetRect = target.getBoundingClientRect();
  const delta = targetRect.top - containerRect.top - container.clientHeight / 3;
  container.scrollTop = Math.max(0, container.scrollTop + delta);
}

function renderCourseSwitcher() {
  let opts = '';
  for (const id of Object.keys(courseMeta)) {
    if (id === '') continue; // skip the "General" pseudo-course in the switcher
    const sel = id === selectedCourse ? ' selected' : '';
    opts += `<option value="${escapeHtml(id)}"${sel}>${escapeHtml(courseMeta[id].name)}</option>`;
  }
  return (
    `<div class="rail-course-row">` +
    `<select id="rail-course-select" class="rail-course-select" data-action="noop">${opts}</select>` +
    `<button class="rail-settings-btn" data-action="new-course" title="Design a new course" aria-label="Design a new course">+</button>` +
    `<button class="rail-settings-btn" data-action="open-settings" title="Course settings" aria-label="Course settings">&#9881;</button>` +
    `</div>`
  );
}

function renderSessionLine(s) {
  return `<div class="rail-other-item" data-action="switch-session" data-session-id="${s.id}">${escapeHtml(s.topic || 'Untitled')}</div>`;
}

function renderOther() {
  let html = '';
  if (scratchSessions.length) {
    html += '<div class="rail-bucket"><div class="rail-other-label">Scratch</div>';
    for (const s of scratchSessions) html += renderSessionLine(s);
    html += '</div>';
  }
  if (detachedSessions.length) {
    html +=
      '<div class="rail-bucket"><div class="rail-other-label">Detached' +
      ` <span class="rail-bucket-hint">task removed from plan</span></div>`;
    for (const s of detachedSessions) html += renderSessionLine(s);
    html += '</div>';
  }
  if (archivedSessions.length) {
    html +=
      '<details class="rail-bucket rail-archive"><summary class="rail-other-label">' +
      `Before the redesign <span class="rail-bucket-hint">${archivedSessions.length}</span></summary>`;
    for (const s of archivedSessions) html += renderSessionLine(s);
    html += '</details>';
  }
  return html;
}

export async function startNewCourseAuthoring() {
  try {
    const resp = await apiFetch('/api/sessions', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({
        course_id: '',
        task_id: '',
        mode: 'authoring',
        topic: 'Design a new course',
      }),
    });
    if (!resp.ok) {
      const text = await resp.text();
      showErrorBanner('Failed to start authoring session: ' + text);
      return;
    }
    const session = await resp.json();
    await switchSession(session.id);
    await loadRail();
    const input = document.getElementById('message-input');
    if (input) input.focus();
  } catch (err) {
    showErrorBanner('Failed to start authoring session: ' + err.message);
  }
}

export function initRail() {
  // Course switcher: change event (not a click data-action) swaps the rail.
  document.getElementById('session-list').addEventListener('change', (e) => {
    const sel = e.target.closest('#rail-course-select');
    if (!sel) return;
    selectCourse(sel.value);
  });

  // Course settings gear.
  document.getElementById('session-list').addEventListener('click', (e) => {
    const btn = e.target.closest('[data-action="open-settings"]');
    if (!btn) return;
    e.stopPropagation();
    if (selectedCourse) openCourseSettings(selectedCourse);
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
      restoreReading(data.last_pdf_id);
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

// restoreReading opens a session's learned PDF on the right in split view.
// Reading is tied to the task/session (ADR 0012). No-op when pdfId is falsy
// (the session never opened a PDF). Used by openTask and by page-load restore.
export function restoreReading(pdfId) {
  if (!pdfId) return;
  setCurrentPdfId(pdfId);
  openPdf(pdfId);
  showView('split');
}

// toggleTask flips a task's done flag via the linear index (matches the backend
// walk order in handler/plan.go) and re-renders the rail.
export async function toggleTask(idx) {
  const fd = new FormData();
  fd.append('course', selectedCourse);
  fd.append('index', String(idx));
  try {
    await fetch('/api/plan/toggle', { method: 'POST', body: fd });
    await loadRail();
  } catch (err) {
    console.error('toggle failed', err);
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
