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

	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("server")

type server struct {
	port     int
	dataChan chan<- *WebhookResponse
}

func newServer(port int, dataChan chan<- *WebhookResponse) *server {
	s := &server{
		port:     port,
		dataChan: dataChan,
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
	// data, _ := io.ReadAll(req.Body)
	// logger.Debug(string(data))

	logger.Debug("got webhook data:")
	logger.Debug(">> UUID: " + data.UUID.String())
	logger.Debug(">> no. Transactions: " + strconv.Itoa(len(data.Content.Transactions)))
	s.dataChan <- data

	rw.WriteHeader(http.StatusOK)
}
