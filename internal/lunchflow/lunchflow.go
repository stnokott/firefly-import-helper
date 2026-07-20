// Package lunchflow is a wrapper for the REST API of Lunchflow.
//
// https://lunchflow.app
package lunchflow

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

const (
	httpRequestTimeout = 60 * time.Second
)

const (
	maxTransactionTimePast = 90 * 24 * time.Hour
)

var logger = log.For("lunchflow")

var lunchflowBaseURL = mustParseURL("https://lunchflow.app/api/v1")

func mustParseURL(u string) *url.URL {
	parsed, err := url.Parse(u)
	if err != nil {
		panic("invalid URL: " + u)
	}
	return parsed
}

type Client struct {
	apiKey     string
	baseURL    *url.URL
	httpClient *http.Client
}

func NewClient(apiKey string) domain.BankConnector {
	return &Client{
		apiKey:  apiKey,
		baseURL: lunchflowBaseURL,
		httpClient: &http.Client{
			Transport: http.DefaultTransport,
			Timeout:   httpRequestTimeout,
		},
	}
}

func (c *Client) GetAccounts(ctx context.Context) ([]domain.BankAccount, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	accounts, err := c.listAccounts(ctx)
	if err != nil {
		return nil, err
	}
	return ConvertAccounts(accounts.Accounts), nil
}

func (c *Client) GetBalance(ctx context.Context, id int) (float64, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	balance, err := c.getAccountBalance(ctx, id)
	if err != nil {
		return -1, err
	}
	return balance.Balance.Amount, nil
}

func (c *Client) GetTransactions(ctx context.Context, id int, from time.Time, to time.Time) ([]domain.BankTransaction, error) {
	minFromTime := c.MinImportTransactionTime()
	if from.Before(minFromTime) {
		return nil, fmt.Errorf("can not query transactions earlier than %v", minFromTime.Format(time.DateTime))
	}

	ctx, cancel := context.WithTimeout(ctx, 60*time.Second)
	defer cancel()
	u := c.baseURL.JoinPath(fmt.Sprintf("/accounts/%d/transactions", id))
	q := u.Query()
	q.Set("from", from.Format(time.DateOnly))
	q.Set("to", to.Format(time.DateOnly))
	u.RawQuery = q.Encode()

	resp, err := c.get(ctx, u.String()) //nolint:bodyclose // will be closed in parseResponse
	if err != nil {
		return nil, err
	}
	respTransactions, err := parseResponse[Transactions](resp, 200)
	if err != nil {
		return nil, err
	}
	return ConvertTransactions(respTransactions.Transactions), nil
}

func (*Client) MinImportTransactionTime() time.Time {
	return time.Now().Truncate(24 * time.Hour).Add(24 * time.Hour).Add(-maxTransactionTimePast)
}

func (c *Client) listAccounts(ctx context.Context) (*Accounts, error) {
	u := c.baseURL.JoinPath("/accounts")
	resp, err := c.get(ctx, u.String()) //nolint:bodyclose // will be closed in parseResponse
	if err != nil {
		return nil, err
	}
	return parseResponse[Accounts](resp, http.StatusOK)
}

func (c *Client) getAccountBalance(ctx context.Context, id int) (*Balance, error) {
	u := c.baseURL.JoinPath("/accounts/", strconv.Itoa(id), "/balance")
	resp, err := c.get(ctx, u.String()) //nolint:bodyclose // will be closed in parseResponse
	if err != nil {
		return nil, err
	}
	return parseResponse[Balance](resp, http.StatusOK)
}

func (c *Client) get(ctx context.Context, u string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, http.NoBody)
	if err != nil {
		return nil, fmt.Errorf("could not create request: %w", err)
	}
	req.Header.Add("X-Api-Key", c.apiKey)
	req.Header.Add("Accept", "application/json")
	req.Header.Add("User-Agent", config.AppName)

	logger.Debugf(">> %s %s", req.Method, req.URL.String())
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	return resp, nil
}

func parseResponse[T any](resp *http.Response, expectStatus int) (*T, error) {
	defer func() {
		if bodyCloseErr := resp.Body.Close(); bodyCloseErr != nil {
			logger.Warnf("failed closing response body: %v", bodyCloseErr)
		}
	}()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("could not read response body: %w", err)
	}

	if resp.StatusCode != expectStatus {
		parsedErr, err := unmarshalOrErr[Error](bodyBytes)
		if err != nil {
			return nil, fmt.Errorf("could not parse response for erroneous status code %d: %w", resp.StatusCode, err)
		}
		return nil, parsedErr
	}

	parsedResp, err := unmarshalOrErr[T](bodyBytes)
	if err != nil {
		return nil, fmt.Errorf("got expected status code %d, but could not parse response: %w", resp.StatusCode, err)
	}
	return parsedResp, nil
}

func unmarshalOrErr[T any](data []byte) (*T, error) {
	parsed := new(T)
	if err := json.Unmarshal(data, parsed); err != nil {
		return nil, fmt.Errorf("%s: %w", string(data), err)
	}
	return parsed, nil
}
