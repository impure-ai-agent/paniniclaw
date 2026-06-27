package main

import (
	"fmt"
	"log"
	"paniniclaw/integrations"
	"paniniclaw/utils"
	"time"
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

	userStore, traceError := utils.CreateUserStore("data/users.json")
	if traceError != nil {
		log.Fatal(traceError.Err)
	}

	openRouter := integrations.NewOpenRouter(
		secrets.OpenRouterAPIKey,
	)

	telegram, err := integrations.NewTelegram(
		secrets.TelegramBotToken,
		db,
		userStore,
		openRouter,
	)
	if err != nil {
		log.Fatal(err)
	}

	// Get owner's Telegram chat ID for scheduler notifications
	owner := userStore.GetOwner()
	var ownerChatId string
	if owner != nil {
		for _, conn := range owner.Connections {
			if conn.Provider == "telegram" {
				switch v := conn.Data["chat_id"].(type) {
				case string:
					ownerChatId = v
				case float64:
					ownerChatId = fmt.Sprintf("%.0f", v)
				}
				break
			}
		}
	}

	// Start scheduler
	if ownerChatId != "" {
		scheduler := utils.NewScheduler(
			"tasks",
			openRouter,
			telegram.SendMessage,
			ownerChatId,
		)
		scheduler.Start()
		telegram.SetScheduler(scheduler)
	} else {
		log.Println("[scheduler] No owner chat ID found, scheduler disabled")
	}

	// Start memory manager — runs every hour
	memory := utils.NewMemoryManager(db, userStore, openRouter)
	go func() {
		// Run once at startup, then every hour
		memory.Run()
		ticker := time.NewTicker(1 * time.Hour)
		defer ticker.Stop()
		for range ticker.C {
			memory.Run()
		}
	}()

	log.Fatal(telegram.Listen())
}
