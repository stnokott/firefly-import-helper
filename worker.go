package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
)

type Worker struct {
	moduleHandler             *moduleHandler
	telegramOptions           *TelegramOptions
	endpointUpdateTransaction string
	endpointAccount           string
	endpointWebhooks          string
	webhookUrl                string
	targetWebhook             webhookAttributes
	fireflyAccessToken        string
	fireflyBaseUrl            string
}

type TelegramOptions struct {
	AccessToken string
	ChatId      string
}

func NewWorker(fireflyAccessToken string, fireflyBaseUrl string, telegramOptions *TelegramOptions) (*Worker, error) {
	// remove trailing slash from Firefly III base URL
	if fireflyBaseUrl[len(fireflyBaseUrl)-1:] == "/" {
		fireflyBaseUrl = fireflyBaseUrl[:len(fireflyBaseUrl)-1]
	}

	webhookUrl := fireflyBaseUrl + "/wh_fix_ing"
	return &Worker{
		moduleHandler:             NewModuleHandler(),
		endpointUpdateTransaction: fireflyBaseUrl + "/api/v1/transactions/%d",
		endpointAccount:           fireflyBaseUrl + "/api/v1/accounts/%s",
		endpointWebhooks:          fireflyBaseUrl + "/api/v1/webhooks",
		webhookUrl:                webhookUrl,
		telegramOptions:           telegramOptions,
		targetWebhook: webhookAttributes{
			Active:   true,
			Title:    "Fix ING transaction descriptions from Importer",
			Response: "RESPONSE_TRANSACTIONS",
			Delivery: "DELIVERY_JSON",
			Trigger:  "TRIGGER_STORE_TRANSACTION",
			Url:      webhookUrl,
		},
		fireflyAccessToken: fireflyAccessToken,
		fireflyBaseUrl:     fireflyBaseUrl,
	}, nil
}

const port = 8080

func (w *Worker) Listen() error {
	log.Println("Ensuring webhook exists...")
	url, err := w.createOrUpdateWebhook()
	if err != nil {
		return err
	}
	log.Println(">> Webhook ready at", url)

	http.HandleFunc("/", w.handleWebhook)
	log.Println()
	log.Printf("Listening for webhooks on port %d...", port)
	err = http.ListenAndServe(fmt.Sprintf(":%d", port), nil)
	if err != nil && err != http.ErrServerClosed {
		log.Println("HTTP server error:", err)
	}
	return nil
}

func (w *Worker) handleWebhook(_ http.ResponseWriter, r *http.Request) {
	log.Println()
	log.Println("### BEGIN NEW WEBHOOK ###")
	defer func() {
		log.Println("######### DONE ##########")
	}()

	//goland:noinspection GoUnhandledErrorResult
	defer r.Body.Close()
	var target struct {
		Version string            `json:"version"`
		Data    whTransactionRead `json:"content"`
	}
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		log.Println("ERROR: could not parse webhook body:", err)
		return
	} else if target.Version == "" {
		log.Println("ERROR: webhook body not in expected structure")
		return
	}

	if err := w.checkAndUpdateTransaction(target.Data); err != nil {
		log.Println(">> WARNING: error updating transaction:", err)
	}
}

func (w *Worker) request(method string, url string, params map[string]string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	r.Header.Add("Authorization", "Bearer "+w.fireflyAccessToken)
	if method == http.MethodPut || method == http.MethodPost {
		r.Header.Add("Content-Type", "application/json")
	}
	r.Header.Add("Accept", "application/json")

	if params != nil {
		for k, v := range params {
			r.URL.Query().Add(k, v)
		}
		r.URL.RawQuery = r.URL.String()
	}

	return http.DefaultClient.Do(r)
}
