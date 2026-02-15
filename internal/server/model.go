package server

import (
	"time"

	"github.com/google/uuid"
	"github.com/stnokott/firefly-import-helper/internal/client"
)

type WebhookResponse struct {
	UUID     uuid.UUID                  `json:"uuid"`
	UserID   int                        `json:"user_id"`
	Trigger  client.WebhookTrigger      `json:"trigger"`
	Response client.WebhookResponse     `json:"response"`
	Version  string                     `json:"version"`
	Content  WebhookResponseTransaction `json:"content"`
}

type WebhookResponseTransaction struct {
	ID              int                              `json:"id"`
	CreatedAt       time.Time                        `json:"created_at"`
	UpdatedAt       time.Time                        `json:"updated_at"`
	User            int                              `json:"user"`
	GroupTitle      string                           `json:"group_title"`
	SubTransactions []*WebhookResponseSubTransaction `json:"transactions"`
}

type WebhookResponseSubTransaction struct {
	Amount               client.Amount                  `json:"amount"`
	CurrencySymbol       string                         `json:"currency_symbol"`
	CategoryID           *string                        `json:"category_id"`
	CategoryName         *string                        `json:"category_name"`
	Date                 time.Time                      `json:"date"`
	Description          string                         `json:"description"`
	DestinationName      string                         `json:"destination_name"`
	SourceName           string                         `json:"source_name"`
	TransactionJournalID *string                        `json:"transaction_journal_id"`
	Type                 client.TransactionTypeProperty `json:"type"`
	User                 int                            `json:"user"`
}
