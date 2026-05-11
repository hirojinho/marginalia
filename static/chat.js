// Chat input, streaming response handler, message rendering.
import { showErrorBanner } from './errorBanner.js';
import { escapeHtml, renderMarkdown, scrollToBottom } from './dom.js';
import { getActiveSessionId, openSessionModal } from './sessions.js';

const MAX_MESSAGE_LEN = 4000;

let currentAssistantMsg = null;

export function initChat(chatEndpoint) {
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
      const resp = await fetch(chatEndpoint, { method: 'POST', body: formData });
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
}
