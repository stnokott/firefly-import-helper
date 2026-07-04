package domain

import (
	"time"

	"github.com/stnokott/firefly-import-helper/internal/log"
)

var logger = log.For("domain")

type BankAccount struct {
	ID          int
	Name        string
	Institution string
	Currency    string
	Status      BankAccountStatus
}

type BankAccountStatus int

const (
	BankAccountStatusActive BankAccountStatus = iota
	BankAccountStatusDisconnected
	BankAccountStatusError
)

type BankTransaction struct {
	Type        BankTransactionType
	AccountID   int
	Amount      int
	Currency    string
	Date        time.Time
	Description string
}

type BankTransactionType int

const (
	BankTransactionTypeWithdrawal BankTransactionType = iota
	BankTransactionTypeDeposit
)

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
