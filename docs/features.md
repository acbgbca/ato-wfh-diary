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
- PDF export is not currently implemented

## Frontend

The frontend is a single-page vanilla HTML/JavaScript application served by the Go backend (embedded in the binary at build time).

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
- A **week navigator** (← Prev / Next →) moves between Monday-anchored weeks; the current week is shown on load
- A **7-row entry grid** (Mon–Sun) shows day type selector and hours input for each day
  - Weekend rows are visually de-emphasised
  - Hours field is enabled only for `wfh` / `part_wfh` day types; automatically disabled and cleared for other types
- **Save Week** submits all 7 rows to the backend; a brief "Saved" confirmation is shown on success

#### Settings

- Accessible via the **Settings** nav link
- **Default WFH hours** input: number of hours pre-applied to `wfh` days on blank weeks
- **Standard week table**: day type selector for each day of the week (Mon–Sun)
- **Save Settings** persists the profile; a brief "Saved" confirmation is shown on success
- On load, the form is populated with the user's current profile (if one exists)

#### Report

- **Financial year selector** defaults to the most recently completed FY; up to 6 years are available
- A **summary block** shows the selected user's name, financial year range, and total WFH hours
- A **detail table** lists every WFH entry (date, type, hours, notes)
- **Export CSV** downloads the report as a CSV file via the backend export endpoint

### E2E Tests

Browser integration tests are written in Go using [Rod](https://go-rod.github.io/) (Chrome DevTools Protocol, no Node.js required). They require Chrome or Chromium to be installed.

Run with:
```
make test-e2e
```

E2E tests use the `e2e` build tag and are excluded from the standard `make test` run.
