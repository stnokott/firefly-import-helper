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
	t.Run("200", func(t *testing.T) {
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
			require.NoError(t, err)
			rw.WriteHeader(200)
			_, _ = rw.Write(respBytes)
		})
		resp, err := c.ListAccounts(context.Background())
		require.NoError(t, err)
		assert.Equal(t, 1, resp.Total)
		assert.Equal(t, 123, resp.Accounts[0].ID)
	})

	t.Run("401", func(t *testing.T) {
		c := newMockedClient(t, func(rw http.ResponseWriter, req *http.Request) {
			respStruct := Error{
				Err:     "Unauthorized",
				Message: "Authentication required.",
			}
			respBytes, err := json.Marshal(respStruct)
			require.NoError(t, err)
			rw.WriteHeader(401)
			_, _ = rw.Write(respBytes)
		})
		resp, err := c.ListAccounts(context.Background())
		require.Nil(t, resp)
		require.IsType(t, &Error{}, err)
		assert.NotEmpty(t, err.(*Error).Err)
		assert.NotEmpty(t, err.(*Error).Message)
	})

	t.Run("invalid response JSON", func(t *testing.T) {
		c := newMockedClient(t, func(rw http.ResponseWriter, req *http.Request) {
			rw.WriteHeader(500)
			_, _ = rw.Write([]byte(
				`<!DOCTYPE html><html><body>Absolutely not</body></html>`,
			))
		})
		resp, err := c.ListAccounts(context.Background())
		require.Nil(t, resp)
		assert.ErrorContains(t, err, "could not parse response")
		assert.ErrorContains(t, err, "<!DOCTYPE html>")
		assert.ErrorContains(t, err, "invalid character")
	})
}

func newMockedClient(t *testing.T, handler http.HandlerFunc) *Client {
	srv := httptest.NewServer(http.HandlerFunc(handler))
	t.Cleanup(srv.Close)
	c := NewClient("123")
	c.httpClient = srv.Client()
	c.baseURL = mustParseURL(srv.URL)
	return c
}
