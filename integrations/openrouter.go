package integrations

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"paniniclaw/utils"
	"strconv"
)

type OpenRouter struct {
	apiKey string
	model  string
}

func NewOpenRouter(apiKey string) *OpenRouter {
	return &OpenRouter{
		apiKey: apiKey,
		model:  "deepseek/deepseek-v4-flash",
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
	Model     string           `json:"model"`
	Messages  []chatMessage    `json:"messages"`
	Reasoning *reasoningConfig `json:"reasoning,omitempty"`
	MaxTokens int              `json:"max_tokens,omitempty"`
	Tools     []Tool           `json:"tools,omitempty"`
	Provider  *providerConfig  `json:"provider,omitempty"` // Added for provider configuration
}

type chatMessage struct {
	Role       string     `json:"role"`
	Content    string     `json:"content"`                // Cannot be empty for tool calls and some terminal commands have no output
	Name       string     `json:"name,omitempty"`         // Used in tool calls
	ToolCallID string     `json:"tool_call_id,omitempty"` // Used in tool calls
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message chatMessage `json:"message"`
	} `json:"choices"`
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

	generalBytes, err := os.ReadFile("notes/general.md")
	if err != nil {
		return chatMessage{}, err
	}

	userJson, err := user.MakeJson()
	if err != nil {
		return chatMessage{}, err
	}

	// os.O_RDONLY - Open the file as read-only.
	// os.O_CREATE - Create the file if it does not exist.
	userNotesFile, err := os.OpenFile(fmt.Sprintf("notes/user%s.md", strconv.Itoa(user.Id)), os.O_RDONLY|os.O_CREATE, 0644)
	if err != nil {
		return chatMessage{}, err
	}
	defer userNotesFile.Close()
	userNotesBytes, err := io.ReadAll(userNotesFile)
	if err != nil {
		return chatMessage{}, err
	}

	return chatMessage{
		Role:    "system",
		Content: fmt.Sprintf("directives/core.md: %s\n\ndirectives/telegram.md: %s\n\nuser: %s\n\nnotes/general.md: %s\n\nnotes/user%s.md: %s\n", soulBytes, telegramBytes, userJson, generalBytes, strconv.Itoa(user.Id), userNotesBytes),
	}, nil
}

func (o *OpenRouter) ChatFromMessages(messages []utils.Message, user utils.User, db *utils.Database, chatId string) (string, error) {
	systemMessage, err := makeSystemMessage(user)
	if err != nil {
		return "", err
	}

	chatMessages := make([]chatMessage, len(messages))
	for i, msg := range messages {
		chatMessages[i] = chatMessage{
			Role:      msg.Data["role"].(string),
			Content:   msg.Data["content"].(string),
			ToolCalls: getToolCalls(msg.Data, "tool_calls"),
		}
	}

	allMessages := append([]chatMessage{systemMessage}, chatMessages...)
	return o.chatWithTools(allMessages, db, chatId, user)
}

func getToolCalls(m map[string]any, key string) []ToolCall {
	raw, ok := m[key]
	if !ok || raw == nil {
		return nil
	}

	// 1. If it's already the correct type, return it
	if tc, ok := raw.([]ToolCall); ok {
		return tc
	}

	// 2. If it was unmarshaled as generic JSON data, convert it
	bytes, err := json.Marshal(raw)
	if err != nil {
		return nil
	}

	var tc []ToolCall
	if err := json.Unmarshal(bytes, &tc); err != nil {
		return nil
	}
	return tc
}

func (o *OpenRouter) chatWithTools(messages []chatMessage, db *utils.Database, chatId string, user utils.User) (string, error) {
	const maxIterations = 5
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
				/*
					Web search causes 500 errors, not sure why. It might have something to do with openrouter_web_search appearing in the chat
					{
						Type: "openrouter:web_search",
					},
				*/
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
			return responseMsg.Content, nil
		}

		if responseMsg.Content != "" {
			sendMessageToPrimaryAccount(responseMsg.Content, user)
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

				print("Executing command for tool call:", args.Command, ".")
				output, err := ExecuteCommand(args.Command)
				if err != nil {
					output = fmt.Sprintf("Error: %v\nOutput: %s", err, output)
				}
				println("Output:", output)

				messages = append(messages, chatMessage{
					Role:       "tool",
					Name:       "execute_command",
					ToolCallID: toolCall.ID,
					Content:    output,
				})

				toolJson, _ := json.Marshal(map[string]interface{}{
					"role":    "tool",
					"content": output,
				})

				db.AddMessage(
					"telegram",
					chatId,
					string(toolJson),
				)

			} else {
				messages = append(messages, chatMessage{
					Role:       "tool",
					Name:       toolCall.Function.Name,
					ToolCallID: toolCall.ID,
					Content:    fmt.Sprintf("Error: Unknown tool %s", toolCall.Function.Name),
				})
			}
		}
	}

	return "", fmt.Errorf("exceeded max tool call iterations limit (%d)", maxIterations)
}

func (o *OpenRouter) rawChat(prompt chatRequest) (chatMessage, error) {
	body, err := json.Marshal(prompt)
	if err != nil {
		return chatMessage{}, err
	}

	println("Making request: ", string(body))

	req, err := http.NewRequest(
		"POST",
		"https://openrouter.ai/api/v1/chat/completions",
		bytes.NewReader(body),
	)
	if err != nil {
		return chatMessage{}, err
	}

	req.Header.Set("Authorization", "Bearer "+o.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-OpenRouter-Title", "PaniniClaw")
	req.Header.Set("HTTP-Referer", "https://github.com/impure/paniniclaw")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return chatMessage{}, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		var errBody bytes.Buffer
		_, _ = errBody.ReadFrom(resp.Body)
		return chatMessage{}, fmt.Errorf("openrouter returned %d: %s", resp.StatusCode, errBody.String())
	}

	var result chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return chatMessage{}, err
	}

	debugResponse, _ := json.MarshalIndent(result, "", "\t")
	println("Got response:", string(debugResponse))

	if len(result.Choices) == 0 {
		return chatMessage{}, fmt.Errorf("no choices returned in response")
	}

	return result.Choices[0].Message, nil
}
