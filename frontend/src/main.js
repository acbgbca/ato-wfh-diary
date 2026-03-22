// Entry point — bootstraps the application.
// Replace this with a framework initialisation (React/Vue) when that decision is made.

import { renderWeekEntry } from './pages/weekEntry.js';

const app = document.getElementById('app');

// TODO: read current user from a /api/me endpoint
// TODO: implement client-side routing (week entry / report pages)

renderWeekEntry(app);
