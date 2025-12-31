package client

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
)

//go:generate go tool oapi-codegen -config .oapi-codegen.yaml firefly-iii-6.4.14-v1.yaml

func WithAccessToken(tok string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+tok)
		return nil
	})
}

func WithRequestLogger() ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		slog.Debug(fmt.Sprintf(">> %s %v %#v", req.Method, req.URL, req.URL.Query()))
		return nil
	})
}
