package lunchflow

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListAccounts(t *testing.T) {
	t.Parallel()
	t.Run("ok", func(t *testing.T) {
		t.Parallel()
		c := newMockedClient(t, func(rw http.ResponseWriter, req *http.Request) {
			respStruct := Accounts{
				Accounts: []Account{
					{
						ID:              123,
						Name:            "Foo",
						InstitutionName: "Bar",
						Provider:        "Fuzz",
						Currency:        new("€"),
						Status:          AccountStatusActive,
					},
				},
				Total: 1,
			}
			respBytes, err := json.Marshal(respStruct)
			assert.NoError(t, err)
			rw.WriteHeader(http.StatusOK)
			_, _ = rw.Write(respBytes)
		})
		resp, err := c.listAccounts(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
		assert.Equal(t, 123, resp.Accounts[0].ID)
	})

	t.Run("unauthorized", func(t *testing.T) {
		t.Parallel()
		c := newMockedClient(t, func(rw http.ResponseWriter, req *http.Request) {
			respStruct := Error{
				Err:     "Unauthorized",
				Message: "Authentication required.",
			}
			respBytes, err := json.Marshal(respStruct)
			assert.NoError(t, err)
			rw.WriteHeader(http.StatusUnauthorized)
			_, _ = rw.Write(respBytes)
		})
		resp, err := c.listAccounts(context.Background())
		require.Nil(t, resp)
		e := new(Error)
		require.ErrorAs(t, err, &e)
		assert.NotEmpty(t, e.Err)
		assert.NotEmpty(t, e.Message)
	})

	t.Run("invalid response JSON", func(t *testing.T) {
		t.Parallel()
		c := newMockedClient(t, func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(http.StatusInternalServerError)
			_, _ = rw.Write([]byte(
				`<!DOCTYPE html><html><body>Absolutely not</body></html>`,
			))
		})
		resp, err := c.listAccounts(context.Background())
		require.Nil(t, resp)
		assert.ErrorContains(t, err, "could not parse response")
		assert.ErrorContains(t, err, "<!DOCTYPE html>")
		assert.ErrorContains(t, err, "invalid character")
	})
}

func newMockedClient(t *testing.T, handler http.HandlerFunc) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c := NewClient("123").(*Client)
	c.httpClient = srv.Client()
	c.baseURL = mustParseURL(srv.URL)
	return c
}
