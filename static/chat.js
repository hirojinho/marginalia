// Chat input, streaming response handler, message rendering.
import { showErrorBanner } from './errorBanner.js';
import { escapeHtml, renderMarkdown, scrollToBottom } from './dom.js';
import {
  getActiveSessionId,
  openSessionModal,
  loadSessions,
  loadActiveSession,
} from './sessions.js';
import {
  createToolPanel,
  createSkillChip,
  appendCompactionNotice,
  updateModelFooter,
} from './chat-events.js';

const MAX_MESSAGE_LEN = 4000;

function autoResizeTextarea(el) {
  el.style.height = 'auto';
  const maxH = 160;
  if (el.scrollHeight > maxH) {
    el.style.height = maxH + 'px';
    el.style.overflowY = 'auto';
  } else {
    el.style.height = el.scrollHeight + 'px';
    el.style.overflowY = 'hidden';
  }
}

export function initChat(chatEndpoint) {
  const input = document.getElementById('message-input');

  input.addEventListener('input', () => autoResizeTextarea(input));

  input.addEventListener('keydown', function (e) {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      document.getElementById('chat-form').dispatchEvent(new Event('submit'));
    }
  });

  document.getElementById('chat-form').addEventListener('submit', async function (e) {
    e.preventDefault();
    const activeSessionId = getActiveSessionId();
    if (!activeSessionId) {
      openSessionModal();
      return;
    }
    const msg = input.value.trim();
    if (!msg) return;
    if (msg.length > MAX_MESSAGE_LEN) {
      showErrorBanner(
        'Message is too long (' + msg.length + '/' + MAX_MESSAGE_LEN + ' characters).',
      );
      return;
    }
    input.value = '';
    input.style.height = 'auto';
    input.style.overflowY = 'hidden';
    const messagesContainer = document.getElementById('messages');
    document.getElementById('send-btn').disabled = true;

    const userDiv = document.createElement('div');
    userDiv.className = 'msg msg-user';
    userDiv.innerHTML =
      '<div class="msg-label">You</div><div class="msg-content">' + escapeHtml(msg) + '</div>';
    messagesContainer.appendChild(userDiv);

    const assistantDiv = document.createElement('div');
    assistantDiv.className = 'msg msg-assistant';
    assistantDiv.innerHTML = '<div class="msg-label">Claw</div><div class="msg-content"></div>';
    messagesContainer.appendChild(assistantDiv);
    let currentAssistantMsg = assistantDiv.querySelector('.msg-content');
    currentAssistantMsg.classList.add('token-cursor');
    let currentSegmentType = null;
    let currentSegmentEl = null;
    let rawAnswer = '';
    let rawThinking = '';
    const activeToolPanels = new Map();

    function ensureAnswerSegment() {
      if (currentSegmentType !== 'answer') {
        const seg = document.createElement('div');
        seg.className = 'answer-segment';
        currentAssistantMsg.appendChild(seg);
        currentSegmentEl = seg;
        currentSegmentType = 'answer';
        rawAnswer = '';
      }
    }

    try {
      const resp = await fetch(chatEndpoint, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_id: activeSessionId, message: msg }),
      });

      if (!resp.ok) throw new Error('HTTP ' + resp.status);

      const reader = resp.body.getReader();
      const decoder = new TextDecoder();
      let buffer = '';
      let eventType = '';

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
            const payload = JSON.parse(data);
            if (eventType === 'reasoning') {
              if (currentSegmentType !== 'reasoning') {
                const details = document.createElement('details');
                details.className = 'thinking-inline';
                details.innerHTML =
                  '<summary>Thinking</summary><div class="thinking-content"></div>';
                currentAssistantMsg.appendChild(details);
                currentSegmentEl = details.querySelector('.thinking-content');
                currentSegmentType = 'reasoning';
                rawThinking = '';
              }
              rawThinking += payload.delta ?? '';
              currentSegmentEl.innerHTML = renderMarkdown(rawThinking);
              scrollToBottom();
            } else if (eventType === 'token') {
              if (currentSegmentType !== 'answer') {
                const seg = document.createElement('div');
                seg.className = 'answer-segment';
                currentAssistantMsg.appendChild(seg);
                currentSegmentEl = seg;
                currentSegmentType = 'answer';
                rawAnswer = '';
              }
              currentAssistantMsg.classList.remove('token-cursor');
              rawAnswer += payload.delta ?? '';
              currentSegmentEl.innerHTML = renderMarkdown(rawAnswer);
              currentAssistantMsg.classList.add('token-cursor');
              scrollToBottom();
            } else if (eventType === 'tool_start') {
              ensureAnswerSegment();
              const panel = createToolPanel(payload.name, payload.input_summary);
              activeToolPanels.set(payload.name, panel);
              currentSegmentEl.appendChild(panel.el);
              scrollToBottom();
            } else if (eventType === 'tool_end') {
              const panel = activeToolPanels.get(payload.name);
              if (panel) {
                panel.complete(payload.output_summary, payload.ok);
                activeToolPanels.delete(payload.name);
              }
              scrollToBottom();
            } else if (eventType === 'skill_start') {
              const chip = createSkillChip(payload.name);
              currentAssistantMsg.insertBefore(chip, currentAssistantMsg.firstChild);
              scrollToBottom();
            } else if (eventType === 'compaction') {
              appendCompactionNotice(currentAssistantMsg, payload.reason);
              scrollToBottom();
            } else if (eventType === 'model_change') {
              updateModelFooter(payload.to);
            } else if (eventType === 'session_topic') {
              loadSessions();
              loadActiveSession();
            } else if (eventType === 'done') {
              if (currentAssistantMsg) currentAssistantMsg.classList.remove('token-cursor');
              currentAssistantMsg = null;
              scrollToBottom();
            } else if (eventType === 'error') {
              if (currentAssistantMsg) currentAssistantMsg.classList.remove('token-cursor');
              if (currentSegmentType === 'answer' && currentSegmentEl) {
                currentSegmentEl.innerHTML = 'Error: ' + escapeHtml(payload.message);
              } else {
                const seg = document.createElement('div');
                seg.className = 'answer-segment';
                seg.innerHTML = 'Error: ' + escapeHtml(payload.message);
                currentAssistantMsg.appendChild(seg);
              }
              currentAssistantMsg = null;
              scrollToBottom();
            }
          }
        }
      }
    } catch (err) {
      if (currentAssistantMsg) {
        currentAssistantMsg.classList.remove('token-cursor');
        if (currentSegmentType === 'answer' && currentSegmentEl) {
          currentSegmentEl.innerHTML = 'Error: ' + escapeHtml(err.message);
        } else {
          const seg = document.createElement('div');
          seg.className = 'answer-segment';
          seg.innerHTML = 'Error: ' + escapeHtml(err.message);
          currentAssistantMsg.appendChild(seg);
        }
      }
    } finally {
      document.getElementById('send-btn').disabled = false;
      input.focus();
    }
  });
}
