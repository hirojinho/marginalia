// PDF panel: upload, viewer state, page navigation, splitter, view layout.
import { apiFetch } from './apiFetch.js';
import { showErrorBanner } from './errorBanner.js';
import { escapeHtml } from './dom.js';

const MAX_PDF_BYTES = 50 * 1024 * 1024;

let currentView = 'chat';
let currentPdfId = null;
let pdfDoc = null;
let currentPage = 1;
let totalPages = 0;
let scale = 1.0;
let currentScale = 1.0;
let viewMode = 'scroll';
let renderedPages = null;
let pageObserver = null;

export function setCurrentPdfId(id) {
  currentPdfId = id;
}
export function getCurrentView() {
  return currentView;
}

const knownPdfCourses = [
  { id: 'ce297', name: 'Safety Models and Techniques (CE-297)' },
  { id: 'ddia', name: 'Designing Data-Intensive Applications' },
  { id: 'dsa-interview', name: 'DSA Interview Prep' },
  { id: 'software-arch', name: 'Software Architecture' },
  { id: 'thesis', name: 'Thesis — Phase 1 Survey' },
];

export function showView(view) {
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

async function uploadPdf(file) {
  const isPdf = file.type === 'application/pdf' || /\.pdf$/i.test(file.name);
  if (!isPdf) {
    showErrorBanner('Only PDF files are supported.');
    return;
  }
  if (file.size > MAX_PDF_BYTES) {
    const mb = (file.size / 1024 / 1024).toFixed(1);
    showErrorBanner('PDF too large: ' + mb + ' MB (max ' + MAX_PDF_BYTES / 1024 / 1024 + ' MB).');
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
  container.innerHTML =
    '<div style="text-align:center;color:var(--text-tertiary);">Loading...</div>';
  try {
    const resp = await apiFetch('/pdf/list');
    const pdfs = await resp.json();
    renderPdfEmptyList(pdfs);
  } catch {
    container.innerHTML = '<div style="text-align:center;color:#DC2626;">Failed to load PDFs</div>';
  }
}

function renderPdfEmptyList(pdfs) {
  const container = document.getElementById('pdf-list-container');
  if (!pdfs || pdfs.length === 0) {
    container.innerHTML =
      '<div style="text-align:center;color:var(--text-tertiary);font-size:13px;">No PDFs uploaded yet</div>';
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
    const courseInfo = knownPdfCourses.find((c) => c.id === key);
    const groupName = courseInfo ? courseInfo.name : 'Library';
    html += '<div class="pdf-list-section"><h4>' + escapeHtml(groupName) + '</h4>';
    for (const pdf of group.pdfs) {
      const progress = pdf.last_page > 1 ? 'p.' + pdf.last_page + ' / ' + pdf.pages : 'Not started';
      html +=
        '<div class="pdf-list-item" data-action="open-pdf" data-pdf-id="' +
        pdf.id +
        '">' +
        '<span class="pdf-name">' +
        escapeHtml(pdf.original_name.replace(/\.pdf$/i, '')) +
        '</span>' +
        '<span class="pdf-progress">' +
        escapeHtml(progress) +
        '</span></div>';
    }
    html += '</div>';
  }
  container.innerHTML = html;
}

export async function openPdf(id) {
  if (!window.pdfjsLib) {
    alert('PDF viewer is still loading. Please try again in a moment.');
    return;
  }

  try {
    const resp = await apiFetch('/pdf/list');
    const pdfs = await resp.json();
    const pdf = pdfs.find((p) => p.id === id);
    if (!pdf) throw new Error('PDF not found');

    currentPdfId = id;
    currentPage = pdf.last_page || 1;
    scale = 1.0;
    currentScale = 1.0;
    document.getElementById('pdf-zoom-level').textContent = '100%';
    document.getElementById('pdf-viewer').style.transform = 'none';

    fetch('/pdf/progress/' + id, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ page: currentPage }),
    });

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
  if (pageObserver) {
    pageObserver.disconnect();
    pageObserver = null;
  }
  viewer.innerHTML = '';
  renderedPages = new Set();

  for (let i = 1; i <= totalPages; i++) {
    const canvas = document.createElement('canvas');
    canvas.className = 'pdf-page-canvas';
    canvas.id = 'pdf-canvas-' + i;
    canvas.dataset.pageNum = i;
    viewer.appendChild(canvas);
  }

  // The observer only lazy-renders pages as they approach the viewport. The
  // current-page number is owned solely by the scroll listener (see initPdf),
  // so the two no longer fight over currentPage.
  pageObserver = new IntersectionObserver(
    (entries) => {
      for (const entry of entries) {
        if (entry.isIntersecting) {
          const pageNum = parseInt(entry.target.dataset.pageNum);
          if (!renderedPages.has(pageNum)) {
            renderedPages.add(pageNum);
            renderPageToCanvas(pageNum, entry.target);
          }
        }
      }
    },
    { root: viewer, rootMargin: '200px', threshold: 0.01 },
  );

  viewer.querySelectorAll('.pdf-page-canvas').forEach((canvas) => {
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
  if (pageObserver) {
    pageObserver.disconnect();
    pageObserver = null;
  }
  renderedPages = null;
  viewer.innerHTML = '';

  const canvas = document.createElement('canvas');
  canvas.className = 'pdf-page-canvas';
  viewer.appendChild(canvas);

  renderPageToCanvas(pageNum, canvas);
  updatePageCounter();
}

function updatePageCounter() {
  const input = document.getElementById('pdf-page-input');
  const total = document.getElementById('pdf-page-total');
  // Don't clobber what the user is mid-typing in the page input.
  if (input && document.activeElement !== input) {
    input.value = currentPage;
  }
  if (total) {
    total.textContent = totalPages;
  }
}

// Jump to a page typed into the toolbar input. Clamps to [1, totalPages],
// scrolls to it in scroll mode or re-renders it in single mode, and persists.
function goToPage(pageNum) {
  if (!pdfDoc || !totalPages) return;
  let target = parseInt(pageNum, 10);
  if (isNaN(target)) {
    updatePageCounter(); // revert the input to currentPage
    return;
  }
  target = Math.max(1, Math.min(totalPages, target));
  currentPage = target;
  if (viewMode === 'single') {
    renderPage(target);
  } else {
    const canvas = document.getElementById('pdf-canvas-' + target);
    if (canvas) canvas.scrollIntoView({ block: 'start' });
  }
  updatePageCounter();
  saveProgress();
}

function saveProgress() {
  if (!currentPdfId) return;
  fetch('/pdf/progress/' + currentPdfId, {
    method: 'PUT',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({ page: currentPage }),
  }).catch((err) => console.error('Save progress failed:', err));
}

let saveProgressTimer = null;
function savePdfProgress(pageNum) {
  if (!currentPdfId) return;
  clearTimeout(saveProgressTimer);
  saveProgressTimer = setTimeout(() => {
    fetch('/pdf/progress/' + currentPdfId, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ page: pageNum }),
    }).catch(() => {});
  }, 2000);
}

