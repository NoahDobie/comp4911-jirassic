package jira

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"
	"sync"
	"time"

	"jirassic-capstone/internal/worklog"
)

// Jira's expected format for the "started" field
const timeLayout = "2006-01-02T15:04:05.000-0700"

// Posts a worklog to an issue and returns the new worklog ID
// Requires: IssueKey must be a valid Jira issue key on the board/backlog, Duration must be > 0, Started must be a valid time
type addWorklogRequest struct {
	TimeSpent string `json:"timeSpent"`
	Started   string `json:"started"`
	Comment   string `json:"comment,omitempty"`
}

// Posts a worklog to an issue and returns the new worklog ID
func (c *Client) AddWorklog(ctx context.Context, e worklog.Entry) (string, error) {
	req := addWorklogRequest{
		TimeSpent: worklog.FormatDuration(e.Duration),
		Started:   e.Started.Format(timeLayout),
		Comment:   e.Comment,
	}

	var resp struct {
		ID string `json:"id"`
	}
	path := "/issue/" + url.PathEscape(e.IssueKey) + "/worklog"
	if err := c.do(ctx, http.MethodPost, path, req, &resp); err != nil {
		return "", err
	}
	return resp.ID, nil
}

// Max issues searched when fetching worklogs for a date range
const worklogIssueLimit = 500

// Max issues whose worklogs are fetched at the same time
// This is a tradeoff between speed and memory usage.
// Each issue's worklogs are fetched in a separate goroutine,
// If the number of issues is large, this can use a lot of memory...
const worklogWorkers = 5

// Worklog already logged in Jira
type Worklog struct {
	ID           string
	IssueKey     string
	IssueSummary string
	Started      time.Time
	Duration     time.Duration
	Comment      string
}

type worklogResponse struct {
	StartAt    int `json:"startAt"`
	MaxResults int `json:"maxResults"`
	Total      int `json:"total"`
	Worklogs   []struct {
		ID               string `json:"id"`
		Author           User   `json:"author"`
		Started          string `json:"started"`
		TimeSpentSeconds int    `json:"timeSpentSeconds"`
		Comment          string `json:"comment"`
	} `json:"worklogs"`
}

// Returns the user's worklogs started in (from, to) with oldest first
func (c *Client) GetWorklogs(ctx context.Context, user User, from, to time.Time) ([]Worklog, error) {
	// worklogDate only matches whole days in Jira's timezone, so searching a day wider on each side, then filter by exact start time
	jql := fmt.Sprintf(`worklogAuthor = currentUser() AND worklogDate >= "%s" AND worklogDate <= "%s"`,
		from.AddDate(0, 0, -1).Format("2006-01-02"), to.AddDate(0, 0, 1).Format("2006-01-02"))
	issues, err := c.Search(ctx, jql, worklogIssueLimit)
	if err != nil {
		return nil, fmt.Errorf("find issues with worklogs: %w", err)
	}

	// Fetch several issues at once - each goroutine writes only to its own index
	results := make([][]Worklog, len(issues))
	errs := make([]error, len(issues))
	slots := make(chan struct{}, worklogWorkers)
	var wg sync.WaitGroup
	for i, is := range issues {
		wg.Add(1)
		go func(i int, is Issue) {
			defer wg.Done()
			slots <- struct{}{}        // wait for a free slot
			defer func() { <-slots }() // free it when done
			results[i], errs[i] = c.issueWorklogs(ctx, is, user, from, to)
		}(i, is)
	}
	wg.Wait()

	var logs []Worklog
	for i, is := range issues {
		if errs[i] != nil {
			return nil, fmt.Errorf("get worklogs for %s: %w", is.Key, errs[i])
		}
		logs = append(logs, results[i]...)
	}

	sort.Slice(logs, func(i, j int) bool { return logs[i].Started.Before(logs[j].Started) })
	return logs, nil
}

// Returns one issue's worklogs by user started in (from, to) fetching every page
func (c *Client) issueWorklogs(ctx context.Context, is Issue, user User, from, to time.Time) ([]Worklog, error) {
	var logs []Worklog
	for startAt := 0; ; {
		var resp worklogResponse
		path := fmt.Sprintf("/issue/%s/worklog?startAt=%d", url.PathEscape(is.Key), startAt)
		if err := c.do(ctx, http.MethodGet, path, nil, &resp); err != nil {
			return nil, err
		}

		for _, w := range resp.Worklogs {
			started, err := time.Parse(timeLayout, w.Started)
			if err != nil {
				return nil, fmt.Errorf("read start time %q: %w", w.Started, err)
			}
			if !sameUser(w.Author, user) || started.Before(from) || !started.Before(to) {
				continue
			}
			logs = append(logs, Worklog{
				ID:           w.ID,
				IssueKey:     is.Key,
				IssueSummary: is.Summary,
				Started:      started,
				Duration:     time.Duration(w.TimeSpentSeconds) * time.Second,
				Comment:      w.Comment,
			})
		}

		startAt += len(resp.Worklogs)
		if len(resp.Worklogs) == 0 || startAt >= resp.Total {
			return logs, nil
		}
	}
}

// v2 identifies users by key, falling back to name if key is missing
func sameUser(a, b User) bool {
	if a.Key != "" && b.Key != "" {
		return a.Key == b.Key
	}
	return a.Name == b.Name
}
