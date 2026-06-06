package utils

import (
	"encoding/json"
	"os"
	"path/filepath"
)

type Role string

const (
	RoleOwner Role = "owner"

// RoleFriend Role = "friend"
// RoleAgent  Role = "agent"
)

type Connection struct {
	Provider string         `json:"provider"`
	Data     map[string]any `json:"data"`
}

type User struct {
	Name        string       `json:"name"`
	Role        Role         `json:"role"`
	Connections []Connection `json:"connections"`
}

type UsersFile struct {
	Users []User `json:"users"`
}
type UserStore struct {
	path string
	data UsersFile
}

func CreateUserStore(path string) (*UserStore, *TraceError) {

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return nil, Wrap(err)
	}
	bytes, err := os.ReadFile(path)

	if err == nil {

		var data UsersFile

		if err := json.Unmarshal(
			bytes,
			&data,
		); err != nil {
			return nil, Wrap(err)
		}

		return &UserStore{
			path: path,
			data: data,
		}, nil

	} else {

		data := UsersFile{
			Users: []User{},
		}
		userStore := UserStore{
			path: path,
			data: data,
		}

		if err := userStore.save(&data); err != nil {
			return nil, err
		}

		return &userStore, nil
	}
}

func (s *UserStore) save(
	data *UsersFile,
) *TraceError {

	bytes, err := json.MarshalIndent(
		data,
		"",
		"\t",
	)

	if err != nil {
		return Wrap(err)
	}

	return Wrap(os.WriteFile(
		s.path,
		bytes,
		0644,
	))
}
