// Global error banner: surfaces window errors and unhandled promise rejections.
export function showErrorBanner(msg) {
  let banner = document.getElementById('error-banner');
  if (!banner) {
    banner = document.createElement('div');
    banner.id = 'error-banner';
    banner.innerHTML = '<span class="error-banner-msg"></span>' +
      '<button class="error-banner-reload" type="button">Reload</button>' +
      '<button class="error-banner-close" type="button" aria-label="Dismiss">&times;</button>';
    document.body.appendChild(banner);
    banner.querySelector('.error-banner-reload').addEventListener('click', function () { location.reload(); });
    banner.querySelector('.error-banner-close').addEventListener('click', function () { banner.classList.remove('visible'); });
  }
  banner.querySelector('.error-banner-msg').textContent = msg;
  banner.classList.add('visible');
}

export function installErrorBanner() {
  window.addEventListener('error', function (e) {
    console.error('window.error', e.error || e.message);
    showErrorBanner('Something broke: ' + (e.message || 'unknown error'));
  });
  window.addEventListener('unhandledrejection', function (e) {
    console.error('unhandledrejection', e.reason);
    var reason = e.reason;
    var msg = reason && reason.message ? reason.message : String(reason);
    showErrorBanner('Network or runtime error: ' + msg);
  });
}
