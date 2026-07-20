# Features

## Authentication

- Authentication is handled by an external **Forward Auth** proxy sitting in front of the application (e.g. Traefik + Authelia)
- The Go backend reads the authenticated username from the forwarded request header (e.g. `X-Forwarded-User` or `X-Remote-User`)
- On first login, a user record is automatically created from the forwarded username
- No passwords or credentials are stored in the application

### Expired session handling

When the auth session expires the reverse proxy redirects API requests to an external login page (a different origin). All `fetch()` calls use `redirect: 'manual'` so the browser stops at the redirect and returns an `opaqueredirect` response rather than throwing a CORS error on iOS Safari. The app detects this condition (`response.type === 'opaqueredirect'`, or HTTP 401/403) and triggers a full-page navigation to the current URL — Traefik/Authelia intercepts the page request, redirects the user to the login screen, and returns them to the app after successful authentication.

For this redirect to reach the proxy, the service worker must **not** serve the cached app shell in response to the navigation. The fetch handler therefore uses a network-first strategy for navigation requests (`request.mode === 'navigate'`); see [Service Worker caching strategy](#service-worker-caching-strategy). A cache-first navigation would short-circuit the request, swallow the reauthentication redirect, and leave the installed PWA stuck showing the shell with no data.

## User Access

- Two users (a couple) share the application
- Each user logs in with their own account
- Either user can **view and edit the other's entries**, as either may be responsible for completing the family's tax return
- There is no concept of private entries — all entries are visible to both users

### Manual User Creation

Users can be manually created from the UI via the **+** button next to the user selector dropdown. This allows tracking WFH hours for a family member who hasn't logged in yet.

- Clicking the **+** button opens a dialog with username and display name fields
- On submission, `POST /api/users` creates the user and they appear in the dropdown, automatically selected
- If the username already exists, a **409 Conflict** error is shown in the dialog
- When a manually-created user eventually logs in via Forward Auth, `GetOrCreateUser` finds their existing record — no conflict

## Weekly Time Entry

- Time is entered **one week at a time**
- The UI presents a week view showing all 7 days (Monday–Sunday), though most entries will cover weekdays only
- For each day, the user selects a **day type** and enters **hours worked from home** (where applicable)
- Hours support up to **2 decimal places** (e.g. 7.50)
- An entry can be created or updated for any date in any financial year — there is no lock on past data

### Day Types

| Value            | Description                              | WFH Hours Required |
|------------------|------------------------------------------|--------------------|
| `wfh`            | Full day worked from home                | Yes                |
| `part_wfh`       | Part of the day worked from home         | Yes                |
| `office`         | Full day worked from the office          | No (0 hours)       |
| `annual_leave`   | Annual (holiday) leave                   | No (0 hours)       |
| `sick_leave`     | Sick leave                               | No (0 hours)       |
| `public_holiday` | Public holiday                           | No (0 hours)       |
| `weekend`        | Weekend day                              | No (0 hours)       |

Only `wfh` and `part_wfh` entries count toward the ATO WFH claim total.

## User Profile and Defaults

Each user can optionally configure a profile via the **Settings** page. The profile provides default values that are applied when opening a week that has no existing entries.

### Profile Settings

| Setting | Description |
|---------|-------------|
| **Default WFH hours** | Number of hours pre-filled for `wfh` days when a week is empty |
| **Standard week** | A default `day_type` for each day of the week (Mon–Sun) |

### Default Application Rules

- Defaults are applied **only when a week has no existing entries** — saved weeks are never overwritten
- For days defaulted to `wfh`, the hours field is pre-filled with `default_hours`
- For days defaulted to `part_wfh`, no hours are pre-filled (the user must enter hours manually)
- For all other day types, the hours field remains empty/zero
- If a user has not configured a profile, the existing behaviour applies: Mon–Fri defaults to `office`, Sat–Sun defaults to `weekend`

### API

- `GET /api/me/profile` — returns the current user's profile; 404 if not configured
- `PUT /api/me/profile` — creates or updates the current user's profile
- `GET /api/users/{id}/profile` — returns the specified user's profile; 404 if not configured or user doesn't exist
- `PUT /api/users/{id}/profile` — creates or updates the specified user's profile; 404 if user doesn't exist

## Financial Year Reporting

- Users can generate a report for any financial year (Australian FY: **1 July – 30 June**)
- The report defaults to the **most recently completed financial year**
- The report covers a single selected user (the user generating the report, or the other user)

### Report Contents

- **Summary:** total WFH hours for the selected financial year
- **Detail:** a list of every day with a `wfh` or `part_wfh` entry, showing the date, day type, and hours worked

### Export

- Reports can be exported for use in tax preparation
- **CSV export** is supported: includes a summary header block (user, financial year, total hours) followed by a detail table of all WFH entries

### Print PDF (calendar view)

Clicking **Print PDF** opens a browser print dialog pre-loaded with a formatted A4 calendar document. PDF generation is entirely browser-side — no server-side PDF library is used.

**Layout:**
- Report header: user display name, financial year range, and total WFH hours
- 12 monthly sections (July → June), each containing:
  - Month heading with the month's total WFH hours
  - Mon–Sun 7-column calendar grid
  - Each day cell shows: day number, abbreviated day-type label, and (for WFH/part-WFH days) hours worked
- Days outside the current month are left blank
- Days with no recorded entry show only the day number
- WFH/part-WFH cells are lightly highlighted green; weekend cells are muted grey (both render clearly in greyscale)
- `page-break-inside: avoid` on each month section; months flow naturally across approximately 3–4 A4 pages

**Day type abbreviations used in cells:**

| Day type | Label |
|----------|-------|
| `wfh` | WFH |
| `part_wfh` | Part WFH |
| `office` | Office |
| `annual_leave` | Leave |
| `sick_leave` | Sick |
| `public_holiday` | P.Hol |
| `weekend` | Wkd |

**API change:** `GET /api/users/{id}/report` now includes an `all_entries` array alongside the existing `entries` array. `entries` remains WFH/part-WFH only (backwards compatible); `all_entries` contains every entry for the financial year, used to populate the calendar grid.

## Frontend

The frontend is a single-page vanilla HTML/JavaScript application served by the Go backend (embedded in the binary at build time).

### Cache Busting

Static JS and CSS assets are cache-busted using query string versioning:

- The build system injects the **git short SHA** (`BUILD_HASH`) into the binary at build time via `-ldflags="-X main.buildHash=<sha>"`
- `index.html` is served as a Go template; `{{.BuildHash}}` is substituted into asset URLs at request time:
  ```html
  <script type="module" src="/js/app.js?v={{.BuildHash}}"></script>
  <link rel="stylesheet" href="/css/app.css?v={{.BuildHash}}">
  ```
- HTTP cache headers are set per asset type:
  - `index.html`: `Cache-Control: no-cache` — browser always revalidates
  - JS / CSS: `Cache-Control: max-age=31536000, immutable` — cached indefinitely (URL changes with each build)

### Styling

- [Pico.css v2](https://picocss.com/) (classless variant, loaded from CDN) provides base styling for all semantic HTML elements
- A small custom stylesheet (`css/app.css`) handles layout overrides for the entry grid, save bar, and report summary

### Responsive Layout

The UI adapts to screen size:

- **Desktop (≥600px):** standard 5-column table (Day, Date, Type, Hours, Notes) with notes inline
- **Mobile (<600px):** each day is displayed as a 2-row compact layout:
  - Top row: day name and date
  - Bottom row: day type selector and hours input
  - Notes are hidden by default; a **Notes** toggle button expands a notes input below each day

### Progressive Web App (PWA)

The application is installable as a PWA on supported browsers and devices:

- `manifest.json` declares app name, display mode (`standalone`), theme colour, and icon
- `sw.js` is a minimal service worker that caches the app shell (HTML, CSS, JS, manifest, icon) for fast subsequent loads; all `/api/` requests always go to the network
- `sw.js` also handles the push events: `push` (show the notification), `notificationclick` (deep-link into the week), and `pushsubscriptionchange` (re-subscribe — see [Subscription lifecycle](#subscription-lifecycle)). It carries its own copy of `urlBase64ToUint8Array` because a service worker cannot import from `js/app.js`
- An SVG app icon is provided at `icons/icon.svg`
- On iOS, the `apple-touch-icon` link enables "Add to Home Screen" support
- The browser's native install prompt is relied upon (no custom install UI)

#### Service Worker caching strategy

`sw.js` is served as a Go template (like `index.html`) with the build hash injected into the cache name:

```
const CACHE = 'wfh-diary-{{.BuildHash}}';
```

This means every new deployment produces a different SW file (different bytes), causing the browser to detect an update, install the new SW, and delete the old versioned cache. The SW is served with `Cache-Control: no-cache` so the browser always checks for a new version.

At install time the SW pre-caches bare asset URLs (`/js/app.js`, etc.). The fetch handler uses `{ ignoreSearch: true }` when looking up cache entries so that versioned URLs like `/js/app.js?v=abc123` are served from the cache without a round trip to the network.

The fetch handler chooses a strategy per request:

- **`/api/` requests** — always go to the network (the SW does not intercept them).
- **Top-level navigations** (`request.mode === 'navigate'`) — **network-first**: the SW fetches from the network and only falls back to the cached `/` shell if the network is unreachable (offline). This ensures an expired-session redirect to the auth proxy is followed by the browser rather than being swallowed by the cached shell (see [Expired session handling](#expired-session-handling)).
- **Static assets** (CSS, JS, icons, manifest) — **cache-first**: served from the cache when present, falling back to the network on a miss.

### Views

#### Diary (default view)

- A **user selector** at the top allows switching between family members
- A **week navigator** (← Prev / Next →) moves between Monday-anchored weeks
- A **7-row entry grid** (Mon–Sun) shows day type selector and hours input for each day
  - Weekend rows are visually de-emphasised
  - Hours field is enabled only for `wfh` / `part_wfh` day types; automatically disabled and cleared for other types
  - When the day type is changed **to `wfh`**, the hours field is auto-populated with the user's `default_hours` from their profile (if set); changing to `part_wfh` enables the field but leaves it blank
- **Save Week** submits all 7 rows to the backend; a brief "Saved" confirmation is shown on success
- A **week status indicator** is displayed below the date range heading:
  - 🔴 **"Week not submitted"** — fewer than 7 entries exist for the displayed week
  - 🟢 **"Week submitted"** — all 7 entries are present for the displayed week
  - The indicator is updated on every `loadWeek()` call using the entry count returned by the existing `getEntries` API — no additional request is needed
  - On save, the indicator temporarily shows "✓ Saved"; once that confirmation clears (after 3 seconds) it reflects the **newly saved** week. Because Save Week always submits all 7 rows, a saved week is always shown as submitted — no re-fetch is required

#### Week Picker

Tapping the week label (e.g. "12 May – 18 May ▾") opens a **bottom sheet dialog** that allows quick navigation to any week in the current financial year.

**Contents:**
- **Header** with a close button (×)
- **Quick-jump buttons:**
  - **Last week** — navigates to the most recently completed Mon–Sun week
  - **First incomplete** — navigates to the earliest week with fewer than 7 saved entries (uses the existing `first-incomplete-week` API)
- **Scrollable week list** showing every week of the current FY, from the week containing July 1 up to the most recently completed week:
  - Each item displays the week's date range (e.g. "7 Jul – 13 Jul 2025")
  - The FY's **first week is the week containing July 1**, even where that week begins in June (e.g. FY2026 starts Tue 1 Jul 2025, so its first week starts Mon 30 Jun 2025). That week's July days belong to the FY and must be filled in, and Smart Initial Load opens it, so the picker lists it too — the picker and the `first-incomplete-week` API agree on where the FY begins.
  - A completion dot indicates status: green (●) for complete, grey (○) for incomplete. A week is complete when it has an entry for every day of the week that falls **inside the FY** — so the straddling first week needs only its July days (e.g. 6 for FY2026), matching the rule the backend applies.
  - The current week is highlighted with a distinct background
  - Tapping a week item navigates to that week and closes the dialog

**Completion data** is fetched from `GET /api/users/{id}/entries/week-status?financial_year={fy}` when the picker opens. The dialog auto-scrolls to the current week on open.

**Responsive behaviour:**
- **Mobile (<600px):** anchored to the bottom of the viewport with rounded top corners (bottom sheet pattern)
- **Desktop (≥600px):** centred on screen with a max width of 480px and full border radius

The dialog is opened with `showModal()` and closed via the close button, backdrop click, Escape key, or selecting a week.

**Accessibility:**
- The week label trigger has `role="button"`, `tabindex="0"`, and `aria-haspopup="dialog"` — it is keyboard-focusable and responds to Enter/Space to open the picker
- The `<dialog>` has `aria-labelledby` pointing to the "Jump to week" heading
- Week list items are `<button>` elements for native keyboard interaction (Tab navigation, Enter/Space to select)
- Focus management: on open, focus moves to the first interactive element inside the sheet; on close, focus returns to the week label
- The close button is reachable via Tab and works with Enter/Space

**Animations:**
- Mobile: a slide-up CSS animation plays when the sheet opens
- Desktop: a subtle fade-in animation plays when the dialog opens

**Loading and error states:**
- While the week-status API is being fetched, the list area shows a "Loading…" message
- If the API call fails, a "Could not load weeks" error message is shown in the list area; the quick-jump buttons (Last week, First incomplete) remain functional since they do not depend on the week-status API
- If the picker is closed before the API responds, the result is discarded

**Edge cases:**
- If the current FY has just started with only one past week, the list displays correctly
- Immediately after the FY rolls over on 1 July, the new FY may have no completed weeks at all. The list then shows "No completed weeks yet this financial year" rather than rendering empty. The quick-jump buttons remain functional.

#### Smart Initial Load

On app load (without a `?week=` query parameter), instead of always showing the current week, the app navigates to the **oldest week in the current financial year that has fewer than 7 entries saved**. If all weeks up to and including the current week are complete, the app falls back to the current week.

- A "week" is considered complete when all 7 days (Monday–Sunday) have entries saved, regardless of day type
- Weeks are checked from the first Monday on or after July 1 of the current FY up to and including the current week's Monday; future weeks are not checked
- The existing `?week=YYYY-MM-DD` URL query parameter still takes precedence over this logic

#### Saving Stays on the Current Week

Saving never navigates away from the week being edited. After a successful save the app shows a "✓ Saved" confirmation and stays put; the user moves between weeks using the week navigation controls.

#### Settings

- Accessible via the **Settings** nav link
- A heading displays **"Settings for [display name]"** so it's clear whose defaults are being edited
- **Default WFH hours** input: number of hours pre-applied to `wfh` days on blank weeks
- **Standard week table**: day type selector for each day of the week (Mon–Sun)
- **Save Settings** persists the profile for the **currently selected user** (not necessarily the logged-in user); a brief "Saved" confirmation is shown on success
- On load, the form is populated with the selected user's profile (if one exists); if no profile is configured, safe defaults are shown (weekdays = `office`, weekends = `weekend`)
- Changing the user dropdown reloads the settings form with the newly selected user's profile
- **Notifications section** (see Push Notifications below)

#### Report

- **Financial year selector** defaults to the most recently completed FY; up to 6 years are available
- A **summary block** shows the selected user's name, financial year range, and total WFH hours
- **Export CSV** downloads the report as a CSV file via the backend export endpoint
- **Print PDF** opens a print dialog with a formatted A4 calendar view of the full financial year (see Print PDF section above)
- The Export CSV / Print PDF buttons sit directly below the summary block, above the week table
- A **week table** lists every week of the financial year (see below)

##### Week table

A per-entry list of every WFH day was too long to scan and duplicated the CSV/PDF exports. The table instead shows one row per week, so the useful question — which weeks are still to be filled in — is answerable at a glance.

| Column | Description |
|---|---|
| **Week starting** | The Monday of the week. The first row is the week **containing 1 July**, so a FY starting mid-week lists a Monday in June; the last row is the week containing 30 June. |
| **Status** | `Submitted`, `Unsubmitted`, or `Future` |
| **WFH Hours** | Total `wfh` + `part_wfh` hours for the week; shown only for submitted weeks (`—` otherwise) |

Status is derived as:

- **Submitted** — the week has an entry for every one of its days that falls **inside the FY**. The weeks straddling 1 July and 30 June therefore need only their in-FY days, matching the rule used by the week picker and the `first-incomplete-week` API.
- **Future** — not submitted, and the week's Monday is after today. The current week is never `Future`.
- **Unsubmitted** — everything else.

Clicking (or pressing Enter/Space on) a week row opens that week in the **Diary** view for editing. Rows are focusable via the keyboard.

All of this is computed client-side from the `all_entries` array already returned by `GET /api/users/{id}/report` — no additional request is made.

Changing the financial year while a report is still loading leaves two requests in flight, and they can complete out of order. Each load is tagged with a sequence number and a response is discarded if a newer load has started since, so a slow response for the previous year cannot repaint over the year the user selected. The rendered year is exposed as `data-fy` on `#report-tbody`.

## Push Notifications

Users who have installed the app as a PWA can opt in to weekly reminders to fill in their hours.

### Behaviour

- A notification is sent when the user has **fewer than 7 entries** saved for the target week
- If all 7 entries already exist, the notification is silently skipped and the schedule advances to the following week
- Clicking the notification opens the app directly to the relevant week

### Schedule

Each user independently configures:

| Setting | Options | Default |
|---------|---------|---------|
| **Day** | Sunday or Monday | Sunday |
| **Time** | Any HH:MM | 17:00 |

- **Sunday**: notification refers to the **current** Mon–Sun week (same week as Sunday)
- **Monday**: notification refers to the **previous** Mon–Sun week

### Settings UI

The **Notifications** section appears in the Settings view:

- **If the app is running as an installed PWA** (`display-mode: standalone`):
  - Toggle to enable/disable notifications
  - When enabled: day selector (Sunday / Monday) and time input
  - Enabling requests `Notification` permission and creates a Web Push subscription
- **If the app is not installed as a PWA**:
  - A message explains that installation is required
  - An **Install App** button is shown (using the browser's `beforeinstallprompt` event); falls back to a "Add to home screen" message if the prompt is not available

### Subscription lifecycle

A Web Push subscription belongs to a specific browser/PWA install, not to the user. Uninstalling the PWA (or the browser discarding the subscription) invalidates it without informing the server, so the app keeps both ends in sync:

- **Re-registration on startup**: whenever the app starts with notification permission granted and notifications enabled, it re-registers the current subscription with the server (creating a new one if the install no longer has one). A reinstalled PWA therefore registers its new endpoint the first time it is opened.
- **Re-subscription on `pushsubscriptionchange`**: the browser fires this event at the service worker when it invalidates a subscription while the install is still in place (a key rotation, or a long stretch of inactivity). `sw.js` handles it by fetching the VAPID key, subscribing again, `POST`ing the new subscription, and `DELETE`ing `event.oldSubscription`'s endpoint if the browser supplied one. This complements the startup re-registration rather than replacing it — the event is not fired when the PWA is uninstalled, because the service worker goes with it, and Safari's support for it is incomplete. A failure here costs at most one missed notification: the next app start re-registers, and the server prunes the dead endpoint on the next failed send.
- **Pruning of dead subscriptions**: when a push service reports a subscription as gone (HTTP `404` or `410` — Apple returns `410 Unregistered` after the PWA is uninstalled), the subscription is deleted from `push_subscriptions`. It is not treated as a retryable failure.
- **Independent delivery per device**: a device that fails does not stop delivery to the user's other devices. A send is a failure only if no device accepted the notification.

If a test notification finds that every registered subscription is dead, they are all pruned and the user is told to turn notifications off and back on to re-register the device.

### Deep-link on notification click

Notification payloads include a `week_start` date. The service worker handles `notificationclick` and opens `/?week=YYYY-MM-DD`. On load, the app checks for this query parameter and navigates directly to the specified week.

### Scheduler

A background goroutine runs every `NOTIFICATION_SCHEDULER_INTERVAL` (default `10m`):

1. Queries `user_notification_prefs` for rows where `enabled = 1` AND `next_notify_at ≤ now`
2. For each matched user, determines the target week and counts entries
3. If the week is incomplete: sends a Web Push notification to all of the user's subscriptions
   - Success → advances `next_notify_at` by one week
   - Failure → logs the error; `next_notify_at` is left unchanged so the attempt is retried on the next tick. A subscription reported as gone is pruned (see Subscription lifecycle) rather than counted as a failure, so a dead device never blocks the schedule.
4. If the week is complete: advances `next_notify_at` without sending

### Configuration (environment variables)

| Variable | Default | Description |
|---|---|---|
| `NOTIFICATION_TIMEZONE` | `Australia/Melbourne` | IANA timezone used for scheduling |
| `NOTIFICATION_TITLE` | `WFH Diary` | Push notification title |
| `NOTIFICATION_BODY` | `Time to log your hours for this week` | Push notification body |
| `NOTIFICATION_SCHEDULER_INTERVAL` | `10m` | How often the scheduler polls for due notifications |
| `VAPID_SUBJECT` | `mailto:admin@example.com` | VAPID contact identifier (required by the Web Push spec) |

VAPID keys are auto-generated on first run and stored in the `app_config` database table.

## Client-Side Error Logging

A global JavaScript error handler is embedded in `index.html` (before the app module loads). It captures:

- Uncaught JavaScript exceptions (`window.onerror`)
- Unhandled promise rejections (`unhandledrejection` event)
- `<script>` tag load failures (e.g. a module fetch failing)

On any error it POSTs to `POST /api/debug/client-error` using `navigator.sendBeacon` (fire-and-forget, survives page navigation). The payload includes:

| Field | Description |
|-------|-------------|
| `message` | Error message (and source location for `onerror`) |
| `stack` | Stack trace (if available) |
| `url` | Current page URL |
| `platform` | `navigator.platform` / `navigator.userAgentData.platform` |
| `displayMode` | `standalone` (PWA) or `browser` |
| `screenWidth` / `screenHeight` | Device screen dimensions |

The server logs the username (from the forward-auth header, if present), `User-Agent`, and all payload fields. The endpoint does **not** require authentication so it can be reached even when auth is failing.

> **Note:** if the reverse proxy applies authentication to all `/api/` paths, `/api/debug/client-error` may need to be whitelisted to receive unauthenticated error reports.

### API (users)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/users` | Returns all users ordered by display name |
| `POST` | `/api/users` | Creates a new user; returns 201 on success, 409 if username exists, 400 if fields are empty |
| `GET` | `/api/me` | Returns the authenticated user, creating on first login |

### API (entries)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/users/{id}/entries?week_start=YYYY-MM-DD` | Returns entries for the 7-day window starting on `week_start` |
| `POST` | `/api/users/{id}/entries` | Creates or updates a batch of day entries for the user |
| `GET` | `/api/users/{id}/entries/first-incomplete-week` | Returns the Monday of the first week with < 7 entries |
| `GET` | `/api/users/{id}/entries/week-status` | Returns completion counts per week for a financial year |

#### `GET /api/users/{id}/entries/first-incomplete-week`

Query params:
- `financial_year` (optional) — defaults to current FY derived from today's date
- `from_date` (optional, `YYYY-MM-DD` Monday) — start searching from this week; defaults to first Monday ≥ July 1 of the FY

Response:
- `{ "week_start": "YYYY-MM-DD" }` — Monday of the first week with < 7 entries
- `{ "week_start": null }` — all weeks up to the current week are complete

Implementation: fetches all entry dates for the user in the FY, then iterates week-by-week in Go to find the first with fewer than 7 entries.

#### `GET /api/users/{id}/entries/week-status`

Returns the number of entries per week for a user in a financial year, for use in week-picker UIs.

Query params:
- `financial_year` (optional) — defaults to current FY derived from today's date

Response:
- `[{"week_start": "2025-07-07", "count": 7}, {"week_start": "2025-07-14", "count": 3}, ...]`
- Returns only weeks with at least 1 entry, up to and including the current week
- Weeks are ordered by `week_start` ascending
- `count` is 1–7 indicating how many days have entries in that week

### API (notifications)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/notifications/vapid-key` | Returns the VAPID public key for browser subscription |
| `GET` | `/api/notifications/prefs` | Returns the current user's notification preferences |
| `PUT` | `/api/notifications/prefs` | Updates the current user's notification preferences; recalculates `next_notify_at` |
| `POST` | `/api/notifications/subscribe` | Saves or updates a Web Push subscription for the current user |
| `DELETE` | `/api/notifications/subscribe` | Removes a Web Push subscription by endpoint |
| `POST` | `/api/notifications/test` | Sends a test notification to the current user's devices; prunes any subscription the push service reports as gone |

### E2E Tests

Browser integration tests are written in Go using [Rod](https://go-rod.github.io/) (Chrome DevTools Protocol, no Node.js required). They require Chrome or Chromium to be installed.

Run with:
```
make test-e2e
```

E2E tests use the `e2e` build tag and are excluded from the standard `make test` run.

#### Pinned clock

The app's behaviour depends on the current date — which financial year is current, which weeks are in the past, which week to open on. The E2E tests assert on concrete FY2026 dates, so they only hold while "today" sits inside FY2026; left on the real system clock the whole suite would break on 1 July each year, when the Australian financial year rolls over.

Both sides of the app are therefore pinned to a fixed date (`2026-03-24`) for E2E runs:

- **Server:** all date-dependent code reads `internal/clock.Now()` rather than `time.Now()`. The in-process E2E server pins it directly; the Docker E2E container pins it via the `WFH_TEST_TODAY` environment variable (set in `docker-compose.test.yml`), which `main` reads at startup.
- **Browser:** `pinBrowserClock` installs a `Date` shim before page scripts run, so `new Date()` and `Date.now()` report the same pinned date. Only the zero-argument constructor and `Date.now()` are redirected — parsing and date arithmetic behave normally.

The pinned date is defined once as `e2eToday` in `backend/e2e/e2e_test.go` and must be kept in sync with `WFH_TEST_TODAY` in `docker-compose.test.yml`.

`WFH_TEST_TODAY` is a **test-only** switch. It is unset in production; when it is set the server logs a loud startup warning.
