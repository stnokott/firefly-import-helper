package telegram

import (
	"bytes"
	"fmt"
	"strconv"
	"strings"
	"text/template"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/domain"
)

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

<b>{{.AccountName}} {{.Type | formatTransactionType}}{{.Amount | formatAmount}}{{.CurrencySymbol}}{{.Type | formatTransactionType}} {{.MerchantName}}</b>

<b>Description:</b> {{.Description}}
<b>Occurred:</b> <tg-time unix="{{.Date | timeUnix}}" format="r">{{.Date | formatTime}}</tg-time>
`,
))

func renderNewTransaction(data *domain.TransactionCreated) (string, error) {
	buf := new(bytes.Buffer)
	if err := tmplNewTransaction.Execute(buf, data); err != nil {
		return "", fmt.Errorf("failed to render template: %w", err)
	}
	return buf.String(), nil
}

func renderImportFinished(sums []domain.Summary) string {
	headers := []string{"Account", "Status", "Info"}
	rows := make([][]string, len(sums))
	for i, summary := range sums {
		status := "ERROR"
		if summary.Success {
			status = "OK"
		}
		rows[i] = []string{
			fmt.Sprintf("%s (%s)", summary.Account, summary.Institution),
			status,
			summary.Info,
		}
	}
	return "✨ <b>Import finished</b> ✨" + renderHTMLTable(headers, rows)
}

func renderHTMLTable(headers []string, rows [][]string) string {
	var builder strings.Builder
	write := func(s ...string) {
		for _, ss := range s {
			_, _ = builder.WriteString(ss)
		}
	}

	write("<pre>")

	// prepend with headers
	rows = append([][]string{headers}, rows...)

	// calculate column sizes for horizontal alignment
	columnSizes := make([]int, len(headers))
	for _, row := range rows {
		for j, s := range row {
			columnSizes[j] = max(columnSizes[j], len(s))
		}
	}

	for i, row := range rows {
		// line above header
		if i == 0 {
			for _, columnSize := range columnSizes {
				write("+", strings.Repeat("-", columnSize+2))
			}
			write("+\n")
		}

		for j, s := range row {
			write(fmt.Sprintf("| %-*s ", columnSizes[j], s))
		}
		write("|\n")

		// header/content separator and footer
		if i == 0 || i == len(rows)-1 {
			for _, columnSize := range columnSizes {
				write("+", strings.Repeat("-", columnSize+2))
			}
			write("+\n")
		}
	}

	write("</pre>\n")
	return builder.String()
}
