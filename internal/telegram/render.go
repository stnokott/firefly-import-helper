package telegram

import (
	"bytes"
	"fmt"
	"strconv"
	"text/template"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/domain"
)

var tmplAccountProblem = template.Must(template.New("tmplAccountProblem").Parse(
	`❓ <b>Problem with account</b> ❓

<b>Foo (Bar)</b>
Reauthorization required.
`,
))

func renderAccountProblem(data domain.AccountProblem) (string, error) {
	return renderTemplate(tmplAccountProblem, data)
}

var tmplNewTransaction = template.Must(template.New("tmplNewTransaction").Funcs(
	template.FuncMap{
		"formatTime": func(t time.Time) string {
			return t.Format(time.DateTime)
		},
		"timeUnix": func(t time.Time) string {
			return strconv.Itoa(int(t.Unix()))
		},
		"formatAmount": func(f float64) string {
			return fmt.Sprintf("%.2f", f)
		},
		"formatTransactionType": func(typ domain.TransactionType) string {
			if typ == domain.TransactionTypeDeposit {
				return "←"
			}
			return "→"
		},
	},
).Parse(
	`✨ <b>New Transaction <a href="{{.FireflyURL}}">#{{.FireflyID}}</a></b> ✨

<b>{{.AccountName}}</b> {{.Type | formatTransactionType}}{{.Amount | formatAmount}}{{.CurrencySymbol}}{{.Type | formatTransactionType}} <b>{{.MerchantName}}</b>

<b>Description:</b> <tg-spoiler>{{.Description}}</tg-spoiler>
<b>Occurred:</b> <tg-time unix="{{.Date | timeUnix}}" format="r">{{.Date | formatTime}}</tg-time>
`,
))

func renderNewTransaction(data *domain.TransactionRead) (string, error) {
	return renderTemplate(tmplNewTransaction, data)
}

func renderTemplate(t *template.Template, data any) (string, error) {
	buf := new(bytes.Buffer)
	if err := t.Execute(buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}
	return buf.String(), nil
}
