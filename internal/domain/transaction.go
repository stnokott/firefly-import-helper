package domain

import (
	"time"

	"github.com/stnokott/firefly-import-helper/internal/client"
)

type Transaction struct {
	ID              int
	CreatedAt       time.Time
	User            int
	SubTransactions []*SubTransaction
}

type SubTransaction struct {
	Amount               client.Amount
	CurrencySymbol       string
	CategoryID           *string
	CategoryName         *string
	Date                 time.Time
	Description          string
	DestinationName      string
	SourceName           string
	TransactionJournalID *string
	Type                 client.TransactionTypeProperty
	User                 int
}
