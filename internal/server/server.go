// Package server listens for webhook payloads sent by the Firefly III server.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
)

var logger = log.For("server")

type server struct {
	port     int
	dataChan chan<- *domain.Transaction
	t        telegram.Bot
}

func newServer(port int, dataChan chan<- *domain.Transaction, t telegram.Bot) *server {
	s := &server{
		port:     port,
		dataChan: dataChan,
		t:        t,
	}
	return s
}

func (s *server) Serve(ctx context.Context) error {
	logger.Debugf("starting webhook listener on port %d", s.port)
	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.port),
		Handler: s,
	}
	go func() {
		<-ctx.Done()
		logger.Debug("context closed, stopping webhook listener")
		// TODO: delete webhook on graceful shutdown
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if shutdownErr := srv.Shutdown(shutdownCtx); shutdownErr != nil {
			logger.ErrorV(fmt.Errorf("error during test server shutdown: %w", shutdownErr))
		}
		close(s.dataChan)
	}()
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	logger.Debug("webhook listener gracefully stopped")
	return nil
}

func (s *server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	logger.Debug("received request on webhook endpoint")
	if req.Method != http.MethodPost {
		logger.Warnf("received webhook with method %s, ignoring", req.Method)
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	defer req.Body.Close()

	data := new(WebhookResponse)
	if err := json.NewDecoder(req.Body).Decode(data); err != nil {
		logger.ErrorV(fmt.Errorf("could not decode webhook content: %w", err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	logger.Debug("got webhook data:")
	logger.Debug(">> UUID: " + data.UUID.String())
	logger.Debug(">> no. Transactions: " + strconv.Itoa(len(data.Content.SubTransactions)))
	s.dataChan <- ConvertWebhookResponse(data)

	rw.WriteHeader(http.StatusOK)
}
