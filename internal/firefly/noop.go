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

func (w noopWriter) CreateTransaction(_ context.Context, _ string, t domain.BankTransaction) (string, error) {
	w.infof(`would create %s dated %s from %s`, string(t.Type), t.Date.Format(time.DateTime), *t.Merchant)
	return "", nil
}
