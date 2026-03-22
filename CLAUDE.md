# ATO WFH Diary — Claude Reference

## Application Summary

A personal web application for an Australian family (two users) to track work-from-home hours for ATO tax purposes. The Australian financial year runs **1 July – 30 June**.

Built with:
- **Backend:** Go microservice (REST API)
- **Frontend:** HTML/JavaScript
- **Database:** SQLite

## Key Design Decisions

- Authentication via **Forward Auth** — no passwords stored; username is read from the forwarded request header and a user record is auto-created on first login
- Both users can **view and edit each other's entries** (shared family access, no row-level restrictions)
- Time is entered **one week at a time** via the UI, stored at **daily granularity**
- Hours support **up to 2 decimal places** (e.g. 7.50)
- `financial_year` is a **generated/stored column** on `work_day_entries`, derived from `entry_date` (months Jul–Dec → year+1, months Jan–Jun → year)
- Reports are for a single financial year (default: last completed FY) and are **exportable**

## Day Types

| Value            | Counts toward WFH claim |
|------------------|------------------------|
| `wfh`            | Yes                    |
| `part_wfh`       | Yes                    |
| `office`         | No                     |
| `annual_leave`   | No                     |
| `sick_leave`     | No                     |
| `public_holiday` | No                     |
| `weekend`        | No                     |

## Project Structure

```
docs/               # Project documentation (keep up to date)
  data_model.md     # Database schema and entity descriptions
  features.md       # Feature specifications
CLAUDE.md           # This file
```

## Documentation Policy

**When making any change to the application — schema, features, behaviour, or architecture — update the relevant file(s) under `docs/` as part of the same task.** Do not leave documentation out of sync with the implementation. Specifically:

- Schema changes → update `docs/data_model.md`
- Feature additions or changes → update `docs/features.md`
- New documentation topics → create a new file under `docs/` and add it to the list above
