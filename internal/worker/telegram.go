package worker

import (
	"bytes"
	"fmt"
	tele "gopkg.in/telebot.v3"
	"html/template"
	"log"
	"strconv"
	"strings"
	"time"
)

type TelegramBot struct {
	chatId int64
	bot    *tele.Bot
}

func NewBot(token string, chatId int64) (*TelegramBot, error) {
	bot, err := tele.NewBot(
		tele.Settings{
			Token:  token,
			Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		},
	)
	if err != nil {
		return nil, err
	}

	return &TelegramBot{chatId: chatId, bot: bot}, nil
}

func (t *TelegramBot) Listen() {
	log.Println("Running Telegram bot...")
	t.bot.Start()
}

type notificationParams struct {
	TransactionId   string
	TransactionHref string
	SubTransactions []notificationTransaction
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
<a href="{{.TransactionHref}}">Transaktion #{{.TransactionId}}</a>
<tg-spoiler>{{range .SubTransactions}}
	📆 <i>{{.DateStr}}</i>
	✏️ {{.Description}}
	⚖️ <i>{{.SourceName}}</i> ➜ <i>{{.DestinationName}}</i>
	💶 <u><b>{{.AmountStr}}</b></u>
{{end}}</tg-spoiler>`))

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

func (t *TelegramBot) notifyNewTransaction(params *notificationParams) error {
	if len(params.SubTransactions) == 0 {
		return nil
	}

	body := bytes.NewBufferString(``)
	if err := notificationTemplate.Execute(body, params); err != nil {
		return err
	}
	log.Println(">> Sending notification...")

	_, err := t.bot.Send(
		&tele.User{ID: t.chatId},
		body.String(),
		tele.ModeHTML,
	)
	return err
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
