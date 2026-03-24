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
const formatDate = d => {
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  return `${y}-${m}-${day}`;
};

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
let me, allUsers, selectedUserId, weekStart, view, reportFY, userProfile, currentReport;
let weekEntryCount = 0;   // track last-loaded entry count for restoring week-status
let statusTimeout  = null; // pending clearStatus timer

// PWA install prompt captured from the beforeinstallprompt event.
let installPrompt = null;
window.addEventListener('beforeinstallprompt', e => {
  e.preventDefault();
  installPrompt = e;
  // If settings view is already showing, reveal the install button.
  const btn = document.getElementById('notif-install-btn');
  if (btn) btn.hidden = false;
});

// ── Initialisation ─────────────────────────────────────────────────────────
async function init() {
  try {
    [me, allUsers] = await Promise.all([api.getMe(), api.getUsers()]);
    selectedUserId = me.id;

    view = 'diary';
    reportFY = defaultFY();

    // Load profile in the background; a missing profile (404) is fine.
    userProfile = await api.getProfile().catch(() => null);

    populateUserSelect();
    populateFYSelect();
    populateProfileSelects();
    bindEvents();

    // Navigate to a specific week if ?week= is set (e.g. from a notification click).
    const weekParam = new URLSearchParams(window.location.search).get('week');
    if (weekParam) {
      weekStart = getMonday(new Date(weekParam + 'T00:00:00'));
    } else {
      // Smart initial load: navigate to oldest incomplete week in current FY,
      // falling back to the current week if all are complete.
      weekStart = await resolveInitialWeek();
    }

    showView('diary');
  } catch (e) {
    document.querySelector('main').innerHTML =
      `<p><strong>Error loading application:</strong> ${escapeHTML(e.message)}</p>`;
  }
}

