package jira

import (
	"context"
	"net/http"
	"net/url"

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
