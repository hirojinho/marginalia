// Entry module. Wires the document-level data-action click delegator by
// importing handler functions from each feature module — no event bus.
import { installErrorBanner } from './errorBanner.js';
import {
  loadCourses,
  loadActiveSession,
  loadSessionMessages,
  switchSession,
  deleteSession,
} from './sessions.js';
import { initChat } from './chat.js';
import { initPlan, openFullPlan, toggleTopic, openPdfFromDrawer } from './plan.js';
import { initPdf, openPdf, triggerUpload, switchPdf, populateCourseSelect } from './pdf.js';
import { initPomodoro } from './pomodoro.js';
import {
  initRail,
  loadRail,
  openTask,
  toggleTask,
  restoreReading,
  scrollRailToLastChecked,
  startNewCourseAuthoring,
  startDesignPlan,
} from './rail.js';

installErrorBanner();

// Document-level click dispatcher for [data-action] elements.
document.addEventListener('click', function (e) {
  const el = e.target.closest('[data-action]');
  if (!el) return;
  const action = el.dataset.action;
  switch (action) {
    case 'switch-session':
      switchSession(parseInt(el.dataset.sessionId, 10));
      break;
    case 'delete-session':
      e.stopPropagation();
      deleteSession(parseInt(el.dataset.sessionId, 10)).then(() => loadRail());
      break;
    case 'open-full-plan':
      openFullPlan(el.dataset.courseId);
      break;
    case 'open-pdf-from-drawer':
      openPdfFromDrawer(parseInt(el.dataset.pdfId, 10));
      break;
    case 'toggle-topic':
      toggleTopic(el.dataset.courseId, parseInt(el.dataset.idx, 10));
      break;
    case 'open-pdf':
      openPdf(parseInt(el.dataset.pdfId, 10));
      break;
    case 'trigger-upload':
      triggerUpload();
      break;
    case 'switch-pdf':
      switchPdf(parseInt(el.dataset.pdfId, 10));
      break;
    case 'toggle-task':
      e.stopPropagation();
      toggleTask(parseInt(el.dataset.idx, 10));
      break;
    case 'open-task':
      openTask(el.dataset.taskId);
      break;
    case 'new-course':
      startNewCourseAuthoring();
      break;
    case 'design-plan':
      startDesignPlan();
      break;
    case 'noop':
      break;
  }
});

// Sidebar toggle (header)
document.getElementById('sidebar-toggle').addEventListener('click', function () {
  const sidebar = document.getElementById('session-sidebar');
  const wasCollapsed = sidebar.classList.contains('collapsed');
  sidebar.classList.toggle('collapsed');
  // On open, jump to where the learner left off. Wait out the 0.2s width
  // transition so the list has reflowed to its final width before we measure.
  if (wasCollapsed) {
    setTimeout(scrollRailToLastChecked, 220);
  }
});

initRail();
initPlan();
initPdf();
initPomodoro();

// Sessions startup
(async function initApp() {
  const chatEndpoint = '/chat-v2';
  initChat(chatEndpoint);
  await loadCourses();
  populateCourseSelect(); // PDF upload dropdown — after courses are merged into courseMeta
  await loadRail();
  // Sidebar starts open, so this is the first view of the plan — position it at
  // the last completed task. rAF lets the freshly-rendered rail lay out first.
  requestAnimationFrame(scrollRailToLastChecked);
  const active = await loadActiveSession();
  if (active && active.id) {
    await loadSessionMessages();
    // Restore the active session's reading on load (deterministic — runs after
    // pdfjsLib is ready, unlike the old pdfjs-ready listener which raced the
    // head module's dispatch). Reading is tied to the session (ADR 0012).
    restoreReading(active.last_pdf_id);
  }
})();

// Health check
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
