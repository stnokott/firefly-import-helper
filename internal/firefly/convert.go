package firefly

import (
	"fmt"
	"time"

	"github.com/oapi-codegen/nullable"
	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/firefly/generated"
)

const (
	unknownDestinationAccountName string = "(unknown destination account)"
	unknownSourceAccountName      string = "(unknown source account)"
)

//go:generate go tool goverter gen -output-constraint= ./

//goverter:converter
//goverter:output:format function
//goverter:output:file convert.gen.go
//goverter:output:package github.com/stnokott/firefly-import-helper/internal/firefly
//goverter:extend copyTime nullableTime nullableString
//goverter:extend mapTransactionType
type Converter interface {
	ConvertAccounts([]generated.AccountRead) []domain.FireflyAccount
	//goverter:map Id ID
	//goverter:map Attributes.Active Active | boolOrTrue
	//goverter:map Attributes.Name Name
	ConvertAccount(generated.AccountRead) domain.FireflyAccount

	//goverter:context ffAccountID
	//goverter:default defaultTransactionSplitStore
	//goverter:ignore BillId BillName BudgetId BudgetName CategoryId CategoryName CurrencyId
	//goverter:ignore DueDate ExternalUrl ForeignAmount ForeignCurrencyCode ForeignCurrencyId InterestDate
	//goverter:ignore InternalReference InvoiceDate Notes Order PaymentDate PiggyBankId PiggyBankName Reconciled
	//goverter:ignore SepaBatchId SepaCc SepaCi SepaCountry SepaCtId SepaCtOp SepaDb SepaEp
	//goverter:ignore Tags
	//goverter:map Date BookDate
	//goverter:map Currency CurrencyCode
	//goverter:map . DestinationId | getDestinationID
	//goverter:map . DestinationName | getDestinationName
	//goverter:map ID ExternalId
	//goverter:map ProcessDate | now
	//goverter:map . SourceId | getSourceID
	//goverter:map . SourceName | getSourceName
	ConvertTransaction(t domain.BankTransaction, ffAccountID string) generated.TransactionSplitStore
}

func boolOrTrue(b *bool) bool {
	if b != nil {
		return *b
	}
	return true
}

func now() nullable.Nullable[time.Time] {
	return nullable.NewNullableWithValue(time.Now())
}

func copyTime(t time.Time) time.Time {
	return t
}

func nullableTime(t time.Time) nullable.Nullable[time.Time] {
	return nullable.NewNullableWithValue(t)
}

func nullableString(s string) nullable.Nullable[string] {
	return nullable.NewNullableWithValue(s)
}

func defaultTransactionSplitStore() generated.TransactionSplitStore {
	notes := fmt.Sprintf("imported at %s using %s", time.Now().Format(time.DateTime), config.AppName)
	return generated.TransactionSplitStore{
		Notes:      nullable.NewNullableWithValue(notes),
		Reconciled: new(true),
	}
}

//goverter:context ffAccountID
func getSourceID(t domain.BankTransaction, ffAccountID string) nullable.Nullable[string] {
	if t.Type != domain.BankTransactionTypeWithdrawal {
		// the source is not our FF account, so we need to go by name instead of ID. So we let getSourceName handle that.
		return nullable.NewNullNullable[string]()
	}
	return nullable.NewNullableWithValue(ffAccountID)
}

func getSourceName(t domain.BankTransaction) nullable.Nullable[string] {
	if t.Type != domain.BankTransactionTypeDeposit {
		// our current FF account is the source. We only have its ID (not the name), so we let getSourceID handle that.
		return nullable.NewNullNullable[string]()
	}
	if t.Merchant == nil {
		logger.Warnf("got empty Merchant value for deposit #%s", t.ID)
		return nullable.NewNullableWithValue(unknownSourceAccountName)
	}
	return nullable.NewNullableWithValue(*t.Merchant)
}

//goverter:context ffAccountID
func getDestinationID(t domain.BankTransaction, ffAccountID string) nullable.Nullable[string] {
	if t.Type != domain.BankTransactionTypeDeposit {
		// the destination is not our FF account, so we need to go by name instead of ID. So we let getDestinationName handle that.
		return nullable.NewNullNullable[string]()
	}
	return nullable.NewNullableWithValue(ffAccountID)
}

func getDestinationName(t domain.BankTransaction) nullable.Nullable[string] {
	if t.Type != domain.BankTransactionTypeWithdrawal {
		// our current FF account is the destination. We only have its ID (not the name), so we let getDestinationID handle that.
		return nullable.NewNullNullable[string]()
	}

	if t.Merchant == nil {
		logger.Warnf("got empty Merchant value for withdrawal #%s", t.ID)
		return nullable.NewNullableWithValue(unknownDestinationAccountName)
	}
	return nullable.NewNullableWithValue(*t.Merchant)
}

func mapTransactionType(in domain.BankTransactionType) generated.TransactionTypeProperty {
	switch in {
	case domain.BankTransactionTypeDeposit:
		return generated.Deposit
	case domain.BankTransactionTypeWithdrawal:
		return generated.Withdrawal
	default:
		panic("unmapped bank transaction type " + string(in))
	}
}
