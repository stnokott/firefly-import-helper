package main

import (
	"firefly-iii-fix-ing/internal/util"
	"firefly-iii-fix-ing/internal/worker"
	"log"
	"os"
	"strconv"

	"golang.org/x/exp/slices"
)

const envBaseUrl = "FIREFLY_HTTPS_URL"
const envAccessToken = "FIREFLY_ACCESS_TOKEN"
const envAutoimporterUrl = "AUTOIMPORTER_URL"
const envAutoimporterPort = "AUTOIMPORTER_PORT"
const envAutoimporterSecret = "AUTOIMPORTER_SECRET"
const envAutoimporterSchedule = "AUTOIMPORTER_CRON_SCHEDULE"
const envTelegramToken = "TELEGRAM_ACCESS_TOKEN"
const envTelegramChatId = "TELEGRAM_CHAT_ID"
const envHealthchecksUrl = "HEALTHCHECKS_URL"

func main() {
	envMap := map[string]string{
		envBaseUrl:              "",
		envAccessToken:          "",
		envTelegramToken:        "",
		envTelegramChatId:       "",
		envAutoimporterUrl:      "",
		envAutoimporterPort:     "",
		envAutoimporterSecret:   "",
		envAutoimporterSchedule: "",
		envHealthchecksUrl:      "",
	}
	envOptionals := []string{
		envHealthchecksUrl,
	}

	for envKey := range envMap {
		envValue := os.Getenv(envKey)
		if envValue == "" && !slices.Contains(envOptionals, envHealthchecksUrl) {
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
		Url:             envMap[envAutoimporterUrl],
		Port:            uint(autoImporterPortInt),
		Secret:          envMap[envAutoimporterSecret],
		CronSchedule:    envMap[envAutoimporterSchedule],
		HealthchecksUrl: envMap[envHealthchecksUrl],
	}

	chatIdInt, err := strconv.ParseInt(envMap[envTelegramChatId], 10, 64)
	if err != nil {
		log.Fatalf("could not parse %s = %s as int", envTelegramChatId, envMap[envTelegramChatId])
	}
	telegramOptions := worker.TelegramOptions{
		AccessToken: envMap[envTelegramToken],
		ChatId:      chatIdInt,
	}
	log.Println("Running", util.Version)
	log.Println("#########################")
	log.Println("###       SETUP       ###")
	log.Println("#########################")
	log.Println()
	w, err := worker.NewWorker(envMap[envAccessToken], envMap[envBaseUrl], &autoImportOptions, &telegramOptions)
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
