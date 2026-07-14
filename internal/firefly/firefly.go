package firefly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/oapi-codegen/nullable"
	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/firefly/generated"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

//go:generate go tool oapi-codegen -config .oapi-codegen.yaml firefly-iii-6.4.14-v1.yaml

var logger = log.For("firefly")

type client struct {
	api generated.ClientWithResponsesInterface
}

type FireflyReadWriter interface {
	domain.FireflyReader
	domain.FireflyWriter
}

func New(baseURL url.URL, token string) (FireflyReadWriter, error) {
	api, err := generated.NewClientWithResponses(
		baseURL.JoinPath("/api").String(),
		generated.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+token)
			req.Header.Add("User-Agent", config.AppName)
			logger.Debugf(">> %s %v?%s", req.Method, req.URL, req.URL.RawQuery)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create API client: %w", err)
	}

	return &client{
		api: api,
	}, nil
}

func (c *client) ListAssetAccounts(ctx context.Context) (domain.FireflyAccounts, error) {
	requestFunc := func(page int32) (int, []byte, error) {
		resp, err := c.api.ListAccountWithResponse(ctx, &generated.ListAccountParams{
			Page: new(page),
			Type: new(generated.AccountTypeFilterAssetAccount),
		})
		return resp.StatusCode(), resp.Body, err
	}
	accounts, err := paginatedRequest[generated.AccountRead](requestFunc)
	if err != nil {
		return nil, err
	}
	return ConvertAccounts(accounts), nil
}

func (c *client) CreateTransaction(ctx context.Context, accountID string, t domain.BankTransaction) (string, error) {
	converted := ConvertTransaction(t, accountID)
	resp, err := c.api.StoreTransactionWithResponse(ctx, nil, generated.StoreTransactionJSONRequestBody{
		ApplyRules:           new(true),
		ErrorIfDuplicateHash: new(true),
		GroupTitle:           nullable.NewNullNullable[string](),
		Transactions:         []generated.TransactionSplitStore{converted},
	})
	if err != nil {
		return "", fmt.Errorf("could not create transaction: %w", err)
	}
	if resp.StatusCode() != 200 {
		return "", errGenericResponse(resp.Status(), resp.Body)
	}
	return resp.ApplicationvndApiJSON200.Data.Id, nil
}

func paginatedRequest[V any](get func(page int32) (status int, body []byte, err error)) ([]V, error) {
	var result []V
	for page := int32(1); ; page++ {
		status, resp, err := get(page)
		if err != nil {
			return nil, err
		}

		if status != 200 {
			return nil, errGenericResponse(http.StatusText(status), resp)
		}

		type paginatedResponse[V any] struct {
			Data []V             `json:"data"`
			Meta *generated.Meta `json:"meta"`
		}

		parsed := new(paginatedResponse[V])
		if err := json.Unmarshal(resp, parsed); err != nil {
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

func errGenericResponse(statusText string, body []byte) error {
	var e struct {
		Message   *string `json:"message"`
		Exception *string `json:"exception"`
	}
	if err := json.Unmarshal(body, &e); err != nil {
		return fmt.Errorf("could not unmarshal response JSON for status %s: %w - '%s'", statusText, err, string(body))
	}

	infos := make([]string, 0, 2)
	if e.Exception != nil {
		infos = append(infos, *e.Exception)
	}
	if e.Message != nil {
		infos = append(infos, *e.Message)
	}
	return fmt.Errorf("%s: %s", statusText, strings.Join(infos, " - "))
}
