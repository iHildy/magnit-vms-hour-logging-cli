package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"strings"
	"time"
)

type Authenticator struct {
	BaseURL string
	Client  *http.Client
}

func NewHTTPClient() (*http.Client, error) {
	jar, err := cookiejar.New(nil)
	if err != nil {
		return nil, fmt.Errorf("create cookie jar: %w", err)
	}
	return &http.Client{
		Jar: jar,
		Timeout: 45 * time.Second,
	}, nil
}

func (a *Authenticator) Login(ctx context.Context, username, password string) error {
	if username == "" || password == "" {
		return fmt.Errorf("username and password are required")
	}

	form := url.Values{}
	form.Set("username", username)
	form.Set("password_login", password)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, strings.TrimRight(a.BaseURL, "/")+"/login.html", strings.NewReader(form.Encode()))
	if err != nil {
		return fmt.Errorf("build login request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("User-Agent", "hour-logging-cli/1.0")

	resp, err := a.Client.Do(req)
	if err != nil {
		return fmt.Errorf("login request failed: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode >= 400 {
		return fmt.Errorf("login failed with status %d", resp.StatusCode)
	}

	_, err = a.CurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("login validation failed: %w", err)
	}

	return nil
}

func (a *Authenticator) CurrentUser(ctx context.Context) (map[string]any, error) {
	endpoint := strings.TrimRight(a.BaseURL, "/") + "/wand2/api/users/current?noCache=true"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("build users/current request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	if token, err := ExtractAccessToken(a.Client, a.BaseURL); err == nil && token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	resp, err := a.Client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("users/current request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return nil, fmt.Errorf("users/current failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("decode users/current response: %w", err)
	}
	return out, nil
}

func ExtractXSRFToken(client *http.Client, baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}

	cookies := client.Jar.Cookies(u)
	for _, c := range cookies {
		name := strings.ToLower(c.Name)
		if name == "xsrf-token" || name == "x-xsrf-token" || name == "xsrftoken" || name == "_xsrf" {
			if c.Value != "" {
				val := strings.Trim(c.Value, "\"")
				if decoded, err := url.QueryUnescape(val); err == nil && decoded != "" {
					return decoded, nil
				}
				return val, nil
			}
		}
	}

	return "", fmt.Errorf("xsrf token cookie not found in session")
}

func ExtractAccessToken(client *http.Client, baseURL string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", fmt.Errorf("parse base url: %w", err)
	}

	cookies := client.Jar.Cookies(u)
	for _, c := range cookies {
		name := strings.ToLower(c.Name)
		if name == "productionaccess_token" || name == "access_token" {
			if c.Value != "" {
				val := strings.Trim(c.Value, "\"")
				if decoded, err := url.QueryUnescape(val); err == nil && decoded != "" {
					return decoded, nil
				}
				return val, nil
			}
		}
	}

	return "", fmt.Errorf("access token cookie not found in session")
}
