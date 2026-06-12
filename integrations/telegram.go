package integrations

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
var primaryAccount *Telegram

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

	if primaryAccount == nil {
		primaryAccount = &Telegram{
			bot:        bot,
			db:         db,
			userStore:  userStore,
			openRouter: openRouter,
		}
	}

	return primaryAccount, nil
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
					update.Message.Chat.ID,
				)
				setupKey = ""
				t.bot.Send(tgbotapi.NewMessage(
					update.Message.Chat.ID,
					"You've been registered as the owner. You can now chat normally.",
				))
				println("Set up user")
			} else {
				t.bot.Send(tgbotapi.NewMessage(
					update.Message.Chat.ID,
					"You're not registered. Please enter the setup key.",
				))
				println("Unregistered user")
			}
			continue
		}

		// Build multimodal content if the message contains a photo
		content := buildMessageContent(update.Message)

		msgJson, _ := json.Marshal(map[string]interface{}{
			"role":    "user",
			"content": content,
		})

		t.db.AddMessage(
			"telegram",
			fmt.Sprintf("%d", update.Message.Chat.ID),
			string(msgJson),
		)

		response, err := WithTyping(
			t.bot,
			update.Message.Chat.ID,
			func() (string, error) {

				history, err := t.db.GetRecentMessages(
					"telegram",
					fmt.Sprintf("%d", update.Message.Chat.ID),
				)
				if err != nil {
					return "", err
				}

				response, err := t.openRouter.ChatFromMessages(history, *user, t.db, fmt.Sprintf("%d", update.Message.Chat.ID))

				msgJson, _ := json.Marshal(map[string]interface{}{
					"role":    "assistant",
					"content": response,
				})

				t.db.AddMessage(
					"telegram",
					fmt.Sprintf("%d", update.Message.Chat.ID),
					string(msgJson),
				)
				return response, err
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

// Note: Telegram typing indicator is displayed at the top of the chat, not inline with the message.
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

func sendMessageToPrimaryAccount(message string, user utils.User) error {
	if primaryAccount == nil {
		return fmt.Errorf("primary account not set")
	}

	var chatId int64

	for _, connection := range user.Connections {
		if connection.Provider == "telegram" {
			chatId = utils.ToInt64(connection.Data["chat_id"])
			break
		}
	}

	if chatId == 0 {
		return fmt.Errorf("telegram chat id not found")
	}

	msg := tgbotapi.NewMessage(
		chatId,
		message,
	)

	_, err := primaryAccount.bot.Send(msg)
	return err
}

// Returns either a plain text string or a multimodal content array.
func buildMessageContent(message *tgbotapi.Message) interface{} {
	/* Images are disabled for now because Deepseek does not support them, maybe Deepseek V5 will support them.
	if len(message.Photo) > 0 {
		// Get the largest photo (last in the array)
		largest := message.Photo[len(message.Photo)-1]
		fileID := largest.FileID

		// Download the photo and convert to base64 data URI
		dataURI, err := downloadTelegramFileAsDataURI(primaryAccount.bot, fileID)
		if err != nil {
			fmt.Println("Error downloading photo:", err)
			// Fall back to just the caption/text
			if message.Caption != "" {
				return message.Caption
			}
			return "[Image could not be downloaded]"
		}

		// Build multimodal content array
		content := []map[string]interface{}{}

		text := message.Caption
		if text == "" {
			text = "What's in this image?"
		}

		content = append(content, map[string]interface{}{
			"type": "text",
			"text": text,
		})
		content = append(content, map[string]interface{}{
			"type": "image_url",
			"image_url": map[string]interface{}{
				"url": dataURI,
			},
		})

		return content
	}
	*/

	// Plain text message
	return message.Text
}

// downloadTelegramFileAsDataURI downloads a file from Telegram by fileID
// and returns it as a base64 data URI string.
func downloadTelegramFileAsDataURI(bot *tgbotapi.BotAPI, fileID string) (string, error) {
	// Get the file info from Telegram
	file, err := bot.GetFile(tgbotapi.FileConfig{FileID: fileID})
	if err != nil {
		return "", fmt.Errorf("failed to get file info: %v", err)
	}

	if file.FilePath == "" {
		return "", fmt.Errorf("file path is empty")
	}

	// Construct the download URL
	fileURL := fmt.Sprintf("https://api.telegram.org/file/bot%s/%s", bot.Token, file.FilePath)

	// Download the file
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Get(fileURL)
	if err != nil {
		return "", fmt.Errorf("failed to download file: %v", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read file data: %v", err)
	}

	// Determine MIME type from file path
	mimeType := "image/jpeg" // default
	ext := file.FilePath
	if len(ext) > 4 {
		extSub := ext[len(ext)-4:]
		switch extSub {
		case ".png":
			mimeType = "image/png"
		case ".jpg", "jpeg":
			mimeType = "image/jpeg"
		case ".gif":
			mimeType = "image/gif"
		case "webp":
			mimeType = "image/webp"
		}
	}

	// Encode to base64 and build data URI
	encoded := base64.StdEncoding.EncodeToString(data)
	dataURI := fmt.Sprintf("data:%s;base64,%s", mimeType, encoded)

	return dataURI, nil
}
