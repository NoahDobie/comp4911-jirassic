package jira

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"jirassic-capstone/internal/worklog"
)

const testToken = "test-token"

// Returns a client pointed at a mock Jira using handler
func newTestServer(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	return NewClient(srv.URL+"/", testToken)
}

func TestCurrentUser(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Path != "/rest/api/2/myself" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer "+testToken {
			t.Errorf("Authorization = %q", got)
		}
		w.Write([]byte(`{"name":"jsmith","key":"jsmith","displayName":"John Smith"}`))
	})

	u, err := c.CurrentUser(context.Background())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.Name != "jsmith" || u.DisplayName != "John Smith" {
		t.Errorf("unexpected user: %+v", u)
	}
}

func TestAddWorklog(t *testing.T) {
	c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/2/issue/DEMO-123/worklog" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		var body addWorklogRequest
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Fatalf("decode body: %v", err)
		}
		want := addWorklogRequest{
			TimeSpent: "1h 30m",
			Started:   "2026-10-12T09:00:00.000+0000",
			Comment:   "Test Comment",
		}
		if body != want {
			t.Errorf("body = %+v, want %+v", body, want)
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"id":"10001"}`))
	})

	e := worklog.Entry{
		IssueKey: "DEMO-123",
		Duration: 90 * time.Minute,
		Comment:  "Test Comment",
		Started:  time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC),
	}
	id, err := c.AddWorklog(context.Background(), e)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "10001" {
		t.Errorf("id = %q, want 10001", id)
	}
}

func TestErrors(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		body    string
		wantErr string
	}{
		{"unauthorized", http.StatusUnauthorized, "", "Authentication failed"},
		{"forbidden", http.StatusForbidden, "", "Permission denied"},
		{"jira message", http.StatusBadRequest, `{"errorMessages": ["Issue does not exist"]}`, "Issue does not exist"},
		{"field error", http.StatusBadRequest, `{"errors":{"timeLogged":"Invalid time"}}`, "timeLogged: Invalid time"},
		{"no body", http.StatusInternalServerError, "", "unexpected status 500"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.status)
				w.Write([]byte(tt.body))
			})
			_, err := c.CurrentUser(context.Background())
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestConnectionError(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", testToken)
	if _, err := c.CurrentUser(context.Background()); err == nil {
		t.Error("expected connection error")
	}
}

func TestIsBadRequest(t *testing.T) {
	for _, tt := range []struct {
		status int
		want   bool
	}{{http.StatusBadRequest, true}, {http.StatusUnauthorized, false}, {http.StatusInternalServerError, false}} {
		c := newTestServer(t, func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(tt.status)
		})
		_, err := c.Search(context.Background(), "not jql", 10)
		if got := IsBadRequest(err); got != tt.want {
			t.Errorf("status %d: IsBadRequest = %v, want %v", tt.status, got, tt.want)
		}
	}
	if IsBadRequest(nil) {
		t.Error("nil is not a bad request")
	}
}
