package telegram

import (
	"bytes"
	"fmt"
	"text/template"

	"github.com/stnokott/firefly-import-helper/internal/importer"
)

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
