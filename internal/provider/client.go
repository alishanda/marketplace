package provider

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"marketplace/internal/domain"
)

type Client struct {
	name    string
	url     string
	http    *http.Client
	hangFor time.Duration
}

func NewClient(name, url string, timeout time.Duration) *Client {
	return &Client{
		name: name,
		url:  url,
		http: &http.Client{Timeout: timeout},
	}
}

func (c *Client) Name() string {
	return c.name
}

func (c *Client) Issue(ctx context.Context, req domain.IssueRequest) (domain.IssueResponse, error) {
	body, err := json.Marshal(req)
	if err != nil {
		return domain.IssueResponse{}, err
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.url, bytes.NewReader(body))
	if err != nil {
		return domain.IssueResponse{}, err
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(httpReq)
	if err != nil {
		if ctx.Err() != nil || isTimeout(err) {
			return domain.IssueResponse{}, domain.ErrProviderTimeout
		}
		return domain.IssueResponse{}, fmt.Errorf("%w: %s", domain.ErrProvider, err)
	}
	defer resp.Body.Close()

	var out domain.IssueResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		if resp.StatusCode >= 500 {
			return domain.IssueResponse{}, domain.ErrProvider
		}
		return domain.IssueResponse{}, fmt.Errorf("%w: decode", domain.ErrProvider)
	}
	if resp.StatusCode == http.StatusOK && out.Status == "ok" && out.Code != "" {
		return out, nil
	}
	if out.Reason == "out_of_stock" {
		return out, domain.ErrOutOfStock
	}
	return out, domain.ErrProvider
}

func isTimeout(err error) bool {
	type timeout interface{ Timeout() bool }
	if t, ok := err.(timeout); ok && t.Timeout() {
		return true
	}
	return false
}
