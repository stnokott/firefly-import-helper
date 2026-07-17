package telegram

import (
	"fmt"
	"strings"

	"github.com/stnokott/firefly-import-helper/internal/domain"
)

func drawImportFinished(sums []domain.Summary) string {
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
	return "✨ Import finished ✨" + drawHTMLTable(headers, rows)
}

func drawHTMLTable(headers []string, rows [][]string) string {
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
