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

const (
	port            = 8822
	webhookPath     = "/wh_fix_ing"
	pathAccounts    = "/api/v1/accounts"
	pathTransaction = "/api/v1/transactions"
	pathWebhooks    = "/api/v1/webhooks"
	pathCategories  = "/api/v1/categories"
)

type endpoints struct {
	account      string
	transactions string
	webhooks     string
	categories   string
}

type fireflyAPI struct {
	srv                *http.Server
	webhookURL         string
	fireflyBaseURL     string
	endpoints          endpoints
	fireflyAccessToken string
	targetWebhook      structs.WebhookAttributes
	moduleHandler      *modules.ModuleHandler
	notifManager       transactioNotifier
}

type transactioNotifier interface {
	NotifyNewTransaction(t *structs.TransactionRead, fireflyBaseURL string, categories []structs.CategoryRead) error
}

func newFireflyAPI(fireflyBaseURL string, fireflyAccessToken string, moduleHandler *modules.ModuleHandler, notifManager transactioNotifier) *fireflyAPI {
	f := fireflyAPI{
		webhookURL:     fireflyBaseURL + webhookPath,
		fireflyBaseURL: fireflyBaseURL,
		endpoints: endpoints{
			account:      fireflyBaseURL + pathAccounts,
			transactions: fireflyBaseURL + pathTransaction,
			webhooks:     fireflyBaseURL + pathWebhooks,
			categories:   fireflyBaseURL + pathCategories,
		},
		fireflyAccessToken: fireflyAccessToken,
		targetWebhook: structs.WebhookAttributes{
			Active:   true,
			Title:    "Fix ING transaction descriptions from Importer",
			Response: "TRANSACTIONS",
			Delivery: "JSON",
			Trigger:  "STORE_TRANSACTION",
			Url:      fireflyBaseURL + webhookPath,
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

func (f *fireflyAPI) Listen() error {
	if f.srv == nil {
		return errors.New("please set callbacks before calling fireflyApi::Listen")
	}

	go func() {
		time.Sleep(1 * time.Second) // give a little time for httpServer to be up and running
		client := http.Client{Timeout: 10 * time.Second}
		resp, err := client.Get(f.webhookURL)
		//goland:noinspection GoUnhandledErrorResult
		if err != nil {
			log.Fatalln("Error validating httpServer connection:", err)
		} else if resp.StatusCode != 200 {
			log.Fatalln("Could not validate connection from webhook URL ", f.webhookURL)
		}
		log.Println("Connection to", f.webhookURL, "validated")
		log.Println("Ready to accept connections!")
	}()

	// start webserver
	log.Printf("Starting httpServer on port %d...", port)
	return f.srv.ListenAndServe()
}

func (f *fireflyAPI) handleNewTransactionWebhook(_ http.ResponseWriter, r *http.Request) {
	var target struct {
		Version string                    `json:"version"`
		Data    structs.WhTransactionRead `json:"content"`
	}
	defer func() {
		if errClose := r.Body.Close(); errClose != nil {
			log.Printf("WARNING: error closing request body: %v", errClose)
		}
	}()
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

func (f *fireflyAPI) createOrUpdateWebhook() (url string, err error) {
	var result *structs.WhUrlResult
	result, err = f.getWebhook()
	if err != nil {
		return
	}
	if result.Exists && !result.NeedsUpdate {
		url = result.Wh.Attributes.Url
		return
	}

	var method string
	var endpoint string
	if !result.Exists {
		// create
		log.Printf("webhook with title '%s' does not exist, creating a new one", f.targetWebhook.Title)
		method = "POST"
		endpoint = f.endpoints.webhooks
	} else {
		// update
		log.Printf("webhook with title '%s' exists, but requires update", f.targetWebhook.Title)
		method = "PUT"
		endpoint = f.endpoints.webhooks + "/" + result.Wh.Id
	}
	var body []byte
	body, err = json.Marshal(f.targetWebhook)
	if err != nil {
		return
	}
	var resp *http.Response
	resp, err = f.request(method, endpoint, bytes.NewBuffer(body))
	if err != nil {
		return
	}
	var resultWebhook struct {
		Data    structs.WebhookRead `json:"data"`
		Message string              `json:"message"`
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	respBytes, _ := io.ReadAll(resp.Body)
	if err = json.Unmarshal(respBytes, &resultWebhook); err != nil {
		return
	}
	if resp.StatusCode != http.StatusOK && resultWebhook.Message != "" {
		err = errors.New(resultWebhook.Message)
		return
	}
	if resultWebhook.Data.Attributes.Title == "" {
		err = errors.New("unknown error")
		return
	}
	url = resultWebhook.Data.Attributes.Url
	return
}

func (f *fireflyAPI) getWebhook() (*structs.WhUrlResult, error) {
	wh, err := f.findWebhookByTitle()
	if err != nil {
		return nil, err
	}
	if wh == nil {
		return &structs.WhUrlResult{
			Exists:      false,
			NeedsUpdate: false,
		}, nil
	}

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
	}

	return &structs.WhUrlResult{
		Exists:      true,
		NeedsUpdate: false,
		Wh:          wh,
	}, nil
}

func (f *fireflyAPI) findWebhookByTitle() (wh *structs.WebhookRead, err error) {
	var resp *http.Response
	resp, err = f.request("GET", f.endpoints.webhooks, nil)
	if err != nil {
		return
	}
	if resp.StatusCode == http.StatusNotFound {
		return
	} else if resp.StatusCode != http.StatusOK {
		err = fmt.Errorf("got invalid status code %d", resp.StatusCode)
		return
	}
	var webhooksResponse struct {
		Webhooks []structs.WebhookRead `json:"data"`
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	if err = json.NewDecoder(resp.Body).Decode(&webhooksResponse); err != nil {
		return
	}

	for _, webhook := range webhooksResponse.Webhooks {
		if webhook.Attributes.Title == f.targetWebhook.Title {
			wh = &webhook
			return
		}
	}
	return
}

func (f *fireflyAPI) checkAndUpdateTransaction(t structs.WhTransactionRead) error {
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
	err = f.notifManager.NotifyNewTransaction(resultTransaction, f.fireflyBaseURL, categories)
	if err == nil {
		log.Println(">> Success.")
	} else {
		return err
	}
	return nil
}

func (f *fireflyAPI) getTransaction(id int) (data *structs.TransactionRead, err error) {
	endpoint := fmt.Sprintf("%s/%d", f.endpoints.transactions, id)
	var resp *http.Response
	resp, err = f.request("GET", endpoint, nil)
	if err != nil {
		return
	}
	var respTransaction struct {
		Data structs.TransactionRead `json:"data"`
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	if err = json.NewDecoder(resp.Body).Decode(&respTransaction); err != nil {
		return
	}
	data = &respTransaction.Data
	return
}

func (f *fireflyAPI) getCategories() (data []structs.CategoryRead, err error) {
	var resp *http.Response
	resp, err = f.request("GET", f.endpoints.categories, nil)
	if err != nil {
		return
	}
	var categoriesResp struct {
		Data []structs.CategoryRead `json:"data"`
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	if err = json.NewDecoder(resp.Body).Decode(&categoriesResp); err != nil {
		return
	}
	data = categoriesResp.Data
	return
}

func (f *fireflyAPI) UpdateTransaction(id int, tu *structs.TransactionUpdate) (data *structs.TransactionRead, err error) {
	endpoint := fmt.Sprintf("%s/%d", f.endpoints.transactions, id)

	var updateObjBytes []byte
	updateObjBytes, err = json.Marshal(tu)
	if err != nil {
		return
	}
	log.Println(">> Communicating with Firefly-III...")
	var resp *http.Response
	resp, err = f.request("PUT", endpoint, bytes.NewBuffer(updateObjBytes))
	if err != nil {
		return
	} else if resp.StatusCode != http.StatusOK {
		err = errors.New(parseResponseError(resp))
		return
	}

	var updateResponse struct {
		Data structs.TransactionRead `json:"data"`
	}
	defer func() {
		err = errors.Join(err, resp.Body.Close())
	}()
	respBytes, _ := io.ReadAll(resp.Body)
	if err = json.Unmarshal(respBytes, &updateResponse); err != nil {
		err = fmt.Errorf("transactions update #%d: %w", id, err)
		return
	}
	data = &updateResponse.Data
	return
}

func (f *fireflyAPI) FireflyBaseUrl() string {
	return f.fireflyBaseURL
}

func (f *fireflyAPI) SetTransactionCategory(id int, categoryName string) (*structs.TransactionRead, error) {
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

	defer func() {
		if errClose := r.Body.Close(); errClose != nil {
			log.Printf("WARNING: error closing response body: %v", errClose)
		}
	}()
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

func (f *fireflyAPI) request(method string, url string, body io.Reader) (*http.Response, error) {
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
