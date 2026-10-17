# Jirassic — COMP 4911 Capstone

A terminal-based, four-panel Jira time-logging tool, built as a TRU COMP 4911 capstone
project for Cayenta.

Supports Jira Server/Data Center (REST API v2) with a personal access token.

## Build and run

```bash
go build -o jirassic ./cmd/jirassic
./jirassic
```

On first launch, enter your Jira URL and personal access token. They are verified, then
saved locally to your user config folder (e.g. `%AppData%\jirassic\config.json` on
Windows, `~/.config/jirassic/config.json` on Linux).

## Logging work

| Field | Format |
|---|---|
| Issue | Issue key, e.g. `DEMO-123` |
| Time | `1h 30m`, `1.5h`, or `45m` |
| Started (optional) | A day, a time, or both, e.g. `9:30am`, `yesterday`, `mon 2pm`, `10/09 9:30`. Days are `today`, `yesterday`, a weekday (most recent, counting today), or a date (`10/09`, `10/09/2026`, `2026-10-09`). Missing day means today, missing time means the current time, blank means now |
| Comment (optional) | Any text |

## Quick actions

Add saved worklogs to `quickActions` in the config file:

```json
"quickActions": [
  { "label": "Standup", "issueKey": "DEMO-1", "duration": "15m", "comment": "Daily standup" }
]
```

## Keys

| Key | Action |
|---|---|
| Shift+Tab | Next panel |
| Tab | Next button or field in the panel |
| Alt+1–4 | Jump to a panel |
| ↑ / ↓ | Move between the quick action buttons and fields |
| ← / → | Quick log: pick a quick action button |
| Enter | Next field, or log the worklog / quick action |
| Esc | Clear the form |
| ← / → | Timesheet: previous / next week |
| T | Timesheet: back to this week |
| R | Timesheet: refresh |
| Enter | Search: run the JQL query, or log time to the selected issue |
| ↓ / ↑ | Search: move into and through the results |
| Ctrl+C | Quit |

## Tests

```bash
go test ./...
```
