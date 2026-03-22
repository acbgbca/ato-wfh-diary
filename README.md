# ATO WFH Diary

A personal web application for tracking work-from-home hours for Australian Tax Office (ATO) expense claims. Built for a family of two, allowing either person to log and view WFH time across the financial year.

The Australian financial year runs **1 July – 30 June**. At tax time, the application generates a report of total WFH hours with a full day-by-day breakdown, ready for use in a tax return.

## Features

- **Weekly time entry** — enter hours one week at a time, with a day type for each day (WFH, part-day WFH, office, leave, public holiday, etc.)
- **Shared access** — both users can view and edit each other's entries, since either may complete the family tax return
- **Financial year reports** — generate a WFH summary and detail report for any financial year, defaulting to the most recently completed year
- **Export** — download reports for use in tax preparation

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go (microservice, REST API) |
| Frontend | HTML / JavaScript |
| Database | SQLite |
| Auth | Forward Auth (external auth proxy) |

## Project Structure

```
backend/            Go microservice
  cmd/server/       Application entry point
  internal/
    api/            HTTP handlers and middleware
    db/             Database access layer
    model/          Domain types
    service/        Business logic
  migrations/       SQL migration files
frontend/
  src/              JavaScript source (pages, components, API client)
  public/           Static assets
docs/               Project documentation
```

## Developing

### Prerequisites

- [Go 1.25+](https://golang.org/dl/)
- [Docker](https://www.docker.com/) (optional, for running via Docker Compose)

### Common commands

```bash
make test            # Run all tests
make test-verbose    # Run all tests with per-test output
make test-cover      # Run tests and show coverage summary
make check           # Format, vet, and test (recommended before committing)

make build           # Compile the server binary to bin/server
make run             # Build and run the server locally on :8080

make docker-up       # Build and start via Docker Compose
make docker-down     # Stop containers
make docker-logs     # Tail container logs

make clean           # Remove build output
```

Run `make` (or `make help`) at any time to see all available targets.

### Running locally

```bash
make run
```

The server starts on `http://localhost:8080`. By default it reads the authenticated username from the `X-Forwarded-User` header. When developing without an auth proxy, you can pass the header directly (e.g. with a browser extension or `curl -H "X-Forwarded-User: alice"`).

The SQLite database file is created at `./data/wfh.db` on first run.

### Running tests

Tests run against a real in-memory SQLite instance — no mocks, no external dependencies:

```bash
make test
```

## Documentation

- [Data model](docs/data_model.md) — database schema and entity descriptions
- [Features](docs/features.md) — full feature specification
