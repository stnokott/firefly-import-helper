package importer

import (
	"context"
	"fmt"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("importer")

type Importer struct {
	cfg          *config.YAML
	msg          Messenger
	bank         domain.BankConnector
	fireflyRead  domain.FireflyReader
	fireflyWrite domain.FireflyWriter
}

func New(cfg *config.YAML, msg Messenger, bank domain.BankConnector, ffRead domain.FireflyReader, ffWrite domain.FireflyWriter) (*Importer, error) {
	im := &Importer{
		cfg:          cfg,
		msg:          msg,
		bank:         bank,
		fireflyRead:  ffRead,
		fireflyWrite: ffWrite,
	}
	if err := im.validateConfig(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}
	return im, nil
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

func (im *Importer) validateConfig() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ffAccounts, err := im.fireflyRead.ListAssetAccounts(ctx)
	if err != nil {
		return fmt.Errorf("could not list Firefly accounts: %w", err)
	}
	return im.cfg.ValidateFireflyIDs(ffAccounts)
}

func (im *Importer) Import(ctx context.Context, dryRun bool) (err error) {
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
		logger.Infof("importing %d/%d: %s @ %s", i+1, len(accounts), acc.Name, acc.Institution)
		summary := Summary{
			Account:     acc.Name,
			Institution: acc.Institution,
		}
		switch {
		case im.cfg.AccountsByBankID[acc.ID].Ignore:
			logger.Info("  account ignored - skipping")
			summary.Success = true
			summary.Info = "Ignored via config"
		case acc.Status == domain.BankAccountStatusDisconnected:
			logger.Info("  account disconnected - skipping")
			summary.Success = false
			summary.Info = "Reauthorization required"
		case acc.Status == domain.BankAccountStatusError:
			logger.Info("  account erroneous - skipping")
			summary.Success = false
			summary.Info = "Issue with data provider"
		case acc.Status == domain.BankAccountStatusActive:
			imported, err := im.importAccount(ctx, acc)
			if err != nil {
				logger.ErrorV(err)
				summary.Success = false
				summary.Info = err.Error()
			} else {
				logger.Infof("%d transactions imported", imported)
				summary.Success = true
				summary.Info = fmt.Sprintf("%d transactions imported", imported)
			}
		}

		sum[i] = summary
	}

	return im.msg.MsgImportFinished(ctx, sum)
}

// importAccount imports transactions for the given account and returns the number of imported transactions.
func (im *Importer) importAccount(ctx context.Context, acc domain.BankAccount) (int, error) {
	from := im.bank.MinTransactionTime()
	to := from.Add(7 * 24 * time.Hour)
	transactions, err := im.bank.GetTransactions(ctx, acc.ID, from, to)
	if err != nil {
		return 0, fmt.Errorf("could not get transactions: %w", err)
	}

	fireflyAccountID := im.cfg.AccountsByBankID[acc.ID].FireflyID // always resolves due to previous validation
	for _, t := range transactions {
		if t.IsPending {
			logger.Debugf("skipping pending transaction %s", t.ID)
			continue
		}

		if err := im.importTransaction(ctx, t, fireflyAccountID); err != nil {
			return 0, fmt.Errorf("failed to import transaction %s: %w", t.ID, err)
		}
	}
	return len(transactions), nil
}

func (im *Importer) importTransaction(ctx context.Context, t domain.BankTransaction, fireflyAccountID string) error {
	id, err := im.fireflyWrite.CreateTransaction(ctx, fireflyAccountID, t)
	if err != nil {
		return fmt.Errorf("failed to create Firefly transaction: %w", err)
	}
	logger.Debugf("created Firefly transaction %s", id)
	return nil
}
