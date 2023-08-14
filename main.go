// Package main parses environment variables and runs the main process
package main

import (
	"firefly-iii-fix-ing/internal/worker"
	"log"
	"os"
	"slices"
	"strconv"
)

var Version = "v0.0.1"

const (
	envBaseURL              = "FIREFLY_HTTPS_URL"
	envAccessToken          = "FIREFLY_ACCESS_TOKEN"
	envAutoimporterURL      = "AUTOIMPORTER_URL"
	envAutoimporterPort     = "AUTOIMPORTER_PORT"
	envAutoimporterSecret   = "AUTOIMPORTER_SECRET"
	envAutoimporterSchedule = "AUTOIMPORTER_CRON_SCHEDULE"
	envTelegramToken        = "TELEGRAM_ACCESS_TOKEN"
	envTelegramChatID       = "TELEGRAM_CHAT_ID"
	envHealthchecksURL      = "HEALTHCHECKS_URL"
)

func main() {
	envMap := map[string]string{
		envBaseURL:              "",
		envAccessToken:          "",
		envTelegramToken:        "",
		envTelegramChatID:       "",
		envAutoimporterURL:      "",
		envAutoimporterPort:     "",
		envAutoimporterSecret:   "",
		envAutoimporterSchedule: "",
		envHealthchecksURL:      "",
	}
	envOptionals := []string{
		envHealthchecksURL,
	}

	for envKey := range envMap {
		envValue := os.Getenv(envKey)
		if envValue == "" && !slices.Contains(envOptionals, envHealthchecksURL) {
			log.Fatalln("required environment variable", envKey, "not set!")
		} else {
			envMap[envKey] = envValue
		}
	}

	autoImporterPortInt, err := strconv.ParseInt(envMap[envAutoimporterPort], 10, 64)
	if err != nil {
		log.Fatalf("could not parse %s = %s as int", envAutoimporterPort, envMap[envAutoimporterPort])
	}
	autoImportOptions := worker.AutoimportOptions{
		URL:             envMap[envAutoimporterURL],
		Port:            uint(autoImporterPortInt),
		Secret:          envMap[envAutoimporterSecret],
		CronSchedule:    envMap[envAutoimporterSchedule],
		HealthchecksURL: envMap[envHealthchecksURL],
	}

	chatIdInt, err := strconv.ParseInt(envMap[envTelegramChatID], 10, 64)
	if err != nil {
		log.Fatalf("could not parse %s = %s as int", envTelegramChatID, envMap[envTelegramChatID])
	}
	telegramOptions := worker.TelegramOptions{
		AccessToken: envMap[envTelegramToken],
		ChatID:      chatIdInt,
	}
	log.Println("Running", Version)
	log.Println("#########################")
	log.Println("###       SETUP       ###")
	log.Println("#########################")
	log.Println()
	w, err := worker.NewWorker(envMap[envAccessToken], envMap[envBaseURL], &autoImportOptions, &telegramOptions)
	if err != nil {
		log.Fatalln(err)
	}
	log.Println()
	log.Println("#########################")
	log.Println("###       START       ###")
	log.Println("#########################")
	log.Println()
	if err := w.Listen(); err != nil {
		log.Fatalln(err)
	}
}
