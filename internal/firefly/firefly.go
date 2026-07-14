package firefly

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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
	api generated.ClientInterface
}

func New(baseURL url.URL, token string) (domain.FireflyConnector, error) {
	api, err := generated.NewClient(
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

func (c *Client) ListAssetAccounts(ctx context.Context) (domain.FireflyAccounts, error) {
	requestFunc := func(page int32) (*http.Response, error) {
		return c.api.ListAccount(ctx, &generated.ListAccountParams{
			Page: new(page),
			Type: new(generated.AccountTypeFilterAssetAccount),
		})
	}
	accounts, err := paginatedRequest[generated.AccountRead](requestFunc)
	if err != nil {
		return nil, err
	}
	return ConvertAccounts(accounts), nil
}

func paginatedRequest[V any](get func(page int32) (*http.Response, error)) ([]V, error) {
	var result []V
	for page := int32(1); ; page++ {
		resp, err := get(page)
		if err != nil {
			return nil, err
		}

		if resp.StatusCode != 200 {
			return nil, unmarshalErr(resp)
		}

		parsed, err := unmarshalResp[V](resp)
		if err != nil {
			return nil, fmt.Errorf("page %d: %w", page, err)
		}
		if result == nil {
			if parsed.Meta != nil && parsed.Meta.Pagination != nil && parsed.Meta.Pagination.Total != nil {
				result = make([]V, 0, *parsed.Meta.Pagination.Total)
			} else {
				result = make([]V, 0)
			}
		}

		result = append(result, parsed.Data...)

		if parsed.Meta == nil ||
			parsed.Meta.Pagination == nil ||
			parsed.Meta.Pagination.CurrentPage == nil ||
			parsed.Meta.Pagination.TotalPages == nil {
			logger.Warn("no pagination metadata found")
			return result, nil
		}
		if *parsed.Meta.Pagination.CurrentPage >= *parsed.Meta.Pagination.TotalPages {
			return result, nil
		}
	}
}

type paginatedResponse[V any] struct {
	Data []V             `json:"data"`
	Meta *generated.Meta `json:"meta"`
}

func unmarshalResp[V any](resp *http.Response) (*paginatedResponse[V], error) {
	defer resp.Body.Close()
	result := new(paginatedResponse[V])
	if err := json.NewDecoder(resp.Body).Decode(result); err != nil {
		return nil, fmt.Errorf("could not unmarshal response body for status %d: %w", resp.StatusCode, err)
	}
	return result, nil
}

func unmarshalErr(resp *http.Response) error {
	type ErrResult struct {
		Message   *string `json:"message"`
		Exception *string `json:"exception"`
	}

	defer resp.Body.Close()
	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("could not read response body for status %d: %w", resp.StatusCode, err)
	}

	e := new(ErrResult)
	if err := json.Unmarshal(respBytes, e); err != nil {
		return fmt.Errorf("could not unmarshal response JSON for status %d: %w - '%s'", resp.StatusCode, err, string(respBytes))
	}

	return errGenericResponse(resp.Status, e.Exception, e.Message)
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
