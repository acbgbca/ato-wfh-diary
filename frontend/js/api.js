async function fetchJSON(url) {
  const r = await fetch(url);
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

  async saveEntries(userId, entries) {
    const r = await fetch(`/api/users/${userId}/entries`, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(entries),
    });
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
    });
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
    });
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
    });
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
    });
    if (!r.ok) {
      const body = await r.json().catch(() => ({}));
      throw new Error(body.error || `HTTP ${r.status}`);
    }
  },
};

export default api;
