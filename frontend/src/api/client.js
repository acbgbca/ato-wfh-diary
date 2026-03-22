// Thin wrapper around fetch for all API calls to the Go backend.
const BASE = '/api';

async function request(method, path, body) {
    const opts = {
        method,
        headers: { 'Content-Type': 'application/json' },
    };
    if (body !== undefined) {
        opts.body = JSON.stringify(body);
    }
    const res = await fetch(`${BASE}${path}`, opts);
    if (!res.ok) {
        throw new Error(`${method} ${path} → ${res.status}`);
    }
    if (res.status === 204) return null;
    return res.json();
}

export const api = {
    // Users
    getUsers: () => request('GET', '/users'),

    // Week entries
    getWeekEntries: (userId, weekStart) =>
        request('GET', `/entries?user_id=${userId}&week_start=${weekStart}`),
    upsertWeekEntries: (entries) =>
        request('POST', '/entries', entries),

    // Reports
    getReport: (userId, financialYear) =>
        request('GET', `/report?user_id=${userId}&financial_year=${financialYear}`),
    exportReport: (userId, financialYear, format) =>
        `${BASE}/report/export?user_id=${userId}&financial_year=${financialYear}&format=${format}`,
};
