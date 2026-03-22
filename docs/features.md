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

## Financial Year Reporting

- Users can generate a report for any financial year (Australian FY: **1 July – 30 June**)
- The report defaults to the **most recently completed financial year**
- The report covers a single selected user (the user generating the report, or the other user)

### Report Contents

- **Summary:** total WFH hours for the selected financial year
- **Detail:** a list of every day with a `wfh` or `part_wfh` entry, showing the date, day type, and hours worked

### Export

- Reports can be exported for use in tax preparation
- Export format: TBD (CSV and/or PDF)
