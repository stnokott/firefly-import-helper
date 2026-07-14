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

func (w noopWriter) CreateTransaction(_ context.Context, accountID string, t domain.BankTransaction) (string, error) {
	w.infof(`create transaction for account %s %.2f%s / %s "%s`, accountID, t.Amount, t.Currency, t.Date.Format(time.DateTime), t.Description)
	return "", nil
}
