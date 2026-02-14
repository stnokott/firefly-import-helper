package client

import (
	"context"
	"net/http"

	"github.com/stnokott/firefly-import-helper/internal/log"
)

//go:generate go tool oapi-codegen -config .oapi-codegen.yaml firefly-iii-6.4.14-v1.yaml

func WithAccessToken(tok string) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		req.Header.Add("Authorization", "Bearer "+tok)
		return nil
	})
}

func WithRequestLogger(logger log.Logger) ClientOption {
	return WithRequestEditorFn(func(_ context.Context, req *http.Request) error {
		logger.Debugf(">> %s %v %#v", req.Method, req.URL, req.URL.Query())
		return nil
	})
}