// Re-render the PDF and keep the user anchored on currentPage instead of
// pixel-scroll. Pixel-scroll restoration is broken in scroll mode because
// renderAllPages creates empty 0x0 canvases that get sized asynchronously,
// so saved pixel offsets no longer point at the same page after re-layout.
async function rerenderPreservingPage() {
  const targetPage = currentPage;
  if (viewMode === 'scroll') {
    renderAllPages();
    const canvas = document.getElementById('pdf-canvas-' + targetPage);
    if (canvas) {
      await renderPageToCanvas(targetPage, canvas);
      if (renderedPages) renderedPages.add(targetPage);
      canvas.scrollIntoView({ block: 'start' });
    }
  } else {
    renderPage(targetPage);
  }
}

function applyZoom(newScale) {
  currentScale = Math.max(0.25, Math.min(3, newScale));
  document.getElementById('pdf-zoom-level').textContent = Math.round(currentScale * 100) + '%';
  const viewer = document.getElementById('pdf-viewer');
  viewer.style.transformOrigin = 'top center';
  if (currentScale > 2.0 && pdfDoc) {
    scale = currentScale;
    viewer.style.transform = 'none';
    rerenderPreservingPage();
  } else {
    viewer.style.transform = `scale(${currentScale})`;
  }
}

function renderPdfDropdown(pdfs) {
  const dropdown = document.getElementById('pdf-dropdown');
  const groups = {};
  for (const pdf of pdfs) {
    const key = pdf.course_id || 'library';
    if (!groups[key]) groups[key] = { name: pdf.course_name || 'Library', pdfs: [] };
    groups[key].pdfs.push(pdf);
  }

  let html =
    '<div class="dropdown-item" style="font-weight:600;" data-action="trigger-upload">↑ Upload PDF</div>';
  html += '<div style="height:1px;background:var(--border);margin:4px 0;"></div>';
  for (const [, group] of Object.entries(groups)) {
    html +=
      '<div class="dropdown-section"><div class="dropdown-section-title">' +
      escapeHtml(group.name) +
      '</div>';
    for (const pdf of group.pdfs) {
      const progress =
        pdf.last_page > 1 ? 'p.' + pdf.last_page + '/' + pdf.pages : pdf.pages + ' pages';
      html +=
        '<div class="dropdown-item" data-action="switch-pdf" data-pdf-id="' +
        pdf.id +
        '">' +
        '<span class="item-name">' +
        escapeHtml(pdf.original_name.replace(/\.pdf$/i, '')) +
        '</span>' +
        '<span class="item-progress">' +
        escapeHtml(progress) +
        '</span></div>';
    }
    html += '</div>';
  }
  dropdown.innerHTML = html;
}

