package server

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/ptr"
)

func getCurrentUser(ctx context.Context, c client.ClientWithResponsesInterface) (*client.UserRead, error) {
	resp, err := c.GetCurrentUserWithResponse(ctx, nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return &resp.ApplicationvndApiJSON200.Data, nil
	case 400:
		return nil, fmt.Errorf("bad request: %s, %s", *resp.JSON400.Exception, *resp.JSON400.Message)
	case 401:
		return nil, fmt.Errorf("unauthenticated: %s, %s", *resp.JSON401.Exception, *resp.JSON401.Message)
	case 500:
		return nil, fmt.Errorf("server exception: %s, %s", *resp.JSON500.Exception, *resp.JSON500.Message)
	default:
		return nil, fmt.Errorf("unknown error, response body: %s", string(resp.Body))
	}
}

func getAnyTransaction(ctx context.Context, c client.ClientWithResponsesInterface) (*client.TransactionRead, error) {
	logger.Debug("querying an arbitrary transaction")
	resp, err := c.ListTransactionWithResponse(ctx, &client.ListTransactionParams{
		Limit: ptr.Of(int32(1)),
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), string(resp.Body))
	}
	if len(resp.ApplicationvndApiJSON200.Data) == 0 {
		return nil, errors.New("no transactions found")
	}

	return &resp.ApplicationvndApiJSON200.Data[0], nil
}

func queryAllWebhooks(ctx context.Context, c client.ClientWithResponsesInterface) ([]client.WebhookRead, error) {
	logger.Debug("querying all webhooks")
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
	logger.Debug("creating webhook")
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
	logger.Debug("updating webhook")
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
