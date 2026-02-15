package server

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
	"golang.org/x/sync/errgroup"
)

const (
	webhookTitle = "firefly-import-helper"
	webhookPort  = 8888
	webhookURL   = "http://192.168.178.58:8888"
	setupTimeout = 60 * time.Second
)

// Setup ensures the webhook exists and validates that the server can receive data from the webhook.
// TODO: refactor. use server instance
func Setup(ctx context.Context, c client.ClientWithResponsesInterface, t telegram.Bot) error {
	ctxSetup, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	user, err := getCurrentUser(ctx, c)
	if err != nil {
		return fmt.Errorf("failed to get current authenticated user: %w", err)
	}
	logger.Infof("authenticated as %s", domain.CensorEmail(user.Attributes.Email))

	logger.Info("setting up webhook")
	webhookID, err := setupWebhook(ctxSetup, c)
	if err != nil {
		return fmt.Errorf("webhook setup failed: %w", err)
	}

	logger.Info("validating webhook")
	if err := validateWebhook(ctxSetup, c, webhookID); err != nil {
		return fmt.Errorf("webhook validation failed: %w", err)
	}
	logger.Info("webhook validated")
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

func validateWebhook(ctx context.Context, c client.ClientWithResponsesInterface, webhookID string) error {
	logger.Debug("querying sample transaction")
	sampleTransaction, err := getAnyTransaction(ctx, c)
	if err != nil {
		return fmt.Errorf("could not query sample transaction: %w", err)
	}

	var matched bool
	dataChan := make(chan *domain.Transaction)
	logger.Debug("starting test server")
	srv := newServer(webhookPort, dataChan, telegram.NewNoopBot())

	ctxServer, cancelServer := context.WithCancel(ctx)
	defer cancelServer()

	var eg errgroup.Group
	eg.Go(func() error {
		select {
		case <-ctxServer.Done():
			logger.Debug("server context closed")
			return nil
		case data, ok := <-dataChan:
			if !ok {
				// channel closed
				return nil
			}
			logger.Debugf("intercepted webhook payload %s with %d transactions", data.ID, len(data.SubTransactions))
			if strconv.Itoa(data.ID) == sampleTransaction.Id {
				matched = true
			} else {
				logger.Debugf("webhook data ID mismatch, want %s, got %d", sampleTransaction.Id, data.ID)
			}
			cancelServer() // no further data expected, can shut down server
		}
		return nil
	})
	eg.Go(func() error {
		if err = srv.Serve(ctxServer); err != nil {
			return fmt.Errorf("could not start test server: %w", err)
		}
		return nil
	})

	time.Sleep(1 * time.Second)
	logger.Debug("triggering webhook on sample transaction")
	if err := triggerWebhook(ctxServer, c, webhookID, sampleTransaction.Id); err != nil {
		return fmt.Errorf("could  not trigger webhook: %w", err)
	}

	// wait for server shutdown, should occur either after timeout or on data match
	if err = eg.Wait(); err != nil {
		return err
	}

	if matched {
		return nil
	} else {
		return fmt.Errorf("no matching webhook message within %v", setupTimeout)
	}
}

func Run(ctx context.Context, dataChan chan<- *domain.Transaction, t telegram.Bot) error {
	logger.Info("starting webhook listener")
	srv := newServer(webhookPort, dataChan, t)
	return srv.Serve(ctx)
}
