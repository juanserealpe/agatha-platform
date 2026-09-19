package dashboard

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
}

func NewClient(
	httpClient *http.Client,
	baseURL string,
) *Client {
	return &Client{
		httpClient: httpClient,
		baseURL:    baseURL,
	}
}

func (c *Client) Render(ctx context.Context,request RenderRequest,
) (*RenderResponse, error) {

	body, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf(
			"marshal dashboard request: %w",
			err,
		)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.baseURL+"/api/v2/dashboard/configurations/10/render",
		bytes.NewReader(body),
	)
	if err != nil {
		return nil, fmt.Errorf(
			"create dashboard request: %w",
			err,
		)
	}

	req.Header.Set("Accept", "*/*")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf(
			"execute dashboard request: %w",
			err,
		)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK ||
		resp.StatusCode >= http.StatusMultipleChoices {
		return nil, fmt.Errorf(
			"DataOS dashboard request failed with status %d",
			resp.StatusCode,
		)
	}

	var result RenderResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf(
			"decode dashboard response: %w",
			err,
		)
	}

	return &result, nil
}
