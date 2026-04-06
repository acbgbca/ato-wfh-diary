// Triggered when the reverse-proxy auth session has expired. The browser is
// sent to the current URL as a full page navigation so Traefik / Authelia can
// intercept it, redirect to the login page, and return the user here after
// successful authentication.
function handleAuthExpiry() {
  window.location.replace(window.location.href);
}

// Returns true when r indicates that the reverse proxy redirected the request
// to an external auth page (opaqueredirect) or explicitly denied access
// (401 / 403). Triggers a full-page reload to kick off the auth flow.
function isAuthFailure(r) {
  if (r.type === 'opaqueredirect' || r.status === 401 || r.status === 403) {
    handleAuthExpiry();
    return true;
  }
  return false;
}

// Fetches url with redirect:manual so that a reverse-proxy redirect to an
// external auth page is detected as an opaqueredirect rather than throwing a
// CORS error on iOS Safari.
async function fetchJSON(url) {
  const r = await fetch(url, { redirect: 'manual' });
  if (isAuthFailure(r)) return new Promise(() => {}); // never resolves — page is navigating away
  if (!r.ok) throw new Error(`HTTP ${r.status}`);
  return r.json();
}

const api = {
  getMe() {
    return fetchJSON('/api/me');
  },

  getUsers() {
    return fetchJSON('/api/users');
  },

  getEntries(userId, weekStart) {
    return fetchJSON(`/api/users/${userId}/entries?week_start=${weekStart}`);
  },

  getFirstIncompleteWeek(userId, financialYear, fromDate) {
    let url = `/api/users/${userId}/entries/first-incomplete-week`;
    const params = new URLSearchParams();
    if (financialYear != null) params.set('financial_year', financialYear);
    if (fromDate != null) params.set('from_date', fromDate);
    const qs = params.toString();
    if (qs) url += '?' + qs;
    return fetchJSON(url);
  },

  async saveEntries(userId, entries) {
    const r = await fetch(`/api/users/${userId}/entries`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(entries),
      redirect: 'manual',
    });
    if (isAuthFailure(r)) return;
    if (!r.ok) {
      const body = await r.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${r.status}`);
    }
  },

  getProfile() {
    return fetchJSON('/api/me/profile');
  },

  async saveProfile(data) {
    const r = await fetch('/api/me/profile', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
      redirect: 'manual',
    });
    if (isAuthFailure(r)) return new Promise(() => {});
    if (!r.ok) {
      const body = await r.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${r.status}`);
    }
    return r.json();
  },

  getReport(userId, fy) {
    return fetchJSON(`/api/users/${userId}/report?financial_year=${fy}`);
  },

  exportURL(userId, fy) {
    return `/api/users/${userId}/report/export?financial_year=${fy}&format=csv`;
  },

  getVapidKey() {
    return fetchJSON('/api/notifications/vapid-key');
  },

  getNotificationPrefs() {
    return fetchJSON('/api/notifications/prefs');
  },

  async saveNotificationPrefs(data) {
    const r = await fetch('/api/notifications/prefs', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(data),
      redirect: 'manual',
    });
    if (isAuthFailure(r)) return new Promise(() => {});
    if (!r.ok) {
      const body = await r.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${r.status}`);
    }
    return r.json();
  },

  async subscribeNotifications(subscription) {
    const r = await fetch('/api/notifications/subscribe', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(subscription),
      redirect: 'manual',
    });
    if (isAuthFailure(r)) return;
    if (!r.ok) {
      const body = await r.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${r.status}`);
    }
  },

  async testNotification() {
    const r = await fetch('/api/notifications/test', { method: 'POST', redirect: 'manual' });
    if (isAuthFailure(r)) return;
    if (!r.ok) {
      const body = await r.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${r.status}`);
    }
  },

  async unsubscribeNotifications(endpoint) {
    const r = await fetch('/api/notifications/subscribe', {
      method: 'DELETE',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ endpoint }),
      redirect: 'manual',
    });
    if (isAuthFailure(r)) return;
    if (!r.ok) {
      const body = await r.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${r.status}`);
    }
  },

  async logClientError(report) {
    try {
      // Use sendBeacon for fire-and-forget reliability; this endpoint has no auth.
      const blob = new Blob([JSON.stringify(report)], { type: 'application/json' });
      navigator.sendBeacon('/api/debug/client-error', blob);
    } catch (_) {}
  },
};

export default api;
