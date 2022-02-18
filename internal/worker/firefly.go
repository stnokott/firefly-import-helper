package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"firefly-iii-fix-ing/internal/modules"
	"firefly-iii-fix-ing/internal/structs"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
	"time"
)

const port = 8080
const webhookPath = "/wh_fix_ing"
const pathAccounts = "/api/v1/accounts"
const pathTransaction = "/api/transactions"
const pathWebhooks = "/api/v1/webhooks"
const pathCategories = "/api/v1/categories"

type endpoints struct {
	account           string
	updateTransaction string
	webhooks          string
	categories        string
}

type fireflyApi struct {
	srv                *http.Server
	webhookUrl         string
	fireflyBaseUrl     string
	endpoints          endpoints
	fireflyAccessToken string
	targetWebhook      structs.WebhookAttributes
	moduleHandler      *modules.ModuleHandler
	notifManager       notificationManager
}

type notificationManager interface {
	SendNotification(params *notificationParams) error
}

func newFireflyApi(fireflyBaseUrl string, fireflyAccessToken string, targetWebhook structs.WebhookAttributes, moduleHandler *modules.ModuleHandler, notifManager notificationManager) *fireflyApi {
	f := fireflyApi{
		webhookUrl:     fireflyBaseUrl + webhookPath,
		fireflyBaseUrl: fireflyBaseUrl,
		endpoints: endpoints{
			account:           fireflyBaseUrl + pathAccounts,
			updateTransaction: fireflyBaseUrl + pathTransaction,
			webhooks:          fireflyBaseUrl + pathWebhooks,
			categories:        fireflyBaseUrl + pathCategories,
		},
		fireflyAccessToken: fireflyAccessToken,
		targetWebhook:      targetWebhook,
		moduleHandler:      moduleHandler,
		notifManager:       notifManager,
	}
	handler := http.NewServeMux()
	handler.HandleFunc("/", f.handleNewTransaction)

	f.srv = &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: handler,
	}
	return &f
}

func (f *fireflyApi) Listen() error {
	if f.srv == nil {
		return errors.New("please set callbacks before calling fireflyApi::Listen")
	}

	go func() {
		time.Sleep(1 * time.Second) // give a little time for httpServer to be up and running
		resp, err := http.Get(f.webhookUrl)
		//goland:noinspection GoUnhandledErrorResult
		if err != nil {
			log.Fatalln("Error validating httpServer connection:", err)
		} else if resp.StatusCode != 200 {
			log.Fatalln("Could not validate connection from webhook URL ", f.webhookUrl)
		}
		log.Println("Connection to", f.webhookUrl, "validated")
		log.Println("Ready to accept connections!")
	}()

	// start webserver
	log.Printf("Starting httpServer on port %d...", port)
	return f.srv.ListenAndServe()
}

func (f *fireflyApi) handleNewTransaction(_ http.ResponseWriter, r *http.Request) {
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

	if err := f.checkAndUpdateTransaction(target.Data); err != nil {
		log.Println(">> WARNING: error updating transaction:", err)
	}
	log.Println("######### DONE ##########")
}

func (f *fireflyApi) createOrUpdateWebhook() (string, error) {
	result, err := f.getWebhook()
	if err != nil {
		return "", err
	}
	if result.Exists && !result.NeedsUpdate {
		return result.Wh.Attributes.Url, nil
	} else {
		var method string
		var endpoint string
		if !result.Exists {
			// create
			method = "POST"
			endpoint = f.endpoints.webhooks
		} else {
			// update
			method = "PUT"
			endpoint = f.endpoints.webhooks + "/" + result.Wh.Id
		}
		body, err := json.Marshal(f.targetWebhook)
		if err != nil {
			return "", err
		}
		resp, err := f.request(method, endpoint, bytes.NewBuffer(body))
		if err != nil {
			return "", err
		}
		var target structs.WebhookAttributes
		//goland:noinspection GoUnhandledErrorResult
		defer resp.Body.Close()
		if err := json.NewDecoder(resp.Body).Decode(&target); err != nil {
			return "", err
		} else if target.Title == "" {
			return "", errors.New(">> webhook create/update unsuccessful")
		} else {
			return target.Url, nil
		}
	}
}

