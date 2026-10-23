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
saved locally to your user config folder (ex. `%AppData%\jirassic\config.json` on
Windows, `~/.config/jirassic/config.json` on Linux).

## Logging work

| Field | Format |
|---|---|
| Issue | Issue key, ex. `DEMO-123` |
| Time | `1h 30m`, `1.5h`, or `45m` |
| Started (optional) | A day, a time, or both, ex. `9:30am`, `yesterday`, `mon 2pm`, `10/09 9:30`. Days are `today`, `yesterday`, a weekday (most recent, counting today), or a date (`10/09`, `10/09/2026`, `2026-10-09`). Missing day means today, missing time means the current time, blank means now |
| Comment (optional) | Any text |

## Quick actions

Add saved worklogs to `quickActions` in the config file:

```json
"quickActions": [
  { "label": "Standup", "issueKey": "DEMO-1", "duration": "15m", "comment": "Daily standup" }
]
```

## Natural language search (optional)

The search panel can turn plain English into JQL using a Matcha Mission. Add these to the
config file:

```json
"matchaUrl": "<Matcha base URL>",
"matchaKey": "<your Matcha API key>",
"matchaMission": 123
```

Type either JQL or plain English in the search panel. Anything Jira can't read as JQL is
sent to Matcha, and the JQL it returns is placed in the search box to check or edit. It only
runs when you press Enter. JQL search works without Matcha.

The config file holds your Jira token and Matcha key. Keep it out of the repo and never
share it.

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
| Enter | Search: search (JQL or plain English), or log time to the selected issue |
| ↓ / ↑ | Search: move into and through the results |
| ↑ / ↓ | History: move through recent worklogs |
| R | History: refresh |
| Ctrl+C | Quit |

## Tests

```bash
go test ./...
```
