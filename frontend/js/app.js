import api from './api.js';

// ── Day type config ────────────────────────────────────────────────────────
const DAY_TYPES = [
  { value: 'wfh',            label: 'Work From Home' },
  { value: 'part_wfh',       label: 'Part WFH'       },
  { value: 'office',         label: 'Office'          },
  { value: 'annual_leave',   label: 'Annual Leave'    },
  { value: 'sick_leave',     label: 'Sick Leave'      },
  { value: 'public_holiday', label: 'Public Holiday'  },
  { value: 'weekend',        label: 'Weekend'         },
];

const WFH_TYPES = new Set(['wfh', 'part_wfh']);
const isWFH = t => WFH_TYPES.has(t);

// ── Date utilities ─────────────────────────────────────────────────────────
const formatDate = d => d.toISOString().slice(0, 10);

function getMonday(date) {
  const d = new Date(date);
  d.setHours(0, 0, 0, 0);
  const day = d.getDay();
  d.setDate(d.getDate() + (day === 0 ? -6 : 1 - day));
  return d;
}

const addDays = (d, n) => { const r = new Date(d); r.setDate(r.getDate() + n); return r; };
const financialYear = d => d.getMonth() >= 6 ? d.getFullYear() + 1 : d.getFullYear();
const currentFY    = () => financialYear(new Date());
const defaultFY    = () => currentFY() - 1;

function fmtLabel(dateStr) {
  return new Date(dateStr + 'T00:00:00').toLocaleDateString('en-AU', {
    weekday: 'short', day: 'numeric', month: 'short',
  });
}

function dayTypeLabel(t) {
  return DAY_TYPES.find(d => d.value === t)?.label ?? t;
}

function escapeHTML(s) {
  return String(s)
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;');
}

const on = (id, evt, fn) => document.getElementById(id).addEventListener(evt, fn);

// ── App state ──────────────────────────────────────────────────────────────
let me, allUsers, selectedUserId, weekStart, view, reportFY;

// ── Initialisation ─────────────────────────────────────────────────────────
async function init() {
  try {
    [me, allUsers] = await Promise.all([api.getMe(), api.getUsers()]);
    selectedUserId = me.id;
    weekStart = getMonday(new Date());
    view = 'diary';
    reportFY = defaultFY();

    populateUserSelect();
    populateFYSelect();
    bindEvents();
    showView('diary');
  } catch (e) {
    document.querySelector('main').innerHTML =
      `<p><strong>Error loading application:</strong> ${escapeHTML(e.message)}</p>`;
  }
}

function populateUserSelect() {
  document.getElementById('user-select').innerHTML = allUsers
    .map(u => `<option value="${u.id}"${u.id === me.id ? ' selected' : ''}>${escapeHTML(u.display_name)}</option>`)
    .join('');
}

function populateFYSelect() {
  const fy = currentFY();
  let html = '';
  for (let y = fy; y >= fy - 5; y--) {
    html += `<option value="${y}"${y === reportFY ? ' selected' : ''}>${y - 1}–${y}</option>`;
  }
  document.getElementById('fy-select').innerHTML = html;
}

function bindEvents() {
  on('nav-diary',  'click', e => { e.preventDefault(); showView('diary');  });
  on('nav-report', 'click', e => { e.preventDefault(); showView('report'); });

  on('user-select', 'change', e => {
    selectedUserId = parseInt(e.target.value, 10);
    view === 'diary' ? loadWeek() : loadReport();
  });

  on('prev-week',    'click', () => { weekStart = addDays(weekStart, -7); loadWeek(); });
  on('next-week',    'click', () => { weekStart = addDays(weekStart,  7); loadWeek(); });
  on('save-entries', 'click', saveWeek);

  on('fy-select',  'change', e => { reportFY = parseInt(e.target.value, 10); loadReport(); });
  on('export-csv', 'click',  () => { window.location.href = api.exportURL(selectedUserId, reportFY); });

  // Toggle hours field when day type changes
  document.getElementById('entry-tbody').addEventListener('change', e => {
    if (!e.target.matches('.day-type-select')) return;
    const hoursEl = e.target.closest('tr').querySelector('.hours-input');
    hoursEl.disabled = !isWFH(e.target.value);
    if (hoursEl.disabled) hoursEl.value = '';
  });
}