// resolveInitialWeek returns the Monday of the first incomplete week in the
// current FY, or the current week's Monday if all weeks are complete.
async function resolveInitialWeek() {
  try {
    const fy = currentFY();
    const data = await api.getFirstIncompleteWeek(me.id, fy, null);
    if (data.week_start) {
      return getMonday(new Date(data.week_start + 'T00:00:00'));
    }
  } catch (_) {
    // On error fall back to current week
  }
  return getMonday(new Date());
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
  on('print-pdf',   'click',  printReport);
  on('save-profile', 'click', saveProfile);

  on('notif-enabled', 'change', e => {
    document.getElementById('notif-schedule').hidden = !e.target.checked;
  });
  on('save-notif', 'click', saveNotificationPrefs);
  on('notif-install-btn', 'click', async () => {
    if (!installPrompt) return;
    installPrompt.prompt();
    await installPrompt.userChoice;
    installPrompt = null;
  });

  // Toggle hours field when day type changes
  document.getElementById('entry-tbody').addEventListener('change', e => {
    if (!e.target.matches('.day-type-select')) return;
    const hoursEl = e.target.closest('tr').querySelector('.hours-input');
    hoursEl.disabled = !isWFH(e.target.value);
    if (hoursEl.disabled) {
      hoursEl.value = '';
    } else if (e.target.value === 'wfh' && userProfile?.default_hours) {
      hoursEl.value = userProfile.default_hours;
    }
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
async function loadWeek({ keepStatus = false } = {}) {
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

  weekEntryCount = entries.length;

  // Animate the week label so the week change is obvious
  const labelEl = document.getElementById('week-label');
  labelEl.classList.remove('week-label-flash');
  void labelEl.offsetWidth; // force reflow to restart animation
  labelEl.classList.add('week-label-flash');

  if (!keepStatus) clearStatus();
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

  const savedWeekStart = weekStart;

  try {
    await api.saveEntries(selectedUserId, entries);

    // Auto-advance: if the saved week is before the current week, navigate
    // to the next incomplete week (or fall back to the current week).
    const currentMonday = getMonday(new Date());
    if (savedWeekStart < currentMonday) {
      const nextFromDate = formatDate(addDays(savedWeekStart, 7));
      const fy = currentFY();
      const data = await api.getFirstIncompleteWeek(selectedUserId, fy, nextFromDate).catch(() => null);
      if (data?.week_start) {
        weekStart = getMonday(new Date(data.week_start + 'T00:00:00'));
      } else {
        weekStart = currentMonday;
      }
      await loadWeek({ keepStatus: true });
    }

    // Scroll to top so the user sees the week heading and the Saved confirmation.
    window.scrollTo({ top: 0, behavior: 'smooth' });

    // Show "Saved" in the week-status bar (top of diary section) and in the
    // save-bar span so existing tests continue to pass.
    setStatus('Saved', false);
    document.getElementById('week-status').textContent = '✓ Saved';
    document.getElementById('week-status').className = 'week-status success';

    if (statusTimeout) clearTimeout(statusTimeout);
    statusTimeout = setTimeout(clearStatus, 3000);
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
  // Restore the week-status indicator to submitted / not-submitted.
  renderWeekStatus(weekEntryCount);
}

function renderWeekStatus(count) {
  const el = document.getElementById('week-status');
  if (count >= 7) {
    el.textContent = '🟢 Week submitted';
    el.className = 'week-status submitted';
  } else {
    el.textContent = '🔴 Week not submitted';
    el.className = 'week-status not-submitted';
  }
}

// ── Settings ───────────────────────────────────────────────────────────────
function loadSettings() {
  if (!userProfile) {
    document.getElementById('profile-sat-type').value = 'weekend';
    document.getElementById('profile-sun-type').value = 'weekend';
  } else {
    document.getElementById('profile-default-hours').value = userProfile.default_hours;
    WEEK_DAYS.forEach(({ key }) => {
      const el = document.getElementById(`profile-${key.replace('_type', '-type')}`);
      if (el) el.value = userProfile[key];
    });
  }
  loadNotificationSettings();
}

async function loadNotificationSettings() {
  const isStandalone = window.matchMedia('(display-mode: standalone)').matches;

  if (!isStandalone) {
    document.getElementById('notif-install-prompt').hidden = false;
    document.getElementById('notif-pwa-section').hidden = true;
    // Show install button only if a prompt is available.
    document.getElementById('notif-install-btn').hidden = !installPrompt;
    return;
  }

  document.getElementById('notif-install-prompt').hidden = true;
  document.getElementById('notif-pwa-section').hidden = false;

  try {
    const prefs = await api.getNotificationPrefs();
    document.getElementById('notif-enabled').checked = prefs.enabled;
    document.getElementById('notif-schedule').hidden = !prefs.enabled;
    document.getElementById('notif-day').value = String(prefs.notify_day);
    document.getElementById('notif-time').value = prefs.notify_time;
  } catch (e) {
    setNotifStatus('Could not load notification settings', true);
  }
}

async function saveNotificationPrefs() {
  const enabled = document.getElementById('notif-enabled').checked;
  const notifyDay = parseInt(document.getElementById('notif-day').value, 10);
  const notifyTime = document.getElementById('notif-time').value;

  try {
    if (enabled) {
      // Request permission and subscribe if not already subscribed.
      const permission = await Notification.requestPermission();
      if (permission !== 'granted') {
        setNotifStatus('Notification permission denied', true);
        return;
      }
      await ensurePushSubscription();
    }

    await api.saveNotificationPrefs({ enabled, notify_day: notifyDay, notify_time: notifyTime });
    setNotifStatus('Saved', false);
    setTimeout(clearNotifStatus, 3000);
  } catch (e) {
    setNotifStatus(e.message, true);
  }
}

// Ensures a Web Push subscription exists for this browser and is registered
// with the server. Does nothing if already subscribed.
async function ensurePushSubscription() {
  const reg = await navigator.serviceWorker.ready;
  let sub = await reg.pushManager.getSubscription();
  if (!sub) {
    const { vapid_public_key: vapidKey } = await api.getVapidKey();
    sub = await reg.pushManager.subscribe({
      userVisibleOnly: true,
      applicationServerKey: urlBase64ToUint8Array(vapidKey),
    });
  }
  const json = sub.toJSON();
  await api.subscribeNotifications({
    endpoint:   json.endpoint,
    p256dh_key: json.keys.p256dh,
    auth_key:   json.keys.auth,
  });
}

// Converts a URL-safe base64 string to a Uint8Array for the Push API.
function urlBase64ToUint8Array(base64String) {
  const padding = '='.repeat((4 - (base64String.length % 4)) % 4);
  const base64 = (base64String + padding).replace(/-/g, '+').replace(/_/g, '/');
  const raw = atob(base64);
  return Uint8Array.from([...raw].map(c => c.charCodeAt(0)));
}

function setNotifStatus(msg, isError) {
  const el = document.getElementById('notif-status');
  el.textContent = msg;
  el.className = 'save-msg ' + (isError ? 'error' : 'success');
}

function clearNotifStatus() {
  const el = document.getElementById('notif-status');
  el.textContent = '';
  el.className = 'save-msg';
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
    currentReport = await api.getReport(selectedUserId, reportFY);
    const report = currentReport;

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

// ── PDF / Print ────────────────────────────────────────────────────────────

// Abbreviated labels used in calendar cells.
const CELL_LABELS = {
  wfh:            'WFH',
  part_wfh:       'Part WFH',
  office:         'Office',
  annual_leave:   'Leave',
  sick_leave:     'Sick',
  public_holiday: 'P.Hol',
  weekend:        'Wkd',
};

function generateReportHTML(report) {
  const fy   = report.financial_year;
  const name = report.display_name;
  const totalHrs = (+report.total_hours).toFixed(2);

  // Build a lookup map: 'YYYY-MM-DD' → entry
  const byDate = {};
  (report.all_entries ?? []).forEach(e => { byDate[e.entry_date] = e; });

  // Financial year months: Jul (fy-1) through Jun (fy)
  const months = [];
  for (let m = 0; m < 12; m++) {
    const year  = m < 6 ? fy - 1 : fy;      // Jul–Dec = fy-1, Jan–Jun = fy
    const month = m < 6 ? 6 + m : m - 6;    // 0-based month index
    months.push({ year, month });
  }

  const MONTH_NAMES = ['January','February','March','April','May','June',
                       'July','August','September','October','November','December'];

  function buildMonthGrid(year, month) {
    // Total WFH hours this month
    let monthTotal = 0;
    const firstDay = new Date(year, month, 1);
    const lastDay  = new Date(year, month + 1, 0).getDate();

    // day-of-week of 1st (0=Sun…6=Sat), convert to Mon-origin (0=Mon…6=Sun)
    const startDow = (firstDay.getDay() + 6) % 7;

    // Build cells
    const cells = [];
    // Leading empty cells
    for (let i = 0; i < startDow; i++) cells.push(null);

    for (let d = 1; d <= lastDay; d++) {
      const dateStr = `${year}-${String(month + 1).padStart(2,'0')}-${String(d).padStart(2,'0')}`;
      const entry   = byDate[dateStr] ?? null;
      if (entry && (entry.day_type === 'wfh' || entry.day_type === 'part_wfh')) {
        monthTotal += +entry.hours;
      }
      cells.push({ d, entry });
    }

    // Pad to complete last row
    while (cells.length % 7 !== 0) cells.push(null);

    // Build rows
    let rows = '';
    for (let r = 0; r < cells.length; r += 7) {
      const week = cells.slice(r, r + 7);
      rows += '<tr>' + week.map(cell => {
        if (!cell) return '<td class="cal-empty"></td>';
        const { d, entry } = cell;
        const dt   = entry?.day_type ?? '';
        const hrs  = entry && (dt === 'wfh' || dt === 'part_wfh') ? (+entry.hours).toFixed(2) : '';
        const lbl  = dt ? CELL_LABELS[dt] ?? dt : '';
        const cls  = dt === 'wfh' || dt === 'part_wfh' ? 'cal-wfh'
                   : dt === 'weekend'                   ? 'cal-wkd'
                   : '';
        const typeHrs = lbl && hrs ? `${escapeHTML(lbl)} - ${hrs}` : lbl ? escapeHTML(lbl) : '';
        return `<td class="cal-day ${cls}">
          <span class="cal-num">${d}</span>
          ${typeHrs ? `<span class="cal-type">${typeHrs}</span>` : ''}
        </td>`;
      }).join('') + '</tr>';
    }

    return { rows, monthTotal };
  }

  let monthSections = '';
  months.forEach(({ year, month }) => {
    const { rows, monthTotal } = buildMonthGrid(year, month);
    monthSections += `
      <div class="month-block">
        <div class="month-header">
          <span class="month-name">${MONTH_NAMES[month]} ${year}</span>
          <span class="month-total">Total: ${monthTotal.toFixed(2)} hrs</span>
        </div>
        <table class="cal-table">
          <thead><tr>
            <th>Mon</th><th>Tue</th><th>Wed</th><th>Thu</th><th>Fri</th><th>Sat</th><th>Sun</th>
          </tr></thead>
          <tbody>${rows}</tbody>
        </table>
      </div>`;
  });

  return `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="UTF-8">
<title>WFH Report — ${escapeHTML(name)} — FY${fy}</title>
<style>
  body { font-family: Arial, sans-serif; font-size: 10px; margin: 10mm; color: #000; }
  h1   { font-size: 14px; margin: 0 0 2px; }
  .subtitle { font-size: 11px; margin: 0 0 6px; color: #444; }
  .month-block { page-break-inside: avoid; margin-bottom: 8mm; }
  .month-header { display: flex; justify-content: space-between; align-items: baseline;
                  border-bottom: 1px solid #333; margin-bottom: 2px; padding-bottom: 2px; }
  .month-name  { font-weight: bold; font-size: 11px; }
  .month-total { font-size: 10px; }
  .cal-table   { width: 100%; border-collapse: collapse; table-layout: fixed; }
  .cal-table th { background: #eee; border: 1px solid #ccc; text-align: center;
                  padding: 2px; font-size: 9px; }
  .cal-table td { border: 1px solid #ddd; vertical-align: top; height: 7mm;
                  padding: 2px; width: 14.28%; }
  .cal-empty   { background: #f9f9f9; }
  .cal-wfh     { background: #e6f4ea; }
  .cal-wkd     { background: #f5f5f5; color: #888; }
  .cal-num     { display: block; font-weight: bold; font-size: 9px; }
  .cal-type    { display: block; font-size: 8px; }
  @media print { @page { size: A4 portrait; margin: 10mm; } body { margin: 0; } }
</style>
</head>
<body>
<h1>WFH Report — ${escapeHTML(name)}</h1>
<p class="subtitle">FY${fy} (1 Jul ${fy - 1} – 30 Jun ${fy}) &nbsp;·&nbsp; Total WFH hours: <strong>${totalHrs}</strong></p>
${monthSections}
</body>
</html>`;
}

function printReport() {
  if (!currentReport) return;
  const html = generateReportHTML(currentReport);
  const win  = window.open('', '_blank');
  if (!win) return;
  win.document.write(html);
  win.document.close();
  win.focus();
  win.print();
}

// ── Boot ───────────────────────────────────────────────────────────────────
document.addEventListener('DOMContentLoaded', init);

if ('serviceWorker' in navigator) {
  window.addEventListener('load', () => navigator.serviceWorker.register('/sw.js'));
}
