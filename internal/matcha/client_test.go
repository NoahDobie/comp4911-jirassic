package matcha

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const testKey = "test-matcha-key"

// Returns a client pointed at a mock Matcha that replies with text
func newTestServer(t *testing.T, status int, text string) *Client {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/rest/api/v1/completions" {
			t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		}
		if got := r.Header.Get("MATCHA-API-KEY"); got != testKey {
			t.Errorf("MATCHA-API-KEY = %q", got)
		}
		var req completionRequest
		json.NewDecoder(r.Body).Decode(&req)
		if req.MissionID != 42 || !strings.Contains(req.Input, "TODAY: Friday, October 16, 2026") || !strings.Contains(req.Input, "REQUEST: my open bugs") {
			t.Errorf("unexpected body: %+v", req)
		}

		w.WriteHeader(status)
		reply, _ := json.Marshal(map[string]any{
			"status": "completed",
			"output": []any{map[string]any{"content": []any{map[string]any{"type": "output_text", "text": text}}}},
		})
		w.Write(reply)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL+"/", testKey, 42)
}

var friday = time.Date(2026, 10, 16, 9, 0, 0, 0, time.UTC)

func TestTranslateToJQL(t *testing.T) {
	c := newTestServer(t, http.StatusOK, `{"jql": "assignee = currentUser() AND type = Bug AND resolution = Unresolved"}`)
	jql, err := c.TranslateToJQL(context.Background(), " my open bugs ", friday)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if jql != "assignee = currentUser() AND type = Bug AND resolution = Unresolved" {
		t.Errorf("jql = %q", jql)
	}
}

func TestTranslateErrors(t *testing.T) {
	tests := []struct {
		name, text string
		status     int
		wantErr    string
	}{
		{"mission refuses", `{"error": "not a Jira search"}`, http.StatusOK, "not a Jira search"},
		{"not json", "Sure! Here is your query: project = DEMO", http.StatusOK, "did not return JQL"},
		{"empty jql", `{"jql": "  "}`, http.StatusOK, "empty query"},
		{"bad key", "", http.StatusUnauthorized, "API key"},
		{"server error", "", http.StatusInternalServerError, "500 Internal Server Error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := newTestServer(t, tt.status, tt.text)
			_, err := c.TranslateToJQL(context.Background(), "my open bugs", friday)
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("error = %v, want it to contain %q", err, tt.wantErr)
			}
			if err != nil && strings.Contains(err.Error(), testKey) {
				t.Error("error must never contain the API key")
			}
		})
	}
}

func TestTranslateEmptyRequest(t *testing.T) {
	c := NewClient("http://127.0.0.1:1", testKey, 42)
	if _, err := c.TranslateToJQL(context.Background(), "  ", friday); err == nil {
		t.Error("expected error for an empty request")
	}
}

func TestParseJQLReply(t *testing.T) {
	tests := []struct{ in, want string }{
		{`{"jql":"project = DEMO"}`, "project = DEMO"},
		{"```json\n{\"jql\": \"project = DEMO\"}\n```", "project = DEMO"},
		{`Here you go: {"jql":"text ~ \"login\""}`, `text ~ "login"`},
	}
	for _, tt := range tests {
		got, err := parseJQLReply(tt.in)
		if err != nil || got != tt.want {
			t.Errorf("parseJQLReply(%q) = %q, %v; want %q", tt.in, got, err, tt.want)
		}
	}
}

func TestBadRequestShowsReason(t *testing.T) {
	tests := []struct{ name, body, want string }{
		{"error field", `{"error": "mission_id must be a string"}`, "400 Bad Request: mission_id must be a string"},
		{"detail field", `{"detail": "Mission not found"}`, "400 Bad Request: Mission not found"},
		{"plain text", `Bad input`, "400 Bad Request: Bad input"},
		{"echoes key", `{"error": "invalid key test-matcha-key"}`, "invalid key <key>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				w.Write([]byte(tt.body))
			}))
			defer srv.Close()

			_, err := NewClient(srv.URL, testKey, 42).TranslateToJQL(context.Background(), "my open bugs", friday)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Errorf("error = %v, want it to contain %q", err, tt.want)
			}
			if err != nil && strings.Contains(err.Error(), testKey) {
				t.Error("error must never contain the API key")
			}
		})
	}
}
