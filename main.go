package main

import (
	"log"
	"paniniclaw/integrations"
	"paniniclaw/utils"
)

func main() {
	secrets, err := utils.LoadSecrets()
	if err != nil {
		log.Fatal(err)
	}

	openRouter := integrations.NewOpenRouter(
		secrets.OpenRouterAPIKey,
	)

	telegram, err := integrations.NewTelegram(
		secrets.TelegramBotToken,
		openRouter,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(telegram.Listen())
}
