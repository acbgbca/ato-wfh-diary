# Features

## Authentication

- Authentication is handled by an external **Forward Auth** proxy sitting in front of the application
- The Go backend reads the authenticated username from the forwarded request header (e.g. `X-Forwarded-User` or `X-Remote-User`)
- On first login, a user record is automatically created from the forwarded username
- No passwords or credentials are stored in the application

## User Access

- Two users (a couple) share the application
- Each user logs in with their own account
- Either user can **view and edit the other's entries**, as either may be responsible for completing the family's tax return
- There is no concept of private entries — all entries are visible to both users

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
- An SVG app icon is provided at `icons/icon.svg`
- On iOS, the `apple-touch-icon` link enables "Add to Home Screen" support
- The browser's native install prompt is relied upon (no custom install UI)

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

#### Smart Initial Load

On app load (without a `?week=` query parameter), instead of always showing the current week, the app navigates to the **oldest week in the current financial year that has fewer than 7 entries saved**. If all weeks up to and including the current week are complete, the app falls back to the current week.

- A "week" is considered complete when all 7 days (Monday–Sunday) have entries saved, regardless of day type
- Weeks are checked from the first Monday on or after July 1 of the current FY up to and including the current week's Monday; future weeks are not checked
- The existing `?week=YYYY-MM-DD` URL query parameter still takes precedence over this logic

#### Auto-Advance After Saving a Past Week

After successfully saving a week that is before the current week, the app automatically advances to the **next incomplete week** (i.e. calls the API with `from_date = savedWeek + 7 days`). If no incomplete week is found from that point, it navigates to the current week. Saving the current week retains existing behaviour (stay on the current week).

#### Settings

- Accessible via the **Settings** nav link
- **Default WFH hours** input: number of hours pre-applied to `wfh` days on blank weeks
- **Standard week table**: day type selector for each day of the week (Mon–Sun)
- **Save Settings** persists the profile; a brief "Saved" confirmation is shown on success
- On load, the form is populated with the user's current profile (if one exists)
- **Notifications section** (see Push Notifications below)

#### Report

- **Financial year selector** defaults to the most recently completed FY; up to 6 years are available
- A **summary block** shows the selected user's name, financial year range, and total WFH hours
- A **detail table** lists every WFH entry (date, type, hours, notes)
- **Export CSV** downloads the report as a CSV file via the backend export endpoint
- **Print PDF** opens a print dialog with a formatted A4 calendar view of the full financial year (see Print PDF section above)

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

### Deep-link on notification click

Notification payloads include a `week_start` date. The service worker handles `notificationclick` and opens `/?week=YYYY-MM-DD`. On load, the app checks for this query parameter and navigates directly to the specified week.

### Scheduler

A background goroutine runs every `NOTIFICATION_SCHEDULER_INTERVAL` (default `10m`):

1. Queries `user_notification_prefs` for rows where `enabled = 1` AND `next_notify_at ≤ now`
2. For each matched user, determines the target week and counts entries
3. If the week is incomplete: sends a Web Push notification to all of the user's subscriptions
   - Success → advances `next_notify_at` by one week
   - Failure → logs the error; `next_notify_at` is left unchanged so the attempt is retried on the next tick
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

### API (entries)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/users/{id}/entries?week_start=YYYY-MM-DD` | Returns entries for the 7-day window starting on `week_start` |
| `POST` | `/api/users/{id}/entries` | Creates or updates a batch of day entries for the user |
| `GET` | `/api/users/{id}/entries/first-incomplete-week` | Returns the Monday of the first week with < 7 entries |

#### `GET /api/users/{id}/entries/first-incomplete-week`

Query params:
- `financial_year` (optional) — defaults to current FY derived from today's date
- `from_date` (optional, `YYYY-MM-DD` Monday) — start searching from this week; defaults to first Monday ≥ July 1 of the FY

Response:
- `{ "week_start": "YYYY-MM-DD" }` — Monday of the first week with < 7 entries
- `{ "week_start": null }` — all weeks up to the current week are complete

Implementation: fetches all entry dates for the user in the FY, then iterates week-by-week in Go to find the first with fewer than 7 entries.

### API (notifications)

| Method | Path | Description |
|---|---|---|
| `GET` | `/api/notifications/vapid-key` | Returns the VAPID public key for browser subscription |
| `GET` | `/api/notifications/prefs` | Returns the current user's notification preferences |
| `PUT` | `/api/notifications/prefs` | Updates the current user's notification preferences; recalculates `next_notify_at` |
| `POST` | `/api/notifications/subscribe` | Saves or updates a Web Push subscription for the current user |
| `DELETE` | `/api/notifications/subscribe` | Removes a Web Push subscription by endpoint |

### E2E Tests

Browser integration tests are written in Go using [Rod](https://go-rod.github.io/) (Chrome DevTools Protocol, no Node.js required). They require Chrome or Chromium to be installed.

Run with:
```
make test-e2e
```

E2E tests use the `e2e` build tag and are excluded from the standard `make test` run.
