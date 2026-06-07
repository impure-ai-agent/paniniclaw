package utils

import (
	"database/sql"
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

				role TEXT NOT NULL,
				content TEXT NOT NULL,

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
	Role           string
	Content        string
	CreatedAt      time.Time
}

func (d *Database) AddMessage(
	provider string,
	conversationID string,
	role string,
	content string,
) error {

	_, err := d.DB.Exec(`
		INSERT INTO messages (
			provider,
			conversation_id,
			role,
			content
		)
		VALUES (?, ?, ?, ?)
	`,
		provider,
		conversationID,
		role,
		content,
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
			role,
			content,
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

		err := rows.Scan(
			&msg.Id,
			&msg.Provider,
			&msg.ConversationId,
			&msg.Role,
			&msg.Content,
			&msg.CreatedAt,
		)
		if err != nil {
			return nil, err
		}

		messages = append(messages, msg)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	sessionStart := 0

	for i := 1; i < len(messages); i++ {

		if messages[i].Role == "user" {
			gap := messages[i].CreatedAt.Sub(messages[i-1].CreatedAt)

			if gap > 4*time.Hour {
				sessionStart = i
			}
		}
	}

	return messages[sessionStart:], nil
}
