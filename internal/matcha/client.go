package matcha

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// Talks to the Matcha AI gateway, which runs a custom configured Mission
type Client struct {
	baseURL string
	apiKey  string
	mission int
	http    *http.Client
}

// Creates a client for one Mission
func NewClient(baseURL, apiKey string, mission int) *Client {
	return &Client{
		baseURL: strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		apiKey:  strings.TrimSpace(apiKey),
		mission: mission,
		http:    &http.Client{Timeout: 30 * time.Second},
	}
}

type completionRequest struct {
	MissionID int    `json:"mission_id"`
	Input     string `json:"input"`
}

type completionResponse struct {
	Error  string `json:"error"`
	Output []struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

// What the JQL Mission is told to reply with: the query, or why it couldn't make one
type jqlReply struct {
	JQL   string `json:"jql"`
	Error string `json:"error"`
}

// Turns a plain-English request into JQL using the Mission
// The result is shown to the user before it runs
func (c *Client) TranslateToJQL(ctx context.Context, request string, now time.Time) (string, error) {
	request = strings.TrimSpace(request)
	if request == "" {
		return "", errors.New("Describe the issues you want to find")
	}
	input := fmt.Sprintf("TODAY: %s\nREQUEST: %s", now.Format("Monday, January 2, 2006"), request)

	text, err := c.complete(ctx, input)
	if err != nil {
		return "", err
	}
	return parseJQLReply(text)
}

// Sends input to the Mission and returns its text reply
func (c *Client) complete(ctx context.Context, input string) (string, error) {
	data, err := json.Marshal(completionRequest{MissionID: c.mission, Input: input})
	if err != nil {
		return "", fmt.Errorf("encode request: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/rest/api/v1/completions", bytes.NewReader(data))
	if err != nil {
		return "", fmt.Errorf("build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("MATCHA-API-KEY", c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("connect to Matcha: %w", err)
	}
	defer resp.Body.Close()

	switch {
	case resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden:
		return "", errors.New("Matcha rejected the API key: check matchaKey in your config")
	case resp.StatusCode < 200 || resp.StatusCode > 299:
		return "", c.statusError(resp)
	}

	var body completionResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		return "", fmt.Errorf("decode Matcha response: %w", err)
	}
	if body.Error != "" {
		return "", fmt.Errorf("matcha: %s", body.Error)
	}
	for _, out := range body.Output {
		for _, c := range out.Content {
			if c.Type == "output_text" && strings.TrimSpace(c.Text) != "" {
				return c.Text, nil
			}
		}
	}
	return "", errors.New("matcha: empty reply")
}

// Turns a failed response into an error with Matcha's reason
func (c *Client) statusError(resp *http.Response) error {
	data, _ := io.ReadAll(io.LimitReader(resp.Body, 2000))

	var body struct {
		Error   string `json:"error"`
		Message string `json:"message"`
		Detail  any    `json:"detail"`
	}
	reason := strings.TrimSpace(string(data))
	if json.Unmarshal(data, &body) == nil {
		switch {
		case body.Error != "":
			reason = body.Error
		case body.Message != "":
			reason = body.Message
		case body.Detail != nil:
			reason = fmt.Sprint(body.Detail)
		}
	}
	if c.apiKey != "" {
		reason = strings.ReplaceAll(reason, c.apiKey, "<key>")
	}
	if reason == "" {
		return fmt.Errorf("matcha: unexpected status %s", resp.Status)
	}
	return fmt.Errorf("matcha: %s: %s", resp.Status, reason)
}

// Reads the Mission's JSON reply, ignoring any text around the braces
func parseJQLReply(text string) (string, error) {
	start, end := strings.Index(text, "{"), strings.LastIndex(text, "}")
	if start < 0 || end < start {
		return "", errors.New("Matcha did not return JQL, try rewording the request")
	}

	var reply jqlReply
	if err := json.Unmarshal([]byte(text[start:end+1]), &reply); err != nil {
		return "", errors.New("Matcha did not return JQL, try rewording the request")
	}
	if reply.Error != "" {
		return "", fmt.Errorf("Matcha couldn't make JQL: %s", reply.Error)
	}
	jql := strings.TrimSpace(reply.JQL)
	if jql == "" {
		return "", errors.New("Matcha returned an empty query, try rewording the request")
	}
	return jql, nil
}
