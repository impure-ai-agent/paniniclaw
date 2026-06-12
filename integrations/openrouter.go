package integrations

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"paniniclaw/utils"
	"path/filepath"
	"strconv"
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

type reasoningConfig struct {
	Effort string `json:"effort,omitempty"`
}

// providerConfig allows specifying routing preferences to OpenRouter
type providerConfig struct {
	Order          []string `json:"order,omitempty"`
	AllowFallbacks *bool    `json:"allow_fallbacks,omitempty"`
}

type chatRequest struct {
	Model     string              `json:"model"`
	Messages  []utils.ChatMessage `json:"messages"`
	Reasoning *reasoningConfig    `json:"reasoning,omitempty"`
	MaxTokens int                 `json:"max_tokens,omitempty"`
	Tools     []Tool              `json:"tools,omitempty"`
	Provider  *providerConfig     `json:"provider,omitempty"` // Added for provider configuration
}

type chatResponse struct {
	Choices []struct {
		Message utils.ChatMessage `json:"message"`
	} `json:"choices"`
}

func makeSystemMessage(user utils.User) (utils.ChatMessage, error) {
	soulBytes, err := ensureFileExists("directives/core.md", `You are PaniniClaw, a helpful assistant that also makes paninis.
If you are unsure about something ask for clarification instead of guessing.
You may edit this file with user permission.
You do not need permission to edit files in the notes directory and should edit them with any information that might be useful later.`)
	if err != nil {
		return utils.ChatMessage{}, err
	}

	telegramBytes, err := ensureFileExists("directives/telegram.md", "You are operating in a telegram chat. Do not use markdown as it is not supported.")
	if err != nil {
		return utils.ChatMessage{}, err
	}

	generalBytes, err := ensureFileExists("notes/general.md", `When executing terminal tasks you are an unprivileged user. If you are unable to do something do not go around in circles trying complicated workarounds. Instead you should ask the user for help. Always be extremely careful with commands that can delete data. This includes commands which may overwrite files.
Do not use curl directly as it wastes tokens and takes forever, instead you can run ./clean_curl.py <url> which strips HTML tags.
`)
	if err != nil {
		return utils.ChatMessage{}, err
	}

	userJson, err := user.MakeJson()
	if err != nil {
		return utils.ChatMessage{}, err
	}

	userNotesBytes, err := ensureFileExists(fmt.Sprintf("notes/user%s.md", strconv.Itoa(user.Id)), "")
	if err != nil {
		return utils.ChatMessage{}, err
	}

	return utils.ChatMessage{
		Role:    "system",
		Content: fmt.Sprintf("directives/core.md: %s\n\ndirectives/telegram.md: %s\n\nuser: %s\n\nnotes/general.md: %s\n\nnotes/user%s.md: %s\n", soulBytes, telegramBytes, userJson, generalBytes, strconv.Itoa(user.Id), userNotesBytes),
	}, nil
}

func ensureFileExists(path string, defaultContent string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			// Create the parent directories if they don't exist
			dir := filepath.Dir(path)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, err
			}

			// Write the default content
			contentBytes := []byte(defaultContent)
			if err := os.WriteFile(path, contentBytes, 0644); err != nil {
				return nil, err
			}
			return contentBytes, nil
		}
		// Return any other error encountered (e.g., permission issues)
		return nil, err
	}
	return data, nil
}

func (o *OpenRouter) ChatFromMessages(messages []utils.Message, user utils.User, db *utils.Database, chatId string) (string, error) {
	systemMessage, err := makeSystemMessage(user)
	if err != nil {
		return "", err
	}

	chatMessages := make([]utils.ChatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = msg.Data
	}

	allMessages := append([]utils.ChatMessage{systemMessage}, chatMessages...)
	return o.chatWithTools(allMessages, db, chatId, user)
}

func getToolCalls(m map[string]any, key string) []utils.ToolCall {
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}

	// 1. If it's already the correct type, return it
	if tc, ok := raw.([]utils.ToolCall); ok {
		return tc
	}

	// 2. If it was unmarshaled as generic JSON data, convert it
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var tc []utils.ToolCall
	if err := json.Unmarshal(bytes, &tc); err != nil {
		return nil
	}
	return tc
}

