package worker

import (
	"bytes"
	"firefly-iii-fix-ing/internal/structs"
	"fmt"
	tele "gopkg.in/telebot.v3"
	"html/template"
	"log"
	"math"
	"strconv"
	"strings"
	"time"
)

type telegramBot struct {
	targetChat *tele.Chat
	bot        *tele.Bot
}

func NewBot(token string, chatId int64) (*telegramBot, error) {
	bot, err := tele.NewBot(
		tele.Settings{
			Token:  token,
			Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		},
	)
	if err != nil {
		return nil, err
	}

	chat, err := bot.ChatByID(chatId)
	if err != nil {
		return nil, err
	}

	telegramBot := &telegramBot{
		targetChat: chat,
		bot:        bot,
	}

	telegramBot.bot.Handle("/start", telegramBot.handleStart)
	return telegramBot, nil
}

func (b *telegramBot) Listen() {
	log.Println("Running Telegram bot...")
	b.bot.Start()
}

func (b *telegramBot) handleStart(c tele.Context) error {
	return c.Send(fmt.Sprintf("Hallo %s!\nDieser Bot ist eingerichtet für Nutzer "+
		"<a href=\"tg://user?id=%d\">%d</a>.", c.Chat().FirstName, b.targetChat.ID, b.targetChat.ID), tele.ModeHTML)
}

type notificationParams struct {
	TransactionId   string
	TransactionHref string
	SubTransactions []transactionNotification
}

type transactionNotification struct {
	SourceName      string
	DestinationName string
	AmountStr       string
	Description     string
	DateStr         string
	CategoryName    string
}

func newNotificationParams(id string, fireflyBaseUrl string, transactions []transactionNotification) *notificationParams {
	uri := fireflyBaseUrl
	if id != "" {
		uri += "/transactions/show/" + id
	} else {
		id = "n/a"
	}
	return &notificationParams{
		id,
		uri,
		transactions,
	}
}

var notificationTemplate = template.Must(template.New("telegramNotification").Parse(`
<b>💸 Neue Firefly-III-Transaktion 💸</b>
<a href="{{.TransactionHref}}">Transaktion #{{.TransactionId}}</a>
<tg-spoiler>{{range .SubTransactions}}
	✏️ {{.Description}}
	🏷️ {{.CategoryName}}
	📆 {{.DateStr}}
	⚖️ {{.SourceName}} ➜ {{.DestinationName}}
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

const buttonsPerRow = 3
const buttonDataDone = "fertig"

func (b *telegramBot) NotifyNewTransaction(t *structs.TransactionRead, fireflyBaseUrl string, categories []structs.CategoryRead) error {
	if len(t.Attributes.Transactions) == 0 {
		return nil
	}

	// assemble transactions
	transactions := make([]transactionNotification, len(t.Attributes.Transactions))
	for i, transaction := range t.Attributes.Transactions {
		transactions[i] = *newTransactionNotification(
			transaction.Date,
			transaction.SourceName,
			transaction.DestinationName,
			transaction.Amount,
			transaction.CurrencySymbol,
			transaction.CategoryName,
			transaction.Description,
		)
	}
	params := newNotificationParams(t.Id, fireflyBaseUrl, transactions)

	body := bytes.NewBufferString(``)
	if err := notificationTemplate.Execute(body, params); err != nil {
		return err
	}

	// assemble menu buttons
	menu := tele.ReplyMarkup{}

	numCategories := len(categories) + 1 // + 1 to add "Done"-Button

	rows := make([]tele.Row, int(math.Ceil(float64(numCategories)/buttonsPerRow)))
	j := 0
	for i := 0; i < numCategories; i += buttonsPerRow {
		end := i + buttonsPerRow
		if end > numCategories {
			end = numCategories
		}

		rowBtns := make([]tele.Btn, len(categories[i:end]))
		for i, category := range categories[i:end] {
			rowBtns[i] = menu.Data(category.Attributes.Name, t.Id+category.Id, t.Id, category.Id)
		}
		// add "done" button if at last iteration
		if i+buttonsPerRow >= numCategories {
			rowBtns[len(rowBtns)-1] = menu.Data("👍 Fertig", t.Id+buttonDataDone, buttonDataDone)
		}
		rows[j] = menu.Row(rowBtns...)
		j++
	}
	menu.Inline(rows...)

	_, err := b.bot.Send(
		b.targetChat,
		body.String(),
		&menu,
		tele.ModeHTML,
	)
	return err
}

func newTransactionNotification(date string, sourceName string, destName string, amount string, currencySymbol string, categoryName string, description string) *transactionNotification {
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
	return &transactionNotification{
		SourceName:      formatStr(sourceName, maxLenAccountName),
		DestinationName: formatStr(destName, maxLenAccountName),
		AmountStr:       amountFormatted,
		Description:     formatStr(description, maxLenDescription),
		DateStr:         dateFormatted,
		CategoryName:    categoryName,
	}
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
