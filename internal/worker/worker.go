package worker

import (
	"encoding/json"
	"firefly-iii-fix-ing/internal/modules"
	"firefly-iii-fix-ing/internal/structs"
	"fmt"
	"log"
	"net/http"
	"time"
)

const httpPort = 8080

type Worker struct {
	moduleHandler             *modules.ModuleHandler
	endpointUpdateTransaction string
	endpointAccount           string
	endpointWebhooks          string
	webhookUrl                string
	targetWebhook             structs.WebhookAttributes
	fireflyAccessToken        string
	fireflyBaseUrl            string
	telegramBot               *TelegramBot
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
	return &Worker{
		moduleHandler:             modules.NewModuleHandler(),
		endpointUpdateTransaction: fireflyBaseUrl + "/api/v1/transactions/%d",
		endpointAccount:           fireflyBaseUrl + "/api/v1/accounts/%s",
		endpointWebhooks:          fireflyBaseUrl + "/api/v1/webhooks",
		webhookUrl:                webhookUrl,
		targetWebhook: structs.WebhookAttributes{
			Active:   true,
			Title:    "Fix ING transaction descriptions from Importer",
			Response: "RESPONSE_TRANSACTIONS",
			Delivery: "DELIVERY_JSON",
			Trigger:  "TRIGGER_STORE_TRANSACTION",
			Url:      webhookUrl,
		},
		fireflyAccessToken: fireflyAccessToken,
		fireflyBaseUrl:     fireflyBaseUrl,
		telegramBot:        bot,
	}, nil
}

func (w *Worker) Listen() error {
	log.Println("Ensuring webhook exists...")
	url, err := w.createOrUpdateWebhook()
	if err != nil {
		return err
	}
	log.Println(">> Webhook ready at", url)
	log.Println()

	// start telegram bot
	go w.telegramBot.Listen()

	srv := w.newHttpServer()

	go func() {
		time.Sleep(1 * time.Second) // give a little time for server to be up and running
		resp, err := http.Get(w.webhookUrl)
		//goland:noinspection GoUnhandledErrorResult
		if err != nil {
			log.Fatalln("Error validating server connection:", err)
		} else if resp.StatusCode != 200 {
			log.Fatalln("Could not validate connection from webhook URL ", w.webhookUrl)
		}
		log.Println("Connection to", w.webhookUrl, "validated")
		log.Println("Ready to accept connections!")
	}()

	// start webserver
	log.Printf("Starting server on port %d...", httpPort)
	err = srv.ListenAndServe()
	if err != nil && err != http.ErrServerClosed {
		log.Println("HTTP server error:", err)
	}

	return nil
}

func (w *Worker) newHttpServer() *http.Server {
	handler := http.NewServeMux()
	handler.HandleFunc("/", w.handleWebhook)
	srv := &http.Server{
		Addr:    fmt.Sprintf(":%d", httpPort),
		Handler: handler,
	}
	return srv
}

func (w *Worker) handleWebhook(_ http.ResponseWriter, r *http.Request) {
	//goland:noinspection GoUnhandledErrorResult
	defer r.Body.Close()
	var target struct {
		Version string                    `json:"version"`
		Data    structs.WhTransactionRead `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil || target.Version == "" {
		log.Println("WARNING: received request with invalid body structure")
		return
	}

	log.Println()
	log.Println("### BEGIN NEW WEBHOOK ###")

	if err := w.checkAndUpdateTransaction(target.Data); err != nil {
		log.Println(">> WARNING: error updating transaction:", err)
	}
	log.Println("######### DONE ##########")
}
