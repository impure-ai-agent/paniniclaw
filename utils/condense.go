package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// CondenseMemory checks all conversations and summarizes old messages
// that fall outside the "last 20 messages" window.
func CondenseMemory(db *Database, client LLMClient) error {
	// Find all distinct conversations in the database
	rows, err := db.DB.Query(`
		SELECT DISTINCT provider, conversation_id
		FROM messages
		ORDER BY provider, conversation_id
	`)
	if err != nil {
		return fmt.Errorf("find conversations: %w", err)
	}
	defer rows.Close()

	var conversations []struct {
		Provider       string
		ConversationID string
	}

	for rows.Next() {
		var c struct {
			Provider       string
			ConversationID string
		}
		if err := rows.Scan(&c.Provider, &c.ConversationID); err != nil {
			return fmt.Errorf("scan conversation: %w", err)
		}
		conversations = append(conversations, c)
	}
	rows.Close()

	for _, conv := range conversations {
		if err := condenseConversation(db, client, conv.Provider, conv.ConversationID); err != nil {
			log.Printf("[condense] Error condensing %s/%s: %v", conv.Provider, conv.ConversationID, err)
		}
	}

	return nil
}

func condenseConversation(db *Database, client LLMClient, provider, conversationID string) error {
	// Get all message IDs and roles, ordered by time
	rows, err := db.DB.Query(`
		SELECT id, data, created_at
		FROM messages
		WHERE provider = ? AND conversation_id = ?
		ORDER BY id ASC
	`, provider, conversationID)
	if err != nil {
		return err
	}
	defer rows.Close()

	type msgInfo struct {
		ID        int64
		Role      string
		Content   string
		CreatedAt time.Time
	}

	var allMsgs []msgInfo
	for rows.Next() {
		var id int64
		var rawData []byte
		var createdAt time.Time
		if err := rows.Scan(&id, &rawData, &createdAt); err != nil {
			return err
		}

		var chatMsg ChatMessage
		if len(rawData) > 0 {
			if err := json.Unmarshal(rawData, &chatMsg); err != nil {
				continue // skip unparseable messages
			}
		}

		content := ""
		switch v := chatMsg.Content.(type) {
		case string:
			content = v
		case []interface{}:
			parts := make([]string, 0, len(v))
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					if t, ok := m["type"]; ok && t == "text" {
						if txt, ok := m["text"]; ok {
							parts = append(parts, fmt.Sprintf("%v", txt))
						}
					}
				}
			}
			content = strings.Join(parts, " ")
		}

		allMsgs = append(allMsgs, msgInfo{
			ID:        id,
			Role:      chatMsg.Role,
			Content:   content,
			CreatedAt: createdAt,
		})
	}
	rows.Close()

	if len(allMsgs) <= 20 {
		return nil // Not enough messages to condense
	}

	// Keep last 20 messages, condense everything before that
	condenseEnd := len(allMsgs) - 20
	toCondense := allMsgs[:condenseEnd]

	// Check existing summaries to see if parts of this have already been condensed
	existingRows, err := db.DB.Query(`
		SELECT id, first_message_id, last_message_id, summary
		FROM conversation_summaries
		WHERE provider = ? AND conversation_id = ?
		ORDER BY first_message_id ASC
	`, provider, conversationID)
	if err != nil {
		return err
	}

	type existingSummary struct {
		ID             int64
		FirstMessageID int64
		LastMessageID  int64
		Summary        string
	}
	var existingSummaries []existingSummary
	for existingRows.Next() {
		var es existingSummary
		if err := existingRows.Scan(&es.ID, &es.FirstMessageID, &es.LastMessageID, &es.Summary); err != nil {
			existingRows.Close()
			return err
		}
		existingSummaries = append(existingSummaries, es)
	}
	existingRows.Close()

	// Find what's not yet summarized
	var toSummarize []msgInfo
	lastSummarizedID := int64(0)
	for _, es := range existingSummaries {
		if es.LastMessageID > lastSummarizedID {
			lastSummarizedID = es.LastMessageID
		}
	}

	for _, msg := range toCondense {
		if msg.ID > lastSummarizedID {
			toSummarize = append(toSummarize, msg)
		}
	}

	if len(toSummarize) == 0 {
		return nil // Already fully condensed
	}

	// Build a summary prompt from the messages to condense
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation. Focus on key topics, decisions, and context that would be useful to remember for future conversations.\n\n")

	for _, msg := range toSummarize {
		if msg.Content == "" {
			continue
		}
		label := msg.Role
		if label == "" {
			label = "unknown"
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", label, msg.Content))
	}

	prompt := sb.String()

	// Use a cheaper/faster model for summarization
	summary, err := client.Chat(context.Background(), prompt, "deepseek/deepseek-v4-flash", nil)
	if err != nil {
		return fmt.Errorf("summarization failed: %w", err)
	}

	// Store the summary
	firstMsg := toSummarize[0]
	lastMsg := toSummarize[len(toSummarize)-1]

	_, err = db.DB.Exec(`
		INSERT INTO conversation_summaries (
			provider, conversation_id,
			summary,
			first_message_id, last_message_id,
			first_message_time, last_message_time
		)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, provider, conversationID, summary, firstMsg.ID, lastMsg.ID, firstMsg.CreatedAt, lastMsg.CreatedAt)
	if err != nil {
		return fmt.Errorf("store summary: %w", err)
	}

	log.Printf("[condense] Condensed %d messages for %s/%s (messages %d-%d)",
		len(toSummarize), provider, conversationID, firstMsg.ID, lastMsg.ID)

	return nil
}
