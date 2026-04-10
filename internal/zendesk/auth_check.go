package zendesk

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/relux-works/skill-zendesk-management/internal/config"
)

type Client struct {
	httpClient *http.Client
}

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
}

func (e *HTTPError) Error() string {
	return fmt.Sprintf("zendesk auth check failed: status %d", e.StatusCode)
}

func NewClient(httpClient *http.Client) *Client {
	if httpClient == nil {
		httpClient = http.DefaultClient
	}
	return &Client{httpClient: httpClient}
}

func (c *Client) CheckAuth(ctx context.Context, instanceURL string, resolved config.ResolvedToken) (AuthCheckResult, error) {
	if strings.TrimSpace(instanceURL) == "" {
		return AuthCheckResult{}, fmt.Errorf("instance URL is required")
	}
	if resolved.AuthType != config.AuthTypeAPIToken {
		return AuthCheckResult{}, fmt.Errorf("unsupported auth type %q", resolved.AuthType)
	}
	if strings.TrimSpace(resolved.Email) == "" {
		return AuthCheckResult{}, fmt.Errorf("email is required for api_token auth")
	}
	if strings.TrimSpace(resolved.Token) == "" {
		return AuthCheckResult{}, fmt.Errorf("api token is required")
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		strings.TrimRight(strings.TrimSpace(instanceURL), "/")+"/api/v2/users/me.json",
		nil,
	)
	if err != nil {
		return AuthCheckResult{}, fmt.Errorf("build auth check request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", basicAuthHeader(resolved.Email, resolved.Token))

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return AuthCheckResult{}, fmt.Errorf("execute auth check request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return AuthCheckResult{}, fmt.Errorf("read auth check response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return AuthCheckResult{}, &HTTPError{
			StatusCode: resp.StatusCode,
			Body:       strings.TrimSpace(string(body)),
		}
	}

	var payload struct {
		User struct {
			ID        int64  `json:"id"`
			Name      string `json:"name"`
			Email     string `json:"email"`
			Role      string `json:"role"`
			Active    bool   `json:"active"`
			Suspended bool   `json:"suspended"`
		} `json:"user"`
	}
	if err := json.Unmarshal(body, &payload); err != nil {
		return AuthCheckResult{}, fmt.Errorf("decode auth check response: %w", err)
	}

	return AuthCheckResult{
		HTTPStatus: resp.StatusCode,
		UserID:     payload.User.ID,
		Name:       payload.User.Name,
		Email:      payload.User.Email,
		Role:       payload.User.Role,
		Active:     payload.User.Active,
		Suspended:  payload.User.Suspended,
	}, nil
}

func basicAuthHeader(email, token string) string {
	value := strings.TrimSpace(email) + "/token:" + strings.TrimSpace(token)
	return "Basic " + base64.StdEncoding.EncodeToString([]byte(value))
}
