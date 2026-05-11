// Small DOM helpers shared across modules.
import { marked } from './marked.js';

export function escapeHtml(str) {
  const div = document.createElement('div');
  div.textContent = str;
  return div.innerHTML;
}

export function escapeHtmlAttr(str) {
  return str.replace(/"/g, '&quot;').replace(/'/g, '&#39;');
}

export function scrollToBottom() {
  const msgs = document.getElementById('messages');
  msgs.scrollTop = msgs.scrollHeight;
}

export function renderContent(content) {
  try {
    return marked.parse(content || '');
  } catch {
    return escapeHtml(content || '');
  }
}

export function renderMarkdown(text) {
  try {
    return marked.parse(text || '');
  } catch {
    return escapeHtml(text || '');
  }
}
