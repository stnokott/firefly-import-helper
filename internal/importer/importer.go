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

	"github.com/adhocore/gronx"
)

var logger = log.For("importer")

type Importer struct {
	cfg     *config.YAML
	msg     domain.Messenger
	bank    domain.BankConnector
	firefly domain.FireflyReadWriter
	lastRun time.Time
}

func New(cfg *config.YAML, msg domain.Messenger, bank domain.BankConnector, ff domain.FireflyReadWriter) (*Importer, error) {
	im := &Importer{
		cfg:     cfg,
		msg:     msg,
		bank:    bank,
		firefly: ff,
		// at first launch, attempt to import full time range
		lastRun: time.Date(1900, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if err := im.validateConfig(); err != nil {
		return nil, fmt.Errorf("config validation error: %w", err)
	}
	return im, nil
}

func (im *Importer) validateConfig() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	ffAccounts, err := im.firefly.ListAssetAccounts(ctx)
	if err != nil {
		return fmt.Errorf("could not list Firefly accounts: %w", err)
	}
	return im.cfg.ValidateFireflyIDs(ffAccounts)
}

func (im *Importer) ScheduleImports(ctx context.Context, cronExpr string) error {
	if !gronx.IsValid(cronExpr) {
		return fmt.Errorf("invalid cron expression %q", cronExpr)
	}

	for {
		next, _ := gronx.NextTick(cronExpr, true)
		logger.Info("next import scheduled for " + next.Format(time.DateTime))
		select {
		case <-ctx.Done():
			return nil
		case <-time.Tick(time.Until(next)):
		}

		if err := im.Import(ctx); err != nil {
			return err
		}
	}
}

func (im *Importer) Import(ctx context.Context) (err error) {
	logger.Info("starting import")
	start := time.Now()
	defer func() {
		if err != nil {
			logger.Infof("import ended with error: %v", err)
		} else {
			logger.Infof("import finished, took %v", time.Since(start))
		}
	}()

	// Firefly categories required later for Messenger interactions
	ffCategories, err := im.firefly.ListCategories(ctx)
	if err != nil {
		return fmt.Errorf("failed to query Firefly categories: %w", err)
	}

	accounts, err := im.bank.GetAccounts(ctx)
	if err != nil {
		return err
	}

	from, to := im.nextImportRange()

	for i, acc := range accounts {
		logger.Infof("importing %d/%d: %s @ %s", i+1, len(accounts), acc.Name, acc.Institution)
		switch {
		case im.cfg.AccountsByBankID[acc.ID].Ignore:
			logger.Info("  account ignored - skipping")
			continue
		case acc.Status == domain.BankAccountStatusDisconnected:
			logger.Info("  account disconnected - skipping")
			_ = im.msg.MsgAccountProblem(ctx, domain.AccountProblem{
				Account: &acc,
				Problem: "Reauthorization required.",
			})
		case acc.Status == domain.BankAccountStatusError:
			logger.Info("  account erroneous - skipping")
			_ = im.msg.MsgAccountProblem(ctx, domain.AccountProblem{
				Account: &acc,
				Problem: "Internal data provider error.",
			})
		case acc.Status == domain.BankAccountStatusActive:
			imported, err := im.importAccount(ctx, acc, from, to, ffCategories)
			if err != nil {
				logger.Errorv(err)
			} else {
				logger.Infof("%d transactions imported", imported)
			}
		}
	}

	im.lastRun = to
	return nil
}

// nextImportRange returns the timeframe for the next import using from and to times.
//
// In most cases, this is simply the timeframe from the last run to now,
// although the time of the last run can be overridden if exceeds the earliest possible import time
// as defined by the bank connector.
func (im *Importer) nextImportRange() (from, to time.Time) {
	minFrom := im.bank.MinImportTransactionTime()
	from = slices.MaxFunc([]time.Time{im.lastRun, minFrom}, time.Time.Compare)
	to = time.Now()
	return
}

// importAccount imports transactions for the given account and returns the number of imported transactions.
func (im *Importer) importAccount(ctx context.Context, acc domain.BankAccount, from, to time.Time, ffCategories []string) (int, error) {
	transactions, err := im.bank.GetTransactions(ctx, acc.ID, from, to)
	if err != nil {
		return 0, fmt.Errorf("could not get transactions: %w", err)
	}
	slices.SortFunc(transactions, func(a, b domain.BankTransaction) int {
		// old to new
		return a.Date.Compare(b.Date)
	})

	created := 0
	fireflyAccountID := im.cfg.AccountsByBankID[acc.ID].FireflyID // always resolves due to previous validation
	for i, t := range transactions {
		if t.IsPending {
			logger.Infof("%03d/%03d - skipping pending transaction %s", i+1, len(transactions), t.ID)
			continue
		}

		fireflyID, err := im.importTransaction(ctx, t, fireflyAccountID, ffCategories)
		if err != nil {
			// duplicate transactions are acceptable - continue
			if errDuplicate, isDuplicate := errors.AsType[firefly.DuplicateTransactionError](err); isDuplicate {
				logger.Infof("%03d/%03d - duplicate transaction #%s ignored", i+1, len(transactions), errDuplicate.DuplicateOf)
				continue
			}
			return created, fmt.Errorf("failed to import transaction: %w", err)
		}
		logger.Infof("%03d/%03d - created transaction #%s", i+1, len(transactions), fireflyID)
		created++
	}
	return created, nil
}

func (im *Importer) importTransaction(ctx context.Context, t domain.BankTransaction, fireflyAccountID string, ffCategories []string) (string, error) {
	created, err := im.firefly.CreateTransaction(ctx, fireflyAccountID, t)
	if err != nil {
		return "", fmt.Errorf("failed to create Firefly transaction: %w", err)
	}
	if err := im.msg.MsgNewTransaction(ctx, created, ffCategories); err != nil {
		logger.Errorv(err)
	}
	return created.FireflyID, nil
}
