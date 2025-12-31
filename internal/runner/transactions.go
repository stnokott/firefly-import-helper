package runner

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/ptr"
)

func getAnyTransaction(ctx context.Context, c client.ClientWithResponsesInterface) (*client.TransactionRead, error) {
	slog.Debug("querying an arbitrary transaction")
	resp, err := c.ListTransactionWithResponse(ctx, &client.ListTransactionParams{
		Limit: ptr.Of(int32(1)),
	})
	if err != nil {
		return nil, err
	}
	if resp.StatusCode() != 200 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode(), string(resp.Body))
	}
	if len(resp.ApplicationvndApiJSON200.Data) == 0 {
		return nil, errors.New("no transactions found")
	}

	return &resp.ApplicationvndApiJSON200.Data[0], nil
}
