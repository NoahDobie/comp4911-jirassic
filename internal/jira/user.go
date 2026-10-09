package jira

import (
	"context"
	"net/http"
)

type User struct {
	Name        string `json:"name"`
	Key         string `json:"key"`
	DisplayName string `json:"displayName"`
}

// Returns the current user, which also verifies the URL and PAT
func (c *Client) CurrentUser(ctx context.Context) (User, error) {
	var u User
	err := c.do(ctx, http.MethodGet, "/myself", nil, &u)
	return u, err
}
