package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// Mock search with 3 matching issues, served in pages of the requested size
func searchHandler(t *testing.T, requests *[]searchRequest) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/search" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var req searchRequest
		json.NewDecoder(r.Body).Decode(&req)
		*requests = append(*requests, req)

		const total = 3
		var issues []string
		for i := req.StartAt; i < min(req.StartAt+req.MaxResults, total); i++ {
			issues = append(issues, fmt.Sprintf(`{"key":"DEMO-%d","fields":{"summary":"Task %d","status":{"name":"Open"}}}`, i+1, i+1))
		}
		fmt.Fprintf(w, `{"total":%d,"issues":[%s]}`, total, strings.Join(issues, ","))
	}
}

func TestSearch(t *testing.T) {
	var requests []searchRequest
	c := newTestServer(t, searchHandler(t, &requests))

	issues, err := c.Search(context.Background(), "project = DEMO", 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 3 || issues[0] != (Issue{Key: "DEMO-1", Summary: "Task 1", Status: "Open"}) {
		t.Errorf("unexpected issues: %+v", issues)
	}
	if len(requests) != 1 || requests[0].JQL != "project = DEMO" {
		t.Errorf("unexpected requests: %+v", requests)
	}
}

func TestSearchLimit(t *testing.T) {
	var requests []searchRequest
	c := newTestServer(t, searchHandler(t, &requests))

	issues, err := c.Search(context.Background(), "project = DEMO", 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 2 || requests[0].MaxResults != 2 {
		t.Errorf("got %d issues with maxResults %d, want 2 and 2", len(issues), requests[0].MaxResults)
	}
}

func TestSearchPaging(t *testing.T) {
	var requests []searchRequest
	c := newTestServer(t, searchHandler(t, &requests))

	issues, err := searchPages(c, 2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(issues) != 3 || len(requests) != 2 || requests[1].StartAt != 2 {
		t.Errorf("got %d issues over %d requests: %+v", len(issues), len(requests), requests)
	}
}

// Runs a search with a smaller page size so paging can be tested with few issues
func searchPages(c *Client, pageSize int) ([]Issue, error) {
	old := searchPageSize
	defer func() { searchPageSize = old }()
	searchPageSize = pageSize
	return c.Search(context.Background(), "project = DEMO", 10)
}
