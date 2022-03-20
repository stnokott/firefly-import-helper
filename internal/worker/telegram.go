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
	targetChat         *tele.Chat
	bot                *tele.Bot
	transactionUpdater transactionUpdater
}

type transactionUpdater interface {
	SetTransactionCategory(id int, categoryName string) (*structs.TransactionRead, error)
	FireflyBaseUrl() string
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

	bot.Handle("/start", telegramBot.handleStart)
	bot.Handle(tele.OnCallback, telegramBot.handleInlineQueries)

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

func (b *telegramBot) handleInlineQueries(c tele.Context) error {
	log.Println("##### BEGIN CALLBACK ####")
	defer func() {
		log.Println("###### END CALLBACK #####")
	}()
	var responseMsg string
	var editBody string

	callbackData := strings.Split(c.Data(), "|")
	callbackData = callbackData[1:]
	targetTransactionId := callbackData[0]
	if targetTransactionId != buttonDataDone {
		// category button was pressed
		if targetTransactionIdInt, err := strconv.ParseInt(targetTransactionId, 10, 64); err != nil {
			// could not cast transaction id from data to int
			responseMsg = fmt.Sprintf("Transaktions-ID %s ungültig!", targetTransactionId)
		} else {
			targetCategoryName := callbackData[1]
			log.Println("Requested category change to", targetCategoryName)
			if updatedTransaction, err := b.transactionUpdater.SetTransactionCategory(int(targetTransactionIdInt), targetCategoryName); err != nil {
				responseMsg = "Update fehlgeschlagen: " + err.Error()
			} else if len(updatedTransaction.Attributes.Transactions) == 0 {
				responseMsg = "Update fehlgeschlagen: ungültiger Rückgabewert vom Server"
			} else {
				// success
				responseMsg = "Kategorie gesetzt auf " + updatedTransaction.Attributes.Transactions[0].CategoryName
				editBody, _ = b.transactionToMessageBody(updatedTransaction, b.transactionUpdater.FireflyBaseUrl())
			}
		}
	} else {
		// "Done"-Button pressed -> do nothing
		log.Println("No option chosen")
		responseMsg = ""
	}
	// remove inline buttons
	var err error
	if editBody != "" {
		err = c.Edit(editBody, &tele.ReplyMarkup{}, tele.ModeHTML)
	} else {
		err = c.Edit(&tele.ReplyMarkup{})
	}
	if err != nil {
		log.Println("WARNING: could not delete inline buttons:", err)
	}
	log.Printf(">> Sending response message: '%s'", responseMsg)
	return c.Respond(&tele.CallbackResponse{
		Text:      responseMsg,
		ShowAlert: false,
	})
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

func (b *telegramBot) transactionToMessageBody(t *structs.TransactionRead, fireflyBaseUrl string) (string, error) {
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
		return "", err
	}
	return body.String(), nil
}

func (b *telegramBot) NotifyNewTransaction(t *structs.TransactionRead, fireflyBaseUrl string, categories []structs.CategoryRead) error {
	if len(t.Attributes.Transactions) == 0 {
		return nil
	}

	// assemble menu buttons
	menu := tele.ReplyMarkup{}

	numCategories := len(categories) + 1 // + 1 to add "Done"-Button

	rows := make([]tele.Row, int(math.Ceil(float64(numCategories)/buttonsPerRow)))
	rowIndex := 0
	for i := 0; i < numCategories; i += buttonsPerRow {
		end := i + buttonsPerRow
		if end > numCategories {
			end = numCategories
		}
		isLastRow := i+buttonsPerRow >= numCategories
		if isLastRow {
			end-- // to prevent out of bounds error when accessing categories slice
		}

		rowBtns := make([]tele.Btn, len(categories[i:end]))
		for i, category := range categories[i:end] {
			rowBtns[i] = menu.Data(category.Attributes.Name, t.Id+category.Id, t.Id, category.Attributes.Name)
		}
		// add "done" button if at last iteration
		if isLastRow {
			rowBtns[len(rowBtns)-1] = menu.Data("👍 Passt", t.Id+buttonDataDone, buttonDataDone)
		}
		rows[rowIndex] = menu.Row(rowBtns...)
		rowIndex++
	}
	menu.Inline(rows...)

	notificationBody, err := b.transactionToMessageBody(t, fireflyBaseUrl)
	if err != nil {
		return err
	}

	_, err = b.bot.Send(
		b.targetChat,
		notificationBody,
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

func (b *telegramBot) NotifyError(err error) error {
	body := fmt.Sprintf("<b>❗️ Firefly-III-Autoimporter Fehler ❗️</b>\n\n<i>%s</i>", err)

	_, err = b.bot.Send(
		b.targetChat,
		body,
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
