package utils

import (
	"database/sql"
	"os"
	"path/filepath"

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

		CREATE TABLE IF NOT EXISTS users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,

			name TEXT NOT NULL,
			role TEXT NOT NULL,

			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		);

		CREATE TABLE IF NOT EXISTS connections (
			id INTEGER PRIMARY KEY AUTOINCREMENT,

			user_id INTEGER NOT NULL,

			provider TEXT NOT NULL,

			data_json TEXT NOT NULL DEFAULT '{}',

			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,

			FOREIGN KEY(user_id) REFERENCES users(id) ON DELETE CASCADE
		);
	`)

	return err
}
