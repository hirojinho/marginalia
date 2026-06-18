// Shim that re-exports the global `marked` exposed by /static/marked.min.js
// (loaded as a classic <script> tag in index.html).
export const marked = window.marked;
