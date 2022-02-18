package worker

import (
	"bytes"
	"encoding/json"
	"errors"
	"firefly-iii-fix-ing/internal/structs"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"strconv"
)

func (w *Worker) createOrUpdateWebhook() (string, error) {
	result, err := w.getWebhook()
	if err != nil {
		return "", err
	}
	if result.Exists && !result.NeedsUpdate {
		return result.Wh.Attributes.Url, nil
	} else {
		var method string
		var endpoint string
		if !result.Exists {
			method = "POST"
			endpoint = w.endpointWebhooks
		} else {
			method = "PUT"
			endpoint = w.endpointWebhooks + "/" + result.Wh.Id
		}
		body, err := json.Marshal(w.targetWebhook)
		if err != nil {
			return "", err
		}
		resp, err := w.request(method, endpoint, nil, bytes.NewBuffer(body))
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

func (w *Worker) getWebhook() (*structs.WhUrlResult, error) {
	wh, err := w.findWebhookByTitle()
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
			wh.Attributes.Delivery != w.targetWebhook.Delivery ||
			wh.Attributes.Response != w.targetWebhook.Response ||
			wh.Attributes.Trigger != w.targetWebhook.Trigger ||
			wh.Attributes.Url != w.targetWebhook.Url {
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

func (w *Worker) findWebhookByTitle() (*structs.WebhookRead, error) {
	resp, err := w.request("GET", w.endpointWebhooks, nil, nil)
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
		if webhook.Attributes.Title == w.targetWebhook.Title {
			return &webhook, nil
		}
	}
	return nil, nil
}

func (w *Worker) checkAndUpdateTransaction(t structs.WhTransactionRead) error {
	var transactionSplitUpdates []structs.TransactionSplitUpdate
	for _, transactionInner := range t.Transactions {
		log.Println(">> ID: #" + strconv.Itoa(t.Id))
		log.Println(">> Description: '" + transactionInner.Description + "'")
		update, err := w.moduleHandler.Process(&transactionInner)
		if err != nil {
			log.Println("WARNING: error running modules:", err)
		} else if update != nil {
			transactionSplitUpdates = append(transactionSplitUpdates, *update)
		}
	}

	if len(transactionSplitUpdates) == 0 {
		log.Println(">>>> No fix applied")
		return w.notifyFromWebhook(&t)
	} else {
		endpoint := fmt.Sprintf(w.endpointUpdateTransaction, t.Id)
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
		resp, err := w.request("PUT", endpoint, nil, bytes.NewBuffer(updateObjBytes))
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
		return w.notifyFromApiResponse(&updateResponse.Data)
	}
}

func (w *Worker) newNotificationParams(id string, transactions []notificationTransaction) *notificationParams {
	uri := w.fireflyBaseUrl
	if id != "" {
		uri += "/transactions/show/" + id
	} else {
		id = "n/a"
	}
	return &notificationParams{id, uri, transactions}
}

func (w *Worker) notifyFromApiResponse(t *structs.TransactionRead) error {
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
	return w.telegramBot.notifyNewTransaction(w.newNotificationParams(t.Id, transactions))
}

func (w *Worker) notifyFromWebhook(t *structs.WhTransactionRead) error {
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
	return w.telegramBot.notifyNewTransaction(w.newNotificationParams(strconv.Itoa(t.Id), transactions))
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
