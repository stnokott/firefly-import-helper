package telegram

import (
	"bytes"
	"fmt"
	"strconv"
	"text/template"
	"time"

	telemodels "github.com/go-telegram/bot/models"

	"github.com/stnokott/firefly-import-helper/internal/server"
)

const templateParseMode = telemodels.ParseModeHTML

var notificationTemplate = template.Must(template.New("telegramNotification").Parse(
	`
<b>💸 Neue Firefly-III-Transaktion 💸</b>
<a href="{{.URL}}">Transaktion #{{.ID}}</a>
{{range .SubTransactions}}
	<tg-spoiler>✏️ {{.Description}}</tg-spoiler>
	<tg-spoiler>🏷️ {{.CategoryName}}</tg-spoiler>
	📆 {{.DateStr}}
	<tg-spoiler>⚖️ {{.SourceName}} ➜ {{.DestinationName}}</tg-spoiler>
	<tg-spoiler>💶 <b>{{.AmountStr}}</b></tg-spoiler>
{{end}}
`))

type templateData struct {
	ID              int
	URL             string
	SubTransactions []templateDataTransaction
}

type templateDataTransaction struct {
	Description     string
	CategoryName    string
	DateStr         string
	SourceName      string
	DestinationName string
	AmountStr       string
}

func (b *Bot) renderTemplate(wr *server.WebhookResponse) (string, error) {
	tData := make([]templateDataTransaction, len(wr.Content.Transactions))
	for i, t := range wr.Content.Transactions {
		tData[i] = templateDataTransaction{
			Description:     formatString(t.Description),
			CategoryName:    strOrDefault(t.CategoryName, "<b>Keine Kategorie gesetzt</b>"),
			DateStr:         timeFormat(t.Date),
			SourceName:      formatString(t.SourceName),
			DestinationName: formatString(t.DestinationName),
			AmountStr:       formatCurrency(float64(t.Amount), t.CurrencySymbol),
		}
	}
	data := templateData{
		ID:              wr.Content.ID,
		URL:             b.fireflyBaseURL + "/transactions/show/" + strconv.Itoa(wr.Content.ID),
		SubTransactions: tData,
	}
	buf := new(bytes.Buffer)
	if err := notificationTemplate.Execute(buf, &data); err != nil {
		return "", fmt.Errorf("could not render message template: %w", err)
	}
	return buf.String(), nil
}

func strOrDefault(v *string, defaultS string) string {
	if v == nil {
		return defaultS
	}
	return *v
}

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

func timeFormat(t time.Time) string {
	day := t.Day()
	month := months[t.Month()-1]
	year := t.Year()
	return fmt.Sprintf("%d. %s %d", day, month, year)
}

func formatCurrency(v float64, symbol string) string {
	return fmt.Sprintf(`%s%.2f`, symbol, v)
}

const maxDescriptionLength = 50

func formatString(v string) string {
	// yes, should ideally use utf8.RuneCountInString, but we don't have to be exact here
	// and we can thus avoid the overhead of the function call and just use len()
	if len(v) <= maxDescriptionLength {
		return v
	}
	additional := len(v) - maxDescriptionLength
	return v[:len(v)-additional] + "..."
}
