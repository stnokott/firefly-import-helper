package structs

type TransactionUpdate struct {
	ApplyRules         bool                     `json:"apply_rules"`
	FireWebhooks       bool                     `json:"fire_webhooks"`
	GroupTitle         string                   `json:"group_title"`
	TransactionUpdates []TransactionSplitUpdate `json:"transactions"`
}

type TransactionSplitUpdate struct {
	JournalId        int    `json:"transaction_journal_id"`
	Description      string `json:"description"`
	MandateReference string `json:"sepa_db"`
	CreditorId       string `json:"destination_iban"`
}

type TransactionRead struct {
	Id         string `json:"id"`
	Attributes struct {
		GroupTitle   string `json:"group_title"`
		Transactions []struct {
			Amount          string `json:"amount"`
			CurrencySymbol  string `json:"currency_symbol"`
			Description     string `json:"description"`
			DestinationName string `json:"destination_name"`
			SourceName      string `json:"source_name"`
			CategoryName    string `json:"category_name"`
			Date            string `json:"date"`
		} `json:"transactions"`
	} `json:"attributes"`
}

type WebhookRead struct {
	Id         string            `json:"id"`
	Attributes WebhookAttributes `json:"attributes"`
}

type WebhookAttributes struct {
	Active   bool   `json:"active"`
	Title    string `json:"title"`
	Response string `json:"response"`
	Delivery string `json:"delivery"`
	Secret   string `json:"secret"`
	Trigger  string `json:"trigger"`
	Url      string `json:"url"`
}

type WhTransactionRead struct {
	Id           int                  `json:"id"`
	GroupTitle   string               `json:"group_title"`
	Transactions []WhTransactionSplit `json:"transactions"`
	Links        []struct {
		Rel string `json:"rel"`
		Uri string `json:"uri"`
	} `json:"links"`
}

type WhTransactionSplit struct {
	JournalId       int    `json:"transaction_journal_id"`
	Date            string `json:"date"`
	Amount          string `json:"amount"`
	CurrencySymbol  string `json:"currency_symbol"`
	Description     string `json:"description"`
	SourceName      string `json:"source_name"`
	DestinationName string `json:"destination_name"`
}

type WhUrlResult struct {
	Exists      bool
	NeedsUpdate bool
	Wh          *WebhookRead
}

type CategoryRead struct {
	Id         string `json:"id"`
	Attributes struct {
		Name string `json:"name"`
	} `json:"attributes"`
}
