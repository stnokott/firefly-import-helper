package worker

import (
	"firefly-iii-fix-ing/internal/modules"
	"firefly-iii-fix-ing/internal/structs"
	"log"
)

type Worker struct {
	fireflyApi  *fireflyApi
	telegramBot *telegramBot
}

type TelegramOptions struct {
	AccessToken string
	ChatId      int64
}

func NewWorker(fireflyAccessToken string, fireflyBaseUrl string, telegramOptions *TelegramOptions) (*Worker, error) {
	// remove trailing slash from Firefly III base URL
	if fireflyBaseUrl[len(fireflyBaseUrl)-1:] == "/" {
		fireflyBaseUrl = fireflyBaseUrl[:len(fireflyBaseUrl)-1]
	}

	bot, err := NewBot(telegramOptions.AccessToken, telegramOptions.ChatId)
	if err != nil {
		return nil, err
	}

	webhookUrl := fireflyBaseUrl + "/wh_fix_ing"

	w := &Worker{
		telegramBot: bot,
	}

	w.fireflyApi = newFireflyApi(
		fireflyBaseUrl,
		fireflyAccessToken,
		structs.WebhookAttributes{
			Active:   true,
			Title:    "Fix ING transaction descriptions from Importer",
			Response: "RESPONSE_TRANSACTIONS",
			Delivery: "DELIVERY_JSON",
			Trigger:  "TRIGGER_STORE_TRANSACTION",
			Url:      webhookUrl,
		},
		modules.NewModuleHandler(),
		w,
	)
	return w, nil
}

func (w *Worker) SendNotification(params *notificationParams) error {
	return w.telegramBot.notifyNewTransaction(params)
}

func (w *Worker) Listen() error {
	log.Println("Ensuring webhook exists...")
	url, err := w.fireflyApi.createOrUpdateWebhook()
	if err != nil {
		return err
	}
	log.Println(">> Webhook ready at", url)
	log.Println()

	// start telegram bot
	go w.telegramBot.Listen()
	return w.fireflyApi.Listen()
}
