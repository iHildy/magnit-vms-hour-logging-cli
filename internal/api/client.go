package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/ihildy/magnit-vms-cli/internal/auth"
)

type Client struct {
	BaseURL string
	HTTP    *http.Client
}

type Engagement struct {
	ID               int64  `json:"id"`
	Status           string `json:"status"`
	StartDate        string `json:"startDate"`
	EndDate          string `json:"endDate"`
	JobTitle         string `json:"jobTitle"`
	BuyerName        string `json:"buyerName"`
	EngagementCode   string `json:"engagementCode"`
	TimecardTemplate int64  `json:"timecardTemplateId"`
}

type SaveBillingItemsResponse struct {
	BillingItemID        int64 `json:"billingItemId"`
	BillingItemIDs       any   `json:"billingItemIds"`
	Errors               any   `json:"errors"`
	BillingItemDetailErr any   `json:"billingItemDetailErrors"`
}

func (c *Client) GetCurrentUser(ctx context.Context) (map[string]any, error) {
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/wand2/api/users/current?noCache=true"
	var out map[string]any
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetEngagementItems(ctx context.Context, pageNo, pageSize int) ([]Engagement, error) {
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/wand2/engagement/api/engagement-items?pageNo=" + strconv.Itoa(pageNo) + "&pageSize=" + strconv.Itoa(pageSize)
	var raw struct {
		Content []Engagement `json:"content"`
	}
	if err := c.getJSON(ctx, endpoint, &raw); err != nil {
		return nil, err
	}
	return raw.Content, nil
}

func (c *Client) GetMetadata(ctx context.Context, engagementID int64, selectedDateMDY string) (map[string]any, error) {
	q := url.Values{}
	q.Set("engagementId", strconv.FormatInt(engagementID, 10))
	q.Set("selectedDate", selectedDateMDY)
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/wand2/api/billing/billing-items/0/metadata?" + q.Encode()

	var out map[string]any
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *Client) GetTotalHours(ctx context.Context, engagementID int64, selectedDateMDY string) (map[string]float64, error) {
	q := url.Values{}
	q.Set("engagementId", strconv.FormatInt(engagementID, 10))
	q.Set("selectedDate", selectedDateMDY)
	endpoint := strings.TrimRight(c.BaseURL, "/") + "/wand2/api/billing/billing-items/0/worker/totalhours?" + q.Encode()

	var out map[string]float64
	if err := c.getJSON(ctx, endpoint, &out); err != nil {
		return nil, err
	}
	if out == nil {
		out = map[string]float64{}
	}
	return out, nil
}

func (c *Client) SaveBillingItems(ctx context.Context, payload map[string]any, xsrfToken string) (SaveBillingItemsResponse, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return SaveBillingItemsResponse{}, fmt.Errorf("marshal save payload: %w", err)
	}

	endpoint := strings.TrimRight(c.BaseURL, "/") + "/wand2/api/billing/billing-items"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return SaveBillingItemsResponse{}, fmt.Errorf("build save request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", strings.TrimRight(c.BaseURL, "/"))
	req.Header.Set("Referer", strings.TrimRight(c.BaseURL, "/")+"/wand/app/worker/index.html")
	req.Header.Set("x-xsrf-token", xsrfToken)
	c.applyAuthHeader(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return SaveBillingItemsResponse{}, fmt.Errorf("save request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return SaveBillingItemsResponse{}, fmt.Errorf("save failed with status %d: %s", resp.StatusCode, strings.TrimSpace(string(data)))
	}

	var out SaveBillingItemsResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return SaveBillingItemsResponse{}, fmt.Errorf("decode save response: %w", err)
	}
	return out, nil
}

func (c *Client) getJSON(ctx context.Context, endpoint string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return fmt.Errorf("build GET request: %w", err)
	}
	req.Header.Set("Accept", "application/json")
	c.applyAuthHeader(req)

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("GET %s failed: %w", endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("GET %s returned status %d: %s", endpoint, resp.StatusCode, strings.TrimSpace(string(data)))
	}

	if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
		return fmt.Errorf("decode GET %s response: %w", endpoint, err)
	}
	return nil
}

func (c *Client) applyAuthHeader(req *http.Request) {
	token, err := auth.ExtractAccessToken(c.HTTP, c.BaseURL)
	if err != nil || token == "" {
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
}