export function triggerUpload() {
  document.getElementById('pdf-dropdown').classList.remove('open');
  document.getElementById('pdf-file-input').click();
}

export async function switchPdf(id) {
  document.getElementById('pdf-dropdown').classList.remove('open');
  currentPdfId = id;
  await openPdf(id);
  showView(currentView);
}

// Splitter (lives in pdf module because it depends on pdfDoc/viewMode/render functions)
function initSplitter() {
  const splitter = document.getElementById('splitter');
  let isDragging = false;

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
    if (pdfDoc) rerenderPreservingPage();
  }

  splitter.addEventListener('mousedown', function (e) {
    e.preventDefault();
    isDragging = true;
    splitter.classList.add('dragging');
    document.body.style.cursor = 'col-resize';
    document.body.style.userSelect = 'none';
    document.addEventListener('mousemove', onSplitterDrag);
    document.addEventListener('mouseup', onSplitterRelease);
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
    if (pdfDoc) rerenderPreservingPage();
  }

  splitter.addEventListener('touchstart', function (e) {
    e.preventDefault();
    isDragging = true;
    splitter.classList.add('dragging');
    document.addEventListener('touchmove', onSplitterTouchDrag, { passive: false });
    document.addEventListener('touchend', onSplitterTouchRelease);
  });
}

