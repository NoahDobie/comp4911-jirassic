package jira

import (
	"context"
	"net/http"
)

// Issues fetched per request
var searchPageSize = 50

// Issue returned by a JQL search
type Issue struct {
	Key     string
	Summary string
	Status  string
}

type searchRequest struct {
	JQL        string   `json:"jql"`
	StartAt    int      `json:"startAt"`
	MaxResults int      `json:"maxResults"`
	Fields     []string `json:"fields"`
}

// Response from a JQL search
type searchResponse struct {
	Total  int `json:"total"`
	Issues []struct {
		Key    string `json:"key"`
		Fields struct {
			Summary string `json:"summary"`
			Status  struct {
				Name string `json:"name"`
			} `json:"status"`
		} `json:"fields"`
	} `json:"issues"`
}

// Runs a JQL search and returns up to limit issues
func (c *Client) Search(ctx context.Context, jql string, limit int) ([]Issue, error) {
	var issues []Issue
	for len(issues) < limit {
		req := searchRequest{
			JQL:        jql,
			StartAt:    len(issues),
			MaxResults: min(searchPageSize, limit-len(issues)),
			Fields:     []string{"summary", "status"},
		}
		var resp searchResponse
		if err := c.do(ctx, http.MethodPost, "/search", req, &resp); err != nil {
			return nil, err
		}

		for _, is := range resp.Issues {
			issues = append(issues, Issue{Key: is.Key, Summary: is.Fields.Summary, Status: is.Fields.Status.Name})
		}
		if len(resp.Issues) == 0 || len(issues) >= resp.Total {
			break
		}
	}
	return issues, nil
}
