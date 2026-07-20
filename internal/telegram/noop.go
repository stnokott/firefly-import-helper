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

func (noop) MsgAccountProblem(_ context.Context, _ domain.AccountProblem) error {
	return nil
}

func (noop) MsgImportStarted(_ context.Context) error {
	return nil
}

func (noop) MsgNewTransaction(_ context.Context, _ *domain.TransactionRead, _ []string) error {
	return nil
}
