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
	MsgImportFinished(ctx context.Context, sum []Summary) error
	// MsgNewTransaction(ctx context.Context, t domain.FireflyTransaction) error
}

type Summary struct {
	Account     string
	Institution string
	Success     bool
	Info        string
}

type BankConnection interface {
	GetAccounts(ctx context.Context) ([]domain.BankAccount, error)
	GetBalance(ctx context.Context, id int) (float64, error)
	GetTransactions(ctx context.Context, id int) ([]domain.BankTransaction, error)
}

func (im *Importer) Import(ctx context.Context) (err error) {
	logger.Info("starting import")
	defer func() {
		if err != nil {
			logger.Infof("import ended with error: %v", err)
		} else {
			logger.Info("import finished")
		}
	}()
	if err := im.msg.MsgImportStarted(ctx); err != nil {
		return err
	}

	accounts, err := im.bank.GetAccounts(ctx)
	if err != nil {
		return err
	}

	sum := make([]Summary, len(accounts))
	for i, acc := range accounts {
		logger.Infof("%d/%d: %s @ %s (%v)", i+1, len(accounts), acc.Name, acc.Institution, acc.Status)
		summary := Summary{
			Account:     acc.Name,
			Institution: acc.Institution,
		}
		switch acc.Status {
		case domain.BankAccountStatusDisconnected:
			summary.Success = false
			summary.Info = "Reauthorization required"
		case domain.BankAccountStatusError:
			summary.Success = false
			summary.Info = "Issue with data provider"
		case domain.BankAccountStatusActive:
			// TODO: import
			summary.Success = true
			summary.Info = "123 transactions imported"
		}

		sum[i] = summary
	}

	return im.msg.MsgImportFinished(ctx, sum)
}
