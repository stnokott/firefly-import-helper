package main

import (
	"firefly-iii-fix-ing/internal/worker"
	"log"
	"os"
	"strconv"
)

const envBaseUrl = "FIREFLY_HTTPS_URL"
const envAccessToken = "FIREFLY_ACCESS_TOKEN"
const envTelegramToken = "TELEGRAM_ACCESS_TOKEN"
const envTelegramChatId = "TELEGRAM_CHAT_ID"

func main() {
	fireflyBaseUrl := os.Getenv(envBaseUrl)
	fireflyAccessToken := os.Getenv(envAccessToken)
	telegramAccessToken := os.Getenv(envTelegramToken)
	telegramChatId := os.Getenv(envTelegramChatId)

	if fireflyBaseUrl == "" {
		log.Fatalln("environment variable ", envBaseUrl, "not set!")
	}
	if fireflyAccessToken == "" {
		log.Fatalln("environment variable ", envAccessToken, "not set!")
	}
	if telegramAccessToken == "" {
		log.Fatalln("environment variable ", envTelegramToken, "not set!")
	}
	if telegramChatId == "" {
		log.Fatalln("environment variable ", envTelegramChatId, "not set!")
	}
	chatIdInt, err := strconv.ParseInt(telegramChatId, 10, 64)
	if err != nil {
		log.Fatalln(err)
	}

	telegramOptions := worker.TelegramOptions{
		AccessToken: telegramAccessToken,
		ChatId:      chatIdInt,
	}
	log.Println("#########################")
	log.Println("###       SETUP       ###")
	log.Println("#########################")
	log.Println()
	w, err := worker.NewWorker(fireflyAccessToken, fireflyBaseUrl, &telegramOptions)
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
