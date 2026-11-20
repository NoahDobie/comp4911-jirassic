# Jirassic - COMP 4911 Capstone

A terminal-based Jira time-logging tool with four panels, built as a TRU COMP 4911
capstone project for Cayenta. It works with Jira Server/Data Center (REST API v2) using a
personal access token.

| Panel | What it does |
|---|---|
| 1 Quick Log | Log time with the form, or fill it in from a saved quick action |
| 2 Timesheet | Hours per issue and day for a week, with daily and weekly totals |
| 3 Search | Find issues with JQL or plain English, then log time to one |
| 4 History | Your worklogs from the last 14 days |

## Build and run

```bash
go build -o jirassic ./cmd/jirassic
./jirassic
```

The terminal needs to be at least 60 x 24.

On first launch, enter your Jira URL and personal access token. They're checked, then
saved to your user config folder (ex. `%AppData%\jirassic\config.json` on Windows,
`~/.config/jirassic/config.json` on Linux). This file holds your Jira token and Matcha key,
so keep it out of the repo and don't share it.

## Settings

Press Ctrl+X to open settings at any time.

| Section | Fields |
|---|---|
| Jira | URL and personal access token (required) |
| Matcha | URL, API key, and Mission ID for plain English search (optional, all three or none) |
| Quick actions | Label, issue key, time, and optional comment for each saved worklog |

## Logging work

| Field | Format |
|---|---|
| Issue | Issue key, ex. `DEMO-123` |
| Time | `1h 30m`, `1.5h`, or `45m` |
| Started (optional) | A day, a time, or both, ex. `9:30am`, `yesterday`, `mon 2pm`, `10/09 9:30`. Days are `today`, `yesterday`, a weekday (most recent, counting today), or a date (`10/09`, `10/09/2026`, `2026-10-09`). Missing day means today, missing time means the current time, blank means now |
| Comment (optional) | Any text |

Press Enter on the last field to log it. The timesheet and history refresh afterwards.

Quick actions show as buttons above the form. Picking one fills in the issue, time, and
comment so you can adjust them before logging.

## Loading times

The timesheet and history can be slow if you log time to busy shared issues (like a team
meetings ticket), since Jira sends every worklog on the issue, not just yours. Loading
happens in the background, and if it fails you can press R to try again.

## Natural language search (optional)

With Matcha set up in settings, you can type plain English in the search panel. Anything
that isn't valid JQL is sent to Matcha, and the JQL it returns goes in the search box so
you can check it before pressing Enter. JQL search works without Matcha.

## Keys

| Key | Where | Action |
|---|---|---|
| Shift+Tab | Anywhere | Next panel |
| Alt+1-4 | Anywhere | Jump to a panel |
| Ctrl+X | Anywhere | Open settings |
| Tab, ↑ / ↓ | Quick log | Move between buttons and fields |
| ← / → | Quick log | Pick a quick action |
| Enter | Quick log | Next field, or log the worklog |
| Esc | Quick log | Clear the form |
| ← / → | Timesheet | Previous / next week |
| T | Timesheet | Back to this week |
| R | Timesheet, History | Refresh |
| Enter | Search | Search, or use the selected issue in the quick log form |
| Esc | Search | Clear the search |
| Enter | Settings | Save |
| Ctrl+N / Ctrl+D | Settings | Add / delete a quick action |
| Esc | Settings | Close without saving |

## Tests

```bash
go test ./...
```

The tests use mock Jira and Matcha servers, so they don't need a live Jira instance.
