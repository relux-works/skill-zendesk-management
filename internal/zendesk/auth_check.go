package zendesk

import (
	"context"
	"fmt"

	"github.com/relux-works/skill-zendesk-management/internal/config"
)

type AuthCheckResult struct {
	HTTPStatus int
	UserID     int64
	Name       string
	Email      string
	Role       string
	Active     bool
	Suspended  bool
}

type HTTPError struct {
	StatusCode int
	Body       string
	Headers    map[string]string
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("zendesk auth check failed: status %d", e.StatusCode)
}

func (c *Client) CheckAuth(ctx context.Context, instanceURL string, resolved config.ResolvedToken) (AuthCheckResult, error) {
	client, err := NewAuthenticatedClient(instanceURL, resolved, c.httpClient)
	if err != nil {
		return AuthCheckResult{}, err
	}

	user, err := client.getObject(ctx, "/api/v2/users/me.json", nil, "user")
	if err != nil {
		return AuthCheckResult{}, err
	}

	return AuthCheckResult{
		HTTPStatus: 200,
		UserID:     int64Value(user["id"]),
		Name:       stringValue(user["name"]),
		Email:      stringValue(user["email"]),
		Role:       stringValue(user["role"]),
		Active:     boolValue(user["active"]),
		Suspended:  boolValue(user["suspended"]),
	}, nil
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func boolValue(value any) bool {
	boolean, _ := value.(bool)
	return boolean
}

func int64Value(value any) int64 {
	switch typed := value.(type) {
	case float64:
		return int64(typed)
	case int64:
		return typed
	default:
		return 0
	}
}
