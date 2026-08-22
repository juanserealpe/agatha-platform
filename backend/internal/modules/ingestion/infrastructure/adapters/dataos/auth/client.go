package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Client struct {
	httpClient *http.Client
	baseURL    string
	username   string
	password   string
}

func NewClient(
	httpClient *http.Client,
	baseURL string,
	username string,
	password string,
) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
		username:   username,
		password:   password,
	}
}

func (c *Client) Login(ctx context.Context) error {
	payload := LoginRequest{
		Username:   c.username,
		Password:   c.password,
		RememberMe: false,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal login request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/auth/login",
		bytes.NewReader(body),
	)
	if err != nil {
		return fmt.Errorf("create login request: %w", err)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("execute login request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf(
			"DataOS login failed with status %d",
			resp.StatusCode,
		)
	}

	return nil
}
