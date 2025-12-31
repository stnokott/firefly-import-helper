package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"

	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
)

const (
	fireflyAPIURL      = "http://firefly.local/api"
	fireflyAccessToken = `eyJ0eXAiOiJKV1QiLCJhbGciOiJSUzI1NiJ9.eyJhdWQiOiIxMTEwIiwianRpIjoiYTE1MTI4MGIxY2QxZGE2YzNiZTg1YTY1MzQxMmJjYjU0Yjk0Y2QxNzYwYzA0ZTkxMDlmNjhhMDQ3YWU0YjQxNDVjMDNiYzNmZGZkNGQ5OTUiLCJpYXQiOjE3NjY4NTAxNDQuMzg3MTI5LCJuYmYiOjE3NjY4NTAxNDQuMzg3MTM1LCJleHAiOjE3OTgzODYxNDMuNjg4NjY2LCJzdWIiOiIxIiwic2NvcGVzIjpbXX0.G7B_ZtbcQbIlWj9_0Kkb-J1QeFEwvYtoN2aKIg9_-VCOCAaPKFhrPKHXkLpTnH5ayH05KfK_-xhjUPFWs-RKrxo1leI5xUrgN5Y0H-B_SBNQPRKoRn8WWHHxb4LtiRDZA85fnH79Z_B-5xUEzFImljduErBAdDtCezezOqAwO_pL0wsJ-iaSjN-O6HwYiEgH3-mvzSwD6ABDOEaEIVtjbyZ-uh-TKNsmAFaP3VHXsu9iwJMJVYTvIiQ1YQ6XolYP18b16C8hnC7JVJageIsLjKLOqBaJPc8EumI0dFhMTAUGzcRUh6aKDYzVZzOLKaqgrdlp1s0lb5X6bZIp9WuMa5LKNq5hp57ZNNlkNszdPyd-OSQLudCj0ykEKyfESmM-CN-m153fil29Zz3GLBlMajeTNM16qrCaIExtJGFFX1Ee47T-_YnUiQRls9d2dFljXhoPgXETFgjt7OIxqhhVioWRdw9RadHd9h5mLKoNz_dvJ0QcPHHxOtsdOeSLr2LBA8312KzfWnvulmAQeyTnXZeVsXqPmWiID08COIY6XtoiGZH9bf1BIKtdBwDAy-FhTn656naCYdHLELecdUcf99gHOIzcQHXuLhujzNPkQvdXEF-sZgPYSzOEgc7dnybtXxTEu_tnBEqP2AJjfic53WCOmVZcL6ry2Tph6mYMWnE`
	telegramBotToken   = "5076697375:AAHnS4OeS7UobqT5eY88lOH9Tef13pXBrVs"
	telegramChatID     = "725149271"
)

func main() {
	log.Setup()

	if err := run(); err != nil {
		slog.Error(err.Error())
		return
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	// c, err := client.NewClientWithResponses(
	// 	fireflyAPIURL,
	// 	client.WithAccessToken(accessToken),
	// 	client.WithRequestLogger(),
	// )
	// if err != nil {
	// 	return fmt.Errorf("could not create API client: %w", err)
	// }
	//
	// if err := runner.Run(ctx, c); err != nil {
	// 	return err
	// }

	bot, err := telegram.NewBot(telegramBotToken, telegramChatID)
	if err != nil {
		return err
	}
	if err = bot.Run(ctx); err != nil {
		return err
	}

	return nil
}
