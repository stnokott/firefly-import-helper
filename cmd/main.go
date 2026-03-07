package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/client"
	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/importer"
	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/server"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
	"golang.org/x/sync/errgroup"
)

// var fireflyImporterConfigs = []string{"lunchflow.json"}

func main() {
	if err := config.Read(".env"); err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

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
	bot, err := telegram.NewBot()
	if err != nil {
		return err
	}
	eg.Go(func() error {
		bot.Run(ctxEg, transactionChan)
		return nil
	})

	// create Firefly API client
	api, err := client.NewClientWithResponses(
		config.C.FireflyBaseURL.JoinPath("/api").String(),
		client.WithAccessToken(config.C.FireflyAccessToken),
		client.WithRequestLogger(log.For("api-client")),
		client.WithUserAgent("firefly-import-helper"), // TODO: add version from goreleaser
	)
	if err != nil {
		return fmt.Errorf("could not create API client: %w", err)
	}
	srv, err := server.NewServer(transactionChan, api)
	if err != nil {
		return fmt.Errorf("could not create server: %w", err)
	}

	if err := srv.Setup(ctx); err != nil {
		return fmt.Errorf("could not set up server: %w", err)
	}
	// start webhook listener server
	eg.Go(func() error {
		return srv.Run(ctxEg)
	})

	im := importer.New(bot)
	// run import
	// TODO: run with cron expression
	eg.Go(func() error {
		time.Sleep(2 * time.Second)
		return im.Run(
			ctxEg,
			"./configs/lunchflow.json",
		)
	})

	return eg.Wait()
}
