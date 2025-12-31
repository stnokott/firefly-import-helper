package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/importer"
	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/server"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
	"golang.org/x/sync/errgroup"
)

const (
	telegramBotToken      = "5076697375:AAHnS4OeS7UobqT5eY88lOH9Tef13pXBrVs"
	telegramChatID        = "725149271"
	fireflyBaseURL        = "http://firefly.local" // TODO: parse and normalize URL first to account for possible trailing slash
	fireflyAccessToken    = `` // TODO: replace me
	fireflyImporterURL    = "http://192.168.178.31:8081"
	fireflyImporterSecret = `W2reJoKxD8CqUB522n25`
)

// var fireflyImporterConfigs = []string{"lunchflow.json"}

func main() {
	logLevel := log.Info
	if strings.ToLower(os.Getenv("DEBUG")) == "true" {
		logLevel = log.Debug
	}
	log.SetDefaultLevel(logLevel)

	if err := run(); err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
}

func run() error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	transactionChan := make(chan *domain.Transaction, 1)
	eg, ctxEg := errgroup.WithContext(ctx)

	// create and start telegram bot
	bot, err := telegram.NewBot(telegramBotToken, telegramChatID, fireflyBaseURL)
	if err != nil {
		return err
	}
	eg.Go(func() error {
		bot.Run(ctxEg, transactionChan)
		return nil
	})

	// create Firefly API client
	c, err := client.NewClientWithResponses(
		fireflyBaseURL+"/api",
		client.WithAccessToken(fireflyAccessToken),
		client.WithRequestLogger(log.For("api-client")),
		client.WithUserAgent("firefly-import-helper"), // TODO: add version from goreleaser
	)
	if err != nil {
		return fmt.Errorf("could not create API client: %w", err)
	}
	// prepare webhook server
	if err = server.Setup(ctx, c, bot); err != nil {
		return fmt.Errorf("could not setup webhook server: %w", err)
	}

	// start webhook listener server
	eg.Go(func() error {
		return server.Run(ctxEg, transactionChan, bot)
	})

	im := importer.New(bot)
	// run import
	// TODO: run with cron expression
	eg.Go(func() error {
		time.Sleep(2 * time.Second)
		return im.Run(
			ctxEg,
			fireflyImporterURL,
			fireflyImporterSecret,
			fireflyAccessToken,
			"./configs/lunchflow.json",
		)
	})

	return eg.Wait()
}
