package firefly

import (
	"net/url"
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
//goverter:enum yes
//goverter:extend copyTime nullableFromTime nullableFromString
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
	//goverter:ignore InternalReference InvoiceDate Notes Order PaymentDate PiggyBankId PiggyBankName ProcessDate
	//goverter:ignore Reconciled SepaBatchId SepaCc SepaCi SepaCountry SepaCtId SepaCtOp SepaDb SepaEp
	//goverter:ignore Tags
	//goverter:map Date BookDate
	//goverter:map Currency CurrencyCode
	//goverter:map . DestinationId | getDestinationID
	//goverter:map . DestinationName | getDestinationName
	//goverter:map ID ExternalId
	//goverter:map . SourceId | getSourceID
	//goverter:map . SourceName | getSourceName
	ConvertTransaction(t domain.BankTransaction, ffAccountID string) *generated.TransactionSplitStore

	//goverter:enum:unknown @panic
	//goverter:enum:map TransactionTypeDeposit Deposit
	//goverter:enum:map TransactionTypeWithdrawal Withdrawal
	//goverter:enum:map TransactionTypeTransfer Transfer
	ConvertTransactionTypeDomain(domain.TransactionType) generated.TransactionTypeProperty

	//goverter:enum:unknown @panic
	//goverter:enum:map Deposit TransactionTypeDeposit
	//goverter:enum:map Withdrawal TransactionTypeWithdrawal
	//goverter:enum:map Transfer TransactionTypeTransfer
	//goverter:enum:map OpeningBalance @panic
	//goverter:enum:map Reconciliation @panic
	ConvertTransactionTypeFirefly(generated.TransactionTypeProperty) domain.TransactionType

	//goverter:ignore FireflyID FireflyURL
	//goverter:map . AccountName | getAccountName
	//goverter:map . MerchantName | getMerchantName
	//goverter:map CategoryName Category | stringOrEmpty
	//goverter:map CurrencySymbol CurrencySymbol | currencySymbolOrDefault
	ConvertTransactionSplit(t generated.TransactionSplit) *domain.TransactionRead
}

func ConvertTransactionRead(in *generated.TransactionRead, fireflyBaseURL url.URL) *domain.TransactionRead {
	converted := ConvertTransactionSplit(in.Attributes.Transactions[0])
	converted.FireflyID = in.Id
	converted.FireflyURL = fireflyBaseURL.JoinPath("/transactions/show/", in.Id).String()
	return converted
}

func boolOrTrue(b *bool) bool {
	if b != nil {
		return *b
	}
	return true
}

func copyTime(t time.Time) time.Time {
	return t
}

func nullableFromTime(t time.Time) nullable.Nullable[time.Time] {
	return nullable.NewNullableWithValue(t)
}

func nullableFromString(s string) nullable.Nullable[string] {
	return nullable.NewNullableWithValue(s)
}

func stringOrEmpty(s nullable.Nullable[string]) string {
	if s.IsNull() {
		return ""
	}
	return s.MustGet()
}

func nullableOrDefault[T any](v nullable.Nullable[T], def T) T {
	if v.IsNull() {
		return def
	}
	vActual, _ := v.Get()
	return vActual
}

func defaultTransactionSplitStore() generated.TransactionSplitStore {
	notes := "imported using " + config.AppName
	return generated.TransactionSplitStore{
		Notes:      nullable.NewNullableWithValue(notes),
		Reconciled: new(true),
	}
}

//goverter:context ffAccountID
func getSourceID(t domain.BankTransaction, ffAccountID string) nullable.Nullable[string] {
	if t.Type != domain.TransactionTypeWithdrawal {
		// the source is not our FF account, so we need to go by name instead of ID. So we let getSourceName handle that.
		return nullable.NewNullNullable[string]()
	}
	return nullable.NewNullableWithValue(ffAccountID)
}

func getSourceName(t domain.BankTransaction) nullable.Nullable[string] {
	if t.Type != domain.TransactionTypeDeposit {
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
	if t.Type != domain.TransactionTypeDeposit {
		// the destination is not our FF account, so we need to go by name instead of ID. So we let getDestinationName handle that.
		return nullable.NewNullNullable[string]()
	}
	return nullable.NewNullableWithValue(ffAccountID)
}

func getDestinationName(t domain.BankTransaction) nullable.Nullable[string] {
	if t.Type != domain.TransactionTypeWithdrawal {
		// our current FF account is the destination. We only have its ID (not the name), so we let getDestinationID handle that.
		return nullable.NewNullNullable[string]()
	}

	if t.Merchant == nil {
		logger.Warnf("got empty Merchant value for withdrawal #%s", t.ID)
		return nullable.NewNullableWithValue(unknownDestinationAccountName)
	}
	return nullable.NewNullableWithValue(*t.Merchant)
}

func getAccountName(in generated.TransactionSplit) string {
	var v nullable.Nullable[string]
	if in.Type == generated.Deposit {
		v = in.DestinationName
	} else {
		v = in.SourceName
	}
	return nullableOrDefault(v, "unknown")
}

func getMerchantName(in generated.TransactionSplit) string {
	var v nullable.Nullable[string]
	if in.Type == generated.Deposit {
		v = in.SourceName
	} else {
		v = in.DestinationName
	}
	return nullableOrDefault(v, "unknown")
}

func currencySymbolOrDefault(cc *string) string {
	if cc != nil {
		return *cc
	}
	return "?"
}
