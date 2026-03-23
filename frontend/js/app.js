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

// Day keys in Mon–Sun order, matching the profile field names and HTML select IDs.
const WEEK_DAYS = [
  { label: 'Mon', key: 'mon_type' },
  { label: 'Tue', key: 'tue_type' },
  { label: 'Wed', key: 'wed_type' },
  { label: 'Thu', key: 'thu_type' },
  { label: 'Fri', key: 'fri_type' },
  { label: 'Sat', key: 'sat_type' },
  { label: 'Sun', key: 'sun_type' },
];

// ── App state ──────────────────────────────────────────────────────────────
let me, allUsers, selectedUserId, weekStart, view, reportFY, userProfile;

// ── Initialisation ─────────────────────────────────────────────────────────
async function init() {
  try {
    [me, allUsers] = await Promise.all([api.getMe(), api.getUsers()]);
    selectedUserId = me.id;
    weekStart = getMonday(new Date());
    view = 'diary';
    reportFY = defaultFY();

    // Load profile in the background; a missing profile (404) is fine.
    userProfile = await api.getProfile().catch(() => null);

    populateUserSelect();
    populateFYSelect();
    populateProfileSelects();
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

function populateProfileSelects() {
  const typeOpts = DAY_TYPES
    .map(dt => `<option value="${dt.value}">${dt.label}</option>`)
    .join('');

  WEEK_DAYS.forEach(({ key }) => {
    const el = document.getElementById(`profile-${key.replace('_type', '-type')}`);
    if (el) el.innerHTML = typeOpts;
  });
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
  on('nav-diary',    'click', e => { e.preventDefault(); showView('diary');    });
  on('nav-report',   'click', e => { e.preventDefault(); showView('report');   });
  on('nav-settings', 'click', e => { e.preventDefault(); showView('settings'); });

  on('user-select', 'change', e => {
    selectedUserId = parseInt(e.target.value, 10);
    view === 'diary' ? loadWeek() : loadReport();
  });

  on('prev-week',    'click', () => { weekStart = addDays(weekStart, -7); loadWeek(); });
  on('next-week',    'click', () => { weekStart = addDays(weekStart,  7); loadWeek(); });
  on('save-entries', 'click', saveWeek);

  on('fy-select',   'change', e => { reportFY = parseInt(e.target.value, 10); loadReport(); });
  on('export-csv',  'click',  () => { window.location.href = api.exportURL(selectedUserId, reportFY); });
  on('save-profile', 'click', saveProfile);

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
  document.getElementById('view-diary').hidden    = v !== 'diary';
  document.getElementById('view-report').hidden   = v !== 'report';
  document.getElementById('view-settings').hidden = v !== 'settings';
  document.getElementById('nav-diary').setAttribute('aria-current',    v === 'diary'    ? 'page' : 'false');
  document.getElementById('nav-report').setAttribute('aria-current',   v === 'report'   ? 'page' : 'false');
  document.getElementById('nav-settings').setAttribute('aria-current', v === 'settings' ? 'page' : 'false');
  if (v === 'diary')    loadWeek();
  if (v === 'report')   loadReport();
  if (v === 'settings') loadSettings();
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

  const weekIsEmpty = entries.length === 0;

  WEEK_DAYS.forEach(({ label, key }, i) => {
    const dateStr   = formatDate(addDays(weekStart, i));
    const entry     = byDate[dateStr];
    const hardDefault = i >= 5 ? 'weekend' : 'office';
    const profileDefault = weekIsEmpty && userProfile ? userProfile[key] : null;
    const dtype   = entry?.day_type ?? profileDefault ?? hardDefault;
    const notes   = entry?.notes ?? '';

    const typeOpts = DAY_TYPES
      .map(dt => `<option value="${dt.value}"${dtype === dt.value ? ' selected' : ''}>${dt.label}</option>`)
      .join('');

    // Main row: Day | Date | Type | Hours | (desktop: notes input; mobile: notes toggle)
    const tr = document.createElement('tr');
    tr.className = 'day-row' + (i >= 5 ? ' weekend-row' : '');
    tr.dataset.date = dateStr;
    tr.innerHTML = `
      <td>${label}</td>
      <td>${dateStr}</td>
      <td><select class="day-type-select">${typeOpts}</select></td>
      <td><input type="number" class="hours-input" min="0.01" max="24" step="any"${isWFH(dtype) ? '' : ' disabled'}></td>
      <td class="cell-notes"><input type="text" class="notes-input" placeholder="Notes"></td>
    `;

    if (isWFH(dtype)) {
      if (entry?.hours) {
        tr.querySelector('.hours-input').value = entry.hours;
      } else if (weekIsEmpty && userProfile && dtype === 'wfh') {
        tr.querySelector('.hours-input').value = userProfile.default_hours;
      }
    }
    tr.querySelector('.notes-input').value = notes;

    // Notes expand row (mobile only — hidden by default, toggled per day)
    const notesRow = document.createElement('tr');
    notesRow.className = 'day-notes-row';
    notesRow.hidden = true;
    notesRow.innerHTML = `<td colspan="5"><input type="text" class="notes-input notes-mobile" placeholder="Notes"></td>`;
    notesRow.querySelector('.notes-mobile').value = notes;

    // Notes toggle button — injected into the 5th cell on mobile (CSS reveals it)
    const toggleBtn = document.createElement('button');
    toggleBtn.type = 'button';
    toggleBtn.className = 'notes-toggle';
    toggleBtn.textContent = notes ? 'Notes \u2022' : 'Notes';
    toggleBtn.setAttribute('aria-label', 'Toggle notes');
    toggleBtn.addEventListener('click', () => {
      notesRow.hidden = !notesRow.hidden;
    });

    // Wrap toggle in its own cell so CSS grid-area works
    const toggleCell = document.createElement('td');
    toggleCell.className = 'notes-toggle-cell';
    toggleCell.appendChild(toggleBtn);
    tr.appendChild(toggleCell);

    tbody.appendChild(tr);
    tbody.appendChild(notesRow);
  });

  clearStatus();
}

async function saveWeek() {
  const entries = [...document.querySelectorAll('#entry-tbody tr.day-row')].map(tr => {
    const dtype = tr.querySelector('.day-type-select').value;
    const hVal  = tr.querySelector('.hours-input').value;

    // On mobile the notes-expand row holds the live input; on desktop use the inline cell.
    const isMobile     = window.innerWidth < 600;
    const notesRow     = tr.nextElementSibling; // always the .day-notes-row
    const notesInput   = isMobile
      ? notesRow?.querySelector('.notes-mobile')
      : tr.querySelector('.cell-notes .notes-input');
    const notes = notesInput?.value.trim() ?? '';

    const entry = {
      entry_date: tr.dataset.date,
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

// ── Settings ───────────────────────────────────────────────────────────────
function loadSettings() {
  if (!userProfile) {
    document.getElementById('profile-sat-type').value = 'weekend';
    document.getElementById('profile-sun-type').value = 'weekend';
    return;
  }
  document.getElementById('profile-default-hours').value = userProfile.default_hours;
  WEEK_DAYS.forEach(({ key }) => {
    const el = document.getElementById(`profile-${key.replace('_type', '-type')}`);
    if (el) el.value = userProfile[key];
  });
}

async function saveProfile() {
  const hoursVal = parseFloat(document.getElementById('profile-default-hours').value);
  if (!hoursVal || hoursVal <= 0 || hoursVal > 24) {
    setProfileStatus('Default hours must be between 0 and 24', true);
    return;
  }

  const data = { default_hours: hoursVal };
  WEEK_DAYS.forEach(({ key }) => {
    const el = document.getElementById(`profile-${key.replace('_type', '-type')}`);
    data[key] = el ? el.value : 'office';
  });

  try {
    userProfile = await api.saveProfile(data);
    setProfileStatus('Saved', false);
    setTimeout(clearProfileStatus, 3000);
  } catch (e) {
    setProfileStatus(e.message, true);
  }
}

function setProfileStatus(msg, isError) {
  const el = document.getElementById('profile-status');
  el.textContent = msg;
  el.className = 'save-msg ' + (isError ? 'error' : 'success');
}

function clearProfileStatus() {
  const el = document.getElementById('profile-status');
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

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => navigator.serviceWorker.register('/sw.js'));
}
