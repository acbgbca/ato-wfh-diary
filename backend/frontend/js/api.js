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

  getReport(userId, fy) {
    return fetchJSON(`/api/users/${userId}/report?financial_year=${fy}`);
  },

  exportURL(userId, fy) {
    return `/api/users/${userId}/report/export?financial_year=${fy}&format=csv`;
  },
};

export default api;
