// Plan drawer: course list, full plan rendering, task toggle.
import { apiFetch } from './apiFetch.js';
import { escapeHtml, escapeHtmlAttr } from './dom.js';
import { marked } from './marked.js';
import { openPdf, showView, setCurrentPdfId } from './pdf.js';

let drawer, overlay, drawerBody, drawerHeader;
let drawerState = 'list';
let currentCourseId = null;

export function initPlan() {
  drawer = document.getElementById('drawer');
  overlay = document.getElementById('drawer-overlay');
  drawerBody = document.getElementById('drawer-body');
  drawerHeader = document.getElementById('drawer-header');

  document.getElementById('plan-btn').addEventListener('click', openDrawer);
  document.getElementById('drawer-close').addEventListener('click', closeDrawer);
  overlay.addEventListener('click', closeDrawer);
  document.getElementById('session-pill').addEventListener('click', openDrawer);
}

export function openDrawer() {
  overlay.classList.add('open');
  drawer.classList.add('open');
  if (drawerState === 'list') {
    fetchPlanList();
  } else if (currentCourseId) {
    fetchFullPlan(currentCourseId);
  }
}

export function closeDrawer() {
  overlay.classList.remove('open');
  drawer.classList.remove('open');
}

function showBackButton() {
  drawerHeader.innerHTML =
    '<button id="drawer-back" style="font-size:11px;font-weight:600;cursor:pointer;color:var(--text-secondary);text-transform:uppercase;letter-spacing:0.8px;padding:5px 10px;border-radius:var(--radius-sm);border:none;background:none;font-family:inherit;transition:color 0.15s,background 0.15s;">&larr; Courses</button><button id="drawer-close" style="font-size:13px;cursor:pointer;padding:4px 10px;border-radius:var(--radius-sm);color:var(--text-secondary);border:none;background:none;font-family:inherit;transition:color 0.15s,background 0.15s;">Close</button>';
  document.getElementById('drawer-back').addEventListener('click', showPlanList);
  document.getElementById('drawer-close').addEventListener('click', closeDrawer);
}

function showPlanHeader() {
  drawerHeader.innerHTML =
    '<h2>Study Plan</h2><button id="drawer-close" style="font-size:13px;cursor:pointer;padding:4px 10px;border-radius:var(--radius-sm);color:var(--text-secondary);border:none;background:none;font-family:inherit;transition:color 0.15s,background 0.15s;">Close</button>';
  document.getElementById('drawer-close').addEventListener('click', closeDrawer);
}

async function fetchPlanList() {
  drawerState = 'list';
  currentCourseId = null;
  showPlanHeader();
  drawerBody.innerHTML =
    '<div style="text-align:center;color:var(--text-tertiary);padding:48px 0;">Loading...</div>';
  try {
    const resp = await apiFetch('/api/plan');
    const data = await resp.json();
    renderCourseList(data);
  } catch {
    drawerBody.innerHTML =
      '<div style="text-align:center;color:#DC2626;padding:48px 0;">Failed to load plans</div>';
  }
}

function renderCourseList(courses) {
  if (!courses || courses.length === 0) {
    drawerBody.innerHTML = '<div class="empty-plan">No courses found</div>';
    return;
  }
  let html = '';
  for (const c of courses) {
    const status = c.hasPlan
      ? c.total === c.done
        ? 'All done'
        : c.done + '/' + c.total + ' done'
      : 'No plan';
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

export function openFullPlan(courseId) {
  drawerState = 'plan';
  currentCourseId = courseId;
  showBackButton();
  drawerBody.innerHTML =
    '<div style="text-align:center;color:var(--text-tertiary);padding:48px 0;">Loading...</div>';
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
      apiFetch('/pdf/list'),
    ]);
    const plan = await planResp.json();
    const allPdfs = await pdfResp.json();
    const coursePdfs = allPdfs.filter((p) => p.course_id === courseId);
    renderFullPlan(plan, coursePdfs);
  } catch {
    drawerBody.innerHTML =
      '<div style="text-align:center;color:#DC2626;padding:48px 0;">Failed to load plan</div>';
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
      const progress =
        pdf.last_page > 1 ? 'p.' + pdf.last_page + '/' + pdf.pages : pdf.pages + ' pages';
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
  const priority = task.priority ? { high: '!', medium: '·', low: '∼' }[task.priority] || '' : '';
  const renderedTitle = marked.parseInline(task.title) || escapeHtml(task.title);
  const notes = task.notes
    ? `<div style="font-size:11px;color:var(--text-secondary);margin:2px 0 6px 26px;line-height:1.4;">${marked.parse(task.notes)}</div>`
    : '';
  return `<div class="topic-row" data-action="toggle-topic" data-course-id="${escapeHtmlAttr(courseId)}" data-idx="${idx}">
    <div class="topic-checkbox ${task.done ? 'done' : ''}">${task.done ? '&#x2713;' : ''}</div>
    <div class="topic-title ${task.done ? 'done' : ''}">
      ${priority ? '<span style="color:var(--text-tertiary);margin-right:4px;">' + escapeHtml(priority) + '</span>' : ''}${renderedTitle}
    </div>
  </div>${notes}`;
}

export async function toggleTopic(courseId, idx) {
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

export function openPdfFromDrawer(pdfId) {
  closeDrawer();
  setCurrentPdfId(pdfId);
  openPdf(pdfId);
  showView('split');
}