func (o *OpenRouter) chatWithTools(messages []utils.ChatMessage, db *utils.Database, chatId string, user utils.User) (string, error) {
	const maxIterations = 100
	for i := 0; i < maxIterations; i++ {
		allowFallbacks := true

		reqBody := chatRequest{
			Model:    o.model,
			Messages: messages,
			Reasoning: &reasoningConfig{
				Effort: "none",
			},
			MaxTokens: 10_000,
			Tools: []Tool{
				TerminalTool,
				{
					Type: "openrouter:web_search",
				},
			},
			Provider: &providerConfig{
				Order:          []string{"novita"}, // Prioritizes Novita
				AllowFallbacks: &allowFallbacks,    // Allows other providers if Novita is down
			},
		}

		responseMsg, err := o.rawChat(reqBody)
		if err != nil {
			return "", err
		}

		// Append the assistant response message to context
		messages = append(messages, responseMsg)

		if len(responseMsg.ToolCalls) == 0 {
			// No tools called, return the text content
			return responseMsg.Content.(string), nil
		}

		if responseMsg.Content != "" {
			if contentStr, ok := responseMsg.Content.(string); ok {
			sendMessageToPrimaryAccount(contentStr, user)
		}
		}

		msgJson, _ := json.Marshal(map[string]interface{}{
			"role":       "assistant",
			"content":    responseMsg.Content,
			"tool_calls": responseMsg.ToolCalls,
		})

		db.AddMessage(
			"telegram",
			chatId,
			string(msgJson),
		)

		// Process tool calls
		for _, toolCall := range responseMsg.ToolCalls {
			if toolCall.Function.Name == "execute_command" {
				var args ExecuteCommandArgs
				if err := json.Unmarshal([]byte(toolCall.Function.Arguments), &args); err != nil {
					return "", fmt.Errorf("failed to parse tool arguments: %v", err)
				}

				print("Executing command for tool call:", args.Command, ". id:", toolCall.ID, ".")
				output, err := ExecuteCommand(args.Command)
				if err != nil {
					output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
				}
				println("Output:", output)

				message := utils.ChatMessage{
					Role:       "tool",
					Name:       "execute_command",
					ToolCallID: toolCall.ID,
					Content:    output,
				}
				messages = append(messages, message)
				toolJson, _ := json.Marshal(message)

				db.AddMessage(
					"telegram",
					chatId,
					string(toolJson),
				)

			} else {
				message := utils.ChatMessage{
					Role:       "tool",
					Name:       toolCall.Function.Name,
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("Error: Unknown tool %s", toolCall.Function.Name),
				}
				messages = append(messages, message)
				toolJson, _ := json.Marshal(message)

				db.AddMessage(
					"telegram",
					chatId,
					string(toolJson),
				)

				println("Error: Unknown tool %s", toolCall.Function.Name)
			}
		}
	}

	return "", fmt.Errorf("exceeded max tool call iterations limit (%d)", maxIterations)
}

func (o *OpenRouter) rawChat(prompt chatRequest) (utils.ChatMessage, error) {
	body, err := json.Marshal(prompt)
	if err != nil {
		return utils.ChatMessage{}, err
	}

	println("Making request: ", string(body))

	req, err := http.NewRequest(
		"POST",
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return utils.ChatMessage{}, err
	}

	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRouter-Title", "PaniniClaw")
	req.Header.Set("HTTP-Referer", "https://github.com/impure/paniniclaw")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return utils.ChatMessage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(resp.Body)
		return utils.ChatMessage{}, fmt.Errorf("openrouter returned %d: %s", resp.StatusCode, errBody.String())
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return utils.ChatMessage{}, err
	}

	debugResponse, _ := json.MarshalIndent(result, "", "\t")
	println("Got response:", string(debugResponse))

	if len(result.Choices) == 0 {
		return utils.ChatMessage{}, fmt.Errorf("no choices returned in response")
	}

	return result.Choices[0].Message, nil
}
