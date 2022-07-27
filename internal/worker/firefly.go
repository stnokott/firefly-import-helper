package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"firefly-iii-fix-ing/internal/modules"
	"firefly-iii-fix-ing/internal/structs"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"
)

const port = 8822
const webhookPath = "/wh_fix_ing"
const pathAccounts = "/api/v1/accounts"
const pathTransaction = "/api/v1/transactions"
const pathWebhooks = "/api/v1/webhooks"
const pathCategories = "/api/v1/categories"

type endpoints struct {
	account      string
	transactions string
	webhooks     string
	categories   string
}

type fireflyApi struct {
	srv                *http.Server
	webhookUrl         string
	fireflyBaseUrl     string
	endpoints          endpoints
	fireflyAccessToken string
	targetWebhook      structs.WebhookAttributes
	moduleHandler      *modules.ModuleHandler
	notifManager       transactioNotifier
}

type transactioNotifier interface {
	NotifyNewTransaction(t *structs.TransactionRead, fireflyBaseUrl string, categories []structs.CategoryRead) error
}

func newFireflyApi(fireflyBaseUrl string, fireflyAccessToken string, moduleHandler *modules.ModuleHandler, notifManager transactioNotifier) *fireflyApi {
	f := fireflyApi{
		webhookUrl:     fireflyBaseUrl + webhookPath,
		fireflyBaseUrl: fireflyBaseUrl,
		endpoints: endpoints{
			account:      fireflyBaseUrl + pathAccounts,
			transactions: fireflyBaseUrl + pathTransaction,
			webhooks:     fireflyBaseUrl + pathWebhooks,
			categories:   fireflyBaseUrl + pathCategories,
		},
		fireflyAccessToken: fireflyAccessToken,
		targetWebhook: structs.WebhookAttributes{
			Active:   true,
			Title:    "Fix ING transaction descriptions from Importer",
			Response: "RESPONSE_TRANSACTIONS",
			Delivery: "DELIVERY_JSON",
			Trigger:  "TRIGGER_STORE_TRANSACTION",
			Url:      fireflyBaseUrl + webhookPath,
		},
		moduleHandler: moduleHandler,
		notifManager:  notifManager,
	}
	handler := http.NewServeMux()
	handler.HandleFunc("/", f.handleNewTransactionWebhook)

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
		client := http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(f.webhookUrl)
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

func (f *fireflyApi) handleNewTransactionWebhook(_ http.ResponseWriter, r *http.Request) {
	var target struct {
		Version string                    `json:"version"`
		Data    structs.WhTransactionRead `json:"content"`
	}
	//goland:noinspection GoUnhandledErrorResult
	defer r.Body.Close()
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil || target.Version == "" {
		log.Println("WARNING: received request with invalid body structure")
		return
	}

	log.Println()
	log.Println("### BEGIN NEW WEBHOOK ###")

	if err := f.checkAndUpdateTransaction(target.Data); err != nil {
		log.Println(">> WARNING: error updating transactions:", err)
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
		var resultWebhook struct {
			Data structs.WebhookRead `json:"data"`
		}
		//goland:noinspection GoUnhandledErrorResult
		defer resp.Body.Close()
		respBytes, _ := io.ReadAll(resp.Body)
		if err := json.Unmarshal(respBytes, &resultWebhook); err != nil {
			return "", err
		} else if resultWebhook.Data.Attributes.Title == "" {
			return "", errors.New(">> webhook create/update unsuccessful")
		} else {
			return resultWebhook.Data.Attributes.Url, nil
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
	if resp.StatusCode == http.StatusNotFound {
		return nil, nil
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
	for i := range t.Transactions {
		transactionInner := t.Transactions[i]
		log.Println(">> ID: #" + strconv.Itoa(t.Id))
		log.Println(">> Description: '" + transactionInner.Description + "'")
		update, err := f.moduleHandler.Process(&transactionInner)
		if err != nil {
			log.Println("WARNING: error running modules:", err)
		} else if update != nil {
			transactionSplitUpdates = append(transactionSplitUpdates, *update)
		}
	}

	var resultTransaction *structs.TransactionRead
	if len(transactionSplitUpdates) == 0 {
		log.Println(">>>> No fix applied")
		transaction, err := f.getTransaction(t.Id)
		if err == nil {
			resultTransaction = transaction
		} else {
			return err
		}
	} else {
		// update transaction
		updateObj := structs.TransactionUpdate{
			ApplyRules:         true,
			FireWebhooks:       false,
			GroupTitle:         transactionSplitUpdates[0].Description,
			TransactionUpdates: transactionSplitUpdates,
		}
		updateResponse, err := f.UpdateTransaction(t.Id, &updateObj)
		if err != nil {
			return err
		}
		resultTransaction = updateResponse
	}
	categories, err := f.getCategories()
	if err != nil {
		categories = []structs.CategoryRead{}
		log.Println("WARNING: could not retrieve category names:", err)
	}
	log.Println(">> Sending notification...")
	err = f.notifManager.NotifyNewTransaction(resultTransaction, f.fireflyBaseUrl, categories)
	if err == nil {
		log.Println(">> Success.")
	} else {
		return err
	}
	return nil
}

func (f *fireflyApi) getTransaction(id int) (*structs.TransactionRead, error) {
	endpoint := fmt.Sprintf("%s/%d", f.endpoints.transactions, id)
	resp, err := f.request("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}
	var respTransaction struct {
		Data structs.TransactionRead `json:"data"`
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&respTransaction); err != nil {
		return nil, err
	}
	return &respTransaction.Data, nil
}

func (f *fireflyApi) getCategories() ([]structs.CategoryRead, error) {
	resp, err := f.request("GET", f.endpoints.categories, nil)
	if err != nil {
		return nil, err
	}
	var categoriesResp struct {
		Data []structs.CategoryRead `json:"data"`
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(&categoriesResp); err != nil {
		return nil, err
	}
	return categoriesResp.Data, nil
}

func (f *fireflyApi) UpdateTransaction(id int, tu *structs.TransactionUpdate) (*structs.TransactionRead, error) {
	endpoint := fmt.Sprintf("%s/%d", f.endpoints.transactions, id)

	updateObjBytes, err := json.Marshal(tu)
	if err != nil {
		return nil, err
	}
	log.Println(">> Communicating with Firefly-III...")
	resp, err := f.request("PUT", endpoint, bytes.NewBuffer(updateObjBytes))
	if err != nil {
		return nil, err
	} else if resp.StatusCode != http.StatusOK {
		return nil, errors.New(parseResponseError(resp))
	}

	var updateResponse struct {
		Data structs.TransactionRead `json:"data"`
	}
	//goland:noinspection GoUnhandledErrorResult
	defer resp.Body.Close()
	respBytes, _ := io.ReadAll(resp.Body)
	if err := json.Unmarshal(respBytes, &updateResponse); err != nil {
		return nil, fmt.Errorf("transactions update #%d: %s", id, err)
	}
	return &updateResponse.Data, nil
}

func (f *fireflyApi) FireflyBaseUrl() string {
	return f.fireflyBaseUrl
}

func (f *fireflyApi) SetTransactionCategory(id int, categoryName string) (*structs.TransactionRead, error) {
	transaction, err := f.getTransaction(id)
	if err != nil {
		return nil, err
	}
	transactionUpdates := make([]structs.TransactionSplitUpdate, len(transaction.Attributes.Transactions))
	for i, transactionSplit := range transaction.Attributes.Transactions {
		journalId, err := strconv.ParseInt(transactionSplit.JournalId, 10, 64)
		if err != nil {
			return nil, err
		}
		transactionUpdates[i] = structs.TransactionSplitUpdate{
			JournalId:    int(journalId),
			CategoryName: categoryName,
		}
	}
	updateObj := &structs.TransactionUpdate{
		ApplyRules:         true,
		FireWebhooks:       false,
		GroupTitle:         transaction.Attributes.GroupTitle,
		TransactionUpdates: transactionUpdates,
	}
	return f.UpdateTransaction(id, updateObj)
}

type responseError struct {
	Message string `json:"message"`
}

func parseResponseError(r *http.Response) string {
	var responseError responseError

	defer r.Body.Close()
	respBytes, _ := io.ReadAll(r.Body)
	if err := json.Unmarshal(respBytes, &responseError); err != nil {
		return fmt.Sprintf("%s ('%s')", err, string(respBytes))
	}
	if responseError.Message != "" {
		return responseError.Message
	} else {
		return fmt.Sprintf("Unknown error ('%s')", string(respBytes))
	}
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
