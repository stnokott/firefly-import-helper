package main

import (
	"bytes"
	"fmt"
	"github.com/containrrr/shoutrrr"
	"html/template"
	"log"
	"strconv"
	"strings"
	"time"
)

type notificationParams struct {
	Id           string
	Href         string
	Transactions []notificationTransaction
}

type notificationTransaction struct {
	SourceName      string
	DestinationName string
	AmountStr       string
	Description     string
	DateStr         string
}

var notificationTemplate = template.Must(template.New("telegramNotification").Parse(`
<b>💸 Neue Firefly-III-Transaktion 💸</b>
<a href="{{.Href}}">Transaktion #{{.Id}}</a>
<tg-spoiler>{{range .Transactions}}
	📆 <i>{{.DateStr}}</i>
	✏️ {{.Description}}
	⚖️ <i>{{.SourceName}}</i> ➜ <i>{{.DestinationName}}</i>
	💶 <u><b>{{.AmountStr}}</b></u>
{{end}}</tg-spoiler>`))

func (w *Worker) newNotificationParams(id string, transactions []notificationTransaction) *notificationParams {
	uri := w.fireflyBaseUrl
	if id != "" {
		uri += "/transactions/show/" + id
	} else {
		id = "n/a"
	}
	return &notificationParams{id, uri, transactions}
}

const maxLenDescription = 50
const maxLenAccountName = 25

var months = []string{
	"Januar",
	"Februar",
	"März",
	"April",
	"Mai",
	"Juni",
	"Juli",
	"August",
	"September",
	"Oktober",
	"November",
	"Dezember",
}

func newNotificationTransaction(date string, sourceName string, destName string, amount string, currencySymbol string, description string) *notificationTransaction {
	var dateFormatted string
	dateParsed, err := time.Parse("2006-01-02T15:04:04-07:00", date)
	if err == nil {
		dateFormatted = fmt.Sprintf(`%d. %s`, dateParsed.Day(), months[dateParsed.Month()-1])
	} else {
		log.Println("WARNING: could not parse date string:", err)
		dateFormatted = "n/a"
	}

	var amountFormatted string
	amountParsed, err := strconv.ParseFloat(amount, 32)
	if err == nil {
		amountFormatted = fmt.Sprintf(`%s%.2f`, currencySymbol, float32(amountParsed))
	} else {
		log.Println("WARNING: could not parse amount to float:", err)
		amountFormatted = "n/a"
	}
	return &notificationTransaction{
		SourceName:      formatStr(sourceName, maxLenAccountName),
		DestinationName: formatStr(destName, maxLenAccountName),
		AmountStr:       amountFormatted,
		Description:     formatStr(description, maxLenDescription),
		DateStr:         dateFormatted,
	}
}

func (w *Worker) notifyFromApiResponse(t *transactionRead) error {
	transactions := make([]notificationTransaction, len(t.Attributes.Transactions))
	for i, transaction := range t.Attributes.Transactions {
		transactions[i] = *newNotificationTransaction(
			transaction.Date,
			transaction.SourceName,
			transaction.DestinationName,
			transaction.Amount,
			transaction.CurrencySymbol,
			transaction.Description,
		)
	}
	return w.notifyNewTransaction(w.newNotificationParams(t.Id, transactions))
}

func (w *Worker) notifyFromWebhook(t *whTransactionRead) error {
	transactions := make([]notificationTransaction, len(t.Transactions))
	for i, transaction := range t.Transactions {
		transactions[i] = *newNotificationTransaction(
			transaction.Date,
			transaction.SourceName,
			transaction.DestinationName,
			transaction.Amount,
			transaction.CurrencySymbol,
			transaction.Description,
		)
	}
	return w.notifyNewTransaction(w.newNotificationParams(strconv.Itoa(t.Id), transactions))
}

func (w *Worker) notifyNewTransaction(params *notificationParams) error {
	if len(params.Transactions) == 0 {
		return nil
	}
	url := fmt.Sprintf(
		"telegram://%s@telegram?chats=%s&parseMode=HTML",
		w.telegramOptions.AccessToken,
		w.telegramOptions.ChatId,
	)

	body := bytes.NewBufferString(``)
	if err := notificationTemplate.Execute(body, params); err != nil {
		return err
	}
	log.Println(">> Sending notification...")
	return shoutrrr.Send(url, body.String())
}

func formatStr(s string, maxLen int) string {
	if len(s) > maxLen {
		s = s[:maxLen-3] + "..."
	}

	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
