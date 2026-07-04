package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/importer"
	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/lunchflow"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
	"golang.org/x/sync/errgroup"
)

const (
	importTimeout = 10 * time.Minute // TODO: set to import interval once configurable
)

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

	eg, ctxEg := errgroup.WithContext(ctx)

	// create and start telegram messenger
	messenger, err := telegram.NewBot()
	if err != nil {
		return err
	}
	eg.Go(func() error {
		messenger.Run(ctxEg)
		return nil
	})

	// create Firefly API client
	// api, err := firefly.NewClientWithResponses(
	// 	config.C.FireflyBaseURL.JoinPath("/api").String(),
	// 	firefly.WithAccessToken(config.C.FireflyAccessToken),
	// 	firefly.WithRequestLogger(log.For("firefly")),
	// 	firefly.WithUserAgent("firefly-import-helper"), // TODO: add version from goreleaser
	// )
	// if err != nil {
	// 	return fmt.Errorf("could not create API client: %w", err)
	// }

	bank := lunchflow.NewClient(config.C.LunchflowAPIKey)

	im := importer.New(messenger, bank)
	eg.Go(func() error {
		ctxImporter, cancelImporter := context.WithTimeoutCause(
			ctxEg,
			importTimeout,
			errors.New("timeout exceeded"),
		)
		defer cancelImporter()
		return im.Import(ctxImporter)
	})

	defer func() {
		// wait for potential shutdown actions in goroutines like sending goodbyte messages
		time.Sleep(3 * time.Second)
	}()
	return eg.Wait()
}
