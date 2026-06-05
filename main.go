package main

import (
	"PanoptiClaw/integrations"
	"PanoptiClaw/utils"
	"log"
)

func main() {
	secrets, err := utils.LoadSecrets()
	if err != nil {
		log.Fatal(err)
	}

	telegram, err := integrations.NewTelegram(
		secrets.TelegramBotToken,
	)
	if err != nil {
		log.Fatal(err)
	}

	log.Fatal(telegram.Listen())
}
