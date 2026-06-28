package importer

import (
	"context"

	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("importer")

type Importer struct {
	msg  Messenger
	bank BankConnection
}

func New(msg Messenger, bank BankConnection) *Importer {
	return &Importer{
		msg:  msg,
		bank: bank,
	}
}

// Messenger handles communication between user and this program.
type Messenger interface {
	MsgImportStarted(ctx context.Context) error
	MsgImportFinished(ctx context.Context) error // TODO: summary per account
	// MsgNewTransaction(ctx context.Context, t domain.FireflyTransaction) error
}

type BankConnection interface {
	GetAccounts(ctx context.Context) ([]domain.BankAccount, error)
	GetBalance(ctx context.Context, id int) (float64, error)
	GetTransactions(ctx context.Context, id int) ([]domain.BankTransaction, error)
}

func (im *Importer) Run(ctx context.Context) error {
	if err := im.msg.MsgImportStarted(ctx); err != nil {
		return err
	}
	defer func() {
		_ = im.msg.MsgImportFinished(ctx)
	}()

	accounts, err := im.bank.GetAccounts(ctx)
	if err != nil {
		return err
	}
	for i, acc := range accounts {
		logger.Infof("%d/%d: %s @ %s (%v)", i+1, len(accounts), acc.Name, acc.InstitutionName, acc.Status)
	}

	return nil
}
