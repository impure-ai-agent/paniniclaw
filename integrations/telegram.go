package integrations

import (
	"time"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Telegram struct {
	bot        *tgbotapi.BotAPI
	openRouter *OpenRouter
}

func NewTelegram(
	token string,
	openRouter *OpenRouter,
) (*Telegram, error) {
	bot, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	return &Telegram{
		bot:        bot,
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
