// Package importer performs the actual account import by leveraging all involved services.
package importer

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/firefly"
	"github.com/stnokott/firefly-import-helper/internal/log"

	_ "time/tzdata"
)

var logger = log.For("importer")

type Importer struct {
	cfg          *config.YAML
	msg          domain.Messenger
	bank         domain.BankConnector
	fireflyRead  domain.FireflyReader
	fireflyWrite domain.FireflyWriter
	lastRun      time.Time
}

func New(cfg *config.YAML, msg domain.Messenger, bank domain.BankConnector, ffRead domain.FireflyReader, ffWrite domain.FireflyWriter) (*Importer, error) {
	im := &Importer{
		cfg:          cfg,
		msg:          msg,
		bank:         bank,
		fireflyRead:  ffRead,
		fireflyWrite: ffWrite,
		lastRun:      time.Date(1900, 1, 1, 0, 0, 0, 0, time.Local),
	}
	if err := im.validateConfig(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}
	return im, nil
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

	sum := make([]domain.Summary, len(accounts))
	for i, acc := range accounts {
		logger.Infof("importing %d/%d: %s @ %s", i+1, len(accounts), acc.Name, acc.Institution)
		summary := domain.Summary{
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
				summary.Info = fmt.Sprintf("error occurred after %d imports: %s", imported, err.Error())
			} else {
				logger.Infof("%d transactions imported", imported)
				summary.Success = true
				summary.Info = fmt.Sprintf("%d transactions imported", imported)
			}
		}

		sum[i] = summary
	}

	im.lastRun = time.Now()
	return im.msg.MsgImportFinished(ctx, sum)
}

// nextImportRange returns the timeframe for the next import using from and to times.
//
// In most cases, this is simply the timeframe from the last run to now,
// although the time of the last run can be overriden if exceeds the earliest possible import time
// as defined by the bank connector.
func (im *Importer) nextImportRange() (from, to time.Time) {
	minFrom := im.bank.MinImportTransactionTime()
	from = slices.MaxFunc([]time.Time{im.lastRun, minFrom}, time.Time.Compare)
	// to = time.Now()
	to = from.Add(7 * 24 * time.Hour)
	return
}

// importAccount imports transactions for the given account and returns the number of imported transactions.
func (im *Importer) importAccount(ctx context.Context, acc domain.BankAccount) (int, error) {
	from, to := im.nextImportRange()
	transactions, err := im.bank.GetTransactions(ctx, acc.ID, from, to)
	if err != nil {
		return 0, fmt.Errorf("could not get transactions: %w", err)
	}

	created := 0
	fireflyAccountID := im.cfg.AccountsByBankID[acc.ID].FireflyID // always resolves due to previous validation
	for i, t := range transactions {
		if t.IsPending {
			logger.Infof("%03d/%03d - skipping pending transaction %s", i+1, len(transactions), t.ID)
			continue
		}

		fireflyID, err := im.importTransaction(ctx, t, fireflyAccountID)
		if err != nil {
			// duplicate transactions are acceptable - continue
			if errDuplicate, isDuplicate := errors.AsType[firefly.ErrDuplicateTransaction](err); isDuplicate {
				logger.Infof("%03d/%03d - duplicate transaction #%s ignored", i+1, len(transactions), errDuplicate.DuplicateOf)
				// TODO: send message
				continue
			}
			return created, fmt.Errorf("failed to import transaction: %w", err)
		}
		logger.Infof("%03d/%03d - created transaction #%s", i+1, len(transactions), fireflyID)
		created++
	}
	return created, nil
}

func (im *Importer) importTransaction(ctx context.Context, t domain.BankTransaction, fireflyAccountID string) (string, error) {
	id, err := im.fireflyWrite.CreateTransaction(ctx, fireflyAccountID, t)
	if err != nil {
		return "", fmt.Errorf("failed to create Firefly transaction: %w", err)
	}
	return id, nil
}
