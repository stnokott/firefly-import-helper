package domain

import (
	"context"
	"iter"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("domain")

type BankConnector interface {
	GetAccounts(ctx context.Context) ([]BankAccount, error)
	GetBalance(ctx context.Context, id int) (float64, error)
	// GetTransactions returns the transactions of the given account within the given timeframe.
	GetTransactions(ctx context.Context, id int, from time.Time, to time.Time) ([]BankTransaction, error)
	MinTransactionTime() time.Time
}

type BankAccount struct {
	ID          int
	Name        string
	Institution string
	Currency    string
	Status      BankAccountStatus
}

type BankAccountStatus string

const (
	BankAccountStatusActive       BankAccountStatus = "ACTIVE"
	BankAccountStatusDisconnected BankAccountStatus = "DISCONNECTED"
	BankAccountStatusError        BankAccountStatus = "ERROR"
)

type BankTransaction struct {
	Type        BankTransactionType
	AccountID   int
	Amount      float64
	Currency    string
	Date        time.Time
	Description string
	Merchant    *string
}

type BankTransactionType int

const (
	BankTransactionTypeWithdrawal BankTransactionType = iota
	BankTransactionTypeDeposit
)

type FireflyConnector interface {
	ListAssetAccounts(ctx context.Context) (FireflyAccounts, error)
}

type FireflyAccount struct {
	ID     string
	Name   string
	Active bool
}

type FireflyAccounts []FireflyAccount

func (a FireflyAccounts) Active() iter.Seq[FireflyAccount] {
	return func(yield func(FireflyAccount) bool) {
		for _, acc := range a {
			if acc.Active && !yield(acc) {
				return
			}
		}
	}
}

type FireflyTransaction struct {
	ID              int
	CreatedAt       time.Time
	User            int
	SubTransactions []*FireflySubTransaction
}

type FireflySubTransaction struct {
	Amount               float64
	CurrencySymbol       string
	CategoryID           *string
	CategoryName         *string
	Date                 time.Time
	Description          string
	DestinationName      string
	SourceName           string
	TransactionJournalID *string
	Type                 BankTransactionType
	User                 int
}
