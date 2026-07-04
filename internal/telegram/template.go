package telegram

import (
	"bytes"
	"fmt"
	"strconv"
	"text/template"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/importer"
)

type tmplDataTransaction struct {
	ID              int
	URL             string
	SubTransactions []tmplDataSubTransaction
}

type tmplDataSubTransaction struct {
	Description     string
	CategoryName    string
	DateStr         string
	SourceName      string
	DestinationName string
	AmountStr       string
}

var tmplTransaction = template.Must(template.New("telegramMsgTransaction").Parse(
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
`,
))

func (b *Bot) renderTmplTransaction(t *domain.FireflyTransaction) (string, error) {
	tData := make([]tmplDataSubTransaction, len(t.SubTransactions))
	for i, t := range t.SubTransactions {
		tData[i] = tmplDataSubTransaction{
			Description:     formatString(t.Description),
			CategoryName:    strOrDefault(t.CategoryName, "<b>Keine Kategorie gesetzt</b>"),
			DateStr:         timeFormat(t.Date),
			SourceName:      formatString(t.SourceName),
			DestinationName: formatString(t.DestinationName),
			AmountStr:       formatCurrency(float64(t.Amount), t.CurrencySymbol),
		}
	}
	data := tmplDataTransaction{
		ID:              t.ID,
		URL:             b.fireflyBaseURL.JoinPath("/transactions/show/", strconv.Itoa(t.ID)).String(),
		SubTransactions: tData,
	}
	return renderTmpl(tmplTransaction, data)
}

type tmplDataSummary struct {
	Summaries []importer.Summary
}

var tmplSummary = template.Must(template.New("telegramMsgSummary").Parse(
	`
<b>Import Summary:</b>
{{range .Summaries}}
	{{if .Success}}✅{{else}}❌{{end}} {{.Account}}({{.Institution}}) ➜ {{.Info}}
{{end}}
`,
))

func (*Bot) renderTmplSummary(sum []importer.Summary) (string, error) {
	data := tmplDataSummary{
		Summaries: sum,
	}
	return renderTmpl(tmplSummary, data)
}

func renderTmpl(tmpl *template.Template, data any) (string, error) {
	buf := new(bytes.Buffer)
	if err := tmpl.Execute(buf, data); err != nil {
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
