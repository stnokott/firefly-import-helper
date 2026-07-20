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
	verbose := flag.Bool("verbose", false, "more logging")
	doInit := flag.Bool("init", false, "when set, will bootstrap a config file and then exit")
	doDryRun := flag.Bool("dry-run", false, "when set, will not modify any Firefly data")
	noNotify := flag.Bool("no-notify", false, "when set, will disable any outward messenger communication")
	flag.Parse()

	env, err := config.ReadEnv()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}

	logLevel := log.Info
	if *verbose || strings.ToLower(os.Getenv("DEBUG")) == "true" {
		logLevel = log.Debug
	}
	log.SetDefaultLevel(logLevel)

	if *doInit {
		err = bootstrapConfig(env)
	} else {
		err = run(env, *doDryRun, *noNotify)
	}

	if err != nil {
		fmt.Println(err)
		os.Exit(1)
		return
	}
}

func bootstrapConfig(env *config.Env) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	bank := lunchflow.NewClient(env.LunchflowAPIKey)
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

func run(env *config.Env, dryRun bool, disableNotifications bool) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, os.Kill)
	defer cancel()

	cfg, err := config.ReadYAML(configFile)
	if err != nil {
		return err
	}

	ff, err := firefly.New(env.FireflyBaseURL.URL, env.FireflyAccessToken)
	if err != nil {
		return fmt.Errorf("could not create Firefly API client: %w", err)
	}

	if dryRun {
		ff = struct {
			domain.FireflyWriter
			domain.FireflyReader
		}{
			FireflyWriter: firefly.NewNoopWriter(),
			FireflyReader: ff,
		}
	}

	messenger := telegram.NewNoop()
	if !disableNotifications && !dryRun {
		if messenger, err = telegram.NewBot(
			env.TelegramBotToken, env.FireflyBaseURL.URL, env.TelegramChatID, ff,
		); err != nil {
			return err
		}
	}

	bank := lunchflow.NewClient(env.LunchflowAPIKey)

	im, err := importer.New(cfg, messenger, bank, ff)
	if err != nil {
		return err
	}

	eg, ctxEg := errgroup.WithContext(ctx)
	eg.Go(func() error {
		messenger.Listen(ctxEg)
		return nil
	})
	eg.Go(func() error {
		ctxImporter, cancelImporter := context.WithTimeoutCause(
			ctxEg,
			importTimeout,
			errors.New("timeout exceeded"),
		)
		defer cancelImporter()
		return im.Import(ctxImporter, dryRun)
	})

	// wait for potential shutdown actions in goroutines like sending goodbye messages
	defer time.Sleep(1 * time.Second)
	return eg.Wait()
}
