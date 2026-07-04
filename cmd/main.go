package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/config"
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/firefly"
	"github.com/stnokott/firefly-import-helper/internal/importer"
	"github.com/stnokott/firefly-import-helper/internal/log"
	"github.com/stnokott/firefly-import-helper/internal/lunchflow"
	"github.com/stnokott/firefly-import-helper/internal/telegram"
	"golang.org/x/sync/errgroup"
)

const (
	configFile    = "config.yaml"
	importTimeout = 10 * time.Minute // TODO: set to import interval once configurable
)

func main() {
	var doInit bool
	flag.BoolVar(&doInit, "init", false, "when set, will bootstrap a config file and then exit")
	flag.Parse()

	// allow empty YAML when performing init since we want to bootstrap it during initialization
	cfg, err := config.Read("config.yaml", doInit)
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	logLevel := log.Info
	if strings.ToLower(os.Getenv("DEBUG")) == "true" {
		logLevel = log.Debug
	}
	log.SetDefaultLevel(logLevel)

	if doInit {
		err = bootstrapConfig(cfg)
	} else {
		err = run(cfg)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
}

func bootstrapConfig(cfg *config.Config) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bank := lunchflow.NewClient(cfg.LunchflowAPIKey)
	accounts, err := bank.GetAccounts(ctx)
	if err != nil {
		return err
	}
	if err := config.WriteBootstrappedConfig(configFile, accounts); err != nil {
		return err
	}
	fmt.Println("config written for", len(accounts), "accounts")
	return nil
}

func run(cfg *config.Config) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	messenger, err := telegram.NewBot(
		cfg.TelegramBotToken, cfg.FireflyBaseURL.URL, cfg.TelegramChatID,
	)
	if err != nil {
		return err
	}

	bank := lunchflow.NewClient(cfg.LunchflowAPIKey)

	ff, err := firefly.New(cfg.FireflyBaseURL.URL, cfg.FireflyAccessToken)
	if err != nil {
		return fmt.Errorf("could not create Firefly API client: %w", err)
	}
	if err := validateWithData(ctx, cfg, ff); err != nil {
		return fmt.Errorf("config validation failed: %w", err)
	}

	im := importer.New(cfg, messenger, bank, ff)

	eg, ctxEg := errgroup.WithContext(ctx)
	eg.Go(func() error {
		messenger.Run(ctxEg)
		return nil
	})
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

func validateWithData(ctx context.Context, cfg *config.Config, ff domain.FireflyConnector) error {
	ffAccounts, err := ff.ListAccounts(ctx)
	if err != nil {
		return fmt.Errorf("could not list Firefly accounts: %w", err)
	}
	return cfg.ValidateFireflyIDs(ffAccounts)
}
