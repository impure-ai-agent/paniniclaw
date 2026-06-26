package utils

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"strings"
	"time"
)

// msgInfo holds minimal message info needed for condensation
type msgInfo struct {
	ID        int64
	Role      string
	Content   string
	CreatedAt time.Time
}

// getAllMessages fetches all messages for a conversation, ordered by ID
func getAllMessages(db *Database, provider, conversationID string) ([]msgInfo, error) {
	rows, err := db.DB.Query(`
		SELECT id, data, created_at
		FROM messages
		WHERE provider = ? AND conversation_id = ?
		ORDER BY id ASC
	`, provider, conversationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var msgs []msgInfo
	for rows.Next() {
		var mi msgInfo
		var rawData []byte
		if err := rows.Scan(&mi.ID, &rawData, &mi.CreatedAt); err != nil {
			return nil, err
		}

		var chatMsg ChatMessage
		if len(rawData) > 0 {
			if err := json.Unmarshal(rawData, &chatMsg); err != nil {
				continue
			}
		}

		mi.Role = chatMsg.Role
		switch v := chatMsg.Content.(type) {
		case string:
			mi.Content = v
		}

		msgs = append(msgs, mi)
	}
	return msgs, rows.Err()
}

// CondenseOneConversation finds one conversation with unsummarized past sessions
// and condenses them. Returns true if any work was done.
func CondenseOneConversation(db *Database, client LLMClient) (bool, error) {
	// Find a conversation that has messages newer than the last summary
	rows, err := db.DB.Query(`
		SELECT DISTINCT m.provider, m.conversation_id
		FROM messages m
		WHERE m.id > COALESCE((
			SELECT MAX(cs.last_message_id)
			FROM conversation_summaries cs
			WHERE cs.provider = m.provider AND cs.conversation_id = m.conversation_id
		), 0)
		LIMIT 1
	`)
	if err != nil {
		return false, fmt.Errorf("find conversation: %w", err)
	}
	defer rows.Close()

	if !rows.Next() {
		return false, nil // nothing to do
	}

	var provider, conversationID string
	if err := rows.Scan(&provider, &conversationID); err != nil {
		return false, fmt.Errorf("scan: %w", err)
	}
	rows.Close()

	// Get all messages
	allMsgs, err := getAllMessages(db, provider, conversationID)
	if err != nil {
		return false, err
	}

	if len(allMsgs) < 3 {
		return false, nil
	}

	// Find sessions (4+ hour gaps between user messages)
	type session struct {
		Messages []msgInfo
	}

	var sessions []session
	currentStart := 0
	for i := 1; i < len(allMsgs); i++ {
		if allMsgs[i].Role == "user" {
			gap := allMsgs[i].CreatedAt.Sub(allMsgs[i-1].CreatedAt)
			if gap > 4*time.Hour {
				sessions = append(sessions, session{
					Messages: allMsgs[currentStart:i],
				})
				currentStart = i
			}
		}
	}
	sessions = append(sessions, session{
		Messages: allMsgs[currentStart:],
	})

	if len(sessions) < 2 {
		return false, nil // only one session, nothing to condense
	}

	// Find the last summarized message ID
	var lastSummarizedID int64 = 0
	err = db.DB.QueryRow(`
		SELECT COALESCE(MAX(last_message_id), 0)
		FROM conversation_summaries
		WHERE provider = ? AND conversation_id = ?
	`, provider, conversationID).Scan(&lastSummarizedID)
	if err != nil {
		lastSummarizedID = 0
	}

	// Condense past sessions (everything before the current/last session)
	pastSessions := sessions[:len(sessions)-1]

	var msgsToSummarize []msgInfo
	for _, sess := range pastSessions {
		for _, msg := range sess.Messages {
			if msg.ID > lastSummarizedID && msg.Content != "" {
				msgsToSummarize = append(msgsToSummarize, msg)
			}
		}
	}

	if len(msgsToSummarize) == 0 {
		return false, nil // already summarized
	}

	// Build summary prompt
	var sb strings.Builder
	sb.WriteString("Summarize the following conversation. Focus on key topics, decisions, and context that would be useful to remember.\n\n")
	for _, msg := range msgsToSummarize {
		label := msg.Role
		if label == "" {
			label = "unknown"
		}
		sb.WriteString(fmt.Sprintf("[%s]: %s\n", label, msg.Content))
	}

	summary, err := client.Chat(context.Background(), sb.String(), "", nil)
	if err != nil {
		return false, fmt.Errorf("summarization failed: %w", err)
	}

	// Store the summary
	firstMsg := msgsToSummarize[0]
	lastMsg := msgsToSummarize[len(msgsToSummarize)-1]

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
		return false, fmt.Errorf("store summary: %w", err)
	}

	log.Printf("[condense] Summarized %d messages for %s/%s (IDs %d-%d)",
		len(msgsToSummarize), provider, conversationID, firstMsg.ID, lastMsg.ID)

	return true, nil
}
