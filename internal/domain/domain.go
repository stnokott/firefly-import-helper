// Package domain provides business structs for cross-package type agreements.
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
	// MinImportTransactionTime returns the earliest possible starting time for a import timeframe.
	MinImportTransactionTime() time.Time
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
	ID          string
	IsPending   bool
	Type        BankTransactionType
	AccountID   int
	Amount      float64
	Currency    string
	Date        time.Time
	Description string
	Merchant    *string
}

type BankTransactionType string

const (
	BankTransactionTypeWithdrawal BankTransactionType = "WITHDRAWAL"
	BankTransactionTypeDeposit    BankTransactionType = "DEPOSIT"
)

type FireflyReader interface {
	ListAssetAccounts(ctx context.Context) (FireflyAccounts, error)
}

type FireflyWriter interface {
	CreateTransaction(ctx context.Context, accountID string, t BankTransaction) (string, error)
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

// Messenger handles communication between user and this program.
type Messenger interface {
	Listen(ctx context.Context)
	MsgImportStarted(ctx context.Context) error
	MsgImportFinished(ctx context.Context, sum []Summary) error
}

type Summary struct {
	Account     string
	Institution string
	Success     bool
	Info        string
}
