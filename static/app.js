// Entry module. Wires the document-level data-action click delegator by
// importing handler functions from each feature module — no event bus.
import { installErrorBanner } from './errorBanner.js';
import {
  loadCourses,
  loadSessions,
  loadActiveSession,
  loadSessionMessages,
  switchSession,
  deleteSession,
  getActiveSessionId,
  initSessionsUI,
} from './sessions.js';
import { initChat } from './chat.js';
import { initPlan, openFullPlan, toggleTopic, openPdfFromDrawer } from './plan.js';
import { initPdf, openPdf, triggerUpload, switchPdf } from './pdf.js';
import { initPomodoro } from './pomodoro.js';

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
      deleteSession(parseInt(el.dataset.sessionId, 10));
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
  }
});

// Sidebar toggle (header)
document.getElementById('sidebar-toggle').addEventListener('click', function () {
  document.getElementById('session-sidebar').classList.toggle('collapsed');
});

async function loadRuntimeEndpoint() {
  try {
    const resp = await fetch('/api/runtime');
    const data = await resp.json();
    if (data.mode === 'pi') {
      return '/chat-v2';
    }
    return '/chat';
  } catch {
    return '/chat';
  }
}

initSessionsUI();
initPlan();
initPdf();
initPomodoro();

// Sessions startup
(async function initApp() {
  const chatEndpoint = await loadRuntimeEndpoint();
  initChat(chatEndpoint);
  await loadCourses();
  await loadSessions();
  await loadActiveSession();
  if (getActiveSessionId()) {
    await loadSessionMessages();
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
