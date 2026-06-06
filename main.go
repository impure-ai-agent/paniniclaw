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

	db, err := utils.CreateDb("data/paniniclaw.db")
	if err != nil {
		log.Fatal(err)
	}

	_, traceError := utils.CreateUserStore("data/users.json")
	if traceError != nil {
		log.Fatal(traceError.Err)
	}

	openRouter := integrations.NewOpenRouter(
		secrets.OpenRouterAPIKey,
	)

	telegram, err := integrations.NewTelegram(
		secrets.TelegramBotToken,
		db,
		openRouter,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(telegram.Listen())
}
