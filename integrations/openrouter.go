package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"paniniclaw/utils"
)

type OpenRouter struct {
	apiKey string
	model  string
}

func NewOpenRouter(apiKey string) *OpenRouter {
	return &OpenRouter{
		apiKey: apiKey,
		model:  "qwen/qwen3.5-flash-02-23",
	}
}

type chatRequest struct {
	Model string        `json:"model"`
	Input []chatMessage `json:"input"`
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Output []struct {
		Type    string `json:"type"`
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	} `json:"output"`
}

func (o *OpenRouter) ChatFromPrompt(prompt string, user utils.User) (string, error) {

	systemMessage, err := makeSystemMessage(user)
	if err != nil {
		return "", err
	}

	reqBody := chatRequest{
		Model: o.model,
		Input: []chatMessage{
			systemMessage,
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	return o.rawChat(reqBody)
}

func makeSystemMessage(user utils.User) (chatMessage, error) {
	soulBytes, err := os.ReadFile("directives/core.md")
	if err != nil {
		return chatMessage{}, err
	}

	telegramBytes, err := os.ReadFile("directives/telegram.md")
	if err != nil {
		return chatMessage{}, err
	}

	userJson, err := user.MakeJson()
	if err != nil {
		return chatMessage{}, err
	}

	return chatMessage{
		Role:    "system",
		Content: fmt.Sprintf("directives/core.md: %s\n\ndirectives/telegram.md: %s\n\nuser: %s", soulBytes, telegramBytes, userJson),
	}, nil
}

func (o *OpenRouter) ChatFromMessages(messages []utils.Message, user utils.User) (string, error) {

	systemMessage, err := makeSystemMessage(user)
	if err != nil {
		return "", err
	}

	chatMessages := make([]chatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = chatMessage{
			Role:    msg.Role,
			Content: msg.Content,
		}
	}

	reqBody := chatRequest{
		Model: o.model,
		Input: append([]chatMessage{
			systemMessage,
		}, chatMessages...),
	}

	return o.rawChat(reqBody)
}

func (o *OpenRouter) rawChat(prompt chatRequest) (string, error) {

	body, err := json.Marshal(prompt)
	if err != nil {
		return "", err
	}

	println("Making request: ", string(body))

	req, err := http.NewRequest(
		"POST",
		"https://openrouter.ai/api/v1/responses",
		bytes.NewReader(body),
	)
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRouter-Title", "PaniniClaw")
	req.Header.Set("HTTP-Referer", "https://github.com/impure/paniniclaw")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter returned %d", resp.StatusCode)
	}

	var result chatResponse

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	debugResponse, _ := json.MarshalIndent(result, "", "\t")
	println("Got response:", string(debugResponse))

	var textResponse string
	for _, out := range result.Output {
		if out.Type == "message" {
			for _, content := range out.Content {
				if content.Type == "output_text" {
					textResponse = content.Text
					break
				}
			}
		}
	}
	if textResponse == "" {
		return "", fmt.Errorf("no output_text found in response")
	}

	return textResponse, nil
}
