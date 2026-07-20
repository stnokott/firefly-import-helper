package firefly

import (
	"context"
	"time"

	domain "github.com/stnokott/firefly-import-helper/internal/domain"
)

type noopWriter struct{}

func NewNoopWriter() domain.FireflyWriter {
	return noopWriter{}
}

func (noopWriter) infof(s string, args ...any) {
	logger.Infof("DRY-RUN: "+s, args...)
}

func (w noopWriter) CreateTransaction(_ context.Context, _ string, t domain.BankTransaction) (*domain.TransactionRead, error) {
	w.infof("would create %s dated %s from %q", string(t.Type), t.Date.Format(time.DateTime), *t.Merchant)
	return &domain.TransactionRead{
		FireflyID: "1234",
	}, nil
}

func (w noopWriter) UpdateTransactionCategory(_ context.Context, transactionID string, category string) (*domain.TransactionRead, error) {
	w.infof("would update transaction #%s with category %q", transactionID, category)
	return &domain.TransactionRead{
		FireflyID: "1234",
	}, nil
}
