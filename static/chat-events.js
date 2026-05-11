// UI render functions for Pi SSE event types (tool use, skill chips, compaction, model footer)

export function createToolPanel(name, inputSummary) {
  const el = document.createElement('details');
  el.className = 'tool-panel';

  const summary = document.createElement('summary');
  summary.textContent = `⚙ ${name}`;

  const spinner = document.createElement('span');
  spinner.className = 'tool-panel-spinner';
  spinner.textContent = ' …';
  summary.appendChild(spinner);

  const inputDiv = document.createElement('div');
  inputDiv.className = 'tool-input';
  inputDiv.textContent = inputSummary;

  el.appendChild(summary);
  el.appendChild(inputDiv);

  function complete(outputSummary, ok) {
    spinner.remove();

    if (ok === false) {
      el.classList.add('tool-panel--error');
    }

    const outputDiv = document.createElement('div');
    outputDiv.className = 'tool-output';
    outputDiv.textContent = outputSummary;
    el.appendChild(outputDiv);
  }

  return { el, complete };
}

export function createSkillChip(name) {
  const el = document.createElement('div');
  el.className = 'skill-chip';
  el.textContent = `▶ skill: ${name}`;
  return el;
}

export function appendCompactionNotice(container, reason) {
  const el = document.createElement('div');
  el.className = 'compaction-notice';
  el.textContent = '↩ context compacted';
  container.appendChild(el);
}

export function updateModelFooter(modelName) {
  const el = document.getElementById('model-footer');
  if (!el) return;
  el.textContent = modelName;
}
