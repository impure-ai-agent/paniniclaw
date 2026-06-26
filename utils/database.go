package utils

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type Database struct {
	DB *sql.DB
}

func CreateDb(path string) (*Database, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, err
	}

	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}

	d := &Database{
		DB: db,
	}

	if err := d.migrate(); err != nil {
		return nil, err
	}

	return d, nil
}

func (d *Database) migrate() error {
	_, err := d.DB.Exec(`
		PRAGMA foreign_keys = ON;

		CREATE TABLE IF NOT EXISTS messages (
			id INTEGER PRIMARY KEY AUTOINCREMENT,

			provider TEXT NOT NULL,
			conversation_id TEXT NOT NULL,

			data TEXT NOT NULL,

			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS conversation_summaries (
			id INTEGER PRIMARY KEY AUTOINCREMENT,

			provider TEXT NOT NULL,
			conversation_id TEXT NOT NULL,

			summary TEXT NOT NULL,

			first_message_id INTEGER NOT NULL,
			last_message_id INTEGER NOT NULL,
			first_message_time DATETIME NOT NULL,
			last_message_time DATETIME NOT NULL,

			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);

		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(provider, conversation_id, created_at);
		CREATE INDEX IF NOT EXISTS idx_summaries_conversation ON conversation_summaries(provider, conversation_id, last_message_time);
	`)

	return err
}

type Message struct {
	Id             int64
	Provider       string
	ConversationId string
	Data           ChatMessage
	CreatedAt      time.Time
}

type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"`                // Cannot be empty for tool calls and some terminal commands have no output
	Name       string      `json:"name,omitempty"`         // Used in tool calls
	ToolCallID string      `json:"tool_call_id,omitempty"` // Used in tool calls
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
}

// ToolCall represents a tool call requested by the model.
type ToolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"` // JSON string
	} `json:"function"`
}

func (d *Database) AddMessage(
	provider string,
	conversationID string,
	data string,
) error {

	_, err := d.DB.Exec(`
		INSERT INTO messages (
			provider,
			conversation_id,
			data
		)
		VALUES (?, ?, ?)
	`,
		provider,
		conversationID,
		data,
	)

	return err
}

func (d *Database) GetRecentMessages(
	provider string,
	conversationID string,
) ([]Message, error) {

	rows, err := d.DB.Query(`
		SELECT
			id,
			provider,
			conversation_id,
			data,
			created_at
		FROM messages
		WHERE
			provider = ?
			AND conversation_id = ?
			AND created_at >= datetime('now', '-24 hours')
		ORDER BY id ASC
	`,
		provider,
		conversationID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var messages []Message

	for rows.Next() {
		var msg Message
		var rawData []byte // Temp container for the database value

		err := rows.Scan(
			&msg.Id,
			&msg.Provider,
			&msg.ConversationId,
			&rawData, // Scan bytes instead of the map
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		// Unmarshal JSON bytes into the map if rawData is not empty
		if len(rawData) > 0 {
			if err := json.Unmarshal(rawData, &msg.Data); err != nil {
				return nil, err
			}
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Find session start (4+ hour gap)
	sessionStart := 0

	for i := 1; i < len(messages); i++ {

		if messages[i].Data.Role == "user" {
			gap := messages[i].CreatedAt.Sub(messages[i-1].CreatedAt)

			if gap > 4*time.Hour {
				sessionStart = i
			}
		}
	}

	// If we have more than 20 messages, use summaries for the condensed portion
	// and keep only the last 20 raw messages
	if len(messages) > 20 && sessionStart < len(messages)-20 {
		// Look up summaries for messages before the last 20
		summaries, err := d.getSummariesBefore(
			provider, conversationID,
			messages[len(messages)-20].Id,
		)
		if err == nil && len(summaries) > 0 {
			// Build a condensed summary message from all summaries
			var summaryText string
			for _, s := range summaries {
				if summaryText != "" {
					summaryText += "\n---\n"
				}
				summaryText += s
			}

			// Create a synthetic system message with the summary
			summaryMsg := Message{
				Id:             0, // synthetic
				Provider:       provider,
				ConversationId: conversationID,
				Data: ChatMessage{
					Role:    "system",
					Content: "[Previous conversation summary]: " + summaryText,
				},
				CreatedAt: messages[len(messages)-20].CreatedAt,
			}

			// Return: summary + last 20 messages
			result := []Message{summaryMsg}
			result = append(result, messages[len(messages)-20:]...)
			return result, nil
		}
	}

	return messages[sessionStart:], nil
}

// getSummariesBefore retrieves all summaries that end before the given message ID.
func (d *Database) getSummariesBefore(provider, conversationID string, beforeMessageID int64) ([]string, error) {
	rows, err := d.DB.Query(`
		SELECT summary
		FROM conversation_summaries
		WHERE
			provider = ?
			AND conversation_id = ?
			AND last_message_id < ?
		ORDER BY last_message_id ASC
	`, provider, conversationID, beforeMessageID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var summaries []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		summaries = append(summaries, s)
	}
	return summaries, rows.Err()
}
