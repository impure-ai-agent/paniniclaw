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

		CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(provider, conversation_id, created_at);
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

	messages, err := scanMessages(rows)
	if err != nil {
		return nil, err
	}

	sessionStart := 0

	for i := 1; i < len(messages); i++ {

		if messages[i].Data.Role == "user" {
			gap := messages[i].CreatedAt.Sub(messages[i-1].CreatedAt)

			if gap > 4*time.Hour {
				sessionStart = i
			}
		}
	}

	return messages[sessionStart:], nil
}

// GetRecentMessagesSince returns the count of messages newer than a given time.
func (d *Database) GetRecentMessagesSince(
	provider string,
	conversationID string,
	since time.Time,
) (int, error) {

	var count int
	err := d.DB.QueryRow(`
		SELECT COUNT(*)
		FROM messages
		WHERE
			provider = ?
			AND conversation_id = ?
			AND created_at >= ?
	`,
		provider,
		conversationID,
		since,
	).Scan(&count)

	return count, err
}

// GetMessagesSince returns all messages after a given time, without session splitting.
// If since is zero time, returns all messages.
func (d *Database) GetMessagesSince(
	provider string,
	conversationID string,
	since time.Time,
) ([]Message, error) {

	var rows *sql.Rows
	var err error

	if since.IsZero() {
		rows, err = d.DB.Query(`
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
			ORDER BY id ASC
		`,
			provider,
			conversationID,
		)
	} else {
		rows, err = d.DB.Query(`
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
				AND created_at > ?
			ORDER BY id ASC
		`,
			provider,
			conversationID,
			since,
		)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanMessages(rows)
}

// scanMessages is a helper to scan rows into Message slices.
func scanMessages(rows *sql.Rows) ([]Message, error) {
	var messages []Message

	for rows.Next() {
		var msg Message
		var rawData []byte

		err := rows.Scan(
			&msg.Id,
			&msg.Provider,
			&msg.ConversationId,
			&rawData,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

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

	return messages, nil
}
