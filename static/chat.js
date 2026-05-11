// Chat input, streaming response handler, message rendering.
import { showErrorBanner } from './errorBanner.js';
import { escapeHtml, renderMarkdown, scrollToBottom } from './dom.js';
import { getActiveSessionId, openSessionModal } from './sessions.js';
import { createToolPanel, createSkillChip, appendCompactionNotice, updateModelFooter } from './chat-events.js';

const MAX_MESSAGE_LEN = 4000;

export function initChat() {
  document.getElementById('chat-form').addEventListener('submit', async function (e) {
    e.preventDefault();
    const activeSessionId = getActiveSessionId();
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
    const messagesContainer = document.getElementById('messages');
    document.getElementById('send-btn').disabled = true;

    const userDiv = document.createElement('div');
    userDiv.className = 'msg msg-user';
    userDiv.innerHTML = '<div class="msg-label">You</div><div class="msg-content">' + escapeHtml(msg) + '</div>';
    messagesContainer.appendChild(userDiv);

    const assistantDiv = document.createElement('div');
    assistantDiv.className = 'msg msg-assistant';
    assistantDiv.innerHTML = '<div class="msg-label">Claw</div><div class="msg-content"><div class="thinking-block" style="display:none;"><details><summary>Thinking</summary><div class="thinking-content"></div></details></div><div class="answer-content"></div></div>';
    messagesContainer.appendChild(assistantDiv);
    let currentAssistantMsg = assistantDiv.querySelector('.msg-content');
    const thinkingBlock = currentAssistantMsg.querySelector('.thinking-block');
    const thinkingContent = currentAssistantMsg.querySelector('.thinking-content');
    const answerContent = currentAssistantMsg.querySelector('.answer-content');
    currentAssistantMsg.classList.add('token-cursor');
    let thinkingActive = false;
    const activeToolPanels = new Map();

    try {
      const resp = await fetch('/chat-v2', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ session_id: activeSessionId, message: msg }),
      });
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
            const payload = JSON.parse(data);
            if (eventType === 'reasoning') {
              if (!thinkingActive) {
                thinkingActive = true;
                thinkingBlock.style.display = 'block';
              }
              const delta = payload.delta ?? '';
              rawThinking += delta;
              thinkingContent.innerHTML = renderMarkdown(rawThinking);
              scrollToBottom();
            } else if (eventType === 'token' && answerContent) {
              currentAssistantMsg.classList.remove('token-cursor');
              const delta = payload.delta ?? '';
              rawAnswer += delta;
              answerContent.innerHTML = renderMarkdown(rawAnswer);
              currentAssistantMsg.classList.add('token-cursor');
              scrollToBottom();
            } else if (eventType === 'tool_start') {
              const panel = createToolPanel(payload.name, payload.input_summary);
              activeToolPanels.set(payload.name, panel);
              answerContent.appendChild(panel.el);
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
              appendCompactionNotice(answerContent, payload.reason);
              scrollToBottom();
            } else if (eventType === 'model_change') {
              updateModelFooter(payload.to);
            } else if (eventType === 'done') {
              if (currentAssistantMsg) currentAssistantMsg.classList.remove('token-cursor');
              currentAssistantMsg = null;
              scrollToBottom();
            } else if (eventType === 'error') {
              if (currentAssistantMsg) currentAssistantMsg.classList.remove('token-cursor');
              answerContent.innerHTML = 'Error: ' + escapeHtml(payload.message);
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
}
