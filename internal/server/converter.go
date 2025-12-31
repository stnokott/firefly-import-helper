package server

import (
	"time"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/domain"
)

//go:generate go tool goverter gen .

// Converter defines methods for converting internal models to domain models.
//
// It is only used for generating code, there is no actual implementation.
// goverter:converter
// goverter:output:format function
// goverter:output:file converter.gen.go
// goverter:extend Convert.*
type Converter interface {
	//goverter:autoMap Content
	ConvertWebhookResponse(wr *WebhookResponse) *domain.Transaction
}

func ConvertTime(t time.Time) time.Time {
	return t
}

func ConvertTransactionType(t client.TransactionTypeProperty) client.TransactionTypeProperty {
	return t
}
