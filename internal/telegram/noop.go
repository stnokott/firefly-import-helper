package telegram

import (
	"context"

	"github.com/stnokott/firefly-import-helper/internal/domain"
)

type noop struct{}

func NewNoop() domain.Messenger {
	return noop{}
}

func (noop) Listen(ctx context.Context) {
	<-ctx.Done()
}

func (noop) MsgImportStarted(_ context.Context) error {
	return nil
}

func (noop) MsgNewTransaction(_ context.Context, _ *domain.TransactionCreated) error {
	return nil
}

func (noop) MsgImportFinished(_ context.Context, _ []domain.Summary) error {
	return nil
}
