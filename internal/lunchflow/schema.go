package lunchflow

import (
	"strings"
	"time"
)

type Error struct {
	Err     string `json:"Error"` // avoid clash with Error() function
	Message string
}

func (e Error) Error() string {
	return e.Err + ": " + e.Message
}

type Accounts struct {
	Total    int
	Accounts []Account
}

type Account struct {
	ID              int
	Name            string
	InstitutionName string `json:"institution_name"`
	Provider        string
	Currency        *string
	Status          AccountStatus
}

type AccountStatus string

const (
	AccountStatusActive       AccountStatus = "ACTIVE"
	AccountStatusDisconnected AccountStatus = "DISCONNECTED"
	AccountStatusError        AccountStatus = "ERROR"
)

type Transactions struct {
	Total        int
	Transactions []Transaction
}

type Transaction struct {
	ID          string
	AccountID   int
	Amount      float64
	Currency    string
	Date        DateTime
	Merchant    *string
	Description *string
	IsPending   *bool
}

type DateTime struct {
	time.Time
}

func (dt *DateTime) UnmarshalJSON(data []byte) (err error) {
	stripQuotes := strings.Trim(string(data), `"`)
	dt.Time, err = time.Parse(time.DateOnly, stripQuotes)
	return
}

type Balance struct {
	Balance struct {
		Amount   float64
		Currency string
	}
}
