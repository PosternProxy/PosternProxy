package caddy

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL    string
	httpClient *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{
		baseURL: baseURL,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// Load replaces the entire Caddy config atomically via POST /load.
func (c *Client) Load(cfg *Config) error {
	return c.do(http.MethodPost, "/load", cfg, nil)
}

// GetConfig retrieves the full running config.
func (c *Client) GetConfig() (json.RawMessage, error) {
	var result json.RawMessage
	if err := c.do(http.MethodGet, "/config/", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetPath sets a value at the given config path (PUT).
func (c *Client) SetPath(path string, val any) error {
	return c.do(http.MethodPut, "/config/"+path, val, nil)
}

// PatchPath updates a value at the given config path (PATCH).
func (c *Client) PatchPath(path string, val any) error {
	return c.do(http.MethodPatch, "/config/"+path, val, nil)
}

// DeletePath removes a value at the given config path.
func (c *Client) DeletePath(path string) error {
	return c.do(http.MethodDelete, "/config/"+path, nil, nil)
}

// AppendToArray appends a value to a config array at the given path (POST).
func (c *Client) AppendToArray(path string, val any) error {
	return c.do(http.MethodPost, "/config/"+path, val, nil)
}

// GetUpstreams returns real-time upstream health status.
func (c *Client) GetUpstreams() ([]UpstreamStatus, error) {
	var result []UpstreamStatus
	if err := c.do(http.MethodGet, "/reverse_proxy/upstreams", nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// Ping checks that the Caddy admin API is reachable.
func (c *Client) Ping() error {
	resp, err := c.httpClient.Get(c.baseURL + "/config/")
	if err != nil {
		return fmt.Errorf("caddy unreachable: %w", err)
	}
	resp.Body.Close()
	return nil
}

type UpstreamStatus struct {
	Address     string `json:"address"`
	NumRequests int    `json:"num_requests"`
	Fails       int    `json:"fails"`
}

func (c *Client) do(method, path string, body, out any) error {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := http.NewRequest(method, c.baseURL+path, bodyReader)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("caddy request %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("caddy %s %s returned %d: %s", method, path, resp.StatusCode, string(respBody))
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("unmarshal response: %w", err)
		}
	}

	return nil
}
