package dataos

import (
	"context"

	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/auth"
	"github.com/juanserealpe/agatha-platform/backend/internal/modules/ingestion/infrastructure/adapters/dataos/dashboard"
)

type Client struct {
	authClient      *auth.Client
	dashboardClient *dashboard.Client
}

func NewClient(
	authClient *auth.Client,
	dashboardClient *dashboard.Client,
) *Client {
	return &Client{
		authClient:      authClient,
		dashboardClient: dashboardClient,
	}
}

func (c *Client) Login(ctx context.Context) error {
	return c.authClient.Login(ctx)
}

func (c *Client) Render(
	ctx context.Context,
	request dashboard.RenderRequest,
) (*dashboard.RenderResponse, error) {
	return c.dashboardClient.Render(ctx, request)
}
