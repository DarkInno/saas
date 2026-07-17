// Package testtoxiproxy provides a small Toxiproxy HTTP client for integration tests.
package testtoxiproxy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	waitInterval = 100 * time.Millisecond
	maxErrorBody = 1024
)

// Client communicates with a Toxiproxy HTTP API.
type Client struct {
	endpoint   string
	httpClient *http.Client
}

// Proxy identifies a proxy created by Client.
type Proxy struct {
	Name   string
	client *Client
}

// New returns a Toxiproxy client for endpoint.
func New(endpoint string) *Client {
	return &Client{
		endpoint:   strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{},
	}
}

// Wait polls Toxiproxy until its version endpoint accepts a request or ctx expires.
func (client *Client) Wait(ctx context.Context) error {
	ticker := time.NewTicker(waitInterval)
	defer ticker.Stop()

	for {
		if err := client.request(ctx, http.MethodGet, "/version", nil); err == nil {
			return nil
		} else if ctx.Err() != nil {
			return ctx.Err()
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

// CreateProxy creates an enabled proxy that listens locally and forwards upstream.
func (client *Client) CreateProxy(ctx context.Context, name, listen, upstream string) (*Proxy, error) {
	request := struct {
		Name     string `json:"name"`
		Listen   string `json:"listen"`
		Upstream string `json:"upstream"`
		Enabled  bool   `json:"enabled"`
	}{
		Name:     name,
		Listen:   listen,
		Upstream: upstream,
		Enabled:  true,
	}
	if err := client.request(ctx, http.MethodPost, "/proxies", request); err != nil {
		return nil, err
	}
	return &Proxy{Name: name, client: client}, nil
}

// SetEnabled enables or disables a proxy.
func (client *Client) SetEnabled(ctx context.Context, name string, enabled bool) error {
	request := struct {
		Enabled bool `json:"enabled"`
	}{Enabled: enabled}
	return client.request(ctx, http.MethodPost, "/proxies/"+url.PathEscape(name), request)
}

// AddTimeout drops downstream traffic until the named toxic is removed.
func (client *Client) AddTimeout(ctx context.Context, proxy, name string) error {
	request := struct {
		Name       string          `json:"name"`
		Type       string          `json:"type"`
		Stream     string          `json:"stream"`
		Toxicity   json.RawMessage `json:"toxicity"`
		Attributes struct {
			Timeout int `json:"timeout"`
		} `json:"attributes"`
	}{
		Name:     name,
		Type:     "timeout",
		Stream:   "downstream",
		Toxicity: json.RawMessage("1.0"),
	}
	request.Attributes.Timeout = 0
	return client.request(ctx, http.MethodPost, "/proxies/"+url.PathEscape(proxy)+"/toxics", request)
}

// RemoveToxic deletes a toxic from a proxy.
func (client *Client) RemoveToxic(ctx context.Context, proxy, name string) error {
	return client.request(ctx, http.MethodDelete, "/proxies/"+url.PathEscape(proxy)+"/toxics/"+url.PathEscape(name), nil)
}

// DeleteProxy removes a proxy.
func (client *Client) DeleteProxy(ctx context.Context, name string) error {
	return client.request(ctx, http.MethodDelete, "/proxies/"+url.PathEscape(name), nil)
}

func (client *Client) request(ctx context.Context, method, path string, payload any) error {
	var body io.Reader
	if payload != nil {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		body = bytes.NewReader(encoded)
	}

	request, err := http.NewRequestWithContext(ctx, method, client.endpoint+path, body)
	if err != nil {
		return err
	}
	if payload != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return err
	}
	defer response.Body.Close()

	if response.StatusCode < http.StatusOK || response.StatusCode >= http.StatusMultipleChoices {
		return responseError(response)
	}
	_, _ = io.Copy(io.Discard, response.Body)
	return nil
}

func responseError(response *http.Response) error {
	body, _ := io.ReadAll(io.LimitReader(response.Body, maxErrorBody))
	if text := strings.TrimSpace(string(body)); text != "" {
		return fmt.Errorf("toxiproxy request failed: %s: %s", response.Status, text)
	}
	return fmt.Errorf("toxiproxy request failed: %s", response.Status)
}
