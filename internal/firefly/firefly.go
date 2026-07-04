package firefly

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/firefly/generated"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

//go:generate go tool oapi-codegen -config .oapi-codegen.yaml firefly-iii-6.4.14-v1.yaml

var logger = log.For("firefly")

type Client struct {
	api generated.ClientWithResponsesInterface
}

func New(baseURL url.URL, token string) (domain.FireflyConnector, error) {
	api, err := generated.NewClientWithResponses(
		baseURL.JoinPath("/api").String(),
		generated.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+token)
			req.Header.Add("User-Agent", "firefly-importer-helper") // TODO: add version from goreleaser
			logger.Debugf(">> %s %v?%s", req.Method, req.URL, req.URL.RawQuery)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create API client: %w", err)
	}

	return &Client{
		api: api,
	}, nil
}

func (c *Client) ListAccounts(ctx context.Context) ([]domain.FireflyAccount, error) {
	resp, err := c.api.ListAccountWithResponse(ctx, nil)
	if err != nil {
		return nil, err
	}
	switch resp.StatusCode() {
	case 200:
		return ConvertAccounts(resp.ApplicationvndApiJSON200.Data), nil
	case 400:
		return nil, errBadRequest(resp.JSON400)
	case 401:
		return nil, errUnauthenticated(resp.JSON401)
	case 404:
		return nil, errNotFound(resp.JSON404)
	case 500:
		return nil, errInternal(resp.JSON500)
	default:
		return nil, fmt.Errorf("unhandled status code %d in ListAccounts", resp.StatusCode())
	}
}

func errBadRequest(resp *generated.BadRequestResponse) error {
	return errGenericResponse("bad request", resp.Exception, resp.Message)
}

func errUnauthenticated(resp *generated.UnauthenticatedResponse) error {
	return errGenericResponse("unauthenticated", resp.Exception, resp.Message)
}

func errNotFound(resp *generated.NotFoundResponse) error {
	return errGenericResponse("not found", resp.Exception, resp.Message)
}

func errInternal(resp *generated.InternalExceptionResponse) error {
	return errGenericResponse("internal error", resp.Exception, resp.Message)
}

func errGenericResponse(exceptionType string, exception, message *string) error {
	infos := make([]string, 0, 2)
	if exception != nil {
		infos = append(infos, *exception)
	}
	if message != nil {
		infos = append(infos, *message)
	}
	return fmt.Errorf("%s: %s", exceptionType, strings.Join(infos, " - "))
}
