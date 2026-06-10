package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client struct {
	Addr       string
	HTTPClient *http.Client
}

func NewClient(addr string) *Client {
	return &Client{
		Addr:       strings.TrimRight(addr, "/"),
		HTTPClient: &http.Client{},
	}
}

func (c *Client) DoJSON(method, path string, payload any) ([]byte, int, error) {
	var body io.Reader
	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return nil, 0, fmt.Errorf("marshal payload: %w", err)
		}
		body = bytes.NewReader(data)
	}
	req, err := http.NewRequest(method, c.Addr+path, body)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req)
}

func (c *Client) DoRaw(method, path string, rawBody []byte) ([]byte, int, error) {
	var body io.Reader
	if rawBody != nil {
		body = bytes.NewReader(rawBody)
	}
	req, err := http.NewRequest(method, c.Addr+path, body)
	if err != nil {
		return nil, 0, fmt.Errorf("create request: %w", err)
	}
	if rawBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.do(req)
}

func (c *Client) do(req *http.Request) ([]byte, int, error) {
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, fmt.Errorf("read response: %w", err)
	}
	return data, resp.StatusCode, nil
}