func (f *fireflyApi) getWebhook() (*structs.WhUrlResult, error) {
	wh, err := f.findWebhookByTitle()
	if err != nil {
		return nil, err
	}
	if wh == nil {
		return &structs.WhUrlResult{
			Exists:      false,
			NeedsUpdate: false,
		}, nil
	} else {
		if !wh.Attributes.Active ||
			wh.Attributes.Delivery != f.targetWebhook.Delivery ||
			wh.Attributes.Response != f.targetWebhook.Response ||
			wh.Attributes.Trigger != f.targetWebhook.Trigger ||
			wh.Attributes.Url != f.targetWebhook.Url {
			return &structs.WhUrlResult{
				Exists:      true,
				NeedsUpdate: true,
				Wh:          wh,
			}, nil
		} else {
			return &structs.WhUrlResult{
				Exists:      true,
				NeedsUpdate: false,
				Wh:          wh,
			}, nil
		}
	}
}

func (f *fireflyApi) findWebhookByTitle() (*structs.WebhookRead, error) {
	resp, err := f.request("GET", f.endpoints.webhooks, nil)
	if err != nil {
		return nil, err
	}
	var webhooksResponse struct {
		Webhooks []structs.WebhookRead `json:"data"`
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&webhooksResponse); err != nil {
		return nil, err
	}

	for _, webhook := range webhooksResponse.Webhooks {
		if webhook.Attributes.Title == f.targetWebhook.Title {
			return &webhook, nil
		}
	}
	return nil, nil
}

func (f *fireflyApi) checkAndUpdateTransaction(t structs.WhTransactionRead) error {
	var transactionSplitUpdates []structs.TransactionSplitUpdate
	for _, transactionInner := range t.Transactions {
		log.Println(">> ID: #" + strconv.Itoa(t.Id))
		log.Println(">> Description: '" + transactionInner.Description + "'")
		update, err := f.moduleHandler.Process(&transactionInner)
		if err != nil {
			log.Println("WARNING: error running modules:", err)
		} else if update != nil {
			transactionSplitUpdates = append(transactionSplitUpdates, *update)
		}
	}

	if len(transactionSplitUpdates) == 0 {
		log.Println(">>>> No fix applied")
		return f.notifyFromWebhook(&t)
	} else {
		endpoint := fmt.Sprintf("%s/%d", f.endpoints.updateTransaction, t.Id)
		updateObj := structs.TransactionUpdate{
			ApplyRules:         true,
			FireWebhooks:       false,
			GroupTitle:         transactionSplitUpdates[0].Description,
			TransactionUpdates: transactionSplitUpdates,
		}
		updateObjBytes, err := json.Marshal(updateObj)
		if err != nil {
			return err
		}
		log.Println(">> Communicating with Firefly-III...")
		resp, err := f.request("PUT", endpoint, bytes.NewBuffer(updateObjBytes))
		if err != nil {
			return err
		}

		var updateResponse structs.TransactionSingle
		//goland:noinspection GoUnhandledErrorResult
		defer resp.Body.Close()
		respBytes, err := ioutil.ReadAll(resp.Body)
		if err := json.Unmarshal(respBytes, &updateResponse); err != nil {
			return errors.New(fmt.Sprintf("transaction update #%d: %s", t.Id, err))
		}
		return f.notifyFromApiResponse(&updateResponse.Data)
	}
}

func (f *fireflyApi) notifyFromApiResponse(t *structs.TransactionRead) error {
	transactions := make([]notificationTransaction, len(t.Attributes.Transactions))
	for i, transaction := range t.Attributes.Transactions {
		transactions[i] = *newNotificationTransaction(
			transaction.Date,
			transaction.SourceName,
			transaction.DestinationName,
			transaction.Amount,
			transaction.CurrencySymbol,
			transaction.Description,
		)
	}
	return f.notifManager.SendNotification(newNotificationParams(t.Id, f.fireflyBaseUrl, transactions))
}

func (f *fireflyApi) notifyFromWebhook(t *structs.WhTransactionRead) error {
	transactions := make([]notificationTransaction, len(t.Transactions))
	for i, transaction := range t.Transactions {
		transactions[i] = *newNotificationTransaction(
			transaction.Date,
			transaction.SourceName,
			transaction.DestinationName,
			transaction.Amount,
			transaction.CurrencySymbol,
			transaction.Description,
		)
	}
	return f.notifManager.SendNotification(newNotificationParams(strconv.Itoa(t.Id), f.fireflyBaseUrl, transactions))
}

func (f *fireflyApi) request(method string, url string, body io.Reader) (*http.Response, error) {
	r, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	r.Header.Add("Authorization", "Bearer "+f.fireflyAccessToken)
	if method == http.MethodPut || method == http.MethodPost {
		r.Header.Add("Content-Type", "application/json")
	}
	r.Header.Add("Accept", "application/json")

	return http.DefaultClient.Do(r)
}
