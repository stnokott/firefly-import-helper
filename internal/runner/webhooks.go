package runner

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/ptr"
)

func queryAllWebhooks(ctx context.Context, c client.ClientWithResponsesInterface) ([]client.WebhookRead, error) {
	slog.Debug("querying all webhooks")
	resp, err := c.ListWebhookWithResponse(ctx, &client.ListWebhookParams{
		Limit: ptr.Of(int32(50)),
		Page:  ptr.Of(int32(1)),
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return resp.ApplicationvndApiJSON200.Data, nil
}

func createWebhook(ctx context.Context, c client.ClientWithResponsesInterface) (string, error) {
	slog.Debug("creating webhook")
	resp, err := c.StoreWebhookWithResponse(ctx, nil, client.WebhookStore{
		Active: ptr.Of(true),
		Title:  webhookTitle,
		Triggers: &client.WebhookTriggerArray{
			client.STORETRANSACTION,
		},
		Responses: &client.WebhookResponseArray{
			client.TRANSACTIONS,
		},
		Deliveries: &client.WebhookDeliveryArray{
			client.JSON,
		},
		Url: webhookURL,
	})
	if err != nil {
		return "", err
	}
	if resp.StatusCode() != 200 {
		return "", fmt.Errorf("HTTP %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return resp.ApplicationvndApiJSON200.Data.Id, nil
}

func updateWebhook(ctx context.Context, c client.ClientWithResponsesInterface, id string) error {
	slog.Debug("updating webhook")
	resp, err := c.UpdateWebhookWithResponse(ctx, id, nil, client.WebhookUpdate{
		Active: ptr.Of(true),
		Title:  ptr.Of(webhookTitle),
		Triggers: &client.WebhookTriggerArray{
			client.STORETRANSACTION,
		},
		Responses: &client.WebhookResponseArray{
			client.TRANSACTIONS,
		},
		Deliveries: &client.WebhookDeliveryArray{
			client.JSON,
		},
		Url: ptr.Of(webhookURL),
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return nil
}

func triggerWebhook(ctx context.Context, c client.ClientWithResponsesInterface, webhookID string, transactionID string) error {
	resp, err := c.TriggerTransactionWebhookWithResponse(ctx, webhookID, transactionID, nil, func(_ context.Context, req *http.Request) error {
		req.Header.Add("Content-Type", "application/json")
		return nil
	})
	if err != nil {
		return err
	}
	if resp.StatusCode() != 204 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode(), string(resp.Body))
	}
	return nil
}
