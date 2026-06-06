package integrations

import (
	"fmt"
	"paniniclaw/utils"
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/google/uuid"
)

type Telegram struct {
	bot        *tgbotapi.BotAPI
	db         *utils.Database
	userStore  *utils.UserStore
	openRouter *OpenRouter
}

var setupKey string

func NewTelegram(
	token string,
	db *utils.Database,
	userStore *utils.UserStore,
	openRouter *OpenRouter,
) (*Telegram, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	if exists, _ := userStore.OwnerExists(); !exists {
		setupKey = uuid.NewString()
		fmt.Println("Enter this key into the Telegram bot to register:")
		fmt.Println(setupKey)
	}

	return &Telegram{
		bot:        bot,
		db:         db,
		userStore:  userStore,
		openRouter: openRouter,
	}, nil
}

func (t *Telegram) Listen() error {
	updateConfig := tgbotapi.NewUpdate(0)
	updateConfig.Timeout = 60

	updates := t.bot.GetUpdatesChan(updateConfig)

	for update := range updates {
		if update.Message == nil {
			continue
		}

		user, err := t.userStore.GetTelegramUser(update.Message.From.ID)
		if err != nil {
			fmt.Println(err)
			continue
		}

		if user == nil {
			if setupKey != "" && update.Message.Text == setupKey {
				t.userStore.CreateOwner(
					update.Message.From.FirstName,
					update.Message.From.ID,
				)
				setupKey = ""
				t.bot.Send(tgbotapi.NewMessage(
					update.Message.Chat.ID,
					"You've been registered as the owner. You can now chat normally.",
				))
			}
			continue
		}

		response, err := WithTyping(
			t.bot,
			update.Message.Chat.ID,
			func() (string, error) {
				return t.openRouter.Chat(update.Message.Text)
			},
		)

		if err != nil {
			response = "Error: " + err.Error()
		}

		msg := tgbotapi.NewMessage(
			update.Message.Chat.ID,
			response,
		)

		t.bot.Send(msg)
	}

	return nil
}

// TODO: figure out why typing indicators aren't working
func WithTyping[T any](
	bot *tgbotapi.BotAPI,
	chatID int64,
	fn func() (T, error),
) (T, error) {

	_, _ = bot.Request(
		tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping),
	)

	done := make(chan struct{})
	defer close(done)

	go func() {
		ticker := time.NewTicker(4 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				_, _ = bot.Request(
					tgbotapi.NewChatAction(chatID, tgbotapi.ChatTyping),
				)
			}
		}
	}()

	return fn()
}
