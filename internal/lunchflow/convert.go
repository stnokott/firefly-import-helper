package lunchflow

import (
	"strings"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/domain"
)

//go:generate go tool goverter gen -output-constraint= ./

//goverter:converter
//goverter:output:format function
//goverter:output:file convert.gen.go
//goverter:output:package github.com/stnokott/firefly-import-helper/internal/lunchflow
//goverter:extend copyDateTime
type Converter interface {
	ConvertAccounts([]Account) []domain.BankAccount
	//goverter:map Currency | currencyOrDefault
	//goverter:map InstitutionName Institution
	ConvertAccount(Account) domain.BankAccount
	//goverter:enum:unknown BankAccountStatusError
	//goverter:enum:map AccountStatusActive BankAccountStatusActive
	//goverter:enum:map AccountStatusDisconnected BankAccountStatusDisconnected
	//goverter:enum:map AccountStatusError BankAccountStatusError
	ConvertAccountStatus(AccountStatus) domain.BankAccountStatus

	ConvertTransactions([]Transaction) []domain.BankTransaction
	//goverter:map Amount Amount | math:Abs
	//goverter:map . Type  | determineTransactionType
	//goverter:map . Description | applyDescriptionRules
	//goverter:map IsPending | boolOrFalse
	ConvertTransaction(Transaction) domain.BankTransaction
}

func currencyOrDefault(c *string) string {
	if c == nil {
		return "€"
	}
	return *c
}

func boolOrFalse(b *bool) bool {
	if b == nil {
		return false
	}
	return *b
}

func determineTransactionType(t Transaction) domain.BankTransactionType {
	if t.Amount < 0 {
		return domain.BankTransactionTypeWithdrawal
	}
	return domain.BankTransactionTypeDeposit
}

func copyDateTime(t DateTime) time.Time {
	return t.Time
}

func applyDescriptionRules(t Transaction) string {
	if t.Description == nil {
		return "<no description>"
	}

	desc := *t.Description
	for name, rule := range descriptionRules {
		logger.Debugf("before description transformation rule '%s': %s", name, desc)
		desc = rule(desc)
		logger.Debugf("after description transformation rule '%s': %s", name, desc)
	}

	return desc
}

type descriptionRule func(string) string

var descriptionRules = map[string]descriptionRule{
	"Semicolons": func(desc string) string {
		// Some transaction descriptions seem to be formatted with "; " in-between characters.
		// E.g. "1234567890123/PP.1234.PP/. Firefly; III sto, re, Your purchase at Firefly II; I store"
		return strings.Join(strings.Split(desc, "; "), "")
	},
}