export function initPdf() {
  window.addEventListener('pdfjs-ready', () => {});

  document.getElementById('pdf-btn').addEventListener('click', function () {
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

  document.getElementById('pdf-upload-zone').addEventListener('click', function () {
    document.getElementById('pdf-file-input').click();
  });
  uploadZone.addEventListener('dragover', function (e) {
    e.preventDefault();
    uploadZone.classList.add('dragover');
  });
  uploadZone.addEventListener('dragleave', function () {
    uploadZone.classList.remove('dragover');
  });
  uploadZone.addEventListener('drop', function (e) {
    e.preventDefault();
    uploadZone.classList.remove('dragover');
    const files = e.dataTransfer.files;
    if (files.length > 0 && files[0].type === 'application/pdf') {
      uploadPdf(files[0]);
    }
  });
  fileInput.addEventListener('change', function () {
    if (fileInput.files.length > 0) {
      uploadPdf(fileInput.files[0]);
    }
  });

  // Scroll-position page tracking — the single source of truth for the
  // current page in scroll mode. The current page is the one with the greatest
  // visible area in the viewport (robust when a tall page fills the screen and
  // its top has scrolled off). rAF-throttled so the counter tracks the scroll
  // smoothly; the network save stays debounced via savePdfProgress.
  let scrollRaf = null;
  document.getElementById('pdf-viewer')?.addEventListener('scroll', function () {
    if (viewMode !== 'scroll') return;
    if (scrollRaf) return;
    scrollRaf = requestAnimationFrame(() => {
      scrollRaf = null;
      const viewer = document.getElementById('pdf-viewer');
      const viewerRect = viewer.getBoundingClientRect();
      let bestPage = 0;
      let bestVisible = 0;
      for (const canvas of viewer.querySelectorAll('.pdf-page-canvas')) {
        const rect = canvas.getBoundingClientRect();
        const visible =
          Math.min(rect.bottom, viewerRect.bottom) - Math.max(rect.top, viewerRect.top);
        if (visible > bestVisible) {
          bestVisible = visible;
          bestPage = parseInt(canvas.dataset.pageNum);
        }
      }
      if (bestPage && bestPage !== currentPage) {
        currentPage = bestPage;
        updatePageCounter();
        savePdfProgress(currentPage);
      }
    });
  });

  // Toolbar
  document.getElementById('pdf-close-btn')?.addEventListener('click', function () {
    currentPdfId = null;
    pdfDoc = null;
    document.getElementById('pdf-open-state').style.display = 'none';
    document.getElementById('pdf-empty-state').style.display = '';
    loadPdfEmptyState();
    showView('chat');
    document.getElementById('pdf-btn').classList.remove('active');
  });

  document.getElementById('pdf-view-toggle')?.addEventListener('click', function () {
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

  document.getElementById('pdf-zoom-in')?.addEventListener('click', function () {
    applyZoom(currentScale + 0.25);
  });
  document.getElementById('pdf-zoom-out')?.addEventListener('click', function () {
    applyZoom(currentScale - 0.25);
  });

  const pageInput = document.getElementById('pdf-page-input');
  pageInput?.addEventListener('keydown', function (e) {
    if (e.key === 'Enter') {
      e.preventDefault();
      goToPage(this.value);
      this.blur();
    }
  });
  // On focus, select all so a typed number replaces the current page cleanly.
  pageInput?.addEventListener('focus', function () {
    this.select();
  });
  // Leaving the field without Enter reverts to the live page rather than
  // jumping — Enter is the deliberate commit.
  pageInput?.addEventListener('blur', updatePageCounter);

  // Keyboard shortcuts
  document.addEventListener('keydown', function (e) {
    if (!currentPdfId || !pdfDoc) return;
    if (document.activeElement === document.getElementById('message-input')) return;
    if (document.activeElement === document.getElementById('pdf-page-input')) return;

    switch (e.key) {
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
  document.getElementById('pdf-viewer')?.addEventListener('mouseup', function () {
    const selection = window.getSelection();
    const text = selection.toString().trim();
    if (!text) return;
    const prefix =
      '[p.' +
      currentPage +
      '] "' +
      text.substring(0, 120) +
      (text.length > 120 ? '...' : '') +
      '" ';
    const input = document.getElementById('message-input');
    if (input && !input.value.includes(prefix)) {
      input.value = input.value ? prefix + input.value : prefix;
      input.focus();
    }
  });

  // Debounced resize — preserves currentPage across re-layout.
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
      rerenderPreservingPage();
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
    } catch {
      // No last opened PDF, that's fine
    }
  });

  // Filename dropdown
  document.getElementById('pdf-filename')?.addEventListener('click', async function (e) {
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

  document.addEventListener('click', function (e) {
    const dropdown = document.getElementById('pdf-dropdown');
    if (dropdown && dropdown.classList.contains('open')) {
      if (!e.target.closest('#pdf-filename') && !e.target.closest('#pdf-dropdown')) {
        dropdown.classList.remove('open');
      }
    }
  });

  initSplitter();
}
