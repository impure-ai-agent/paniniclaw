package utils

import (
	"encoding/json"
	"fmt"
	"os"
)

type Secrets struct {
	OpenRouterAPIKey string `json:"openrouter_api_key"`
	TelegramBotToken string `json:"telegram_bot_token"`
}

func LoadSecrets() (*Secrets, error) {

	secretsPath := "secrets.json"

	data, err := os.ReadFile(secretsPath)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to read secrets file %s: %w",
			secretsPath,
			err,
		)
	}

	var secrets Secrets

	if err := json.Unmarshal(data, &secrets); err != nil {
		return nil, fmt.Errorf(
			"failed to parse secrets file: %w",
			err,
		)
	}

	return &secrets, nil
}