// ── Views ──────────────────────────────────────────────────────────────────
function showView(v) {
  view = v;
  document.getElementById('view-diary').hidden  = v !== 'diary';
  document.getElementById('view-report').hidden = v !== 'report';
  document.getElementById('nav-diary').setAttribute('aria-current',  v === 'diary'  ? 'page' : 'false');
  document.getElementById('nav-report').setAttribute('aria-current', v === 'report' ? 'page' : 'false');
  v === 'diary' ? loadWeek() : loadReport();
}

// ── Diary ──────────────────────────────────────────────────────────────────
async function loadWeek() {
  const ws = formatDate(weekStart);
  document.getElementById('week-label').textContent =
    `${fmtLabel(ws)} — ${fmtLabel(formatDate(addDays(weekStart, 6)))}`;

  const entries = await api.getEntries(selectedUserId, ws).catch(() => []);
  const byDate  = Object.fromEntries(entries.map(e => [e.entry_date, e]));

  const tbody = document.getElementById('entry-tbody');
  tbody.innerHTML = '';

  ['Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat', 'Sun'].forEach((label, i) => {
    const dateStr = formatDate(addDays(weekStart, i));
    const entry   = byDate[dateStr];
    const defType = i >= 5 ? 'weekend' : 'office';
    const dtype   = entry?.day_type ?? defType;

    const typeOpts = DAY_TYPES
      .map(dt => `<option value="${dt.value}"${dtype === dt.value ? ' selected' : ''}>${dt.label}</option>`)
      .join('');

    const tr = document.createElement('tr');
    if (i >= 5) tr.className = 'weekend-row';
    tr.innerHTML = `
      <td>${label}</td>
      <td>${dateStr}</td>
      <td><select class="day-type-select">${typeOpts}</select></td>
      <td><input type="number" class="hours-input" min="0.01" max="24" step="any"${isWFH(dtype) ? '' : ' disabled'}></td>
      <td><input type="text" class="notes-input" placeholder="Notes"></td>
    `;

    if (isWFH(dtype) && entry?.hours) tr.querySelector('.hours-input').value = entry.hours;
    tr.querySelector('.notes-input').value = entry?.notes ?? '';

    tbody.appendChild(tr);
  });

  clearStatus();
}

async function saveWeek() {
  const entries = [...document.querySelectorAll('#entry-tbody tr')].map(tr => {
    const dtype  = tr.querySelector('.day-type-select').value;
    const hVal   = tr.querySelector('.hours-input').value;
    const notes  = tr.querySelector('.notes-input').value.trim();
    const entry  = {
      entry_date: tr.querySelector('td:nth-child(2)').textContent,
      day_type:   dtype,
      hours:      isWFH(dtype) ? (parseFloat(hVal) || 0) : 0,
    };
    if (notes) entry.notes = notes;
    return entry;
  });

  try {
    await api.saveEntries(selectedUserId, entries);
    setStatus('Saved', false);
    setTimeout(clearStatus, 3000);
  } catch (e) {
    setStatus(e.message, true);
  }
}

function setStatus(msg, isError) {
  const el = document.getElementById('save-status');
  el.textContent = msg;
  el.className = 'save-msg ' + (isError ? 'error' : 'success');
}

function clearStatus() {
  const el = document.getElementById('save-status');
  el.textContent = '';
  el.className = 'save-msg';
}

// ── Report ─────────────────────────────────────────────────────────────────
async function loadReport() {
  try {
    const report = await api.getReport(selectedUserId, reportFY);

    document.getElementById('report-summary').innerHTML =
      `<p><strong>${escapeHTML(report.display_name)}</strong> &nbsp;&middot;&nbsp; ` +
      `FY${reportFY} (${reportFY - 1}&#8211;${reportFY}) &nbsp;&middot;&nbsp; ` +
      `Total WFH: <strong>${(+report.total_hours).toFixed(2)} hours</strong></p>`;

    const rows = report.entries ?? [];
    document.getElementById('report-tbody').innerHTML = rows.length === 0
      ? '<tr><td colspan="4" class="empty-msg">No WFH entries for this financial year.</td></tr>'
      : rows.map(e => `
          <tr>
            <td>${e.entry_date}</td>
            <td>${dayTypeLabel(e.day_type)}</td>
            <td>${(+e.hours).toFixed(2)}</td>
            <td>${escapeHTML(e.notes ?? '')}</td>
          </tr>`).join('');

    document.getElementById('report-total').innerHTML =
      `<strong>${(+report.total_hours).toFixed(2)}</strong>`;
  } catch (e) {
    document.getElementById('report-summary').innerHTML =
      `<p class="error">${escapeHTML(e.message)}</p>`;
  }
}

// ── Boot ───────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', init);
