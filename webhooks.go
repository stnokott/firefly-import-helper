package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"strconv"
)

type webhookRead struct {
	Id         string            `json:"id"`
	Attributes webhookAttributes `json:"attributes"`
}

type webhookAttributes struct {
	Active   bool   `json:"active"`
	Title    string `json:"title"`
	Response string `json:"response"`
	Delivery string `json:"delivery"`
	Secret   string `json:"secret"`
	Trigger  string `json:"trigger"`
	Url      string `json:"url"`
}

type whTransactionRead struct {
	Id           int                  `json:"id"`
	GroupTitle   string               `json:"group_title"`
	Transactions []whTransactionSplit `json:"transactions"`
	Links        []struct {
		Rel string `json:"rel"`
		Uri string `json:"uri"`
	} `json:"links"`
}

type whTransactionSplit struct {
	JournalId       int    `json:"transaction_journal_id"`
	Date            string `json:"date"`
	Amount          string `json:"amount"`
	CurrencySymbol  string `json:"currency_symbol"`
	Description     string `json:"description"`
	SourceName      string `json:"source_name"`
	DestinationName string `json:"destination_name"`
}

type whUrlResult struct {
	exists      bool
	needsUpdate bool
	wh          *webhookRead
}

func (w *Worker) createOrUpdateWebhook() (string, error) {
	result, err := w.getWebhook()
	if err != nil {
		return "", err
	}
	if result.exists && !result.needsUpdate {
		return result.wh.Attributes.Url, nil
	} else {
		var method string
		var endpoint string
		if !result.exists {
			method = "POST"
			endpoint = w.endpointWebhooks
		} else {
			method = "PUT"
			endpoint = w.endpointWebhooks + "/" + result.wh.Id
		}
		body, err := json.Marshal(w.targetWebhook)
		if err != nil {
			return "", err
		}
		resp, err := w.request(method, endpoint, nil, bytes.NewBuffer(body))
		if err != nil {
			return "", err
		}
		var target webhookAttributes
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

func (w *Worker) getWebhook() (*whUrlResult, error) {
	wh, err := w.findWebhookByTitle()
	if err != nil {
		return nil, err
	}
	if wh == nil {
		return &whUrlResult{
			exists:      false,
			needsUpdate: false,
		}, nil
	} else {
		if !wh.Attributes.Active ||
			wh.Attributes.Delivery != w.targetWebhook.Delivery ||
			wh.Attributes.Response != w.targetWebhook.Response ||
			wh.Attributes.Trigger != w.targetWebhook.Trigger ||
			wh.Attributes.Url != w.targetWebhook.Url {
			return &whUrlResult{
				exists:      true,
				needsUpdate: true,
				wh:          wh,
			}, nil
		} else {
			return &whUrlResult{
				exists:      true,
				needsUpdate: false,
				wh:          wh,
			}, nil
		}
	}
}

func (w *Worker) findWebhookByTitle() (*webhookRead, error) {
	resp, err := w.request("GET", w.endpointWebhooks, nil, nil)
	if err != nil {
		return nil, err
	}
	var webhooksResponse struct {
		Webhooks []webhookRead `json:"data"`
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

type transactionUpdate struct {
	ApplyRules         bool                     `json:"apply_rules"`
	FireWebhooks       bool                     `json:"fire_webhooks"`
	GroupTitle         string                   `json:"group_title"`
	TransactionUpdates []transactionSplitUpdate `json:"transactions"`
}

type transactionSplitUpdate struct {
	JournalId        int    `json:"transaction_journal_id"`
	Description      string `json:"description"`
	MandateReference string `json:"sepa_db"`
	CreditorId       string `json:"destination_iban"`
}

type transactionSingle struct {
	Data transactionRead `json:"data"`
}

type transactionRead struct {
	Id         string `json:"id"`
	Attributes struct {
		GroupTitle   string `json:"group_title"`
		Transactions []struct {
			Amount          string `json:"amount"`
			CurrencySymbol  string `json:"currency_symbol"`
			Description     string `json:"description"`
			DestinationName string `json:"destination_name"`
			SourceName      string `json:"source_name"`
			Date            string `json:"date"`
		} `json:"transactions"`
	} `json:"attributes"`
}

func (w *Worker) checkAndUpdateTransaction(t whTransactionRead) error {
	var transactionSplitUpdates []transactionSplitUpdate
	for _, transactionInner := range t.Transactions {
		log.Println(">> ID: #" + strconv.Itoa(t.Id))
		log.Println(">> Description: '" + transactionInner.Description + "'")
		update, err := w.moduleHandler.processIncremental(&transactionInner)
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
		updateObj := transactionUpdate{
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

		var updateResponse transactionSingle
		//goland:noinspection GoUnhandledErrorResult
		defer resp.Body.Close()
		respBytes, err := ioutil.ReadAll(resp.Body)
		if err := json.Unmarshal(respBytes, &updateResponse); err != nil {
			return errors.New(fmt.Sprintf("transaction update #%d: %s", t.Id, err))
		}
		return w.notifyFromApiResponse(&updateResponse.Data)
	}
}
