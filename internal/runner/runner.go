// Package runner is responsible for the main application loop, including webhook and server management.
package runner

import (
	"context"
	"fmt"
	"log/slog"
	"slices"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/server"
)

const (
	webhookTitle = "firefly-import-helper"
	webhookPort  = 8888
	webhookURL   = "http://192.168.178.22:8888"
)

func Run(ctx context.Context, c client.ClientWithResponsesInterface) error {
	slog.Info("setting up webhook")
	webhookID, err := setupWebhook(ctx, c)
	if err != nil {
		return fmt.Errorf("webhook setup failed: %w", err)
	}

	slog.Info("validating webhook")
	if err := validateWebhook(ctx, c, webhookID); err != nil {
		return fmt.Errorf("webhook validation failed: %w", err)
	}

	return nil
}

func setupWebhook(ctx context.Context, c client.ClientWithResponsesInterface) (string, error) {
	// query all webhooks
	webhooks, err := queryAllWebhooks(ctx, c)
	if err != nil {
		return "", fmt.Errorf("could not query webhooks: %w", err)
	}

	// check if webhook already exists
	whIndex := slices.IndexFunc(webhooks, func(e client.WebhookRead) bool {
		return e.Attributes.Title == webhookTitle
	})
	var webhookID string
	if whIndex != -1 {
		webhookID = webhooks[whIndex].Id
		if err = updateWebhook(ctx, c, webhookID); err != nil {
			return "", fmt.Errorf("could not update webook: %w", err)
		}
	} else {
		if webhookID, err = createWebhook(ctx, c); err != nil {
			return "", fmt.Errorf("could not create webhook: %w", err)
		}
	}

	return webhookID, nil
}

const webhookValidationTimeout = 60 * time.Second

func validateWebhook(ctx context.Context, c client.ClientWithResponsesInterface, webhookID string) error {
	slog.Debug("querying sample transaction")
	sampleTransaction, err := getAnyTransaction(ctx, c)
	if err != nil {
		return fmt.Errorf("could not query sample transaction: %w", err)
	}

	ctxServer, cancel := context.WithTimeout(ctx, webhookValidationTimeout)
	defer cancel()
	var matched atomic.Bool
	webhookIntercept := func(data server.WebhookResponse) {
		slog.Debug(fmt.Sprintf("intercepted webhook payload %s with %d transactions", data.UUID, len(data.Content.Transactions)))
		if strconv.Itoa(data.Content.ID) == sampleTransaction.Id {
			matched.Store(true)
		} else {
			slog.Debug(fmt.Sprintf("webhook data ID mismatch, want %s, got %d", sampleTransaction.Id, data.Content.ID))
		}
		cancel() // no further message expected, server can be shut down
	}
	slog.Debug("starting test server")
	srv := server.New(webhookPort, server.WithIntercept(webhookIntercept))
	var wg sync.WaitGroup
	wg.Go(func() {
		if err = srv.Serve(ctxServer); err != nil {
			slog.Error("could not start test server: " + err.Error())
		}
	})

	time.Sleep(1 * time.Second)
	slog.Debug("triggering webhook on sample transaction")
	if err := triggerWebhook(ctx, c, webhookID, sampleTransaction.Id); err != nil {
		return fmt.Errorf("could  not trigger webhook: %w", err)
	}

	// wait for server shutdown, should occur either after timeout or on data match
	wg.Wait()

	if success := matched.Load(); success {
		return nil
	} else {
		return fmt.Errorf("no matching webhook message within %v", webhookValidationTimeout)
	}
}
