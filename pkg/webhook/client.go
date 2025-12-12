package webhook

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/internal/domain"
	"github.com/onurcolak/insider-message-service/pkg/logger"
)

type Client struct {
	httpClient *resty.Client
	webhookURL string
}

func NewWebhookClient(cfg environments.WebhookConfig) *Client {
	client := resty.New().
		SetTimeout(cfg.Timeout).
		SetRetryCount(3).
		SetRetryWaitTime(500*time.Millisecond).
		SetRetryMaxWaitTime(2*time.Second).
		SetHeader("Content-Type", "application/json").
		SetHeader("Accept", "application/json").
		SetHeader("x-ins-auth-key", cfg.AuthKey)

	return &Client{
		httpClient: client,
		webhookURL: cfg.URL,
	}
}

func (c *Client) SendMessage(ctx context.Context, phoneNumber, content string) (*domain.WebhookResponse, error) {
	// Prepare request payload
	payload := domain.WebhookRequest{
		To:      phoneNumber,
		Content: content,
	}

	var webhookResp domain.WebhookResponse

	startTime := time.Now()

	resp, err := c.httpClient.R().
		SetContext(ctx).
		SetBody(payload).
		SetResult(&webhookResp).
		Post(c.webhookURL)

	duration := time.Since(startTime)

	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}

	logger.Infof("Webhook request to %s completed in %v (status: %d)", c.webhookURL, duration, resp.StatusCode())

	if resp.StatusCode() != http.StatusAccepted {
		return nil, fmt.Errorf("unexpected status code: %d (expected 202), body: %s", resp.StatusCode(), resp.String())
	}

	return &webhookResp, nil
}

func (c *Client) GetURL() string {
	return c.webhookURL
}
