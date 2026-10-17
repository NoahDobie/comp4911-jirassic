package jira

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestGetWorklogs(t *testing.T) {
	me := User{Name: "jdoe", Key: "JIRAUSER1"}
	other := `{"name":"asmith","key":"JIRAUSER2"}`
	mine := `{"name":"jdoe","key":"JIRAUSER1"}`

	// DEMO-1 worklogs come back over two pages to test paging
	pages := map[string]string{
		"DEMO-1?startAt=0": `{"startAt":0,"total":3,"worklogs":[
			{"id":"1","author":` + mine + `,"started":"2026-10-13T09:00:00.000+0000","timeSpentSeconds":3600,"comment":"Standup"},
			{"id":"2","author":` + other + `,"started":"2026-10-13T10:00:00.000+0000","timeSpentSeconds":1800}]}`,
		"DEMO-1?startAt=2": `{"startAt":2,"total":3,"worklogs":[
			{"id":"3","author":` + mine + `,"started":"2026-10-11T09:00:00.000+0000","timeSpentSeconds":900}]}`,
		"DEMO-2?startAt=0": `{"startAt":0,"total":2,"worklogs":[
			{"id":"4","author":` + mine + `,"started":"2026-10-12T08:00:00.000+0000","timeSpentSeconds":5400},
			{"id":"5","author":` + mine + `,"started":"2026-10-19T00:00:00.000+0000","timeSpentSeconds":600}]}`,
	}

	c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/search" {
			var req searchRequest
			json.NewDecoder(r.Body).Decode(&req)
			if !strings.Contains(req.JQL, `worklogDate >= "2026-10-11"`) || !strings.Contains(req.JQL, `worklogDate <= "2026-10-20"`) {
				t.Errorf("unexpected JQL: %s", req.JQL)
			}
			w.Write([]byte(`{"total":2,"issues":[
				{"key":"DEMO-1","fields":{"summary":"Meetings"}},
				{"key":"DEMO-2","fields":{"summary":"Feature work"}}]}`))
			return
		}

		key := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/rest/api/2/issue/"), "/worklog")
		body, ok := pages[fmt.Sprintf("%s?startAt=%s", key, r.URL.Query().Get("startAt"))]
		if !ok {
			t.Errorf("unexpected request %s", r.URL)
		}
		w.Write([]byte(body))
	})

	from := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	logs, err := c.GetWorklogs(context.Background(), me, from, from.AddDate(0, 0, 7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Keeps only my worklogs inside Mon 12 - Sun 18, oldest first
	if len(logs) != 2 {
		t.Fatalf("got %d worklogs, want 2: %+v", len(logs), logs)
	}
	first := Worklog{
		ID:           "4",
		IssueKey:     "DEMO-2",
		IssueSummary: "Feature work",
		Started:      time.Date(2026, 10, 12, 8, 0, 0, 0, time.UTC),
		Duration:     90 * time.Minute,
	}
	if logs[0].ID != first.ID || logs[0].IssueKey != first.IssueKey || logs[0].IssueSummary != first.IssueSummary ||
		!logs[0].Started.Equal(first.Started) || logs[0].Duration != first.Duration {
		t.Errorf("first = %+v, want %+v", logs[0], first)
	}
	if logs[1].ID != "1" || logs[1].Comment != "Standup" || logs[1].Duration != time.Hour {
		t.Errorf("second = %+v", logs[1])
	}
}

func TestGetWorklogsError(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte(`{"errorMessages":["Bad JQL"]}`))
	})
	_, err := c.GetWorklogs(context.Background(), User{Name: "jdoe"}, time.Now(), time.Now())
	if err == nil || !strings.Contains(err.Error(), "Bad JQL") {
		t.Errorf("error = %v, want it to mention Bad JQL", err)
	}
}

func TestSameUser(t *testing.T) {
	tests := []struct {
		a, b User
		want bool
	}{
		{User{Name: "jdoe", Key: "K1"}, User{Name: "jdoe", Key: "K1"}, true},
		{User{Name: "jdoe", Key: "K1"}, User{Name: "jdoe", Key: "K2"}, false},
		{User{Name: "jdoe"}, User{Name: "jdoe", Key: "K1"}, true},
		{User{Name: "jdoe"}, User{Name: "asmith"}, false},
	}
	for _, tt := range tests {
		if got := sameUser(tt.a, tt.b); got != tt.want {
			t.Errorf("sameUser(%+v, %+v) = %v, want %v", tt.a, tt.b, got, tt.want)
		}
	}
}

func TestGetWorklogsConcurrent(t *testing.T) {
	const issueCount = 12
	var mu sync.Mutex
	inFlight, peak := 0, 0

	c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/rest/api/2/search" {
			var issues []string
			for i := 1; i <= issueCount; i++ {
				issues = append(issues, fmt.Sprintf(`{"key":"DEMO-%d"}`, i))
			}
			fmt.Fprintf(w, `{"total":%d,"issues":[%s]}`, issueCount, strings.Join(issues, ","))
			return
		}

		mu.Lock()
		inFlight++
		peak = max(peak, inFlight)
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)
		w.Write([]byte(`{"total":1,"worklogs":[{"id":"1","author":{"key":"K1"},"started":"2026-10-13T09:00:00.000+0000","timeSpentSeconds":60}]}`))

		mu.Lock()
		inFlight--
		mu.Unlock()
	})

	from := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	logs, err := c.GetWorklogs(context.Background(), User{Key: "K1"}, from, from.AddDate(0, 0, 7))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(logs) != issueCount {
		t.Errorf("got %d worklogs, want %d", len(logs), issueCount)
	}
	if peak < 2 || peak > worklogWorkers {
		t.Errorf("peak requests at once = %d, want 2 to %d", peak, worklogWorkers)
	}
}
