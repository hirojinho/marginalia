// apiFetch — retry + exponential backoff for idempotent GETs.
// Non-GET methods are passed through with a single attempt to avoid
// duplicating writes. Pass { noRetry: true } to opt a GET out of retry.
export async function apiFetch(url, opts) {
  opts = opts || {};
  const method = (opts.method || 'GET').toUpperCase();
  const retriable = !opts.noRetry && method === 'GET';
  const maxAttempts = retriable ? 3 : 1;
  let lastErr;
  for (let attempt = 1; attempt <= maxAttempts; attempt++) {
    try {
      const resp = await fetch(url, opts);
      if (resp.status >= 500 && attempt < maxAttempts) {
        lastErr = new Error('HTTP ' + resp.status);
      } else {
        return resp;
      }
    } catch (err) {
      if (err.name === 'AbortError') throw err;
      lastErr = err;
      if (attempt >= maxAttempts) throw err;
    }
    const delay = 200 * Math.pow(2, attempt - 1) + Math.random() * 100;
    await new Promise(function (r) {
      setTimeout(r, delay);
    });
  }
  throw lastErr;
}
