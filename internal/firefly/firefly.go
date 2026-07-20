// Package firefly interacts with the Firefly instance via its REST API.
package firefly

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/oapi-codegen/nullable"
	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/firefly/generated"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

//go:generate go tool oapi-codegen -config .oapi-codegen.yaml firefly-iii-6.8.6-v1.yaml

var logger = log.For("firefly")

type client struct {
	api     generated.ClientWithResponsesInterface
	baseURL url.URL
}

func New(baseURL url.URL, token string) (domain.FireflyReadWriter, error) {
	api, err := generated.NewClientWithResponses(
		baseURL.JoinPath("/api").String(),
		generated.WithRequestEditorFn(func(ctx context.Context, req *http.Request) error {
			req.Header.Add("Authorization", "Bearer "+token)
			req.Header.Add("Accept", "application/json")
			req.Header.Add("User-Agent", config.AppName)
			logger.Debugf(">> %s %v?%s", req.Method, req.URL, req.URL.RawQuery)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("could not create API client: %w", err)
	}

	return &client{
		api:     api,
		baseURL: baseURL,
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

func (c *client) ListCategories(ctx context.Context) ([]string, error) {
	requestFunc := func(page int32) (int, []byte, error) {
		resp, err := c.api.ListCategoryWithResponse(ctx, &generated.ListCategoryParams{
			Page: new(page),
		})
		return resp.StatusCode(), resp.Body, err
	}
	categoriesFull, err := paginatedRequest[generated.CategoryRead](requestFunc)
	if err != nil {
		return nil, err
	}
	categories := make([]string, len(categoriesFull))
	for i := range categoriesFull {
		categories[i] = categoriesFull[i].Attributes.Name
	}
	return categories, nil
}

func (c *client) getTransactionByID(ctx context.Context, id string) (*generated.TransactionRead, error) {
	resp, err := c.api.GetTransactionWithResponse(ctx, id, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to query transaction %q: %w", id, err)
	}

	if resp.StatusCode() != http.StatusOK {
		return nil, errFromResponse(resp.StatusCode(), resp.Body)
	}
	return &resp.ApplicationvndApiJSON200.Data, nil
}

func (c *client) CreateTransaction(ctx context.Context, accountID string, t domain.BankTransaction) (*domain.TransactionRead, error) {
	// validate we dont have the same external ID in Firefly already
	if existingTransaction, err := c.searchTransactionByExternalID(ctx, t.ID); err != nil {
		return nil, fmt.Errorf("failed to search for transaction by external ID %s: %w", t.ID, err)
	} else if existingTransaction != nil {
		return nil, DuplicateTransactionError{
			DuplicateOf:  existingTransaction.Id,
			byExternalID: true,
		}
	}

	converted := ConvertTransaction(t, accountID)
	resp, err := c.api.StoreTransactionWithResponse(ctx, nil, generated.StoreTransactionJSONRequestBody{
		ApplyRules:           new(true),
		ErrorIfDuplicateHash: new(true),
		GroupTitle:           nullable.NewNullNullable[string](),
		Transactions:         []generated.TransactionSplitStore{*converted},
	})
	if err != nil {
		return nil, fmt.Errorf("could not create transaction: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, errFromResponse(resp.StatusCode(), resp.Body)
	}
	created := ConvertTransactionRead(&resp.ApplicationvndApiJSON200.Data, c.baseURL)
	return created, nil
}

func (c *client) UpdateTransactionCategory(ctx context.Context, transactionID string, category string) (*domain.TransactionRead, error) {
	// For update, we need transaction journal IDs, so need to query full transaction first.
	// See https://docs.firefly-iii.org/references/firefly-iii/api/specials/#transaction-update.
	current, err := c.getTransactionByID(ctx, transactionID)
	if err != nil {
		return nil, err
	}

	updateSplits := make([]generated.TransactionSplitUpdate, len(current.Attributes.Transactions))
	for i := range current.Attributes.Transactions {
		updateSplits[i] = generated.TransactionSplitUpdate{
			TransactionJournalId: current.Attributes.Transactions[i].TransactionJournalId,
			CategoryName:         nullable.NewNullableWithValue(category),
		}
	}

	update := generated.TransactionUpdate{
		ApplyRules:   new(false),
		FireWebhooks: new(false),
		GroupTitle:   current.Attributes.GroupTitle,
		Transactions: &updateSplits,
	}
	resp, err := c.api.UpdateTransactionWithResponse(ctx, transactionID, nil, update)
	if err != nil {
		return nil, fmt.Errorf("failed to update transaction: %w", err)
	}
	if resp.StatusCode() != http.StatusOK {
		return nil, errFromResponse(resp.StatusCode(), resp.Body)
	}

	return ConvertTransactionRead(&resp.ApplicationvndApiJSON200.Data, c.baseURL), nil
}

func (c *client) searchTransactionByExternalID(ctx context.Context, extID string) (*generated.TransactionRead, error) {
	requestFunc := func(page int32) (int, []byte, error) {
		resp, err := c.api.SearchTransactionsWithResponse(ctx, &generated.SearchTransactionsParams{
			Limit: new(int32(1)),
			Query: "external_id_is:" + extID,
		})
		return resp.StatusCode(), resp.Body, err
	}
	transactions, err := paginatedRequest[generated.TransactionRead](requestFunc)
	if err != nil {
		return nil, err
	}
	if len(transactions) == 0 {
		return nil, nil
	}
	return &transactions[0], nil
}

func paginatedRequest[V any](get func(page int32) (status int, body []byte, err error)) ([]V, error) {
	var result []V
	for page := int32(1); ; page++ {
		status, resp, err := get(page)
		if err != nil {
			return nil, err
		}

		if status != http.StatusOK {
			return nil, errFromResponse(status, resp)
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

func errFromResponse(status int, body []byte) error {
	switch status {
	case 400:
		e := generated.BadRequestResponse{}
		if err := json.Unmarshal(body, &e); err != nil {
			return fmt.Errorf("failed to unmarshal HTTP response for status 400: %w - %q", err, string(body))
		}
		return e
	case 401:
		e := generated.UnauthenticatedResponse{}
		if err := json.Unmarshal(body, &e); err != nil {
			return fmt.Errorf("failed to unmarshal HTTP response for status 401: %w - %q", err, string(body))
		}
		return e
	case 404:
		e := generated.NotFoundResponse{}
		if err := json.Unmarshal(body, &e); err != nil {
			return fmt.Errorf("failed to unmarshal HTTP response for status 404: %w - %q", err, string(body))
		}
		return e
	case 422:
		e := generated.ValidationErrorResponse{}
		if err := json.Unmarshal(body, &e); err != nil {
			return fmt.Errorf("failed to unmarshal HTTP response for status 422: %w - %q", err, string(body))
		}
		if duplicateID, isDuplicateErr := e.IsDuplicateTransactionErr(); isDuplicateErr {
			return DuplicateTransactionError{DuplicateOf: duplicateID}
		}
		return e
	case 500:
		e := generated.InternalExceptionResponse{}
		if err := json.Unmarshal(body, &e); err != nil {
			return fmt.Errorf("failed to unmarshal HTTP response for status 500: %w - %q", err, string(body))
		}
		return e
	default:
		logger.Warnf("unhandled HTTP response %d", status)
		var e any
		if err := json.Unmarshal(body, &e); err != nil {
			return fmt.Errorf("invalid JSON for HTTP response %d: %w - %q", status, err, string(body))
		}
		return fmt.Errorf("unknown HTTP response %d: %v", status, e)
	}
}

type DuplicateTransactionError struct {
	DuplicateOf  string
	byExternalID bool
}

func (e DuplicateTransactionError) Error() string {
	if e.byExternalID {
		return "duplicate of transaction #" + e.DuplicateOf + " (by external ID)"
	}
	return "duplicate of transaction #" + e.DuplicateOf
}
