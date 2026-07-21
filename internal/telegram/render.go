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
	`<h3>Problem with account</h3>
<hr/>
<b>{{.Account.Name}}</b> ({{.Account.Institution}}): {{.Problem}}
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
			switch typ {
			case domain.TransactionTypeDeposit:
				return "Deposit ⤵️"
			case domain.TransactionTypeWithdrawal:
				return "Withdrawal ⤴️"
			default:
				return "Transfer ➡️"
			}
		},
		"transactionTypeArrow": func(typ domain.TransactionType) string {
			if typ == domain.TransactionTypeDeposit {
				return "←"
			}
			return "→"
		},
	},
).Parse(
	`<h3>✨ New Transaction <a href="{{.FireflyURL}}">#{{.FireflyID}}</a> ✨</h3>
<hr/>
<b>{{.AccountName}}</b> {{.Type | transactionTypeArrow}}{{.Amount | formatAmount}}{{.CurrencySymbol}}{{.Type | transactionTypeArrow}} <tg-spoiler><b>{{.MerchantName}}</b></tg-spoiler>
<hr/>
<b>Type:</b> {{.Type | formatTransactionType}}
<br/>
<b>Description:</b> <tg-spoiler>{{.Description}}</tg-spoiler>
<br/>
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
