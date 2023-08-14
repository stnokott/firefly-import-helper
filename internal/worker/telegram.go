package worker

import (
	"bytes"
	"firefly-iii-fix-ing/internal/structs"
	"fmt"
	"html/template"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	tele "gopkg.in/telebot.v3"
)

// TelegramBot handles sending Telegram messages and receiving commands.
type TelegramBot struct {
	targetChat         *tele.Chat
	bot                *tele.Bot
	transactionUpdater transactionUpdater
}

type transactionUpdater interface {
	SetTransactionCategory(id int, categoryName string) (*structs.TransactionRead, error)
	FireflyBaseURL() string
}

// NewBot creates a new telegramBot instance
func NewBot(token string, chatID int64) (*TelegramBot, error) {
	bot, err := tele.NewBot(
		tele.Settings{
			Token:  token,
			Poller: &tele.LongPoller{Timeout: 10 * time.Second},
		},
	)
	if err != nil {
		return nil, err
	}

	chat, err := bot.ChatByID(chatID)
	if err != nil {
		return nil, err
	}

	telegramBot := &TelegramBot{
		targetChat: chat,
		bot:        bot,
	}

	bot.Handle("/start", telegramBot.handleStart)
	bot.Handle(tele.OnCallback, telegramBot.handleInlineQueries)

	return telegramBot, nil
}

// Listen starts the telegram bot. Blocking.
func (b *TelegramBot) Listen() {
	log.Println("Running Telegram bot...")
	b.bot.Start()
}

func (b *TelegramBot) handleStart(c tele.Context) error {
	return c.Send(fmt.Sprintf("Hallo %s!\nDieser Bot ist eingerichtet für Nutzer "+
		"<a href=\"tg://user?id=%d\">%d</a>.", c.Chat().FirstName, b.targetChat.ID, b.targetChat.ID), tele.ModeHTML)
}

func (b *TelegramBot) handleInlineQueries(c tele.Context) error {
	log.Println("##### BEGIN CALLBACK ####")
	defer log.Println("###### END CALLBACK #####")
	var responseMsg string
	var editBody string

	callbackData := strings.Split(c.Data(), "|")[1:]
	targetTransactionID := callbackData[0]
	if targetTransactionID != buttonDataDone {
		// category button was pressed
		if targetTransactionIDInt, err := strconv.ParseInt(targetTransactionID, 10, 64); err != nil {
			// could not cast transaction id from data to int
			responseMsg = fmt.Sprintf("Transaktions-ID %s ungültig!", targetTransactionID)
		} else {
			targetCategoryName := callbackData[1]
			log.Println("Requested category change to", targetCategoryName)
			if updatedTransaction, err := b.transactionUpdater.SetTransactionCategory(int(targetTransactionIDInt), targetCategoryName); err != nil {
				responseMsg = "Update fehlgeschlagen: " + err.Error()
			} else if len(updatedTransaction.Attributes.Transactions) == 0 {
				responseMsg = "Update fehlgeschlagen: ungültiger Rückgabewert vom Server"
			} else {
				// success
				responseMsg = "Kategorie gesetzt auf " + updatedTransaction.Attributes.Transactions[0].CategoryName
				editBody, _ = b.transactionToMessageBody(updatedTransaction, b.transactionUpdater.FireflyBaseURL())
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
	TransactionID   string
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

func newNotificationParams(id string, fireflyBaseURL string, transactions []transactionNotification) *notificationParams {
	uri := fireflyBaseURL
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

func (b *TelegramBot) transactionToMessageBody(t *structs.TransactionRead, fireflyBaseURL string) (string, error) {
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
	params := newNotificationParams(t.Id, fireflyBaseURL, transactions)

	body := bytes.NewBufferString(``)
	if err := notificationTemplate.Execute(body, params); err != nil {
		return "", err
	}
	return body.String(), nil
}

func (b *TelegramBot) NotifyNewTransaction(t *structs.TransactionRead, fireflyBaseURL string, categories []structs.CategoryRead) error {
	if len(t.Attributes.Transactions) == 0 {
		return nil
	}

	// assemble menu buttons
	menu := tele.ReplyMarkup{}

	numCategories := len(categories)

	rows := make([]tele.Row, int(math.Ceil(float64(numCategories)/buttonsPerRow))+1) // + 1 to add row with "Done"-button
	// insert one button for each category
	for rowIndex := 0; rowIndex < len(rows)-1; rowIndex++ { // -1 for "Done"-button
		categoriesFirstIndex := rowIndex * buttonsPerRow
		categoriesLastIndex := int(math.Min(float64(categoriesFirstIndex+buttonsPerRow), float64(len(categories)-1)))
		categoriesThisRow := categories[categoriesFirstIndex:categoriesLastIndex]

		row := make([]tele.Btn, len(categoriesThisRow))
		for btnIndex := range row {
			category := categoriesThisRow[btnIndex]
			row[btnIndex] = menu.Data(category.Attributes.Name, t.Id+category.Id, t.Id, category.Attributes.Name)
		}
		rows[rowIndex] = menu.Row(row...)
	}
	// insert "Done"-button
	rows[len(rows)-1] = menu.Row(menu.Data("👍 Passt", t.Id+buttonDataDone, buttonDataDone))

	menu.Inline(rows...)

	notificationBody, err := b.transactionToMessageBody(t, fireflyBaseURL)
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

func (b *TelegramBot) NotifyError(err error) error {
	log.Println("ERROR:", err)
	body := fmt.Sprintf("<b>❗️ Firefly-III-Autoimporter Fehler ❗️</b>\n\n<i>%s</i>", err)

	if _, err = b.bot.Send(b.targetChat, body, tele.ModeHTML); err != nil {
		log.Println("error sending notification:", err)
		log.Println("initial error:", err)
		return err
	}
	return nil
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
