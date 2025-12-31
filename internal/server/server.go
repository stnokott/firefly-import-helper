// Package server listens for webhook payloads sent by the Firefly III server.
package server

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"time"
)

type Server struct {
	port          int
	interceptFunc InterceptFunc
}

func New(port int, opts ...Option) *Server {
	s := &Server{
		port: port,
	}
	for _, opt := range opts {
		opt(s)
	}
	return s
}

type Option func(*Server)

func WithIntercept(interceptFunc InterceptFunc) Option {
	return func(s *Server) {
		s.interceptFunc = interceptFunc
	}
}

type InterceptFunc func(WebhookResponse)

func (s *Server) Serve(ctx context.Context) error {
	slog.Info(fmt.Sprintf("starting webhook listener on port %d", s.port))
	srv := &http.Server{
		Addr:    fmt.Sprintf("0.0.0.0:%d", s.port),
		Handler: s,
	}
	go func() {
		<-ctx.Done()
		slog.Info("stopping webhook listener")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdownCtx)
	}()
	err := srv.ListenAndServe()
	if err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	slog.Debug("webhook listener gracefully stopped")
	return nil
}

func (s *Server) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	slog.Debug("received request on webhook endpoint")
	if req.Method != http.MethodPost {
		slog.Warn(fmt.Sprintf("received webhook with method %s, ignoring", req.Method))
		rw.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	var data WebhookResponse
	if err := json.NewDecoder(req.Body).Decode(&data); err != nil {
		slog.Error(fmt.Sprintf("could not decode webhook content: %v", err))
		rw.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Debug("got webhook data:")
	slog.Debug(">> UUID: " + data.UUID.String())
	slog.Debug(">> no. Transactions: " + strconv.Itoa(len(data.Content.Transactions)))

	if s.interceptFunc != nil {
		s.interceptFunc(data)
	}
	rw.WriteHeader(http.StatusOK)
}
