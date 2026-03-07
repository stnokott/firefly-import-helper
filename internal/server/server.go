// Package server listens for webhook payloads sent by the Firefly III server.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/log"
	"golang.org/x/sync/errgroup"
)

var logger = log.For("server")

const (
	webhookTitle = "firefly-import-helper"
	setupTimeout = 60 * time.Second
)

type Server struct {
	port       int
	webhookURL string
	dataChan   chan<- *domain.Transaction
	api        client.ClientWithResponsesInterface
}

func NewServer(dataChan chan<- *domain.Transaction, c client.ClientWithResponsesInterface) (*Server, error) {
	// assemble webhook URL from external URL + port
	externalURL := config.C.ExternalURL
	if externalURL.Port() != "" {
		return nil, fmt.Errorf("external URL must not contain port, got '%s'", externalURL.String())
	}
	externalURL.Host = fmt.Sprintf("%s:%d", externalURL.Host, config.C.FireflyWebhookPort)
	s := &Server{
		port:       config.C.FireflyWebhookPort,
		webhookURL: externalURL.String(),
		dataChan:   dataChan,
		api:        c,
	}
	return s, nil
}

// Setup ensures the webhook exists and validates that the server can receive data from the webhook.
//
// It should return a nil error before [Run] is called.
func (s *Server) Setup(ctx context.Context) error {
	ctxSetup, cancel := context.WithTimeout(ctx, setupTimeout)
	defer cancel()

	user, err := s.getCurrentUser(ctx)
	if err != nil {
		return fmt.Errorf("failed to get current authenticated user: %w", err)
	}
	logger.Infof("authenticated as %s", domain.CensorEmail(user.Attributes.Email))

	logger.Info("setting up webhook")
	webhookID, err := s.setupWebhook(ctxSetup)
	if err != nil {
		return fmt.Errorf("webhook setup failed: %w", err)
	}

	logger.Info("validating webhook")
	if err := s.validateWebhook(ctxSetup, webhookID); err != nil {
		return fmt.Errorf("webhook validation failed: %w", err)
	}
	logger.Info("webhook validated")
	return nil
}

func (s *Server) setupWebhook(ctx context.Context) (string, error) {
	// query all webhooks
	webhooks, err := s.queryAllWebhooks(ctx)
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
		if err = s.updateWebhook(ctx, webhookID); err != nil {
			return "", fmt.Errorf("could not update webook: %w", err)
		}
	} else {
		if webhookID, err = s.createWebhook(ctx); err != nil {
			return "", fmt.Errorf("could not create webhook: %w", err)
		}
	}

	return webhookID, nil
}

func (s *Server) validateWebhook(ctx context.Context, webhookID string) error {
	logger.Debug("querying sample transaction")
	sampleTransaction, err := s.getAnyTransaction(ctx)
	if err != nil {
		return fmt.Errorf("could not query sample transaction: %w", err)
	}

	var matched bool
	dataChan := make(chan *domain.Transaction)
	logger.Debug("starting test server")

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
			logger.Debugf("intercepted webhook payload %d with %d transactions", data.ID, len(data.SubTransactions))
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
		// TODO: once Telegram bot is added for server, use noop-bot for this serve
		if err = serve(ctxServer, s.port, dataChan); err != nil {
			return fmt.Errorf("could not start test server: %w", err)
		}
		return nil
	})

	time.Sleep(1 * time.Second)
	logger.Debug("triggering webhook on sample transaction")
	if err := s.triggerWebhook(ctxServer, webhookID, sampleTransaction.Id); err != nil {
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

func (s *Server) Run(ctx context.Context) error {
	// TODO: delete webhook on graceful shutdown
	return serve(ctx, s.port, s.dataChan)
}

func serve(ctx context.Context, port int, dataChan chan<- *domain.Transaction) error {
	logger.Debugf("starting webhook listener on port %d", port)
	srv := &http.Server{
		Addr: fmt.Sprintf("0.0.0.0:%d", port),
		Handler: &webhookListener{
			dataChan: dataChan,
		},
	}
	go func() {
		<-ctx.Done()
		logger.Debug("context closed, stopping webhook listener")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			logger.ErrorV(fmt.Errorf("error during test server shutdown: %w", shutdownErr))
		}
		close(dataChan)
	}()
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Debug("webhook listener gracefully stopped")
	return nil
}

type webhookListener struct {
	dataChan chan<- *domain.Transaction
}

func (lis *webhookListener) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	logger.Debug("received request on webhook endpoint")
	if req.Method != http.MethodPost {
		logger.Warnf("received webhook with method %s, ignoring", req.Method)
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	defer req.Body.Close() //nolint:errcheck // read-only response, no errcheck required

	data := new(WebhookResponse)
	if err := json.NewDecoder(req.Body).Decode(data); err != nil {
		logger.ErrorV(fmt.Errorf("could not decode webhook content: %w", err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Debug("got webhook data:")
	logger.Debug(">> UUID: " + data.UUID.String())
	logger.Debug(">> no. Transactions: " + strconv.Itoa(len(data.Content.SubTransactions)))
	lis.dataChan <- ConvertWebhookResponse(data)

	rw.WriteHeader(http.StatusOK)
}
